import "../../scss/components/postCard.scss";
import "../../scss/components/profileCard.scss";
import {IsValidAddress, WalletGetExplorerAddressLink, WalletGetExplorerTxLink} from "./blockchain/wallet";
import {IsValidBlockchain, XSSSanitizeUrl, XSSSanitizeValue, XSSSanitizeTinyMCEHtml} from "./security";

export async function CreatePostCard(postData: any): Promise<HTMLDivElement> { // returns a post div element when given a post's data set profile to true if calling from a users profile
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
    postDiv.appendChild(reactionDiv);

    // embed media
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
    postTextDiv.innerHTML = XSSSanitizeTinyMCEHtml(postText);
    //postTextDiv.innerHTML = postText; // todo debug
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
