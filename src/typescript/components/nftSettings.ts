import {HttpGetJson, HttpPostJson} from "../util/network";
import {WalletGetExplorerTxLink, WalletGetSmallestUnit} from "../util/blockchain/wallet";
import {InitNFTTemplate} from "./nftTemplate";

type ChainConfig = {
    network: string; chainId: string; sender: string; contract: string; codeHash: string;
    metadataUri: string; metadataHash: string; name: string; unit: string; maxCost: string; maxTopUp: number;
};
type ChainForm = {config: ChainConfig; include: HTMLInputElement; metadata: HTMLInputElement; name: HTMLInputElement; unit: HTMLInputElement; cost: HTMLInputElement; topUp: HTMLInputElement};

export function InitNFTSettings() {
    const DOM = {
        root: document.getElementById("nftSettings"),
        form: document.getElementById("nftForm") as HTMLFormElement,
        enabled: document.getElementById("nftEnabled") as HTMLInputElement,
        limitEnabled: document.getElementById("nftLimitEnabled") as HTMLInputElement,
        limit: document.getElementById("nftDailyLimit") as HTMLInputElement,
        chains: document.getElementById("nftChains")!,
        counts: document.getElementById("nftCounts")!,
        status: document.getElementById("nftStatus")!,
        queue: document.getElementById("nftQueue")!,
        page: document.getElementById("nftPage")!,
        previous: document.getElementById("nftPrevious") as HTMLButtonElement,
        next: document.getElementById("nftNext") as HTMLButtonElement,
        refresh: document.getElementById("nftRefresh") as HTMLButtonElement,
        save: document.getElementById("nftSave") as HTMLButtonElement,
        csrf: document.getElementById("csrfToken") as HTMLInputElement,
    };
    if (DOM.root?.dataset.operator !== "true") return;
    let forms: ChainForm[] = [];
    let offset = 0;
    let loading = false;
    const templatePicker = InitNFTTemplate(template => {
        for (const form of forms) {
            form.metadata.value = template.metadataUri;
            form.name.value = template.assetName;
            form.unit.value = template.unit;
        }
    });

    function field(parent: HTMLElement, label: string, value: string, readonly = false, numeric = false): HTMLInputElement {
        const wrapper = document.createElement("label");
        const text = document.createElement("span");
        const input = document.createElement("input");
        wrapper.className = "nftField";
        text.textContent = label;
        input.className = "form-control";
        input.value = value;
        input.readOnly = readonly;
        input.type = numeric ? "number" : "text";
        if (numeric) { input.min = "0"; input.step = "1"; }
        wrapper.append(text, input);
        parent.append(wrapper);
        return input;
    }
    async function load() {
        const [status, response] = await HttpGetJson("/settings/content/nft");
        if (status !== 200 || !response) { DOM.status.textContent = "Settings unavailable"; return; }
        DOM.enabled.checked = response.config.enabled;
        DOM.limitEnabled.checked = response.config.dailyLimit > 0;
        DOM.limit.value = String(response.config.dailyLimit || 100);
        templatePicker.update(response.config.template, response.registration);
        DOM.chains.replaceChildren();
        forms = [];
        for (const registered of response.registration.chains || []) {
            const saved = response.config.chains?.find((item: ChainConfig) => item.network === registered.network);
            const config: ChainConfig = {...registered, ...saved, sender: registered.sender, chainId: registered.chainId, contract: registered.contract, codeHash: registered.codeHash};
            const details = document.createElement("details");
            const summary = document.createElement("summary");
            const grid = document.createElement("div");
            const label = document.createElement("label");
            const include = document.createElement("input");
            const readiness = document.createElement("span");
            summary.textContent = config.network;
            grid.className = "nftFields";
            include.type = "checkbox";
            include.className = "form-check-input";
            include.checked = !!saved;
            label.className = "form-check-label nftInclude";
            label.append(include, document.createTextNode(" Enabled"));
            readiness.textContent = response.ready[config.network] ? "Credentials ready" : "Credentials required";
            readiness.className = "nftReadiness";
            readiness.dataset.network = config.network;
            details.append(summary, label, readiness, grid);
            field(grid, "Registered sender", config.sender, true);
            field(grid, "Network ID", config.chainId, true);
            if (config.contract) { field(grid, "Collection", config.contract, true); field(grid, "Reviewed code hash", config.codeHash, true); }
            const metadata = field(grid, "Metadata URI", response.config.template?.metadataUri || config.metadataUri || "", true);
            const name = field(grid, "Asset name", response.config.template?.assetName || config.name || "", true);
            const unit = field(grid, "Unit name", response.config.template?.unit || config.unit || "", true);
            const cost = field(grid, "Maximum transaction cost (" + WalletGetSmallestUnit(config.network) + ")", config.maxCost || "");
            cost.inputMode = "numeric";
            const topUp = field(grid, "Maximum recipient funding (" + WalletGetSmallestUnit(config.network) + ")", String(config.maxTopUp || 0), false, true);
            if (config.contract) { name.parentElement!.hidden = true; unit.parentElement!.hidden = true; topUp.parentElement!.hidden = true; }
            DOM.chains.append(details);
            forms.push({config, include, metadata, name, unit, cost, topUp});
        }
        DOM.enabled.disabled = DOM.save.disabled = DOM.refresh.disabled = false;
        DOM.status.textContent = response.config.enabled ? "Auto-Minting enabled" : "Auto-Minting paused";
        updateControls();
        await queue();
    }
    async function queue() {
        if (loading) return;
        loading = true;
        try {
            const [status, response] = await HttpGetJson("/settings/content/nft/grants?offset=" + offset);
            if (status !== 200 || !response) { DOM.status.textContent = "Queue unavailable"; return; }
            DOM.queue.replaceChildren();
            for (const grant of response.grants) {
                const row = document.createElement("div");
                const recipient = document.createElement("span");
                const stage = document.createElement("span");
                const retry = document.createElement("button");
                const icon = document.createElement("i");
                row.className = "nftQueueRow";
                recipient.textContent = grant.config.network + " · " + grant.recipient;
                stage.textContent = grant.stage.replaceAll("_", " ") + (grant.reason ? ": " + grant.reason.replaceAll("_", " ") : "");
                retry.className = "btn btn-secondary";
                retry.type = "button";
                retry.title = "Retry delivery";
                retry.setAttribute("aria-label", "Retry delivery");
                icon.className = "bi bi-arrow-clockwise";
                retry.append(icon);
                retry.disabled = grant.stage === "delivered";
                retry.addEventListener("click", async () => {
                    retry.disabled = true;
                    const [code] = await HttpPostJson("/settings/content/nft/retry", {id: grant.id}, DOM.csrf.value);
                    DOM.status.textContent = code === 200 ? "Retry queued" : "Retry unavailable";
                    await queue();
                });
                row.append(recipient, stage, retry);
                const hash = grant.transaction?.hash || grant.lastTransactionHash;
                if (hash) {
                    const link = document.createElement("a");
                    link.href = WalletGetExplorerTxLink(hash, grant.config.network);
                    link.textContent = hash;
                    link.target = "_blank";
                    link.rel = "noopener noreferrer";
                    row.append(link);
                }
                DOM.queue.append(row);
            }
            if (!response.grants.length) DOM.queue.textContent = "No deliveries";
            DOM.previous.disabled = offset === 0;
            DOM.next.disabled = response.grants.length < 25;
            DOM.page.textContent = String(offset / 25 + 1);
            const [code, settings] = await HttpGetJson("/settings/content/nft");
            if (code === 200 && settings) {
                DOM.counts.textContent = Object.entries(settings.counts).map(([stage, count]) => stage.replaceAll("_", " ") + ": " + count).join(" · ");
                DOM.chains.querySelectorAll<HTMLElement>(".nftReadiness").forEach(label => { label.textContent = settings.ready[label.dataset.network!] ? "Credentials ready" : "Credentials required"; });
            }
        } finally { loading = false; }
    }
    function updateControls() {
        DOM.limitEnabled.disabled = !DOM.enabled.checked;
        DOM.limit.disabled = !DOM.enabled.checked || !DOM.limitEnabled.checked;
        DOM.chains.querySelectorAll<HTMLInputElement>("input").forEach(input => { input.disabled = !DOM.enabled.checked; });
    }
    DOM.enabled.addEventListener("change", updateControls);
    DOM.limitEnabled.addEventListener("change", updateControls);
    DOM.refresh.addEventListener("click", queue);
    DOM.previous.addEventListener("click", () => { if (!loading) { offset = Math.max(0, offset - 25); void queue(); } });
    DOM.next.addEventListener("click", () => { if (!loading) { offset += 25; void queue(); } });
    DOM.form.addEventListener("submit", async event => {
        event.preventDefault();
        const chains = forms.filter(form => form.include.checked).map(form => ({...form.config, metadataUri: form.metadata.value.trim(), name: form.name.value, unit: form.unit.value, maxCost: form.cost.value.trim(), maxTopUp: Number(form.topUp.value)}));
        DOM.save.disabled = true;
        DOM.status.textContent = "Saving...";
        const [status, response] = await HttpPostJson("/settings/content/nft", {enabled: DOM.enabled.checked, dailyLimit: DOM.limitEnabled.checked ? Number(DOM.limit.value) : 0, chains}, DOM.csrf.value);
        DOM.save.disabled = false;
        DOM.status.textContent = status === 200 ? "Saved" : (response?.status || "Unable to save settings");
        if (status === 200) await load();
    });
    void load();
}
