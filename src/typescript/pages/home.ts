window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/home.scss";
import "../components/addPost";
import "../components/menu";
import {CreatePostCard, CreateProfileCard} from "../util/domFactory";
import {HttpGetJson} from "../util/network";
import {IsValidURL, XSSSanitizeUrl} from "../util/security";
import {WalletGetAvatar, WalletGetDescription, WalletGetName, GetAddress, GetChain} from "../util/blockchain/wallet";
import {CIDToSubdomainURL} from "../util/ipfs";
import {LogError} from "../util/log";
import {GetNotifications} from "../components/toast";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            searchInput: document.getElementById("searchInput")! as HTMLInputElement,
            resultsDiv: document.getElementById("resultsDiv")! as HTMLDivElement,
            followersFeedDiv: document.getElementById("followersFeedDiv")! as HTMLDivElement,
            isCookieAuthenticated: document.getElementById("isCookieAuthenticated")! as HTMLInputElement,
        }
        type user = { // address is the map key
            blockchain: string,
            avatar: string,
            name: string,
        }

        async function loadFollowersFeed() {
            if (DOM.isCookieAuthenticated.value !== "true") {
                return;
            }
            
            try {
                interface Profile {
                    name: string | null;
                    avatar: URL | null;
                }
                // Get current user's address and blockchain from wallet functions
                const userAddress = GetAddress();
                const userBlockchain = GetChain();
                if (!userAddress || !userBlockchain) {
                    return;
                }
                DOM.followersFeedDiv.replaceChildren();
                let resp = await HttpGetJson(`/feed/${userBlockchain}/${userAddress}?limit=20`);
                if (resp[0] !== 200 || !resp[1].posts) {
                    return;
                }
                let posts: any[] = resp[1].posts;
                const ProfileCache: Record<string, Profile> = {};
                const pendingCards: HTMLDivElement[] = [];
                // Create all cards with placeholder data first
                for (let i = 0; i < posts.length; i++) {
                    posts[i].author = "Loading...";
                    posts[i].avatarSrc = "/static/image/avatar.png";
                    let postDiv = await CreatePostCard(posts[i]);
                    if (i % 2 === 0) {
                        postDiv.classList.add("shaded");
                    }
                    pendingCards.push(postDiv);
                    DOM.followersFeedDiv.appendChild(postDiv);
                }
                // Fetch profile data for each unique author
                const profilePromises = posts.map(async post => {
                    let blockchain = post.blockchain;
                    let address = post.address;
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
                                    if (IsValidURL(avatarURL)) {
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
                        ProfileCache[key] = {name, avatar};
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
            } catch (error) {
                console.error("Error loading followers feed:", error);
            }
        }
        async function handleSearch() {
            DOM.resultsDiv.replaceChildren();
            DOM.followersFeedDiv.style.display = "none";
            let query = DOM.searchInput.value;
            if (query.length <= 0) {
                DOM.followersFeedDiv.style.display = "block";
                return;
            }
            DOM.resultsDiv.style.display = "block";
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
                                if (IsValidURL(avatarURL)) {
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

        /* --- Searching debounce functions --- */
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

        /* --- Initialize page variables and start loading --- */
        DOM.searchInput.value = "";
        DOM.resultsDiv.style.display = "none"; // Initialize followers feed on page load
        DOM.followersFeedDiv.style.display = "block";
        GetNotifications().catch(error => LogError("Notification loading failed: " + error)); // Load notifications in background - don't block page loading
        loadFollowersFeed().then();
    }
})();