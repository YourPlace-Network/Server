import "../../scss/components/imageLoader.scss";
import "../../scss/components/postCard.scss";
import { CreateCommentThread } from "./commentThread";
import { CreatePostControlsBar } from "./postControls";
import { OEmbedCard } from "./oEmbedCard";
import { ProcessPostContentForPreviews } from "./postPreviewCard";
import { ShowAddCommentUI } from "./addComment";
import {ShowAvatarMediaViewer, ShowModalMediaViewer} from "./modalMediaViewer";
import { XcomOEmbedCard } from "./xcomOEmbedCard";
import { IsValidAddress, WalletGetExplorerTxLink, WalletGetYourPlaceAddressLink, WalletGetAvatar } from "../util/blockchain/wallet";
import { IsValidIpfsCid, IsValidURL, XSSSanitizeTinyMCEHtml, XSSSanitizeUrl, XSSSanitizeValue } from "../util/security";
import { ApplyIpfsImageLoadPolicy, CIDToSubdomainURL, getIpfsAvatarUrl, ProbeIpfsMediaType, ResolveIpfsContentUrl } from "../util/ipfs";
import { getFileIcon, formatFileSize } from "../util/files";
import { LogError } from "../util/log";
import { getBlockchainIconPath, getBlockchainUrl, processTextWithTags } from "../util/domFactory";

async function canRenderInlineImage(url: string): Promise<boolean> {
    return new Promise(resolve => {
        const sanitizedUrl = XSSSanitizeUrl(url);
        if (sanitizedUrl === "#") {
            resolve(false);
            return;
        }
        const image = new Image();
        let finished = false;
        const finish = (result: boolean) => {
            if (finished) {
                return;
            }
            finished = true;
            window.clearTimeout(timeoutId);
            image.onload = null;
            image.onerror = null;
            resolve(result);
        };
        const timeoutId = window.setTimeout(() => finish(false), 8000);
        image.decoding = "async";
        image.onload = () => finish(true);
        image.onerror = () => finish(false);
        image.src = sanitizedUrl;
    });
}
async function createImageEmbed(url: string): Promise<HTMLElement | null> {
    const imageRegex = /^https:\/\/.*\.(jpg|jpeg|gif|webp|png|svg)$/i;
    if (!imageRegex.test(url)) {
        return null;
    }
    const img = document.createElement("img") as HTMLImageElement;
    img.classList.add("postCardEmbeddedImage");
    img.crossOrigin = "anonymous";
    img.referrerPolicy = "no-referrer";
    img.src = XSSSanitizeUrl(url);
    return img;
}
function createIpfsImageEmbed(url: string, cid: string): HTMLImageElement | null {
    const sanitizedUrl = XSSSanitizeUrl(url);
    if (sanitizedUrl === "#") {
        return null;
    }
    const img = document.createElement("img") as HTMLImageElement;
    img.classList.add("postCardEmbeddedImage");
    img.crossOrigin = "anonymous";
    img.referrerPolicy = "no-referrer";
    ApplyIpfsImageLoadPolicy(img, sanitizedUrl);
    img.src = sanitizedUrl;
    img.alt = `ipfs://${cid}`;
    return img;
}
function createIpfsVideoEmbed(url: string): HTMLVideoElement | null {
    const sanitizedUrl = XSSSanitizeUrl(url);
    if (sanitizedUrl === "#") {
        return null;
    }
    const video = document.createElement("video") as HTMLVideoElement;
    video.classList.add("postCardInlineVideo");
    video.controls = true;
    video.playsInline = true;
    video.preload = "metadata";
    video.addEventListener("loadedmetadata", () => {
        if (video.videoWidth > 0) {
            video.style.maxWidth = `${video.videoWidth}px`;
        }
    });
    video.src = sanitizedUrl;
    return video;
}
function createYoutubeEmbed(url: string): HTMLIFrameElement | null {
    const youtubeRegex = /^https:\/\/((?:www\.)?youtube\.com\/watch\?v=|youtu\.be\/)([a-zA-Z0-9_-]{11})(?:[?&].*)?$/;
    const match = url.match(youtubeRegex);
    if (!match) {
        return null;
    }
    const videoId = match[2];
    const iframe = document.createElement("iframe") as HTMLIFrameElement;
    iframe.classList.add("postCardEmbeddedIframe");
    let embedURL = `https://www.youtube-nocookie.com/embed/${videoId}`;
    iframe.src = XSSSanitizeUrl(embedURL);
    iframe.allow = "encrypted-media; picture-in-picture";
    iframe.allowFullscreen = true;
    iframe.setAttribute("loading", "lazy");
    iframe.setAttribute("credentialless", "");
    return iframe;
}
async function expandView(event: MouseEvent | PointerEvent) {
    const clickedDiv = event.currentTarget as HTMLDivElement;
    clickedDiv.classList.add("initiator");
    const specificPostAttachmentDiv = clickedDiv.closest(".postCardAttachmentDiv") as HTMLDivElement;
    const expandables = specificPostAttachmentDiv.querySelectorAll(".expandable") as NodeListOf<HTMLDivElement>;
    const clonedExpandablesArray = Array.from(expandables).map(expandable => expandable.cloneNode(true) as HTMLDivElement);
    clickedDiv.classList.remove("initiator");
    if (clonedExpandablesArray.length === 1) {
        ShowModalMediaViewer(clonedExpandablesArray[0]);
    }
    if (clonedExpandablesArray.length > 1) {
        const carousel = await CreateCarousel(clonedExpandablesArray);
        carousel.querySelectorAll(".active").forEach(element => {
            element.classList.remove("active");
        })
        const index = clonedExpandablesArray.findIndex(element => element.classList.contains("initiator"));
        const clonedClickedElement = carousel.querySelector(".initiator") as HTMLElement;
        const firstDisplayedSlide = clonedClickedElement.closest(".carousel-item") as HTMLDivElement;
        firstDisplayedSlide.classList.add("active");
        const firstIndicator = carousel.querySelector(`[data-bs-slide-to="${index}"]`) as HTMLLIElement;
        firstIndicator.classList.add("active");
        const prevControl = carousel.querySelector(".carousel-control-prev") as HTMLAnchorElement;
        const nextControl = carousel.querySelector(".carousel-control-next") as HTMLAnchorElement;
        if (prevControl) prevControl.id = "mediaViewerCarouselPrev";
        if (nextControl) nextControl.id = "mediaViewerCarouselNext";
        ShowModalMediaViewer(carousel);
    }
}
async function grid2Attachments(attachments: HTMLElement[]): Promise<HTMLDivElement> {
    const container = document.createElement("div") as HTMLDivElement;
    const row = document.createElement("div") as HTMLDivElement;
    const column1 = document.createElement("div") as HTMLDivElement;
    const column2 = document.createElement("div") as HTMLDivElement;
    container.classList.add("container", "attachmentGrid");
    row.classList.add("row", "gx-2");
    column1.classList.add("col", "attachmentGridItem");
    column1.style.borderTopLeftRadius = "1em";
    column1.style.borderBottomLeftRadius = "1em";
    column2.classList.add("col", "attachmentGridItem");
    column2.style.borderTopRightRadius = "1em";
    column2.style.borderBottomRightRadius = "1em";
    column1.appendChild(attachments[0]);
    column2.appendChild(attachments[1]);
    row.appendChild(column1);
    row.appendChild(column2);
    container.appendChild(row);
    return container;
}
async function grid3Attachments(attachments: HTMLElement[]): Promise<HTMLDivElement> {
    const container = document.createElement("div") as HTMLDivElement;
    const mainRow = document.createElement("div") as HTMLDivElement;
    const column1 = document.createElement("div") as HTMLDivElement;
    const column2 = document.createElement("div") as HTMLDivElement;
    const subRow1 = document.createElement("div") as HTMLDivElement;
    const subRow2 = document.createElement("div") as HTMLDivElement;
    container.classList.add("container", "attachmentGrid");
    column1.classList.add("col", "attachmentGridItem");
    column1.style.borderTopLeftRadius = "1em";
    column1.style.borderBottomLeftRadius = "1em";
    column2.classList.add("col");
    mainRow.classList.add("row", "gx-2");
    subRow1.classList.add("row", "attachmentGridItem");
    subRow1.style.borderTopRightRadius = "1em";
    subRow1.style.margin = "0";
    subRow2.classList.add("row", "attachmentGridItem");
    subRow2.style.borderBottomRightRadius = "1em";
    subRow2.style.paddingTop = "0.5rem";
    subRow2.style.margin = "0";
    column1.appendChild(attachments[0]);
    subRow1.appendChild(attachments[1]);
    subRow2.appendChild(attachments[2]);
    column2.appendChild(subRow1);
    column2.appendChild(subRow2);
    mainRow.appendChild(column1);
    mainRow.appendChild(column2);
    container.appendChild(mainRow);
    return container;
}
async function grid4Attachments(attachments: HTMLElement[]): Promise<HTMLDivElement> {
    const container = document.createElement("div") as HTMLDivElement;
    const row1 = document.createElement("div") as HTMLDivElement;
    const row2 = document.createElement("div") as HTMLDivElement;
    const column1 = document.createElement("div") as HTMLDivElement;
    const column2 = document.createElement("div") as HTMLDivElement;
    const column3 = document.createElement("div") as HTMLDivElement;
    const column4 = document.createElement("div") as HTMLDivElement;
    container.classList.add("container", "attachmentGrid");
    row1.classList.add("row", "gx-2");
    row2.classList.add("row", "gx-2");
    row2.style.paddingTop = "0.5rem";
    column1.classList.add("col", "attachmentGridItem");
    column1.style.borderTopLeftRadius = "1em";
    column2.classList.add("col", "attachmentGridItem");
    column2.style.borderTopRightRadius = "1em";
    column3.classList.add("col", "attachmentGridItem");
    column3.style.borderBottomLeftRadius = "1em";
    column4.classList.add("col", "attachmentGridItem");
    column4.style.borderBottomRightRadius = "1em";
    column1.appendChild(attachments[0]);
    column2.appendChild(attachments[1]);
    column3.appendChild(attachments[2]);
    column4.appendChild(attachments[3]);
    row1.appendChild(column1);
    row1.appendChild(column2);
    row2.appendChild(column3);
    row2.appendChild(column4);
    container.appendChild(row1);
    container.appendChild(row2);
    return container;
}

