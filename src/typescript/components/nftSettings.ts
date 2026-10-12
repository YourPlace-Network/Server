import {HttpGetJson, HttpPostJson} from "../util/network";
import {WalletGetExplorerTxLink, WalletGetNFTNetworks, WalletGetSmallestUnit, WalletValidateNFTField} from "../util/blockchain/wallet";
import {ShowDialogModal, ShowDialogModalHTML} from "./modalDialog";
import {InitNFTTemplate} from "./nftTemplate";

type ChainConfig = {
    network: string; chainId: string; sender: string; contract: string; codeHash: string;
    metadataUri: string; metadataHash: string; name: string; unit: string; maxCost: string; maxTopUp: number;
};
type ChainForm = {config: ChainConfig; include: HTMLInputElement; metadata: HTMLInputElement; name: HTMLInputElement; unit: HTMLInputElement; cost: HTMLInputElement; topUp: HTMLInputElement};
type MintEstimate = {expires: number; loading: boolean; usd?: number};
type Registration = {operatorNetwork: string; operatorAddress: string; chains: ChainConfig[]};
type RegistrationForm = {network: string; label: string; panel: HTMLDivElement; include: HTMLInputElement; sender: HTMLInputElement; chainId?: HTMLInputElement; contract?: HTMLInputElement; codeHash?: HTMLInputElement};

