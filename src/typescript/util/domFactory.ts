import "../../scss/components/postCard.scss";
import "../../scss/components/profileCard.scss";
import "../../scss/components/imageLoader.scss";
import {IsValidAddress, WalletGetExplorerAddressLink, WalletGetExplorerTxLink} from "./blockchain/wallet";
import {IsValidBlockchain, XSSSanitizeUrl, XSSSanitizeValue, XSSSanitizeTinyMCEHtml, HashString} from "./security";
import {CIDToSubdomainURL} from "./ipfs";
import {LogInfo} from "./log";

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
        let attachmentElements: HTMLElement[] = [];
        for (let i = 0; i < postData.attachments.length; i ++) { // creates proper element for each attachment
            let attachment = postData.attachments[i];
            let mimeType = attachment[1];
            let mimeTypePrefix = mimeType.split("/")[0];
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
                    let imageLoader = await CreateImageLoader(image);
                    if (postData.attachments.length === 1) {
                        imageLoader.classList.add("postAttachment");
                        attachmentDiv.appendChild(imageLoader);
                        postDiv.appendChild(attachmentDiv);
                    }else {
                        attachmentElements.push(imageLoader);
                    }
                    break;
                default:
                    LogInfo("unsupported attachment type");
                    break;
            }
        }
        if (postData.attachments.length > 1) {
            let attachmentCarousel = await CreateCarousel(attachmentElements);
            let carouselInnerDiv = attachmentCarousel.children[1] as HTMLDivElement;
            carouselInnerDiv.classList.add("postAttachment");
            attachmentDiv.appendChild(attachmentCarousel);
            postDiv.appendChild(attachmentDiv);
        }
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
export async function CreateCarousel(elements: HTMLElement[]): Promise<HTMLDivElement> { // Creates carousel div element when passed an array of any elements
    let carouselDiv = document.createElement("div") as HTMLDivElement;
    let carouselList = document.createElement("ol") as HTMLOListElement;
    let carouselInnerDiv = document.createElement("div") as HTMLDivElement;
    let previousButton = document.createElement("a") as HTMLAnchorElement;
    let previousIcon = document.createElement("span") as HTMLSpanElement;
    let nextButton = document.createElement("a") as HTMLAnchorElement;
    let nextIcon = document.createElement("span") as HTMLSpanElement;
    const elementsUUID = crypto.randomUUID();
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
        previousButton.appendChild(previousIcon);
        nextButton.appendChild(nextIcon);
        carouselList.appendChild(selector);
        carouselInnerDiv.appendChild(item);
    }
    carouselDiv.appendChild(carouselList);
    carouselDiv.appendChild(carouselInnerDiv);
    carouselDiv.appendChild(previousButton);
    carouselDiv.appendChild(nextButton);
    return carouselDiv;
}
export async function CreateImageLoader(image: HTMLImageElement): Promise<HTMLDivElement> { // Adds an image to a div that displays /www/image/imagefail.png if the image fails to load
    const imageLoader = document.createElement("div") as HTMLDivElement;
    imageLoader.classList.add("image-container");
    const spinner = document.createElement("div") as HTMLDivElement;
    spinner.classList.add("spinner-border", "text-primary", "spinner-div");
    spinner.setAttribute("role", "status");
    imageLoader.style.paddingTop = "56.25%"
    image.style.opacity = "0";
    image.style.width = "100%";
    image.style.height = "auto";
    image.style.display = "block";
    image.style.objectFit = "contain";
    image.style.borderRadius = "inherit";
    imageLoader.appendChild(spinner);
    imageLoader.appendChild(image);
    const timeout = window.setTimeout(() => {
        if (!image.complete) {
            spinner.remove();
            image.style.opacity = "1";
            image.src = "/static/image/imagefail.png";
        }
    }, 10000);
    image.onload = () => {
        clearTimeout(timeout);
        spinner.remove();
        image.style.opacity = "1";
        imageLoader.style.removeProperty("padding-top");
    };
    image.onerror = () => {
        clearTimeout(timeout);
        spinner.remove();
        image.src = "/static/image/imagefail.png";
        image.style.opacity = "1";
        imageLoader.style.removeProperty("padding-top");

    }
    return imageLoader
}