import "../../scss/components/imageLoader.scss";
import "../../scss/components/postCard.scss";
import { CreateCommentThread } from "./commentThread";
import { CreatePostControlsBar, FetchReactionCounts, FetchUserHasCommented } from "./postControls";
import { ProcessPostContentForPreviews } from "./postPreviewCard";
import { ShowAddCommentUI } from "./addComment";
import { ShowModalMediaViewer } from "./modalMediaViewer";
import { XcomOEmbedCard } from "./xcomOEmbedCard";
import { GetAddress, IsValidAddress, WalletGetExplorerTxLink, WalletGetYourPlaceAddressLink, WalletGetAvatar } from "../util/blockchain/wallet";
import { IsValidBlockchain, IsValidURL, XSSSanitizeTinyMCEHtml, XSSSanitizeUrl, XSSSanitizeValue } from "../util/security";
import { CIDToSubdomainURL, getIpfsAvatarUrl } from "../util/ipfs";
import { getFileIcon, formatFileSize } from "../util/files";
import { LogError } from "../util/log";
import { getBlockchainIconPath, getBlockchainUrl, processTextWithTags } from "../util/domFactory";

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
async function fetchAndUpdatePostControls(controlsBar: HTMLDivElement, blockchain: string, txHash: string): Promise<void> {
    try {
        const address = GetAddress();
        const [counts, hasCommented] = await Promise.all([
            FetchReactionCounts(blockchain, txHash, address || undefined),
            address ? FetchUserHasCommented(blockchain, txHash, address) : Promise.resolve(false)
        ]);
        if (!counts) return;
        const userEmojiReaction = counts.userEmojiReaction || null;
        const userReaction = counts.userReaction || null;
        const commentControl = controlsBar.querySelector(".postControlItem.comment");
        const dislikeControl = controlsBar.querySelector(".postControlItem.dislike");
        const likeControl = controlsBar.querySelector(".postControlItem.like");
        const reactControl = controlsBar.querySelector(".postControlItem.react");
        if (commentControl && hasCommented) {
            commentControl.classList.add("active");
        }
        if (likeControl) {
            const countSpan = likeControl.querySelector(".count");
            if (countSpan) {
                countSpan.textContent = counts.likes > 0 ? counts.likes.toString() : "";
            }
            if (userReaction === "like") {
                likeControl.classList.add("active");
            }
        }
        if (dislikeControl) {
            const countSpan = dislikeControl.querySelector(".count");
            if (countSpan) {
                countSpan.textContent = counts.dislikes > 0 ? counts.dislikes.toString() : "";
            }
            if (userReaction === "dislike") {
                dislikeControl.classList.add("active");
            }
        }
        if (reactControl) {
            let emojiCount = 0;
            if (counts.emoji) {
                for (const count of Object.values(counts.emoji) as number[]) {
                    emojiCount += count;
                }
            }
            const countSpan = reactControl.querySelector(".count");
            if (countSpan) {
                countSpan.textContent = emojiCount > 0 ? emojiCount.toString() : "";
            }
            if (userEmojiReaction) {
                reactControl.classList.add("active");
                const existingEmoji = reactControl.querySelector(".reactEmoji");
                const existingIcon = reactControl.querySelector("i.bi");
                if (existingIcon) existingIcon.remove();
                if (existingEmoji) {
                    existingEmoji.textContent = userEmojiReaction;
                } else {
                    const emojiSpan = document.createElement("span");
                    emojiSpan.classList.add("reactEmoji");
                    emojiSpan.textContent = userEmojiReaction;
                    reactControl.insertBefore(emojiSpan, reactControl.firstChild);
                }
            }
        }
    } catch (e) {
        LogError("Failed to fetch post controls data: " + e);
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
async function handleAvatarLoad(avatarImg: HTMLImageElement, cardElement: HTMLElement) {
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
                if (avatarUrl && avatarUrl !== "" && !avatarImg.src.endsWith(XSSSanitizeUrl(avatarUrl))) {
                    avatarImg.src = XSSSanitizeUrl(avatarUrl);
                }
            } catch (error) {
                LogError("Failed to fetch avatar: " + error);
            }
        }
    }
}