export function InitNFTSettings() {
    const DOM = {
        root: document.getElementById("nftSettings"),
        form: document.getElementById("nftForm") as HTMLFormElement,
        enabled: document.getElementById("nftEnabled") as HTMLInputElement,
        autoMintControls: document.getElementById("nftAutoMintControls")!,
        dailyLimitControls: document.getElementById("nftDailyLimitControls")!,
        limitEnabled: document.getElementById("nftLimitEnabled") as HTMLInputElement,
        limit: document.getElementById("nftDailyLimit") as HTMLInputElement,
        limitLabel: document.getElementById("nftDailyLimitLabel")!,
        chains: document.getElementById("nftChains")!,
        counts: document.getElementById("nftCounts")!,
        status: document.getElementById("nftStatus")!,
        queue: document.getElementById("nftQueue")!,
        queuePanel: document.getElementById("collapseNFTQueue")!,
        page: document.getElementById("nftPage")!,
        previous: document.getElementById("nftPrevious") as HTMLButtonElement,
        next: document.getElementById("nftNext") as HTMLButtonElement,
        refresh: document.getElementById("nftRefresh") as HTMLButtonElement,
        save: document.getElementById("nftSave") as HTMLButtonElement,
        csrf: document.getElementById("csrfToken") as HTMLInputElement,
        registrationForm: document.getElementById("nftRegistrationForm") as HTMLFormElement,
        registrationHelp: document.getElementById("nftRegistrationHelp") as HTMLButtonElement,
        registrationChains: document.getElementById("nftRegistrationChains")!,
        registrationSave: document.getElementById("nftRegistrationSave") as HTMLButtonElement,
        registrationSource: document.getElementById("nftRegistrationSource")!,
        registrationStatus: document.getElementById("nftRegistrationStatus")!,
        operatorNetwork: document.getElementById("nftOperatorNetwork") as HTMLSelectElement,
        operatorAddress: document.getElementById("nftOperatorAddress") as HTMLInputElement,
    };
    if (DOM.root?.dataset.operator !== "true") return;
    let forms: ChainForm[] = [];
    let registrationForms: RegistrationForm[] = [];
    let offset = 0;
    let loading = false;
    let estimateGeneration = 0;
    let operatorDraftNetwork = "";
    const operatorDrafts = new Map<string, string>();
    const mintEstimates = new Map<string, MintEstimate>();
    const templatePicker = InitNFTTemplate(template => {
        for (const form of forms) {
            form.metadata.value = template.metadataUri;
            form.name.value = template.assetName;
            form.unit.value = template.unit;
        }
        mintEstimates.clear();
        estimateGeneration++;
        updateDailyEstimate();
    });

    function accordion(parent: HTMLElement, id: string, title: string): HTMLDivElement {
        const item = document.createElement("div");
        const heading = document.createElement("h2");
        const button = document.createElement("button");
        const text = document.createElement("b");
        const panel = document.createElement("div");
        const body = document.createElement("div");
        item.className = "accordion-item";
        heading.className = "accordion-header";
        heading.id = id + "Heading";
        button.className = "accordion-button button collapsed";
        button.type = "button";
        button.dataset.bsToggle = "collapse";
        button.dataset.bsTarget = "#" + id;
        button.setAttribute("aria-expanded", "false");
        button.setAttribute("aria-controls", id);
        text.textContent = title;
        button.append(text);
        heading.append(button);
        panel.id = id;
        panel.className = "accordion-collapse collapse";
        panel.dataset.bsParent = "#" + parent.id;
        panel.setAttribute("aria-labelledby", heading.id);
        body.className = "accordion-body";
        panel.append(body);
        item.append(heading, panel);
        parent.append(item);
        return body;
    }
    function field(parent: HTMLElement, label: string, value: string, readonly = false, numeric = false, help?: {id: string; text: string}): HTMLInputElement {
        const wrapper = document.createElement(help ? "div" : "label");
        const text = document.createElement("span");
        const input = document.createElement("input");
        wrapper.className = "nftField";
        text.textContent = label;
        input.className = "form-control";
        input.value = value;
        input.readOnly = readonly;
        input.type = numeric ? "number" : "text";
        if (numeric) { input.min = "0"; input.step = "1"; }
        if (help) {
            const heading = document.createElement("div");
            const caption = document.createElement("label");
            heading.className = "nftFieldHeading";
            input.id = help.id;
            caption.htmlFor = input.id;
            caption.append(text);
            heading.append(caption, helpButton(label, help.text));
            wrapper.append(heading, input);
        } else {
            wrapper.append(text, input);
        }
        parent.append(wrapper);
        return input;
    }
    function helpButton(title: string, message: string): HTMLButtonElement {
        const button = document.createElement("button");
        const icon = document.createElement("i");
        button.type = "button";
        button.className = "nftHelpButton";
        button.title = title + " help";
        button.setAttribute("aria-label", title + " help");
        button.setAttribute("aria-haspopup", "dialog");
        icon.className = "bi bi-question-circle";
        icon.setAttribute("aria-hidden", "true");
        button.append(icon);
        button.addEventListener("click", () => {
            document.getElementById("modalDialog")!.addEventListener("hidden.bs.modal", () => button.focus(), {once: true});
            ShowDialogModal(title + ": " + message);
        });
        return button;
    }
    function registrationFields(registration: Registration, source: string) {
        const locked = source === "environment";
        DOM.operatorNetwork.replaceChildren();
        DOM.registrationChains.replaceChildren();
        registrationForms = [];
        operatorDrafts.clear();
        operatorDraftNetwork = registration.operatorNetwork;
        operatorDrafts.set(operatorDraftNetwork, registration.operatorAddress);
        DOM.operatorAddress.value = registration.operatorAddress;
        DOM.operatorAddress.disabled = DOM.operatorNetwork.disabled = DOM.registrationSave.disabled = locked;
        DOM.registrationSource.textContent = locked ? "Gateway environment override" : source === "server_owner" ? "Server owner" : "";
        DOM.registrationSource.hidden = !DOM.registrationSource.textContent;
        for (const chain of WalletGetNFTNetworks()) {
            const option = document.createElement("option");
            const label = document.createElement("label");
            const include = document.createElement("input");
            const body = document.createElement("div");
            const toggle = document.createElement("div");
            const grid = document.createElement("div");
            const config = registration.chains?.find(item => item.network === chain.network);
            option.value = chain.network;
            option.textContent = chain.label;
            DOM.operatorNetwork.append(option);
            body.id = "nftRegisteredPanel-" + chain.network;
            body.setAttribute("role", "group");
            body.setAttribute("aria-label", chain.label + " issuing wallet");
            body.hidden = true;
            include.type = "checkbox";
            include.id = "nftRegistered-" + chain.network;
            include.setAttribute("role", "switch");
            include.className = "form-check-input";
            include.checked = !!config;
            include.disabled = locked;
            label.className = "form-check-label";
            label.htmlFor = include.id;
            label.textContent = "Enabled";
            toggle.className = "form-check form-switch nftInclude";
            toggle.append(include, label, helpButton(chain.label + " enabled", "Register an issuing wallet for " + chain.label + ". Saving registration pauses Auto-Minting and clears signing credentials; provision them again before re-enabling issuance. This switch alone does not start minting."));
            grid.className = "nftFields";
            const sender = field(grid, "Issuing wallet address", config?.sender || "", false, false, {id: "nftSender-" + chain.network, text: "Public address of the " + chain.label + " wallet that mints and sends welcome NFTs. Fund it for fees and required balances. Its signing key must match this address and be provisioned separately through GitHub Secrets and deployment; never enter a private key or recovery phrase here."});
            const chainId = chain.chainIdLabel ? field(grid, chain.chainIdLabel, config?.chainId || "", false, false, {id: "nftGenesis-" + chain.network, text: "The base64 genesis hash identifying the Algorand network. Copy it from your configured node's suggested transaction parameters, not the genesis ID. It must match the network used by the issuing wallet and recipients."}) : undefined;
            const contract = chain.collection ? field(grid, "Collection contract", config?.contract || "", false, false, {id: "nftContract-" + chain.network, text: "Address of the deployed YourPlace NFT collection on " + chain.label + " mainnet, not a wallet address or token ID. Its implementation must be reviewed for compatibility and have no issuer seizure, freeze, forced-burn, or upgrade powers."}) : undefined;
            const codeHash = chain.collection ? field(grid, "Reviewed runtime code hash", config?.codeHash || "", false, false, {id: "nftCodeHash-" + chain.network, text: "Keccak-256 hash of the collection's reviewed deployed runtime bytecode: 0x followed by 64 hexadecimal characters. Obtain the code from your configured node and hash it after security review. This is not a transaction hash; matching it does not replace reviewing the contract."}) : undefined;
            const update = () => {
                grid.querySelectorAll<HTMLInputElement>("input").forEach(input => { input.disabled = locked || !include.checked; input.required = include.checked; });
                validateRegistration();
            };
            include.addEventListener("change", update);
            body.append(toggle, grid);
            DOM.registrationChains.append(body);
            registrationForms.push({network: chain.network, label: chain.label, panel: body, include, sender, chainId, contract, codeHash});
            grid.addEventListener("input", () => validateRegistration());
            update();
        }
        DOM.operatorNetwork.value = registration.operatorNetwork;
        updateRegistrationNetwork();
    }
    async function load() {
        const [status, response] = await HttpGetJson("/settings/content/nft");
        if (status !== 200 || !response) { DOM.status.textContent = "Settings unavailable"; return; }
        DOM.enabled.checked = response.config.enabled;
        DOM.limitEnabled.checked = response.config.dailyLimit > 0;
        DOM.limit.value = String(response.config.dailyLimit || 100);
        registrationFields(response.registration, response.registrationSource);
        templatePicker.update(response.config.template, response.registration);
        DOM.chains.replaceChildren();
        forms = [];
        for (const registered of response.registration.chains || []) {
            const saved = response.config.chains?.find((item: ChainConfig) => item.network === registered.network);
            const config: ChainConfig = {...registered, ...saved, sender: registered.sender, chainId: registered.chainId, contract: registered.contract, codeHash: registered.codeHash};
            const grid = document.createElement("div");
            const label = document.createElement("label");
            const include = document.createElement("input");
            const chain = WalletGetNFTNetworks().find(chain => chain.network === config.network);
            const body = accordion(DOM.chains, "nftChainPanel-" + config.network, chain?.label || config.network);
            const toggle = document.createElement("div");
            const readiness = document.createElement("span");
            grid.className = "nftFields";
            include.type = "checkbox";
            include.id = "nftChain-" + config.network;
            include.setAttribute("role", "switch");
            include.className = "form-check-input";
            include.checked = !!saved;
            label.className = "form-check-label";
            label.htmlFor = include.id;
            label.textContent = "Enabled";
            toggle.className = "form-check form-switch nftInclude";
            toggle.append(include, label);
            readiness.textContent = response.ready[config.network] ? "Credentials ready" : "Credentials required";
            readiness.className = "nftReadiness";
            readiness.dataset.network = config.network;
            body.append(toggle, readiness, grid);
            field(grid, "Registered sender", config.sender, true);
            if (chain?.chainIdLabel) field(grid, chain.chainIdLabel, config.chainId, true);
            if (config.contract) { field(grid, "Collection", config.contract, true); field(grid, "Reviewed code hash", config.codeHash, true); }
            const metadata = field(grid, "Metadata URI", response.config.template?.metadataUri || config.metadataUri || "", true);
            const name = field(grid, "Asset name", response.config.template?.assetName || config.name || "", true);
            const unit = field(grid, "Unit name", response.config.template?.unit || config.unit || "", true);
            const cost = field(grid, "Maximum transaction cost (" + WalletGetSmallestUnit(config.network) + ")", config.maxCost || "");
            cost.inputMode = "numeric";
            const topUp = field(grid, "Maximum recipient funding (" + WalletGetSmallestUnit(config.network) + ")", String(config.maxTopUp || 0), false, true);
            if (config.contract) { name.parentElement!.hidden = true; unit.parentElement!.hidden = true; topUp.parentElement!.hidden = true; }
            forms.push({config, include, metadata, name, unit, cost, topUp});
            include.addEventListener("change", updateDailyEstimate);
        }
        mintEstimates.clear();
        estimateGeneration++;
        DOM.enabled.disabled = DOM.save.disabled = DOM.refresh.disabled = false;
        DOM.status.textContent = response.config.enabled ? "Auto-Minting enabled" : "Auto-Minting paused";
        updateControls();
        if (DOM.queuePanel.classList.contains("show")) await queue();
    }
    async function mintEstimate(network: string, generation: number) {
        const [status, response] = await HttpGetJson("/settings/content/nft/estimate?network=" + encodeURIComponent(network));
        if (generation !== estimateGeneration) return;
        const usd = status === 200 && typeof response?.mintUSD === "number" && Number.isFinite(response.mintUSD) && response.mintUSD > 0 ? response.mintUSD : undefined;
        mintEstimates.set(network, {usd, loading: false, expires: Date.now() + 60000});
        updateDailyEstimate();
    }
    async function queue() {
        if (loading || !DOM.queuePanel.classList.contains("show") || !DOM.queuePanel.getClientRects().length) return;
        loading = true;
        DOM.refresh.disabled = true;
        DOM.queue.setAttribute("aria-busy", "true");
        if (!DOM.queue.hasChildNodes()) DOM.queue.textContent = "Loading...";
        try {
            const [status, response] = await HttpGetJson("/settings/content/nft/grants?offset=" + offset);
            if (status !== 200 || !response) { DOM.queue.textContent = "Queue unavailable. Try refreshing."; return; }
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
            DOM.counts.textContent = Object.entries(response.counts || {}).map(([stage, count]) => stage.replaceAll("_", " ") + ": " + count).join(" · ");
            DOM.chains.querySelectorAll<HTMLElement>(".nftReadiness").forEach(label => { label.textContent = response.ready?.[label.dataset.network!] ? "Credentials ready" : "Credentials required"; });
        } finally {
            loading = false;
            DOM.refresh.disabled = false;
            DOM.queue.setAttribute("aria-busy", "false");
        }
    }
    function updateControls() {
        DOM.autoMintControls.hidden = !DOM.enabled.checked;
        DOM.enabled.setAttribute("aria-expanded", String(DOM.enabled.checked));
        DOM.dailyLimitControls.hidden = !DOM.limitEnabled.checked;
        DOM.limitEnabled.setAttribute("aria-expanded", String(DOM.limitEnabled.checked));
        DOM.limitEnabled.disabled = false;
        DOM.limit.disabled = !DOM.enabled.checked || !DOM.limitEnabled.checked;
        DOM.chains.querySelectorAll<HTMLInputElement>("input").forEach(input => { input.disabled = !DOM.enabled.checked; });
        updateDailyEstimate();
    }
    function updateDailyEstimate() {
        const active = forms.filter(form => form.include.checked);
        const count = DOM.limit.valueAsNumber;
        const explanation = "Estimated minting fees and gas at current prices, including the minter's fee safety margins. Assumes the entire daily limit uses the most expensive enabled chain. Excludes NFT delivery, recipient funding and locked balances. Not a spending cap; actual costs vary. Refreshes once per minute while visible.";
        DOM.limitLabel.title = explanation;
        DOM.limitLabel.setAttribute("aria-description", explanation);
        DOM.limitLabel.textContent = "Mints Per Day (USD estimate unavailable)";
        if (!DOM.enabled.checked || !DOM.limitEnabled.checked || !DOM.dailyLimitControls.getClientRects().length || !DOM.limit.validity.valid || !Number.isSafeInteger(count) || count < 1 || !active.length) return;
        for (const form of active) {
            const network = form.config.network;
            const estimate = mintEstimates.get(network);
            if (!estimate || estimate.expires <= Date.now()) {
                mintEstimates.set(network, {loading: true, expires: Infinity});
                void mintEstimate(network, estimateGeneration);
            }
        }
        const estimates = active.map(form => mintEstimates.get(form.config.network)!);
        if (estimates.some(estimate => estimate.loading)) {
            DOM.limitLabel.textContent = "Mints Per Day (estimating USD...)";
            return;
        }
        if (estimates.some(estimate => estimate.usd === undefined)) return;
        const daily = count * Math.max(...estimates.map(estimate => estimate.usd!));
        if (!Number.isFinite(daily) || daily > Number.MAX_SAFE_INTEGER) return;
        const dollars = daily < 0.01 ? "<$0.01" : new Intl.NumberFormat("en-US", {style: "currency", currency: "USD"}).format(daily);
        DOM.limitLabel.textContent = "Mints Per Day (~" + dollars + " USD)";
    }
    function updateRegistrationNetwork() {
        operatorDrafts.set(operatorDraftNetwork, DOM.operatorAddress.value);
        operatorDraftNetwork = DOM.operatorNetwork.value;
        DOM.operatorAddress.value = operatorDrafts.get(operatorDraftNetwork) || "";
        for (const form of registrationForms) form.panel.hidden = form.network !== DOM.operatorNetwork.value;
        validateRegistration();
    }
    function validateField(input: HTMLInputElement, network: string, kind: "address" | "codeHash" | "genesis", submitted: boolean) {
        const value = input.value.trim();
        const valid = WalletValidateNFTField(network, kind, value);
        const message = kind === "address" ? "Enter a nonzero wallet or contract address with a valid checksum for the selected network." : kind === "codeHash" ? "Enter a nonzero 32-byte hash: 0x followed by 64 hexadecimal characters." : "Enter a nonzero 32-byte genesis hash in canonical padded base64.";
        const invalid = !input.disabled && !valid && (value.length > 0 || submitted);
        input.setCustomValidity(input.disabled || valid ? "" : message);
        input.classList.toggle("is-valid", !input.disabled && value.length > 0 && valid);
        input.classList.toggle("is-invalid", invalid);
        input.setAttribute("aria-invalid", String(invalid));
        input.title = value.length > 0 && valid ? "Valid format. Network and contract checks are performed when saving." : invalid ? message : "";
    }
    function validateRegistration(submitted = false) {
        validateField(DOM.operatorAddress, DOM.operatorNetwork.value, "address", submitted);
        for (const form of registrationForms) {
            validateField(form.sender, form.network, "address", submitted);
            if (form.contract) validateField(form.contract, form.network, "address", submitted);
            if (form.codeHash) validateField(form.codeHash, form.network, "codeHash", submitted);
            if (form.chainId) validateField(form.chainId, form.network, "genesis", submitted);
        }
    }
    DOM.operatorNetwork.previousElementSibling!.append(helpButton("Operator network", "Network used to sign in as the NFT operator. This also selects the issuing-wallet settings shown below; other networks keep their values. Saving changes the operator's login network. Base and Ethereum are separate login identities."));
    DOM.operatorAddress.previousElementSibling!.append(helpButton("Operator wallet address", "One operator network/address pair is saved for this server. Each network keeps a separate draft while this form is open. Saving selects the displayed pair; sign in with it after changing the operator. Base and Ethereum may use the same address, but Algorand requires its own address. Never enter a private key or recovery phrase."));
    DOM.enabled.addEventListener("change", updateControls);
    DOM.operatorNetwork.addEventListener("change", updateRegistrationNetwork);
    DOM.operatorAddress.addEventListener("input", () => validateRegistration());
    DOM.limitEnabled.addEventListener("change", updateControls);
    DOM.limit.addEventListener("input", updateDailyEstimate);
    DOM.root.addEventListener("shown.bs.collapse", updateDailyEstimate);
    window.setInterval(updateDailyEstimate, 60000);
    DOM.refresh.addEventListener("click", queue);
    DOM.queuePanel.addEventListener("shown.bs.collapse", event => { if (event.target === DOM.queuePanel) void queue(); });
    DOM.previous.addEventListener("click", () => { if (!loading) { offset = Math.max(0, offset - 25); void queue(); } });
    DOM.next.addEventListener("click", () => { if (!loading) { offset += 25; void queue(); } });
    DOM.registrationHelp.addEventListener("click", () => {
        window.bootstrap.Tooltip.getInstance(DOM.registrationHelp)?.hide();
        document.getElementById("modalDialog")!.addEventListener("hidden.bs.modal", () => DOM.registrationHelp.focus(), {once: true});
        ShowDialogModalHTML(
            "<div class='modalBodyLeft'><b>Operator and Issuing Wallets</b>" +
            "<p>The operator manages welcome NFT settings and selects the source NFT. Issuing wallets mint and send new copies, paying the fees.</p>" +
            "<ul><li><b>Operator:</b> wallet network and public address.</li>" +
            "<li><b>Each enabled chain:</b> a funded issuing wallet's public address.</li>" +
            "<li><b>Base / Ethereum:</b> collection contract address and the runtime code hash of its security-reviewed implementation.</li>" +
            "<li><b>Algorand:</b> network genesis hash (base64).</li></ul>" +
            "<p>Provision signing keys separately through GitHub Secrets and deployment, never in these fields.</p></div>"
        );
    });
    DOM.registrationForm.noValidate = true;
    DOM.registrationForm.addEventListener("submit", async event => {
        event.preventDefault();
        validateRegistration(true);
        if (!DOM.operatorNetwork.reportValidity() || !DOM.operatorAddress.reportValidity()) return;
        for (const form of registrationForms) {
            for (const input of [form.sender, form.chainId, form.contract, form.codeHash]) {
                if (!input || input.checkValidity()) continue;
                if (form.panel.hidden) {
                    DOM.registrationStatus.textContent = "Complete the " + form.label + " issuing-wallet fields before saving. Select that network to edit them.";
                } else {
                    input.reportValidity();
                }
                return;
            }
        }
        const registration = {
            operatorNetwork: DOM.operatorNetwork.value,
            operatorAddress: DOM.operatorAddress.value.trim(),
            chains: registrationForms.filter(form => form.include.checked).map(form => ({network: form.network, sender: form.sender.value.trim(), chainId: form.chainId?.value.trim(), contract: form.contract?.value.trim() || "", codeHash: form.codeHash?.value.trim() || ""})),
        };
        DOM.registrationSave.disabled = true;
        DOM.registrationStatus.textContent = "Saving...";
        try {
            const [status, response] = await HttpPostJson("/settings/content/nft/registration", registration, DOM.csrf.value);
            if (status !== 200) {
                DOM.registrationStatus.textContent = response?.status?.replaceAll("_", " ") || "Unable to save operator and wallets";
                return;
            }
            if (response.operatorChanged) { window.location.reload(); return; }
            await load();
            DOM.registrationStatus.textContent = "Saved. Auto-Minting paused; signing credentials require provisioning.";
            DOM.status.textContent = "Operator and wallets saved. Auto-Minting paused.";
        } finally { DOM.registrationSave.disabled = false; }
    });
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
