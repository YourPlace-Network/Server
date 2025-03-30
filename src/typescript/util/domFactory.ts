import "../../scss/components/postCard.scss";
import "../../scss/components/profileCard.scss";
import {IsValidAddress, WalletGetExplorerAddressLink, WalletGetExplorerTxLink} from "./blockchain/wallet";
import {IsValidBlockchain, XSSSanitizeUrl, XSSSanitizeValue, XSSSanitizeTinyMCEHtml} from "./security";
import {CIDToSubdomainURL} from "./ipfs";
import bootstrap from "bootstrap";

export async function CreatePostCard(postData: any): Promise<HTMLDivElement> {// returns a post div element when given a post's data set profile to true if calling from a users profile
    function hasAttachments(): boolean {
        return "attachments" in postData;
    }
    function createAttachmentCarousel(): HTMLDivElement {
        let carouselDiv = document.createElement("div") as HTMLDivElement;
        carouselDiv.classList.add("carousel", "slide");
        carouselDiv.id = postData.txHash;
        let carouselList = document.createElement("ol") as HTMLOListElement;
        carouselList.classList.add("carousel-indicators");
        let carouselInnerDiv = document.createElement("div") as HTMLDivElement;
        carouselInnerDiv.classList.add("carousel-inner");
        let previousButton = document.createElement("a") as HTMLAnchorElement;
        previousButton.classList.add("carousel-control-prev");
        previousButton.href = "#" + postData.txHash;
        previousButton.role = "button";
        previousButton.setAttribute("data-bs-slide", "prev");
        let previousIcon = document.createElement("span") as HTMLSpanElement;
        previousIcon.classList.add("carousel-control-prev-icon");
        previousIcon.ariaHidden = "true";
        previousButton.appendChild(previousIcon);
        let nextButton = document.createElement("a") as HTMLAnchorElement;
        nextButton.classList.add("carousel-control-next");
        nextButton.href = "#" + postData.txHash;
        nextButton.role = "button";
        nextButton.setAttribute("data-bs-slide", "next");
        let nextIcon = document.createElement("span") as HTMLSpanElement;
        nextIcon.classList.add("carousel-control-next-icon");
        nextIcon.ariaHidden = "true";
        nextButton.appendChild(nextIcon);
        for (let i = 0; i < postData.attachments.length; i++) {
            let attachment = postData.attachments[i];
            let fileUrl = attachment[0];
            let contentType = attachment[1];
            let selector = document.createElement("li") as HTMLLIElement;
            let item = document.createElement("div") as HTMLDivElement;
            if (i == 0) {
                selector.classList.add("active");
                item.classList.add("active");
            }
            selector.setAttribute("data-bs-target", "#" + postData.txHash);
            selector.setAttribute("data-bs-slide-to", i.toString());
            item.classList.add("carousel-item")
            let contentTypePrefix = contentType.split("/")[0];
            switch (contentTypePrefix) {
                case "image":
                    if (fileUrl.startsWith("ipfs://")) {
                        fileUrl = CIDToSubdomainURL(fileUrl);
                    }
                    let image = document.createElement("img") as HTMLImageElement;
                    image.classList.add("d-block");
                    image.classList.add("w-100");
                    image.src = fileUrl;
                    item.appendChild(image);
            }
            carouselList.appendChild(selector);
            carouselInnerDiv.appendChild(item);
        }
        carouselDiv.appendChild(carouselList);
        carouselDiv.appendChild(carouselInnerDiv);
        carouselDiv.appendChild(previousButton);
        carouselDiv.appendChild(nextButton);
        return carouselDiv;
    }
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
    postID.value = XSSSanitizeValue(postData.txHash);
    postBlockchain.type = "hidden";
    postBlockchain.classList.add("postCardBlockchain");
    postBlockchain.value = XSSSanitizeValue(postData.blockchain);
    postAddress.type = "hidden";
    postAddress.classList.add("postCardAddress");
    postAddress.value = XSSSanitizeValue(postData.address);
    avatarDiv.classList.add("postCardAvatar");
    let blockchain = postData.blockchain;
    let address = postData.address;
    if (postData.resultType != "profile post" && IsValidBlockchain(blockchain) && IsValidAddress(address, blockchain)) {
        avatarDiv.classList.add("clickable");
        avatarDiv.addEventListener('click', () => {
            window.location.replace("/p/" + blockchain + "/" + address);
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
    if (hasAttachments()) {
        let attachmentDiv = createAttachmentCarousel();
        postDiv.appendChild(attachmentDiv);
        setTimeout(() => {
            const carouselElement = document.getElementById(postData.txHash);
            if (carouselElement) {
                try {
                    new bootstrap.Carousel(carouselElement);
                } catch (e) {
                    console.error("Error initializing carousel:", e);
                }
            }
        }, 0);
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
    let addressDiv = document.createElement("div") as HTMLDivElement;// append to identity div 2nd
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
