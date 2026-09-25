import {type CollectibleData, WalletGetCollectibles} from "../util/blockchain/wallet";
import {ApplyIpfsImageLoadPolicy, CIDToSubdomainURL, GetConfiguredIpfsGateway} from "../util/ipfs";
import {HttpPostJson} from "../util/network";
import {IsValidURL} from "../util/security";

export type NFTTemplate = {
    source: {network: string; contract: string; tokenId: string};
    metadataUri: string; metadataHash: string; name: string; description: string;
    image: string; mimeType: string; assetName: string; unit: string;
};
type Operator = {operatorNetwork: string; operatorAddress: string};

export function InitNFTTemplate(onSelected: (template: NFTTemplate) => void) {
    const DOM = {
        cancel: document.getElementById("nftCancelTemplate") as HTMLButtonElement,
        choose: document.getElementById("nftChooseTemplate") as HTMLButtonElement,
        collection: document.getElementById("nftCollection")!,
        csrf: document.getElementById("csrfToken") as HTMLInputElement,
        grid: document.getElementById("nftCollectionGrid")!,
        next: document.getElementById("nftCollectionNext") as HTMLButtonElement,
        page: document.getElementById("nftCollectionPage")!,
        preview: document.getElementById("nftTemplatePreview")!,
        previous: document.getElementById("nftCollectionPrevious") as HTMLButtonElement,
        refresh: document.getElementById("nftCollectionRefresh") as HTMLButtonElement,
        search: document.getElementById("nftCollectionSearch") as HTMLInputElement,
        status: document.getElementById("nftTemplateStatus")!,
        use: document.getElementById("nftUseTemplate") as HTMLButtonElement,
    };
    let collection: CollectibleData[] = [];
    let operator: Operator;
    let candidate: CollectibleData | undefined;
    let offset = 0;
    let busy = false;
    let loaded = false;
    let selected: NFTTemplate | undefined;

    async function loadCollection() {
        if (busy || !operator) return;
        busy = true;
        DOM.refresh.disabled = true;
        DOM.status.textContent = "Loading collection...";
        try {
            await GetConfiguredIpfsGateway();
            collection = (await WalletGetCollectibles(operator.operatorAddress, operator.operatorNetwork)).map(item => ({
                ...item,
                name: typeof item.name === "string" ? item.name : "NFT #" + item.tokenId,
                description: typeof item.description === "string" ? item.description : "",
                imageUrl: typeof item.imageUrl === "string" ? item.imageUrl : "",
                mimeType: typeof item.mimeType === "string" ? item.mimeType : "",
            }));
            loaded = true;
            offset = 0;
            renderCollection();
            DOM.status.textContent = collection.length ? "Select an NFT" : "No NFTs found in this wallet's collection";
        } catch (_) {
            DOM.status.textContent = "Collection unavailable. Try refreshing.";
        } finally {
            busy = false;
            DOM.refresh.disabled = false;
        }
    }
    function media(uri: string, mimeType: string, name: string): HTMLElement {
        const url = typeof uri === "string" && uri.startsWith("ipfs://") ? CIDToSubdomainURL(uri) : uri;
        const fallback = document.createElement("i");
        fallback.className = "bi bi-image nftMediaPlaceholder";
        fallback.setAttribute("aria-label", "Preview unavailable");
        if (typeof url !== "string" || !IsValidURL(url)) return fallback;
        if (typeof mimeType === "string" && mimeType.startsWith("video/")) {
            const video = document.createElement("video");
            video.src = url;
            video.muted = true;
            video.playsInline = true;
            video.preload = "metadata";
            video.setAttribute("aria-label", name);
            video.addEventListener("error", () => video.replaceWith(fallback), {once: true});
            return video;
        }
        const image = document.createElement("img");
        ApplyIpfsImageLoadPolicy(image, url);
        image.src = url;
        image.alt = name;
        image.loading = "lazy";
        image.referrerPolicy = "no-referrer";
        image.addEventListener("error", () => image.replaceWith(fallback), {once: true});
        return image;
    }
    function preview(name: string, description: string, image: string, mimeType: string, tokenId: string) {
        const visual = document.createElement("div");
        const text = document.createElement("div");
        const title = document.createElement("h4");
        const body = document.createElement("p");
        const token = document.createElement("small");
        visual.className = "nftTemplateMedia";
        visual.append(media(image, mimeType, name));
        if (visual.firstChild instanceof HTMLVideoElement) visual.firstChild.controls = true;
        title.textContent = name;
        body.textContent = description;
        token.textContent = "Source token #" + tokenId;
        text.append(title, body, token);
        DOM.preview.replaceChildren(visual, text);
        DOM.preview.hidden = false;
    }
    function renderCollection() {
        const search = DOM.search.value.trim().toLocaleLowerCase();
        const matches = collection.filter(item => String(item.name).toLocaleLowerCase().includes(search) || item.tokenId.includes(search));
        DOM.grid.replaceChildren();
        for (const item of matches.slice(offset, offset + 12)) {
            const button = document.createElement("button");
            const title = document.createElement("span");
            const token = document.createElement("small");
            button.type = "button";
            button.className = "nftChoice";
            const active = candidate ? candidate === item : selected?.source.network === item.blockchain && selected.source.contract === item.contractAddress && selected.source.tokenId === item.tokenId;
            button.setAttribute("aria-pressed", String(!!active));
            title.textContent = item.name;
            token.textContent = "#" + item.tokenId;
            button.append(media(item.imageUrl, item.mimeType, item.name), title, token);
            button.addEventListener("click", () => {
                if (busy) return;
                candidate = item;
                preview(item.name, item.description, item.imageUrl, item.mimeType, item.tokenId);
                DOM.use.hidden = false;
                DOM.use.disabled = false;
                DOM.cancel.hidden = false;
                DOM.cancel.disabled = false;
                DOM.status.textContent = "Selection not applied";
                renderCollection();
            });
            DOM.grid.append(button);
        }
        if (!matches.length) DOM.grid.textContent = "No matching NFTs";
        DOM.previous.disabled = offset === 0;
        DOM.next.disabled = offset + 12 >= matches.length;
        DOM.page.textContent = String(offset / 12 + 1);
    }
    function update(template: NFTTemplate | undefined, registration: Operator) {
        selected = template;
        operator = registration;
        candidate = undefined;
        DOM.choose.disabled = false;
        DOM.search.disabled = DOM.refresh.disabled = false;
        DOM.use.hidden = true;
        DOM.cancel.hidden = true;
        DOM.collection.hidden = true;
        DOM.choose.setAttribute("aria-expanded", "false");
        DOM.preview.hidden = !template;
        if (template) preview(template.name, template.description, template.image, template.mimeType, template.source.tokenId);
        DOM.status.textContent = template ? "Selected for new visitors" : "No NFT selected";
        if (loaded) renderCollection();
    }
    DOM.choose.addEventListener("click", () => {
        if (busy) return;
        DOM.collection.hidden = !DOM.collection.hidden;
        DOM.choose.setAttribute("aria-expanded", String(!DOM.collection.hidden));
        if (!DOM.collection.hidden && !loaded) void loadCollection();
    });
    DOM.cancel.addEventListener("click", () => { if (!busy) update(selected, operator); });
    DOM.refresh.addEventListener("click", loadCollection);
    DOM.search.addEventListener("input", () => { offset = 0; renderCollection(); });
    DOM.previous.addEventListener("click", () => { offset = Math.max(0, offset - 12); renderCollection(); });
    DOM.next.addEventListener("click", () => { offset += 12; renderCollection(); });
    DOM.use.addEventListener("click", async () => {
        if (busy || !candidate) return;
        busy = true;
        DOM.use.disabled = true;
        DOM.status.textContent = "Verifying ownership and pinning content...";
        try {
            const [status, response] = await HttpPostJson("/settings/content/nft/template", {network: candidate.blockchain, contract: candidate.contractAddress, tokenId: candidate.tokenId}, DOM.csrf.value);
            if (status === 200 && response?.template) {
                update(response.template, operator);
                onSelected(response.template);
            } else {
                DOM.status.textContent = response?.status?.replaceAll("_", " ") || "Unable to select this NFT";
            }
        } finally {
            busy = false;
            DOM.use.disabled = false;
        }
    });
    return {update};
}
