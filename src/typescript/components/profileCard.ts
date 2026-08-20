import "../../scss/components/profileCard.scss";
import {processTextWithTags} from "../util/domFactory";
import {ApplyIpfsImageLoadPolicy, CIDToSubdomainURL, getIpfsAvatarUrl} from "../util/ipfs";
import {XSSSanitizeUrl, XSSSanitizeValue} from "../util/security";
import {WalletGetAvatar, WalletGetDescription, WalletGetName} from "../util/blockchain/wallet";

export async function CreateProfileCard(profileData: any): Promise<HTMLAnchorElement> {
    let profileDiv = document.createElement("a") as HTMLAnchorElement;
    let avatarDiv = document.createElement("div") as HTMLDivElement;
    let identityDiv = document.createElement("div") as HTMLDivElement;
    let avatarImg = document.createElement("img") as HTMLImageElement;
    let nameDiv = document.createElement("div") as HTMLDivElement;
    let addressDiv = document.createElement("div") as HTMLDivElement;
    let addressInput = document.createElement("input") as HTMLInputElement;
    let descriptionDiv = document.createElement("div") as HTMLDivElement;
    let profileBlockchain = document.createElement("input") as HTMLInputElement;
    profileDiv.classList.add("clickable");
    profileDiv.classList.add("profileCard");
    profileDiv.href = "/p/" + encodeURIComponent(profileData.blockchain) + "/" + encodeURIComponent(profileData.address);
    avatarDiv.classList.add("profileCardAvatar");
    avatarImg.classList.add("profileCardAvatar");
    avatarImg.crossOrigin = "anonymous";
    avatarImg.referrerPolicy = "no-referrer";
    let profileAvatarSrc = profileData.avatarSrc;
    if (profileAvatarSrc && profileAvatarSrc.startsWith("ipfs://")) {
        const converted = CIDToSubdomainURL(profileAvatarSrc);
        profileAvatarSrc = converted || "/static/image/avatar.svg";
    }
    const profileAvatarUrl = XSSSanitizeUrl(profileAvatarSrc || "/static/image/avatar.svg");
    ApplyIpfsImageLoadPolicy(avatarImg, profileAvatarUrl);
    avatarImg.src = profileAvatarUrl;
    nameDiv.classList.add("profileCardName");
    nameDiv.textContent = profileData.name || "Anonymous";
    addressDiv.classList.add("profileCardAddress");
    const addr = profileData.address;
    addressDiv.textContent = addr.length > 12 ? addr.slice(0, 6) + "..." + addr.slice(-4) : addr;
    addressInput.type = "hidden";
    addressInput.classList.add("profileCardAddressInput");
    addressInput.value = XSSSanitizeValue(profileData.address);
    descriptionDiv.classList.add("profileCardDescription");
    descriptionDiv.textContent = profileData.description || "";
    processTextWithTags(descriptionDiv);
    profileBlockchain.type = "hidden";
    profileBlockchain.classList.add("profileCardBlockchain");
    profileBlockchain.value = XSSSanitizeValue(profileData.blockchain);
    profileDiv.appendChild(profileBlockchain);
    profileDiv.appendChild(addressInput)
    profileDiv.appendChild(avatarDiv);
    profileDiv.appendChild(identityDiv);
    profileDiv.appendChild(descriptionDiv);
    avatarDiv.appendChild(avatarImg);
    identityDiv.classList.add("profileCardIdentity");
    identityDiv.appendChild(nameDiv);
    identityDiv.appendChild(addressDiv);
    return profileDiv;
}
export async function FetchAndUpdateProfileCard(profileCard: HTMLElement, blockchain: string, address: string) {
    let name: string | null = await WalletGetName(blockchain, address);
    let avatarStr: string | null = await getIpfsAvatarUrl(blockchain, address);
    if (!avatarStr || avatarStr === "") {
        avatarStr = await WalletGetAvatar(blockchain, address);
    }
    let description: string | null = await WalletGetDescription(blockchain, address);
    const nameDiv = profileCard.querySelector('.profileCardName') as HTMLDivElement;
    const avatarImg = profileCard.querySelector('img.profileCardAvatar') as HTMLImageElement;
    const descriptionDiv = profileCard.querySelector('.profileCardDescription') as HTMLDivElement;
    if (nameDiv) nameDiv.textContent = name || "Anonymous";
    if (descriptionDiv) descriptionDiv.textContent = description || "";
    if (avatarImg) {
        const defaultPath = "/static/image/avatar.svg";
        if (avatarStr) {
            let avatarSrc = avatarStr;
            if (avatarSrc.startsWith("ipfs://")) {
                avatarSrc = CIDToSubdomainURL(avatarSrc) || defaultPath;
            }
            const avatarUrl = XSSSanitizeUrl(avatarSrc);
            avatarImg.onerror = () => {
                avatarImg.src = defaultPath;
                avatarImg.onerror = null;
            };
            ApplyIpfsImageLoadPolicy(avatarImg, avatarUrl);
            avatarImg.src = avatarUrl;
        } else {
            avatarImg.src = defaultPath;
        }
    }
}