export async function CreateAttachmentCard(attachment: any[]): Promise<HTMLDivElement> {
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
    if (attachment[0].startsWith("ipfs://")) {
        attachmentURL = CIDToSubdomainURL(attachment[0]);
    } else {
        attachmentURL = attachment[0];
    }
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
        element.classList.add("d-block", "w-100");
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
    postDiv.classList.add("postCard", "postCardClickable");
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
    if (postData.resultType != "profile post" && IsValidBlockchain(postData.blockchain) && IsValidAddress(postData.address, postData.blockchain)) {
        avatarDiv.classList.add("clickable");
        avatarDiv.addEventListener("click", () => {
            window.location.href = "/p/" + postData.blockchain + "/" + postData.address;
        });
    }
    avatarImg.classList.add("postCardAvatar");
    avatarImg.crossOrigin = "anonymous";
    avatarImg.referrerPolicy = "no-referrer";
    if (postData.avatarSrc === "" || postData.avatarSrc === null || postData.avatarSrc === undefined) {
        avatarImg.src = "/static/image/avatar.png";
    } else {
        let avatarSrc = postData.avatarSrc;
        if (avatarSrc.startsWith("ipfs://")) {
            const converted = CIDToSubdomainURL(avatarSrc);
            avatarSrc = converted || "/static/image/avatar.png";
        }
        avatarImg.src = XSSSanitizeUrl(avatarSrc);
    }
    avatarImg.addEventListener("load", function(): void {
        handleAvatarLoad(avatarImg, postDiv);
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
    ellipsesMenuItemExplorer.appendChild(ellipsesMenuItemExplorerLink);
    ellipsesMenu.appendChild(ellipsesMenuItemExplorer);
    ellipsesDiv.appendChild(ellipsesMenu);
    postHeaderDiv.appendChild(ellipsesDiv);
    postDiv.appendChild(postTextDiv);
    if ("attachments" in postData) {
        let attachmentDiv = document.createElement("div") as HTMLDivElement;
        attachmentDiv.classList.add("postCardAttachmentDiv");
        let renderedAttachmentElements: HTMLElement[] = [];
        let listedAttachmentElements: HTMLElement[] = [];
        for (let i = 0; i < postData.attachments.length; i ++) {
            let attachment = postData.attachments[i];
            let mimeType = attachment[1];
            let fileUrl = attachment[0];
            if (fileUrl.startsWith("ipfs://")) {
                fileUrl = CIDToSubdomainURL(fileUrl);
            }
            switch (mimeType) {
                case "image/jpeg":
                case "image/png":
                case "image/webp":
                case "image/gif":
                    let image = document.createElement("img") as HTMLImageElement;
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
                default:
                    let attachmentCard = await CreateAttachmentCard(postData.attachments[i]).catch( e =>{
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
    reactionDiv.appendChild(controlsBar);
    postDiv.appendChild(reactionDiv);
    postDiv.appendChild(addCommentContainer);
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
    postDiv.appendChild(commentThreadContainer);
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
    fetchAndUpdatePostControls(controlsBar, postData.blockchain, postData.txHash);
    const attachmentUrls = new Set<string>();
    if ("attachments" in postData) {
        for (const attachment of postData.attachments) {
            let fileUrl = attachment[0];
            if (fileUrl.startsWith("ipfs://")) {
                fileUrl = CIDToSubdomainURL(fileUrl);
            }
            attachmentUrls.add(fileUrl);
        }
    }
    const urlRegex = /(https:\/\/[^\s"<>]+)/g;
    let postText = postData.payload;
    const urls = postData.payload.match(urlRegex);
    if (urls) {
        for (const url of urls) {
            if (attachmentUrls.has(url)) {
                postText = postText.replace(url, "").trim();
                continue;
            }
            const imageEmbed = await createImageEmbed(url);
            if (imageEmbed) {
                embedDiv.appendChild(imageEmbed);
                postText = postText.replace(url, "").trim();
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
        }
    }
    postTextDiv.innerHTML = XSSSanitizeTinyMCEHtml(postText);
    postTextDiv.querySelectorAll("p").forEach(p => {
        if (p.textContent?.trim() === "") { p.remove(); }
    });
    processTextWithTags(postTextDiv);
    postTextDiv.appendChild(embedDiv);
    const images = postTextDiv.querySelectorAll("img");
    images.forEach(img => {
        const src = img.getAttribute("src");
        if (src && src.startsWith("ipfs://")) {
            const converted = CIDToSubdomainURL(src);
            if (converted) {
                img.src = converted;
                img.classList.add("postCardInlineImage");
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
    ProcessPostContentForPreviews(postTextDiv);
    const postUrl = `/post/${postData.blockchain}/${postData.txHash}`;
    postDiv.addEventListener("click", (e: MouseEvent) => {
        const target = e.target as HTMLElement;
        if (target.closest("a, button, iframe, video, .addCommentContainer, .blockchainBadge, .commentThreadContainer, .postCardAttachmentDiv, .postCardAvatar, .postCardEmbedDiv, .postCardEllipsesDiv, .postControlsBar")) return;
        window.location.href = postUrl;
    });
    return postDiv;
}
