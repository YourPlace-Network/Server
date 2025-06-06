import "../../scss/components/postCard.scss";
import "../../scss/components/profileCard.scss";
import "../../scss/components/imageLoader.scss";
import {IsValidAddress, WalletGetExplorerAddressLink, WalletGetExplorerTxLink} from "./blockchain/wallet";
import {IsValidBlockchain, IsValidURL, XSSSanitizeTinyMCEHtml, XSSSanitizeUrl, XSSSanitizeValue} from "./security";
import {CIDToSubdomainURL} from "./ipfs";
import {LogInfo} from "./log";
import {getFileIcon, formatFileSize} from "./files";
import path from "path";
import {extensionToMimeType} from "./mimeTypes";
import {ShowModalMediaViewer} from "../components/modalMediaViewer";
import {base64decode} from "byte-base64";

export async function CreatePostCard(postData: any): Promise<HTMLDivElement> { // returns a post div element when given a post's data
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
    let postTextDiv = document.createElement("div") as HTMLDivElement;
    let embedDiv = document.createElement("div") as HTMLDivElement;
    let reactionDiv = document.createElement("div") as HTMLDivElement;
    let unixpostdate = postData.timestamp;
    let postdatevalue = new Date(unixpostdate * 1000).toLocaleDateString(undefined, {month: 'short', day: 'numeric', year: 'numeric', hour: 'numeric', minute: '2-digit', hour12: true});
    let walletAddressLink = WalletGetExplorerAddressLink(postData.address);
    let walletTxLink = WalletGetExplorerTxLink(postData.txHash);

    // adding attributes to elements
    postDiv.classList.add("postCard");
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
    if (postData.resultType != "profile post" && IsValidBlockchain(postData.blockchain) && IsValidAddress(postData.address, postData.blockchain)) { // makes a post card avatar a clickable link to its authors profile
        avatarDiv.classList.add("clickable");
        avatarDiv.addEventListener("click", () => {
            window.location.replace("/p/" + postData.blockchain + "/" + postData.address);
        });
    }
    avatarImg.classList.add("postCardAvatar");
    avatarImg.crossOrigin = "anonymous";
    avatarImg.referrerPolicy = "no-referrer";
    if (postData.avatarSrc === "" || postData.avatarSrc === null || postData.avatarSrc === undefined) {
        avatarImg.src = "/static/image/avatar.png";
    } else {
        avatarImg.src = XSSSanitizeUrl(postData.avatarSrc);
    }
    postHeaderDiv.classList.add("postCardHeaderDiv");
    postAuthorLink.classList.add("postCardAuthorLink");
    postAuthorLink.href = XSSSanitizeUrl(walletAddressLink);
    postAuthorLink.target = "_blank";
    postAuthor.classList.add("postCardAuthor");
    postAuthor.textContent = postData.author;
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

    // defining each elements' relationship to the others
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
    postDiv.appendChild(embedDiv);
    if ("attachments" in postData) { // attachment handling
        let attachmentDiv = document.createElement("div") as HTMLDivElement;
        attachmentDiv.classList.add("postCardAttachmentDiv");
        let renderedAttachmentElements: HTMLElement[] = [];
        let listedAttachmentElements: HTMLElement[] = [];
        for (let i = 0; i < postData.attachments.length; i ++) { // creates proper element for each attachment
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
                    })
                    if (attachmentCard !instanceof HTMLDivElement) {
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
    postDiv.appendChild(reactionDiv);
    // Embed Rich Media
    const urlRegex = /(https:\/\/[^\s]+)/g;
    let postText = postData.payload;
    const urls = postData.payload.match(urlRegex);
    if (urls) {
        for (const url of urls) {
            const imageEmbed = createImageEmbed(url);
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
        }
    }

    // Post Rendering
    postTextDiv.innerHTML = XSSSanitizeTinyMCEHtml(postText);
    return postDiv;
}
export async function CreateProfileCard (profileData: any): Promise<HTMLDivElement>{
    let profileDiv = document.createElement("div") as HTMLDivElement;
    let avatarDiv = document.createElement("div") as HTMLDivElement; // append to profile div 1st
    let identityDiv = document.createElement("div") as HTMLDivElement; //append to profile div 2nd
    let avatarImg = document.createElement("img") as HTMLImageElement; // append to avatar div
    let nameDiv = document.createElement("div") as HTMLDivElement; // append to identity div 1st
    let addressDiv = document.createElement("div") as HTMLDivElement; // append to identity div 2nd
    let addressInput = document.createElement("input") as HTMLInputElement;
    let descriptionDiv = document.createElement("div") as HTMLDivElement; //append to profile div 3rd
    let profileBlockchain = document.createElement("input") as HTMLInputElement;

    // adding attributes to elements
    profileDiv.classList.add("clickable");
    profileDiv.classList.add("profileCard");
    profileDiv.addEventListener("click", () => {
        window.location.replace("/p/" + profileData.blockchain + "/" + profileData.address);
    });
    avatarDiv.classList.add("profileCardAvatar");
    avatarImg.classList.add("profileCardAvatar");
    avatarImg.crossOrigin = "anonymous";
    avatarImg.referrerPolicy = "no-referrer";
    avatarImg.src = XSSSanitizeUrl(profileData.avatarSrc);
    nameDiv.classList.add("profileCardName");
    nameDiv.textContent = profileData.name;
    addressDiv.classList.add("profileCardAddress");
    addressDiv.textContent = profileData.address;
    addressInput.type = "hidden";
    addressInput.classList.add("profileCardAddressInput");
    addressInput.value = XSSSanitizeValue(profileData.address);
    descriptionDiv.classList.add("profileCardDescription");
    descriptionDiv.textContent = profileData.description;
    profileBlockchain.type = "hidden";
    profileBlockchain.classList.add("profileCardBlockchain");
    profileBlockchain.value = XSSSanitizeValue(profileData.blockchain);

    // defining each element's relationship to the others
    profileDiv.appendChild(profileBlockchain);
    profileDiv.appendChild(addressInput)
    profileDiv.appendChild(avatarDiv);
    profileDiv.appendChild(identityDiv);
    profileDiv.appendChild(descriptionDiv);
    avatarDiv.appendChild(avatarImg);
    identityDiv.classList.add("profileCardIdentity");
    identityDiv.appendChild(nameDiv);
    identityDiv.appendChild(addressDiv);
    return profileDiv
}

// --- Media Embed Functions --- //
function createImageEmbed(url: string): HTMLElement | null {
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
    const youtubeRegex = /^https:\/\/((?:www\.)?youtube\.com\/watch\?v=|youtu\.be\/)([a-zA-Z0-9_-]{11})$/;
    const match = url.match(youtubeRegex);
    if (!match) {
        return null;
    }
    const videoId = match[2];
    const iframe = document.createElement("iframe") as HTMLIFrameElement;
    iframe.classList.add("postCardEmbeddedIframe");
    let embedURL = `https://www.youtube-nocookie.com/embed/${videoId}`;
    iframe.src = XSSSanitizeUrl(embedURL);
    iframe.width = "100%";
    iframe.height = "auto";
    iframe.allow = "encrypted-media; picture-in-picture";
    iframe.allowFullscreen = true;
    iframe.setAttribute("loading", "lazy");
    iframe.setAttribute("credentialless", "");
    return iframe;
}
export async function CreateCarousel(elements: HTMLElement[]): Promise<HTMLDivElement> {// Creates carousel div element when passed an array of any elements
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
async function expandView(event: MouseEvent | PointerEvent) {
    const clickedDiv = event.currentTarget as HTMLDivElement; //Renderable attachments should be wrapped in a loader div
    clickedDiv.classList.add("initiator"); // class added so the expanded attachment carousel knows which image to start on
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
        ShowModalMediaViewer(carousel);
    }
}
export async function CreateImageLoader(image: HTMLImageElement): Promise<HTMLDivElement> { // Adds an image to a div that displays /www/image/imagefail.png if the image fails to load
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
export async function CreateAttachmentPreview(file: File): Promise<HTMLDivElement> {
    const previewDiv = document.createElement("div") as HTMLDivElement;
    const icon = document.createElement("i") as HTMLElement;
    const removeButton = document.createElement("button") as HTMLButtonElement;
    const removeIcon = document.createElement("i") as HTMLElement;
    const fileNameText = document.createElement("span") as HTMLSpanElement;
    const mimeType = file.type;
    const iconType = getFileIcon(mimeType);
    previewDiv.setAttribute("id", XSSSanitizeValue(file.name));
    removeButton.classList.add("removeButton");
    removeIcon.classList.add("bi", "bi-x-lg", "removeIcon");
    icon.classList.add("icon", "attachmentIcon", iconType);
    previewDiv.classList.add("attachmentUploadGridItem");
    fileNameText.classList.add("fileNameSpan");
    fileNameText.textContent = file.name;
    removeButton.appendChild(removeIcon);
    previewDiv.appendChild(removeButton);
    previewDiv.appendChild(icon);
    previewDiv.appendChild(fileNameText);
    return previewDiv;
}
export async function CreateAttachmentCard(attachment: any[]):Promise<HTMLDivElement> {// TODO: File names?
    const attachmentCard = document.createElement("div") as HTMLDivElement;
    const fileIcon = document.createElement("i") as HTMLElement;
    const downloadAnchor = document.createElement("a") as HTMLAnchorElement;
    const downloadButton = document.createElement("button") as HTMLButtonElement;
    const downloadIcon = document.createElement("i") as HTMLElement;
    const fileNameSpan = document.createElement("span") as HTMLSpanElement;
    const fileSizeSpan = document.createElement("span") as HTMLSpanElement;
    const iconClass = getFileIcon(attachment[1]);
    let attachmentURL: string;
    fileIcon.classList.add("icon", "attachmentCardIcon", iconClass);
    if (attachment[0].startsWith("ipfs://")) {
        attachmentURL = CIDToSubdomainURL(attachment[0]);
    } else {
        attachmentURL = attachment[0];
    }
    if (!IsValidURL(attachmentURL)) {
        return Promise.reject("Invalid URL");
    }
    if (attachment[3] !== "") {
        const fileNameBase64 = attachment[3];
        const fileName = base64decode(fileNameBase64);
        fileNameSpan.textContent = XSSSanitizeValue(fileName);
    }
    downloadAnchor.href = XSSSanitizeUrl(attachmentURL);
    downloadAnchor.download = "";
    const fileSize = await formatFileSize(attachment[2]);
    fileSizeSpan.innerText = fileSize;
    downloadButton.classList.add("downloadButton", "btn");
    downloadIcon.classList.add("downloadIcon", "bi", "bi-download");
    attachmentCard.appendChild(fileIcon);
    attachmentCard.appendChild(fileNameSpan);
    attachmentCard.appendChild(fileSizeSpan);
    attachmentCard.appendChild(downloadAnchor);
    downloadAnchor.appendChild(downloadButton);
    downloadButton.appendChild(downloadIcon);
    return attachmentCard;
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