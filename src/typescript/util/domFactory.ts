import "../../scss/components/collectibleCard.scss";
import type {CollectibleData} from "./blockchain/wallet";
import {IsValidURL, XSSSanitizeUrl, XSSSanitizeValue} from "./security";
import {CIDToSubdomainURL} from "./ipfs";
import {getFileIcon} from "./files";
import {ShowModalMediaViewer} from "../components/modalMediaViewer";

const blockchainIcons: Record<string, string> = {
    "algorand": "/static/image/algorand.svg",
    "avalanche": "/static/image/avalanche.svg",
    "base": "/static/image/base-square.svg",
    "cardano": "/static/image/cardano.svg",
    "ethereum": "/static/image/ethereum.svg",
    "optimism": "/static/image/optimism.svg",
    "solana": "/static/image/solana.svg",
};
const blockchainUrls: Record<string, string> = {
    "algorand": "https://algorand.com",
    "avalanche": "https://avax.network",
    "base": "https://base.org",
    "cardano": "https://cardano.org",
    "ethereum": "https://ethereum.org",
    "optimism": "https://optimism.io",
    "solana": "https://solana.com",
};
interface TagPattern {
    regex: RegExp;
    createLink: (match: string) => HTMLAnchorElement | null;
}
const tagPatterns: TagPattern[] = [
    {
        regex: /(^|\s)@(\S+?)(?=\s|$)/g,
        createLink: (username: string) => {
            if (IsValidURL(username)) return null;
            const link = document.createElement("a");
            link.href = "/p/" + encodeURIComponent(username);
            link.textContent = "@" + username;
            link.classList.add("mention-link");
            return link;
        }
    },
    {
        regex: /(^|\s)(https:\/\/[^\s"<>]+)/g,
        createLink: (url: string) => {
            const sanitizedUrl = XSSSanitizeUrl(url);
            if (sanitizedUrl === "#") return null;
            const link = document.createElement("a");
            link.href = sanitizedUrl;
            link.textContent = url.replace(/^https:\/\/(www\.)?/, "").replace(/[?#].*$/, "");
            link.target = "_blank";
            link.rel = "noopener noreferrer";
            return link;
        }
    },
];

export function getBlockchainIconPath(blockchain: string): string | null {
    const key = blockchain.toLowerCase();
    return blockchainIcons[key] || null;
}
export function getBlockchainUrl(blockchain: string): string | null {
    const key = blockchain.toLowerCase();
    return blockchainUrls[key] || null;
}
export function processTextWithTags(element: HTMLElement): void {
    const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT, null);
    const textNodes: Text[] = [];
    let node: Text | null;
    while ((node = walker.nextNode() as Text | null)) {
        textNodes.push(node);
    }
    for (const textNode of textNodes) {
        const text = textNode.textContent || "";
        const fragment = document.createDocumentFragment();
        let lastIndex = 0;
        let hasMatches = false;
        const allMatches: { index: number; length: number; beforeSpace: string; tag: string; pattern: TagPattern }[] = [];
        for (const pattern of tagPatterns) {
            pattern.regex.lastIndex = 0;
            let match;
            while ((match = pattern.regex.exec(text)) !== null) {
                allMatches.push({
                    index: match.index,
                    length: match[0].length,
                    beforeSpace: match[1],
                    tag: match[2],
                    pattern: pattern
                });
            }
        }
        allMatches.sort((a, b) => a.index - b.index);
        for (const match of allMatches) {
            if (match.index < lastIndex) continue;
            if (match.index > lastIndex) {
                fragment.appendChild(document.createTextNode(text.slice(lastIndex, match.index)));
            }
            if (match.beforeSpace) {
                fragment.appendChild(document.createTextNode(match.beforeSpace));
            }
            const link = match.pattern.createLink(match.tag);
            if (link) {
                hasMatches = true;
                fragment.appendChild(link);
            } else {
                fragment.appendChild(document.createTextNode(match.beforeSpace + match.tag));
            }
            lastIndex = match.index + match.length;
        }
        if (hasMatches) {
            if (lastIndex < text.length) {
                fragment.appendChild(document.createTextNode(text.slice(lastIndex)));
            }
            textNode.parentNode?.replaceChild(fragment, textNode);
        }
    }
}
export async function CreateAttachmentPreview(file: File): Promise<HTMLDivElement> {
    const previewDiv = document.createElement("div") as HTMLDivElement;
    const removeButton = document.createElement("button") as HTMLButtonElement;
    const removeIcon = document.createElement("i") as HTMLElement;
    const fileNameText = document.createElement("span") as HTMLSpanElement;
    const mimeType = file.type;
    previewDiv.setAttribute("id", XSSSanitizeValue(file.name));
    removeButton.classList.add("removeButton");
    removeIcon.classList.add("bi", "bi-x-lg", "removeIcon");
    previewDiv.classList.add("attachmentUploadGridItem");
    fileNameText.classList.add("fileNameSpan");
    fileNameText.textContent = file.name;
    removeButton.appendChild(removeIcon);
    previewDiv.appendChild(removeButton);
    if (mimeType.startsWith("image/")) {
        const imgPreview = document.createElement("img") as HTMLImageElement;
        imgPreview.classList.add("attachmentImagePreview");
        const objectUrl = URL.createObjectURL(file);
        imgPreview.src = objectUrl;
        imgPreview.onload = () => URL.revokeObjectURL(objectUrl);
        previewDiv.appendChild(imgPreview);
    } else {
        const icon = document.createElement("i") as HTMLElement;
        const iconType = getFileIcon(mimeType);
        icon.classList.add("icon", "attachmentIcon", iconType);
        previewDiv.appendChild(icon);
    }
    previewDiv.appendChild(fileNameText);
    return previewDiv;
}
export function CreateCollectibleCard(data: CollectibleData, isOwner: boolean): HTMLDivElement {
    let card = document.createElement("div");
    card.classList.add("collectibleCard");
    let blockchainInput = document.createElement("input");
    blockchainInput.type = "hidden";
    blockchainInput.classList.add("collectibleBlockchain");
    blockchainInput.value = XSSSanitizeValue(data.blockchain);
    card.appendChild(blockchainInput);
    let contractInput = document.createElement("input");
    contractInput.type = "hidden";
    contractInput.classList.add("collectibleContractAddress");
    contractInput.value = XSSSanitizeValue(data.contractAddress);
    card.appendChild(contractInput);
    let tokenIdInput = document.createElement("input");
    tokenIdInput.type = "hidden";
    tokenIdInput.classList.add("collectibleTokenId");
    tokenIdInput.value = XSSSanitizeValue(data.tokenId);
    card.appendChild(tokenIdInput);
    let mediaDiv = document.createElement("div");
    mediaDiv.classList.add("collectibleCardMedia");
    let mediaUrl = data.imageUrl;
    if (mediaUrl) {
        const convertedUrl = CIDToSubdomainURL(mediaUrl);
        if (convertedUrl) {
            mediaUrl = convertedUrl;
        }
    }
    if (data.mimeType && data.mimeType.startsWith("video/")) {
        let video = document.createElement("video");
        video.classList.add("collectibleMediaElement");
        video.src = XSSSanitizeUrl(mediaUrl);
        video.muted = true;
        video.loop = true;
        video.playsInline = true;
        video.preload = "metadata";
        mediaDiv.addEventListener("mouseenter", () => { video.play().catch(() => {}); });
        mediaDiv.addEventListener("mouseleave", () => { video.pause(); video.currentTime = 0; });
        mediaDiv.appendChild(video);
    } else {
        let img = document.createElement("img");
        img.classList.add("collectibleMediaElement");
        img.src = XSSSanitizeUrl(mediaUrl);
        img.alt = XSSSanitizeValue(data.name);
        img.loading = "lazy";
        mediaDiv.appendChild(img);
    }
    mediaDiv.addEventListener("click", () => {
        let wrapper = document.createElement("div");
        let mediaClone = mediaDiv.querySelector(".collectibleMediaElement");
        if (mediaClone) wrapper.appendChild(mediaClone.cloneNode(true));
        ShowModalMediaViewer(wrapper);
    });
    card.appendChild(mediaDiv);
    let infoDiv = document.createElement("div");
    infoDiv.classList.add("collectibleCardInfo");
    let nameDiv = document.createElement("div");
    nameDiv.classList.add("collectibleCardName");
    nameDiv.textContent = XSSSanitizeValue(data.name);
    infoDiv.appendChild(nameDiv);
    if (data.description) {
        let descDiv = document.createElement("div");
        descDiv.classList.add("collectibleCardDescription");
        descDiv.textContent = XSSSanitizeValue(data.description);
        infoDiv.appendChild(descDiv);
    }
    card.appendChild(infoDiv);
    if (isOwner) {
        let burnBtn = document.createElement("button");
        burnBtn.classList.add("collectibleBurnBtn");
        burnBtn.innerHTML = '<i class="bi bi-trash"></i>';
        card.appendChild(burnBtn);
        let sendBtn = document.createElement("button");
        sendBtn.classList.add("collectibleSendBtn");
        sendBtn.innerHTML = '<i class="bi bi-send"></i>';
        card.appendChild(sendBtn);
    }
    return card;
}