function resolveAvatarMediaViewerUrl(avatarUrl: string | null | undefined): string | null {
    if (!avatarUrl || avatarUrl.trim() === "" || avatarUrl === "/static/image/avatar.svg") {
        return null;
    }
    let resolvedAvatarUrl = avatarUrl.trim();
    if (resolvedAvatarUrl.startsWith("ipfs://")) {
        resolvedAvatarUrl = CIDToSubdomainURL(resolvedAvatarUrl) || "";
    }
    if (!resolvedAvatarUrl || !IsValidURL(resolvedAvatarUrl)) {
        return null;
    }
    const sanitizedAvatarUrl = XSSSanitizeUrl(resolvedAvatarUrl);
    if (sanitizedAvatarUrl === "#" || sanitizedAvatarUrl === "/static/image/avatar.svg") {
        return null;
    }
    return sanitizedAvatarUrl;
}

function isAvatarVideoMediaViewerUrl(avatarUrl: string): boolean {
    const normalizedAvatarUrl = avatarUrl.split(/[?#]/)[0].toLowerCase();
    return normalizedAvatarUrl.endsWith(".mov") ||
        normalizedAvatarUrl.endsWith(".mp4") ||
        normalizedAvatarUrl.endsWith(".ogg") ||
        normalizedAvatarUrl.endsWith(".webm");
}

function setAvatarViewerState(avatarElement: HTMLElement, avatarMediaViewerUrl: string | null) {
    if (avatarMediaViewerUrl) {
        avatarElement.dataset.mediaViewerSrc = avatarMediaViewerUrl;
        avatarElement.classList.add("clickable");
        return;
    }
    delete avatarElement.dataset.mediaViewerSrc;
    avatarElement.classList.remove("clickable");
}

function setAvatarImageSource(avatarImg: HTMLImageElement, avatarElement: HTMLElement, avatarUrl: string | null | undefined) {
    const defaultAvatarPath = "/static/image/avatar.svg";
    const avatarMediaViewerUrl = resolveAvatarMediaViewerUrl(avatarUrl);
    avatarImg.onerror = () => {
        avatarImg.src = defaultAvatarPath;
        avatarImg.onerror = null;
        if (avatarMediaViewerUrl && isAvatarVideoMediaViewerUrl(avatarMediaViewerUrl)) {
            setAvatarViewerState(avatarElement, avatarMediaViewerUrl);
            return;
        }
        setAvatarViewerState(avatarElement, null);
    };
    if (avatarMediaViewerUrl) {
        ApplyIpfsImageLoadPolicy(avatarImg, avatarMediaViewerUrl);
        avatarImg.src = avatarMediaViewerUrl;
        setAvatarViewerState(avatarElement, avatarMediaViewerUrl);
        return;
    }
    avatarImg.src = defaultAvatarPath;
    setAvatarViewerState(avatarElement, null);
}

async function handleAvatarLoad(avatarImg: HTMLImageElement, avatarElement: HTMLDivElement, cardElement: HTMLElement) {
    if (avatarImg.dataset.avatarLoaded === "true") {
        return;
    }
    avatarImg.dataset.avatarLoaded = "true";
    const blockchainInput = cardElement.querySelector('.postCardBlockchain, .profileCardBlockchain') as HTMLInputElement;
    const addressInput = cardElement.querySelector('.postCardAddress, .profileCardAddressInput') as HTMLInputElement;
    if (blockchainInput && addressInput) {
        const blockchain = blockchainInput.value;
        const address = addressInput.value;
        if (blockchain && address && IsValidAddress(address, blockchain)) {
            try {
                let avatarUrl: string | null = null;
                avatarUrl = await getIpfsAvatarUrl(blockchain, address);
                if (!avatarUrl || avatarUrl === "") {
                    avatarUrl = await WalletGetAvatar(blockchain, address);
                }
                const avatarMediaViewerUrl = resolveAvatarMediaViewerUrl(avatarUrl);
                if (avatarMediaViewerUrl && !avatarImg.src.endsWith(avatarMediaViewerUrl)) {
                    setAvatarImageSource(avatarImg, avatarElement, avatarMediaViewerUrl);
                } else if (!avatarMediaViewerUrl) {
                    setAvatarViewerState(avatarElement, null);
                }
            } catch (error) {
                LogError("Failed to fetch avatar: " + error);
            }
        }
    }
}

function resolveAttachmentUrl(attachmentRef: string, isLocalPost: boolean): string {
    if (isLocalPost && IsValidIpfsCid(attachmentRef)) {
        return `/files/download/${encodeURIComponent(attachmentRef)}`;
    }
    const converted = CIDToSubdomainURL(attachmentRef);
    if (converted) {
        return converted;
    }
    return attachmentRef;
}
function normalizeAttachmentCID(attachmentRef: string | null | undefined): string {
    if (!attachmentRef) {
        return "";
    }
    const trimmedRef = attachmentRef.trim();
    if (trimmedRef === "") {
        return "";
    }
    if (IsValidIpfsCid(trimmedRef)) {
        return trimmedRef.startsWith("ipfs://") ? trimmedRef.substring("ipfs://".length) : trimmedRef;
    }
    if (trimmedRef.startsWith("/files/download/")) {
        const cid = decodeURIComponent(trimmedRef.substring("/files/download/".length));
        return IsValidIpfsCid(cid) ? cid : "";
    }
    try {
        const parsedUrl = new URL(trimmedRef, window.location.origin);
        if (parsedUrl.pathname.startsWith("/files/download/")) {
            const cid = decodeURIComponent(parsedUrl.pathname.substring("/files/download/".length));
            return IsValidIpfsCid(cid) ? cid : "";
        }
        const pathParts = parsedUrl.pathname.split("/").filter(Boolean);
        if (pathParts.length >= 2 && pathParts[0] === "ipfs" && IsValidIpfsCid(pathParts[1])) {
            return pathParts[1];
        }
        const hostnameParts = parsedUrl.hostname.split(".");
        if (hostnameParts.length > 2 && hostnameParts[1] === "ipfs" && IsValidIpfsCid(hostnameParts[0])) {
            return hostnameParts[0];
        }
    } catch (error) {
        return "";
    }
    return "";
}
function getEmbeddedAttachmentCIDs(payload: string): Set<string> {
    const embeddedCIDs = new Set<string>();
    const parser = new DOMParser();
    const doc = parser.parseFromString(payload, "text/html");
    doc.body.querySelectorAll("img[src], video[src], source[src]").forEach((element) => {
        const ref = element.getAttribute("src");
        const cid = normalizeAttachmentCID(ref);
        if (cid !== "") {
            embeddedCIDs.add(cid);
        }
    });
    const bareMatches = extractBareIpfsMatches(doc.body.textContent || "");
    for (const match of bareMatches) {
        embeddedCIDs.add(match.cid);
    }
    return embeddedCIDs;
}

export async function CreateAttachmentCard(attachment: any[], isLocalPost: boolean = false): Promise<HTMLDivElement> {
    const attachmentCard = document.createElement("div") as HTMLDivElement;
    const iconRow = document.createElement("div") as HTMLDivElement;
    const fileIcon = document.createElement("i") as HTMLElement;
    const nameRow = document.createElement("div") as HTMLDivElement;
    const bottomRow = document.createElement("div") as HTMLDivElement;
    const downloadAnchor = document.createElement("a") as HTMLAnchorElement;
    const downloadButton = document.createElement("button") as HTMLButtonElement;
    const downloadIcon = document.createElement("i") as HTMLElement;
    const fileNameSpan = document.createElement("span") as HTMLSpanElement;
    const fileSizeSpan = document.createElement("span") as HTMLSpanElement;
    const iconClass = getFileIcon(attachment[1]);
    let attachmentURL: string;
    attachmentCard.classList.add("attachmentCard");
    iconRow.classList.add("attachmentCardIconRow");
    nameRow.classList.add("attachmentCardNameRow");
    bottomRow.classList.add("attachmentCardBottomRow");
    fileIcon.classList.add("icon", "attachmentCardIcon", iconClass);
    attachmentURL = resolveAttachmentUrl(attachment[0], isLocalPost);
    if (!IsValidURL(attachmentURL)) {
        return Promise.reject("Invalid URL");
    }
    const fileName = attachment[3];
    fileNameSpan.textContent = XSSSanitizeValue(fileName);
    fileNameSpan.classList.add("attachmentCardFileName");
    downloadAnchor.href = XSSSanitizeUrl(attachmentURL);
    downloadAnchor.download = XSSSanitizeValue(fileName);
    const fileSize = await formatFileSize(attachment[2]);
    fileSizeSpan.innerText = fileSize;
    fileSizeSpan.classList.add("attachmentCardFileSize");
    downloadButton.classList.add("downloadButton", "btn");
    downloadIcon.classList.add("downloadIcon", "bi", "bi-download");
    iconRow.appendChild(fileIcon);
    nameRow.appendChild(fileNameSpan);
    bottomRow.appendChild(fileSizeSpan);
    bottomRow.appendChild(downloadAnchor);
    attachmentCard.appendChild(iconRow);
    attachmentCard.appendChild(nameRow);
    attachmentCard.appendChild(bottomRow);
    downloadAnchor.appendChild(downloadButton);
    downloadButton.appendChild(downloadIcon);
    return attachmentCard;
}
export async function CreateCarousel(elements: HTMLElement[]): Promise<HTMLDivElement> {
    let carouselDiv = document.createElement("div") as HTMLDivElement;
    let carouselList = document.createElement("ol") as HTMLOListElement;
    let carouselInnerDiv = document.createElement("div") as HTMLDivElement;
    let previousButton = document.createElement("a") as HTMLAnchorElement;
    let previousIcon = document.createElement("span") as HTMLSpanElement;
    let nextButton = document.createElement("a") as HTMLAnchorElement;
    let nextIcon = document.createElement("span") as HTMLSpanElement;
    let nextIconDiv = document.createElement("div") as HTMLDivElement;
    let prevIconDiv = document.createElement("div") as HTMLDivElement;
    const elementsUUID = crypto.randomUUID();
    prevIconDiv.classList.add("prevIconDiv");
    nextIconDiv.classList.add("nextIconDiv");
    carouselDiv.classList.add("carousel", "slide");
    carouselDiv.id = elementsUUID;
    carouselList.classList.add("carousel-indicators");
    carouselInnerDiv.classList.add("carousel-inner");
    previousButton.classList.add("carousel-control-prev");
    previousButton.href = "#" + elementsUUID;
    previousButton.role = "button";
    previousButton.setAttribute("data-bs-slide", "prev");
    previousIcon.classList.add("carousel-control-prev-icon");
    previousIcon.ariaHidden = "true";
    nextButton.classList.add("carousel-control-next");
    nextButton.href = "#" + elementsUUID;
    nextButton.role = "button";
    nextButton.setAttribute("data-bs-slide", "next");
    nextIcon.classList.add("carousel-control-next-icon");
    nextIcon.ariaHidden = "true";
    for (let i = 0; i < elements.length; i++) {
        let element = elements[i];
        let selector = document.createElement("li") as HTMLLIElement;
        let item = document.createElement("div") as HTMLDivElement;
        if (i == 0) {
            selector.classList.add("active");
            item.classList.add("active");
        }
        selector.setAttribute("data-bs-target", "#" + elementsUUID);
        selector.setAttribute("data-bs-slide-to", i.toString());
        item.classList.add("carousel-item");
        element.classList.add("d-block");
        if (!element.classList.contains("postCardIntrinsicMedia")) {
            element.classList.add("w-100");
        }
        item.appendChild(element);
        prevIconDiv.appendChild(previousIcon);
        previousButton.appendChild(prevIconDiv);
        nextIconDiv.appendChild(nextIcon)
        nextButton.appendChild(nextIconDiv);
        carouselList.appendChild(selector);
        carouselInnerDiv.appendChild(item);
    }
    carouselDiv.appendChild(carouselList);
    carouselDiv.appendChild(carouselInnerDiv);
    carouselDiv.appendChild(previousButton);
    carouselDiv.appendChild(nextButton);
    return carouselDiv;
}
export async function CreateImageLoader(image: HTMLImageElement): Promise<HTMLDivElement> {
    const imageLoader = document.createElement("div") as HTMLDivElement;
    imageLoader.classList.add("image-container");
    const spinner = document.createElement("div") as HTMLDivElement;
    spinner.classList.add("spinner-border", "text-primary", "spinner-div");
    spinner.setAttribute("role", "status");
    imageLoader.style.paddingTop = "56.25%"
    image.style.opacity = "0";
    image.style.display = "block";
    image.style.borderRadius = "inherit";
    imageLoader.appendChild(spinner);
    imageLoader.appendChild(image);
    let imageLoaded = false;
    const timeout = window.setTimeout(() => {
        if (!imageLoaded) {
            spinner.remove();
            image.style.opacity = "1";
            image.src = "/static/image/imagefail.png";
            image.style.objectFit = "contain";
            return imageLoader;
        }
    }, 10000);
    image.onload = () => {
        imageLoaded = true;
        clearTimeout(timeout);
        spinner.remove();
        image.style.opacity = "1";
        imageLoader.style.removeProperty("padding-top");
    };
    image.onerror = () => {
        imageLoaded = true;
        clearTimeout(timeout);
        spinner.remove();
        image.src = "/static/image/imagefail.png";
        image.style.opacity = "1";
        image.style.objectFit = "contain";
        imageLoader.style.removeProperty("padding-top");
    }
    return imageLoader
}
async function getInlineImageRenderability(html: string): Promise<Map<string, boolean>> {
    const parser = new DOMParser();
    const doc = parser.parseFromString(html, "text/html");
    const inlineImageUrls = new Set<string>();
    doc.body.querySelectorAll("img[src]").forEach(image => {
        const src = image.getAttribute("src")?.trim() || "";
        if (!src.startsWith("https://") || !IsValidURL(src)) {
            return;
        }
        inlineImageUrls.add(src);
    });
    const renderChecks = await Promise.all(Array.from(inlineImageUrls).map(async url => {
        return [url, await canRenderInlineImage(url)] as const;
    }));
    const renderability = new Map<string, boolean>();
    for (const [url, canRender] of renderChecks) {
        renderability.set(url, canRender);
    }
    return renderability;
}
function cleanupEmptyPostParagraphs(postTextDiv: HTMLElement): void {
    postTextDiv.querySelectorAll("p").forEach((p) => {
        if (p.textContent?.trim() === "" && !p.querySelector("iframe, img, video")) {
            p.remove();
        }
    });
}
function extractBareIpfsMatches(text: string): {cid: string; end: number; start: number; token: string}[] {
    const matches = Array.from(text.matchAll(/ipfs:\/\/[A-Za-z0-9]+/g));
    const results: {cid: string; end: number; start: number; token: string}[] = [];
    for (const match of matches) {
        const token = match[0];
        const start = match.index || 0;
        const end = start + token.length;
        const nextChar = text.charAt(end);
        if (nextChar === "/" || nextChar === "?" || nextChar === "#") {
            continue;
        }
        if (!IsValidIpfsCid(token)) {
            continue;
        }
        results.push({
            cid: token.substring("ipfs://".length),
            end: end,
            start: start,
            token: token,
        });
    }
    return results;
}
async function linkifyBareIpfsText(element: HTMLElement, representedCIDs: Set<string>): Promise<{anchor: HTMLAnchorElement; cid: string}[]> {
    const nodeFilter = element.ownerDocument.defaultView?.NodeFilter || window.NodeFilter;
    const walker = element.ownerDocument.createTreeWalker(element, nodeFilter.SHOW_TEXT, {
        acceptNode(node) {
            const parentElement = node.parentElement;
            if (!parentElement || parentElement.closest("a, img, video, source, iframe")) {
                return nodeFilter.FILTER_REJECT;
            }
            if (!(node.textContent || "").includes("ipfs://")) {
                return nodeFilter.FILTER_REJECT;
            }
            return nodeFilter.FILTER_ACCEPT;
        }
    });
    const textNodes: Text[] = [];
    let currentNode = walker.nextNode();
    while (currentNode) {
        textNodes.push(currentNode as Text);
        currentNode = walker.nextNode();
    }
    const candidates: {anchor: HTMLAnchorElement; cid: string}[] = [];
    for (const textNode of textNodes) {
        const text = textNode.textContent || "";
        const matches = extractBareIpfsMatches(text);
        if (matches.length === 0) {
            continue;
        }
        const fragment = textNode.ownerDocument.createDocumentFragment();
        let cursor = 0;
        let didMutate = false;
        for (const match of matches) {
            if (match.start > cursor) {
                fragment.appendChild(textNode.ownerDocument.createTextNode(text.slice(cursor, match.start)));
            }
            if (representedCIDs.has(match.cid)) {
                didMutate = true;
                cursor = match.end;
                continue;
            }
            const resolvedUrl = ResolveIpfsContentUrl(match.cid);
            const sanitizedUrl = XSSSanitizeUrl(resolvedUrl);
            if (resolvedUrl === "" || sanitizedUrl === "#") {
                fragment.appendChild(textNode.ownerDocument.createTextNode(match.token));
                cursor = match.end;
                continue;
            }
            const anchor = textNode.ownerDocument.createElement("a");
            anchor.href = sanitizedUrl;
            anchor.rel = "noopener noreferrer";
            anchor.target = "_blank";
            anchor.textContent = match.token;
            fragment.appendChild(anchor);
            representedCIDs.add(match.cid);
            candidates.push({anchor, cid: match.cid});
            didMutate = true;
            cursor = match.end;
        }
        if (!didMutate) {
            continue;
        }
        if (cursor < text.length) {
            fragment.appendChild(textNode.ownerDocument.createTextNode(text.slice(cursor)));
        }
        textNode.parentNode?.replaceChild(fragment, textNode);
    }
    return candidates;
}
async function upgradeBareIpfsLinksToEmbeds(candidates: {anchor: HTMLAnchorElement; cid: string}[]): Promise<void> {
    if (candidates.length === 0) {
        return;
    }
    const results = await Promise.all(candidates.map(async (candidate) => {
        return {
            anchor: candidate.anchor,
            cid: candidate.cid,
            mediaType: await ProbeIpfsMediaType(candidate.cid),
            resolvedUrl: ResolveIpfsContentUrl(candidate.cid),
        };
    }));
    for (const result of results) {
        if (!result.anchor.isConnected || result.resolvedUrl === "") {
            continue;
        }
        if (result.mediaType === "image") {
            const imageEmbed = createIpfsImageEmbed(result.resolvedUrl, result.cid);
            if (imageEmbed) {
                result.anchor.replaceWith(imageEmbed);
            }
            continue;
        }
        if (result.mediaType === "video") {
            const videoEmbed = createIpfsVideoEmbed(result.resolvedUrl);
            if (videoEmbed) {
                result.anchor.replaceWith(videoEmbed);
            }
        }
    }
}
async function applyFileCardSummary(postTextDiv: HTMLSpanElement, attachments: any[]): Promise<void> {
    if (!attachments || attachments.length === 0) {
        return;
    }
    const summary = document.createElement("span");
    const badge = document.createElement("span");
    const title = document.createElement("span");
    const meta = document.createElement("span");
    let totalSize = 0;
    for (const attachment of attachments) {
        totalSize += Number(attachment[2] || 0);
    }
    const formattedSize = await formatFileSize(totalSize);
    summary.classList.add("fileCardSummary");
    badge.classList.add("fileCardBadge");
    title.classList.add("fileCardName");
    meta.classList.add("fileCardMeta");
    badge.textContent = attachments.length === 1 ? "File" : "Files";
    if (attachments.length === 1) {
        title.textContent = XSSSanitizeValue(attachments[0][3] || "Uploaded file");
        meta.textContent = formattedSize;
    } else {
        title.textContent = `${attachments.length} uploaded files`;
        meta.textContent = `${formattedSize} total`;
    }
    postTextDiv.innerHTML = "";
    summary.appendChild(badge);
    summary.appendChild(title);
    summary.appendChild(meta);
    postTextDiv.appendChild(summary);
}
export async function CreatePostCard(postData: any): Promise<HTMLDivElement> {
    let postDiv = document.createElement("div") as HTMLDivElement;
    let postID = document.createElement("input") as HTMLInputElement;
    let postAddress = document.createElement("input") as HTMLInputElement;
    let postBlockchain = document.createElement("input") as HTMLInputElement;
    let avatarDiv = document.createElement("div") as HTMLDivElement;
    let avatarImg = document.createElement("img") as HTMLImageElement;
    let postHeaderDiv = document.createElement("div") as HTMLDivElement;
    let postAuthorLink = document.createElement("a") as HTMLAnchorElement;
    let postAuthor = document.createElement("b") as HTMLElement;
    let postDate = document.createElement("div") as HTMLDivElement;
    let ellipsesDiv = document.createElement("div") as HTMLDivElement;
    let ellipsesBtn = document.createElement("button") as HTMLButtonElement;
    let ellipses = document.createElement("i") as HTMLElement;
    let ellipsesMenu = document.createElement("ul") as HTMLUListElement;
    let ellipsesMenuItemExplorer = document.createElement("li") as HTMLLIElement;
    let ellipsesMenuItemExplorerLink = document.createElement("a") as HTMLAnchorElement;
    let postTextDiv = document.createElement("span") as HTMLSpanElement;
    let embedDiv = document.createElement("div") as HTMLDivElement;
    let reactionDiv = document.createElement("div") as HTMLDivElement;
    let unixpostdate = postData.timestamp;
    let postdatevalue = new Date(unixpostdate * 1000).toLocaleDateString(undefined, {month: 'short', day: 'numeric', year: 'numeric', hour: 'numeric', minute: '2-digit', hour12: true});
    let walletAddressLink = WalletGetYourPlaceAddressLink(postData.address);
    let walletTxLink = WalletGetExplorerTxLink(postData.txHash, postData.blockchain);
    postDiv.classList.add("postCard");
    if (!postData.localPost) {
        postDiv.classList.add("postCardClickable");
    }
    postID.type = "hidden";
    postID.classList.add("postCardID");
    postID.value = postData.txHash;
    postBlockchain.type = "hidden";
    postBlockchain.classList.add("postCardBlockchain");
    postBlockchain.value = XSSSanitizeValue(postData.blockchain);
    postAddress.type = "hidden";
    postAddress.classList.add("postCardAddress");
    postAddress.value = XSSSanitizeValue(postData.address);
    avatarDiv.classList.add("postCardAvatar");
    avatarDiv.addEventListener("click", (event: MouseEvent) => {
        event.preventDefault();
        event.stopPropagation();
        const avatarMediaViewerUrl = avatarDiv.dataset.mediaViewerSrc;
        if (!avatarMediaViewerUrl) {
            return;
        }
        ShowAvatarMediaViewer(avatarMediaViewerUrl, postData.author || "avatar");
    });
    avatarImg.classList.add("postCardAvatar");
    avatarImg.crossOrigin = "anonymous";
    avatarImg.referrerPolicy = "no-referrer";
    setAvatarImageSource(avatarImg, avatarDiv, postData.avatarSrc);
    avatarImg.addEventListener("load", function(): void {
        handleAvatarLoad(avatarImg, avatarDiv, postDiv);
    });
    postHeaderDiv.classList.add("postCardHeaderDiv");
    postAuthorLink.classList.add("postCardAuthorLink");
    postAuthorLink.href = XSSSanitizeUrl(walletAddressLink);
    postAuthor.classList.add("postCardAuthor");
    postAuthor.textContent = postData.author || "Anonymous";
    postDate.classList.add("postCardDate");
    postDate.textContent = postdatevalue;
    ellipsesDiv.classList.add("postCardEllipsesDiv");
    ellipsesDiv.classList.add("clickable");
    ellipsesDiv.classList.add("dropstart");
    ellipsesDiv.classList.add("btn-group");
    ellipsesBtn.classList.add("btn");
    ellipsesBtn.classList.add("btn-secondary");
    ellipsesBtn.classList.add("ellipsesBtn");
    ellipsesBtn.dataset.bsToggle = "dropdown";
    ellipsesBtn.ariaExpanded = "false";
    ellipses.classList.add("bi");
    ellipses.classList.add("bi-three-dots");
    ellipses.classList.add("postCardContextIcon");
    ellipsesMenu.classList.add("dropdown-menu");
    ellipsesMenu.classList.add("ellipsesMenu");
    ellipsesMenuItemExplorerLink.innerText = "View On Explorer";
    ellipsesMenuItemExplorerLink.href = XSSSanitizeUrl(walletTxLink);
    ellipsesMenuItemExplorerLink.target = "_blank";
    ellipsesMenuItemExplorerLink.classList.add("postCardEllipsesLink");
    postTextDiv.classList.add("postCardTextDiv");
    postTextDiv.textContent = postData.payload;
    embedDiv.classList.add("postCardEmbedDiv");
    reactionDiv.classList.add("postCardReactionDiv");
    const inlineAttachmentCIDs = getEmbeddedAttachmentCIDs(postData.payload || "");
    const separatelyRenderedAttachmentCIDs = new Set<string>();
    postDiv.appendChild(postID);
    postDiv.appendChild(postBlockchain);
    postDiv.appendChild(postAddress);
    avatarDiv.appendChild(avatarImg);
    postDiv.appendChild(avatarDiv);
    postDiv.appendChild(postHeaderDiv);
    postAuthorLink.appendChild(postAuthor);
    postHeaderDiv.appendChild(postAuthorLink);
    postHeaderDiv.appendChild(postDate);
    ellipsesBtn.appendChild(ellipses);
    ellipsesDiv.appendChild(ellipsesBtn);
    if (!postData.localPost) {
        ellipsesMenuItemExplorer.appendChild(ellipsesMenuItemExplorerLink);
        ellipsesMenu.appendChild(ellipsesMenuItemExplorer);
        ellipsesDiv.appendChild(ellipsesMenu);
        postHeaderDiv.appendChild(ellipsesDiv);
    }
    postDiv.appendChild(postTextDiv);
    if ("attachments" in postData) {
        let attachmentDiv = document.createElement("div") as HTMLDivElement;
        attachmentDiv.classList.add("postCardAttachmentDiv");
        let renderedAttachmentElements: HTMLElement[] = [];
        let listedAttachmentElements: HTMLElement[] = [];
        for (let i = 0; i < postData.attachments.length; i ++) {
            let attachment = postData.attachments[i];
            let attachmentCID = normalizeAttachmentCID(attachment[0]);
            if (attachmentCID !== "" && inlineAttachmentCIDs.has(attachmentCID)) {
                continue;
            }
            if (attachmentCID !== "") {
                separatelyRenderedAttachmentCIDs.add(attachmentCID);
            }
            let mimeType = attachment[1];
            let fileUrl = resolveAttachmentUrl(attachment[0], !!postData.localPost);
            switch (mimeType) {
                case "image/jpeg":
                case "image/png":
                case "image/webp":
                case "image/gif":
                    let image = document.createElement("img") as HTMLImageElement;
                    ApplyIpfsImageLoadPolicy(image, fileUrl);
                    image.src = fileUrl;
                    image.classList.add("postAttachment", "postCardAttachmentImage");
                    let imageLoader = await CreateImageLoader(image);
                    imageLoader.classList.add("expandable");
                    imageLoader.addEventListener("click", expandView);
                    if (postData.attachments.length === 1) {
                        imageLoader.classList.add("postAttachment");
                        renderedAttachmentElements.push(imageLoader);
                    } else {
                        renderedAttachmentElements.push(imageLoader);
                    }
                    break;
                case "video/mp4":
                case "video/mpeg":
                case "video/quicktime":
                case "video/webm":
                case "video/x-matroska":
                    if (postData.resultType === "file") {
                        const localPreviewUrl = typeof attachment[4] === "string" ? attachment[4] : "";
                        const videoUrl = localPreviewUrl || (postData.localPost && attachmentCID !== "" ? `/files/preview/${encodeURIComponent(attachmentCID)}` : fileUrl);
                        const video = createIpfsVideoEmbed(videoUrl);
                        if (video) {
                            video.classList.add("postCardFileVideo", "postCardIntrinsicMedia");
                            renderedAttachmentElements.push(video);
                            break;
                        }
                    }
                    let videoAttachmentCard = await CreateAttachmentCard(postData.attachments[i], !!postData.localPost).catch( e =>{
                        return "failed"
                    });
                    if (!(videoAttachmentCard instanceof HTMLDivElement)) {
                        break;
                    }
                    (videoAttachmentCard as unknown as HTMLDivElement).classList.add("postAttachment");
                    listedAttachmentElements.push(videoAttachmentCard as unknown as HTMLDivElement);
                    break;
                default:
                    let attachmentCard = await CreateAttachmentCard(postData.attachments[i], !!postData.localPost).catch( e =>{
                        return "failed"
                    });
                    if (!(attachmentCard instanceof HTMLDivElement)) {
                        break;
                    }
                    (attachmentCard as unknown as HTMLDivElement).classList.add("postAttachment");
                    listedAttachmentElements.push(attachmentCard as unknown as HTMLDivElement);
                    break;
            }
        }
        const attachments = [...renderedAttachmentElements, ...listedAttachmentElements];
        const chunkedAttachments: HTMLElement[][] = [];
        const attachmentPages: HTMLElement[] = [];
        for (let i = 0; i < attachments.length; i += 4) {
            chunkedAttachments.push(attachments.slice(i, i + 4));
        }
        for (let i = 0; i < chunkedAttachments.length; i++) {
            switch (chunkedAttachments[i].length) {
                case 1:
                    const attachment = chunkedAttachments[i][0];
                    attachment.style.borderRadius = "1em";
                    attachmentPages.push(attachment);
                    break;
                case 2:
                    const pageOf2 = await grid2Attachments(chunkedAttachments[i]);
                    attachmentPages.push(pageOf2);
                    break;
                case 3:
                    const pageOf3 = await grid3Attachments(chunkedAttachments[i]);
                    attachmentPages.push(pageOf3);
                    break;
                case 4:
                    const pageOf4 = await grid4Attachments(chunkedAttachments[i]);
                    attachmentPages.push(pageOf4);
                    break;
            }
        }
        if (attachmentPages.length === 1) {
            attachmentDiv.appendChild(attachmentPages[0]);
        } else {
            const carousel = await CreateCarousel(attachmentPages);
            carousel.classList.add("postAttachmentCarousel");
            attachmentDiv.appendChild(carousel);
        }
        postDiv.appendChild(attachmentDiv);
    }
    const addCommentContainer = document.createElement("div");
    addCommentContainer.classList.add("addCommentContainer");
    const closeCommentSection = () => {
        if (addCommentContainer.classList.contains("expanded")) {
            addCommentContainer.classList.remove("expanded");
            addCommentContainer.innerHTML = "";
            const commentIcon = postDiv.querySelector(".postControlItem.comment i") as HTMLElement | null;
            if (commentIcon) {
                commentIcon.style.color = "";
            }
        }
    };
    document.addEventListener("click", (e: MouseEvent) => {
        if (addCommentContainer.classList.contains("expanded") && !postDiv.contains(e.target as Node)) {
            closeCommentSection();
        }
    });
    const controlsBar = CreatePostControlsBar({
        txHash: postData.txHash,
        blockchain: postData.blockchain,
        targetType: 'post',
        initialLikes: postData.likes || 0,
        initialDislikes: postData.dislikes || 0,
        initialComments: postData.commentCount || 0,
        initialEmojiCount: postData.emojiCount || 0,
        userReaction: postData.userReaction || null,
        userEmojiReaction: postData.userEmojiReaction || null,
        userHasCommented: postData.userHasCommented || false,
        onCommentClick: () => {
            const commentBtn = controlsBar.querySelector(".comment") as HTMLElement;
            const commentIcon = commentBtn?.querySelector("i") as HTMLElement | null;
            if (addCommentContainer.children.length > 0) {
                addCommentContainer.innerHTML = "";
                addCommentContainer.classList.remove("expanded");
                if (commentIcon) {
                    commentIcon.style.color = "";
                }
            } else {
                const commentUI = ShowAddCommentUI(postData.txHash, postData.blockchain, () => {
                    addCommentContainer.classList.remove("expanded");
                }, commentBtn);
                addCommentContainer.appendChild(commentUI);
                addCommentContainer.classList.add("expanded");
            }
        },
        onRepostClick: () => {
            const postUrl = `/post/${postData.blockchain}/${postData.txHash}`;
            const addPostTextarea = document.getElementById("addPostTextarea") as HTMLTextAreaElement;
            if (addPostTextarea) {
                addPostTextarea.value = postUrl;
                addPostTextarea.focus();
            } else if (typeof window.tinymce !== 'undefined') {
                const editor = window.tinymce.get("tinyMceEditor");
                if (editor) {
                    editor.setContent(postUrl);
                    editor.focus();
                }
            }
        }
    });
    if (!postData.localPost) {
        reactionDiv.appendChild(controlsBar);
        postDiv.appendChild(reactionDiv);
        postDiv.appendChild(addCommentContainer);
    }
    const commentThreadContainer = document.createElement("div");
    commentThreadContainer.classList.add("commentThreadContainer");
    let commentThreadLoaded = false;
    const toggleCommentThread = () => {
        if (commentThreadContainer.classList.contains("expanded")) {
            commentThreadContainer.classList.remove("expanded");
            commentThreadContainer.innerHTML = "";
            commentThreadLoaded = false;
        } else {
            commentThreadContainer.classList.add("expanded");
            if (!commentThreadLoaded) {
                const thread = CreateCommentThread({
                    blockchain: postData.blockchain,
                    parentTxHash: postData.txHash,
                });
                commentThreadContainer.appendChild(thread);
                commentThreadLoaded = true;
            }
        }
    };
    if (!postData.localPost) {
        postDiv.appendChild(commentThreadContainer);
    }
    const blockchainIconPath = getBlockchainIconPath(postData.blockchain);
    if (blockchainIconPath) {
        let blockchainBadge = document.createElement("div") as HTMLDivElement;
        let blockchainIcon = document.createElement("img") as HTMLImageElement;
        let blockchainUrl = getBlockchainUrl(postData.blockchain);
        blockchainBadge.classList.add("blockchainBadge");
        blockchainBadge.title = postData.blockchain;
        blockchainIcon.src = blockchainIconPath;
        blockchainIcon.classList.add("blockchainBadgeIcon");
        if (blockchainUrl) {
            let blockchainLink = document.createElement("a") as HTMLAnchorElement;
            blockchainLink.href = blockchainUrl;
            blockchainLink.target = "_blank";
            blockchainLink.rel = "noopener noreferrer";
            blockchainLink.appendChild(blockchainIcon);
            blockchainBadge.appendChild(blockchainLink);
        } else {
            blockchainBadge.appendChild(blockchainIcon);
        }
        postDiv.appendChild(blockchainBadge);
    }
    const attachmentUrls = new Set<string>();
    if ("attachments" in postData) {
        for (const attachment of postData.attachments) {
            let fileUrl = resolveAttachmentUrl(attachment[0], !!postData.localPost);
            attachmentUrls.add(fileUrl);
            const attachmentCID = normalizeAttachmentCID(attachment[0]);
            if (attachmentCID !== "") {
                attachmentUrls.add(`ipfs://${attachmentCID}`);
                attachmentUrls.add(`/files/download/${encodeURIComponent(attachmentCID)}`);
            }
        }
    }
    const representedIpfsCIDs = new Set<string>(separatelyRenderedAttachmentCIDs);
    const urlRegex = /(https:\/\/[^\s"<>]+)/g;
    const inlineImageRenderability = await getInlineImageRenderability(postData.payload);
    let postText = postData.payload;
    const urls = postData.payload.match(urlRegex);
    if (urls) {
        for (const url of urls) {
            if (attachmentUrls.has(url)) {
                postText = postText.replace(url, "").trim();
                continue;
            }
            const inlineImageCanRender = inlineImageRenderability.get(url);
            if (inlineImageCanRender === true) {
                continue;
            }
            const imageEmbed = await createImageEmbed(url);
            if (imageEmbed) {
                embedDiv.appendChild(imageEmbed);
                if (inlineImageCanRender !== false) {
                    postText = postText.replace(url, "").trim();
                }
                continue;
            }
            const youtubeEmbed = createYoutubeEmbed(url);
            if (youtubeEmbed) {
                embedDiv.appendChild(youtubeEmbed);
                postText = postText.replace(url, "").trim();
                continue;
            }
            const xcomEmbed = await XcomOEmbedCard(url);
            if (xcomEmbed) {
                embedDiv.appendChild(xcomEmbed);
                postText = postText.replace(url, "").trim();
                continue;
            }
            const oembedCard = await OEmbedCard(url);
            if (oembedCard) {
                embedDiv.appendChild(oembedCard);
                postText = postText.replace(url, "").trim();
                continue;
            }
        }
    }
    postTextDiv.innerHTML = XSSSanitizeTinyMCEHtml(postText);
    cleanupEmptyPostParagraphs(postTextDiv);
    processTextWithTags(postTextDiv);
    const images = postTextDiv.querySelectorAll("img");
    images.forEach(img => {
        const src = img.getAttribute("src");
        if (src && inlineImageRenderability.get(src) === false) {
            img.remove();
            return;
        }
        img.classList.add("postCardBodyImage");
        if (src && src.startsWith("ipfs://")) {
            const converted = CIDToSubdomainURL(src);
            if (converted) {
                ApplyIpfsImageLoadPolicy(img, converted);
                img.src = converted;
            }
        }
    });
    const videos = postTextDiv.querySelectorAll("video");
    videos.forEach(video => {
        const src = video.getAttribute("src");
        if (src && src.startsWith("ipfs://")) {
            const converted = CIDToSubdomainURL(src);
            if (converted) {
                video.src = converted;
                video.classList.add("postCardInlineVideo");
            }
        }
    });
    const bareIpfsLinkCandidates = await linkifyBareIpfsText(postTextDiv, representedIpfsCIDs);
    await upgradeBareIpfsLinksToEmbeds(bareIpfsLinkCandidates);
    cleanupEmptyPostParagraphs(postTextDiv);
    postTextDiv.appendChild(embedDiv);
    ProcessPostContentForPreviews(postTextDiv);
    if (postData.resultType === "file") {
        postDiv.classList.add("fileCard");
        await applyFileCardSummary(postTextDiv, postData.attachments || []);
    }
    if (!postData.localPost) {
        const postUrl = `/post/${postData.blockchain}/${postData.txHash}`;
        postDiv.addEventListener("click", (e: MouseEvent) => {
            const target = e.target as HTMLElement;
            if (target.closest("a, button, iframe, video, .addCommentContainer, .blockchainBadge, .commentThreadContainer, .postCardAttachmentDiv, .postCardAvatar, .postCardEmbedDiv, .postCardEllipsesDiv, .postControlsBar")) return;
            window.location.href = postUrl;
        });
    }
    return postDiv;
}
