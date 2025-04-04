import {CIDToSubdomainURL} from "../util/ipfs";

window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/home.scss";
import "../components/addPost";
import "../components/menu";
import {CreatePostCard, CreateProfileCard} from "../util/domFactory";
import {HttpGetJson} from "../util/network";
import {IsValidHttpUrl, XSSSanitizeUrl} from "../util/security";
import {WalletGetAvatar, WalletGetDescription, WalletGetName} from "../util/blockchain/wallet";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            searchInput: document.getElementById("searchInput")! as HTMLInputElement,
            resultsDiv: document.getElementById("resultsDiv")! as HTMLDivElement,
        }
        type user = { // address is the map key
            blockchain: string,
            avatar: string,
            name: string,
        }

        async function handleSearch() {
            DOM.resultsDiv.replaceChildren();
            let query = DOM.searchInput.value;
            if (query.length <= 0) {
                return;
            }
            let resp = await HttpGetJson("/s/?q=" + query);
            if (resp[0] !== 200 || resp[1].results === null) {
                return;
            }

            let results: any[] = resp[1].results;
            interface Profile {
                name: string | null;
                avatar: URL | null;
            }
            const ProfileCache: Record<string, Profile> = {};
            const pendingCards: HTMLDivElement[] = [];

            // Create all cards with placeholder data first
            for (let i = 0; i < results.length; i++) {
                if (results[i].resultType == "post") {
                    results[i].author = "Loading...";
                    results[i].avatarSrc = "/static/image/avatar.png";
                    let postDiv = await CreatePostCard(results[i]);
                    if (i % 2 === 0) {
                        postDiv.classList.add("shaded");
                    }
                    pendingCards.push(postDiv);
                    DOM.resultsDiv.appendChild(postDiv);
                } else if (results[i].resultType == "profile") {
                    results[i].name = "loading...";
                    results[i].avatarSrc = "/static/image/avatar.png";
                    let profileDiv = await CreateProfileCard(results[i]);
                    if (i % 2 === 0) {
                        profileDiv.classList.add("shaded");
                    }
                    pendingCards.push(profileDiv);
                    DOM.resultsDiv.appendChild(profileDiv);
                }
            }
            // Fetch profile data once per unique profile
            const profilePromises = results.map(async result => {
                let blockchain = result.blockchain;
                let address = result.address;
                let key = blockchain + address;
                if (!(key in ProfileCache)) {
                    let name = await WalletGetName(blockchain, address);
                    if (name === null || name.length === 0) {
                        let response = await HttpGetJson("/profile/name/" + blockchain + "/" + address);
                        if (response[0] === 200) {
                            if (response[1] && response[1].name.length > 0) {
                                name = response[1].name;
                            }
                        }
                    }
                    let avatar: URL | null = new URL(await WalletGetAvatar(blockchain, address));
                    if (avatar === null) {
                        let response = await HttpGetJson("/profile/avatar/" + blockchain + "/" + address);
                        if (response[0] === 200 && response[1] && response[1].avatarAddress) {
                            const avatarAddress = response[1].avatarAddress.trim();
                            if (avatarAddress.length > 0) {
                                const avatarURL = CIDToSubdomainURL(avatarAddress);
                                if (IsValidHttpUrl(avatarURL)) {
                                    avatar = new URL(avatarURL);
                                } else {
                                    avatar = null;
                                }
                            } else {
                                avatar = null;
                            }
                        } else {
                            avatar = null;
                        }
                    }
                    let description: string | null = null;
                    if (result.type == "profile") {
                        description = await WalletGetDescription(result.blockchain, result.address);
                        if (description === null || description.length === 0) {
                            let response = await HttpGetJson("/profile/description/" + blockchain + "/" + address);
                            if (response[0] === 200) {
                                if (response[1] && response[1].description.length > 0) {
                                    description = response[1].description;
                                }

                            }
                        }
                    }
                    ProfileCache[key] = {name, avatar};

                    pendingCards.forEach(profileDiv => {
                        const profileAddress = profileDiv.querySelector('.profileCardAddressInput') as HTMLInputElement;
                        const profileBlockchain = profileDiv.querySelector('.profileCardBlockchain') as HTMLInputElement;
                        if (profileAddress && profileBlockchain) {
                            const profileKey = profileBlockchain.value + profileAddress.value;
                            if (profileKey === key) {
                                const nameDiv = profileDiv.querySelector('.profileCardName') as HTMLDivElement;
                                const avatarImg = profileDiv.querySelector('img.profileCardAvatar') as HTMLImageElement;
                                const descriptionDiv = profileDiv.querySelector('.profileCardDescription') as HTMLDivElement;
                                if (nameDiv) nameDiv.textContent = name || "Unknown";
                                if (avatarImg) {
                                    const defaultPath = "/static/image/avatar.png";
                                    avatarImg.src = avatar ? XSSSanitizeUrl(avatar.toString()) : defaultPath;
                                }
                                if (descriptionDiv) descriptionDiv.textContent = description || "";
                            }
                        }
                    });
                    // Update all posts for this profile
                    pendingCards.forEach(postDiv => {
                        const postAddress = postDiv.querySelector('.postCardAddress') as HTMLInputElement;
                        const postBlockchain = postDiv.querySelector('.postCardBlockchain') as HTMLInputElement;
                        if (postAddress && postBlockchain) {
                            const postKey = postBlockchain.value + postAddress.value;
                            if (postKey === key) {
                                const authorElement = postDiv.querySelector('.postCardAuthor') as HTMLElement;
                                const avatarElement = postDiv.querySelector('img.postCardAvatar') as HTMLImageElement;
                                if (authorElement) authorElement.textContent = name || "Unknown";
                                if (avatarElement) {
                                    const defaultPath = "/static/image/avatar.png";
                                    avatarElement.src = avatar ? XSSSanitizeUrl(avatar.toString()) : defaultPath;
                                }
                            }
                        }
                    });
                }
            });

            await Promise.all(profilePromises);
        }
        function debounce<T extends (...args: any[]) => void>(func: T, delay: number): (...args: Parameters<T>) => void {
            let timeoutId: number;
            return (...args: Parameters<T>) => {
                window.clearTimeout(timeoutId);
                timeoutId = window.setTimeout(() => {
                    func(...args);
                }, delay);
            };
        }
        const handleInput = (event: Event) => {
            handleSearch().then();
        };
        const debounceHandler = debounce(handleInput, 2000);

        ["keyup", "cut", "paste"].forEach(event => DOM.searchInput.addEventListener(event, debounceHandler, false));

        DOM.searchInput.value = "";
    }
})();