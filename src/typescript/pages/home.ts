window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/home.scss";
import "../components/addPost";
import "../components/menu";
import {CreatePostCard, CreateProfileCard} from "../util/domFactory";
import {HttpGetJson} from "../util/network";
import {IsValidURL, XSSSanitizeUrl} from "../util/security";
import {WalletGetAvatar, WalletGetDescription, WalletGetName, WalletGetAddressFromName, GetAddress, GetChain} from "../util/blockchain/wallet";
import {CIDToSubdomainURL} from "../util/ipfs";
import {ShowNotifications} from "../util/notifications";
import {globalProfileCache, type ProfileData} from "../util/cache";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            searchInput: document.getElementById("searchInput")! as HTMLInputElement,
            searchClearBtn: document.getElementById("searchClearBtn")! as HTMLButtonElement,
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
                    const cached = globalProfileCache.get(key);
                    if (cached === null) {
                        let name: string | null = null;
                        try {
                            name = await WalletGetName(blockchain, address);
                        } catch (e) {
                            console.warn("Failed to get ENS name:", e);
                        }
                        if (name === null || name.length === 0) {
                            let response = await HttpGetJson("/profile/name/" + blockchain + "/" + address);
                            if (response[0] === 200) {
                                if (response[1] && response[1].name && response[1].name.length > 0) {
                                    name = response[1].name;
                                }
                            } else if (response[0] !== 200) {
                                console.warn("Failed to fetch profile name:", response[0], blockchain, address, response[1]);
                            }
                        }
                        let avatarStr: string | null = null;
                        try {
                            avatarStr = await WalletGetAvatar(blockchain, address);
                        } catch (e) {
                            console.warn("Failed to get ENS avatar:", e);
                        }
                        if (!avatarStr || avatarStr === "") {
                            let response = await HttpGetJson("/profile/avatar/" + blockchain + "/" + address);
                            if (response[0] === 200 && response[1] && response[1].avatarAddress) {
                                const avatarAddress = response[1].avatarAddress.trim();
                                if (avatarAddress.length > 0) {
                                    const avatarURL = CIDToSubdomainURL(avatarAddress);
                                    if (IsValidURL(avatarURL)) {
                                        avatarStr = avatarURL;
                                    }
                                }
                            }
                        }
                        globalProfileCache.set(key, {name, avatar: avatarStr || null, description: null, address, blockchain});
                    }
                    const profileData = globalProfileCache.get(key) as ProfileData;
                    // Update all posts for this profile
                    pendingCards.forEach(postDiv => {
                        const postAddress = postDiv.querySelector('.postCardAddress') as HTMLInputElement;
                        const postBlockchain = postDiv.querySelector('.postCardBlockchain') as HTMLInputElement;
                        if (postAddress && postBlockchain) {
                            const postKey = postBlockchain.value + postAddress.value;
                            if (postKey === key) {
                                const authorElement = postDiv.querySelector('.postCardAuthor') as HTMLElement;
                                const avatarElement = postDiv.querySelector('img.postCardAvatar') as HTMLImageElement;
                                if (authorElement) authorElement.textContent = profileData.name || "Unknown";
                                if (avatarElement) {
                                    const defaultPath = "/static/image/avatar.png";
                                    if (profileData.avatar) {
                                        const avatarUrl = XSSSanitizeUrl(profileData.avatar);
                                        avatarElement.onerror = () => {
                                            avatarElement.src = defaultPath;
                                            avatarElement.onerror = null;
                                        };
                                        avatarElement.src = avatarUrl;
                                    } else {
                                        avatarElement.src = defaultPath;
                                    }
                                }
                            }
                        }
                    });
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
            let spinnerDiv = document.createElement("div");
            spinnerDiv.className = "search-spinner-container";
            let spinner = document.createElement("div");
            spinner.classList.add("spinner-border", "text-primary");
            spinner.setAttribute("role", "status");
            let hiddenText = document.createElement("span");
            hiddenText.className = "visually-hidden";
            hiddenText.textContent = "Searching...";
            spinner.appendChild(hiddenText);
            spinnerDiv.appendChild(spinner);
            let visibleText = document.createElement("span");
            visibleText.className = "search-spinner-text";
            visibleText.textContent = "Searching...";
            spinnerDiv.appendChild(visibleText);
            DOM.resultsDiv.appendChild(spinnerDiv);
            const ensSuffixes = [".base.eth"];
            const ensPromises: Promise<any>[] = [];
            if (query.endsWith(".base.eth")) {
                console.log("[DEBUG] Attempting direct ENS resolution for:", query);
                ensPromises.push((async () => {
                    const address = await WalletGetAddressFromName("base", query);
                    if (address) {
                        console.log("[DEBUG] Direct ENS resolution SUCCESS:", query, "->", address);
                        return {
                            resultType: "profile",
                            address: address,
                            blockchain: "base",
                            ensName: query
                        };
                    } else {
                        console.log("[DEBUG] Direct ENS resolution FAILED for:", query);
                    }
                    return null;
                })());
            } else if (!query.includes(".")) {
                ensSuffixes.forEach(suffix => {
                    const ensName = query + suffix;
                    console.log("[DEBUG] Attempting ENS resolution for:", ensName);
                    ensPromises.push((async () => {
                        const address = await WalletGetAddressFromName("base", ensName);
                        if (address) {
                            console.log("[DEBUG] ENS resolution SUCCESS:", ensName, "->", address);
                            return {
                                resultType: "profile",
                                address: address,
                                blockchain: "base",
                                ensName: ensName
                            };
                        } else {
                            console.log("[DEBUG] ENS resolution FAILED for:", ensName);
                        }
                        return null;
                    })());
                });
            }
            const [backendResponse, ...ensResults] = await Promise.all([
                HttpGetJson("/s/?q=" + query),
                ...ensPromises
            ]);
            DOM.resultsDiv.replaceChildren();
            let resp = backendResponse;
            let results: any[] = [];
            if (resp[0] === 200 && resp[1] && resp[1].results !== null) {
                results = resp[1].results;
            } else if (resp[0] !== 200) {
                console.error("Search failed with status:", resp[0], "Response:", resp[1]);
            }
            const ensProfileResults = ensResults.filter(result => result !== null);
            const backendAddresses = new Set(results.map(r => r.address.toLowerCase()));
            for (const ensProfile of ensProfileResults) {
                if (!backendAddresses.has(ensProfile!.address.toLowerCase())) {
                    results.push(ensProfile);
                }
            }
            if (results.length === 0) {
                let noResultsDiv = document.createElement("div");
                noResultsDiv.className = "no-results-dropdown";
                noResultsDiv.textContent = "No results found";
                DOM.resultsDiv.appendChild(noResultsDiv);
                return;
            }
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
                console.log("[DEBUG] Processing result for key:", key, "resultType:", result.resultType, "ensName:", result.ensName);
                const cached = globalProfileCache.get(key);
                const needsDescriptionFetch = cached && result.resultType === "profile" && !cached.description;
                if (cached === null || needsDescriptionFetch) {
                    let name: string | null = null;
                    let avatarStr: string | null = null;

                    if (needsDescriptionFetch) {
                        console.log("[DEBUG] Cached profile missing description, re-fetching for:", cached.address);
                        // Reuse cached name and avatar
                        name = cached.name;
                        avatarStr = cached.avatar;
                    } else {
                        // Fetch name
                        if (result.ensName) {
                            console.log("[DEBUG] Using preserved ENS name from search:", result.ensName);
                            name = result.ensName;
                        } else {
                            console.log("[DEBUG] No preserved ENS name, attempting reverse resolution for:", address);
                            try {
                                name = await WalletGetName(blockchain, address);
                                console.log("[DEBUG] Reverse resolution result:", name);
                            } catch (e) {
                                console.warn("Failed to get ENS name:", e);
                            }
                        }
                        if (name === null || name.length === 0) {
                            let response = await HttpGetJson("/profile/name/" + blockchain + "/" + address);
                            if (response[0] === 200) {
                                if (response[1] && response[1].name && response[1].name.length > 0) {
                                    name = response[1].name;
                                }
                            } else if (response[0] !== 200) {
                                console.warn("Failed to fetch profile name:", response[0], blockchain, address, response[1]);
                            }
                        }
                        // Fetch avatar
                        console.log("[DEBUG] Fetching avatar for:", address, "with name:", name);
                        try {
                            avatarStr = await WalletGetAvatar(blockchain, address);
                            console.log("[DEBUG] Avatar fetch result:", avatarStr ? "found" : "empty");
                        } catch (e) {
                            console.warn("Failed to get ENS avatar:", e);
                        }
                        if (!avatarStr || avatarStr === "") {
                            let response = await HttpGetJson("/profile/avatar/" + blockchain + "/" + address);
                            if (response[0] === 200 && response[1] && response[1].avatarAddress) {
                                const avatarAddress = response[1].avatarAddress.trim();
                                if (avatarAddress.length > 0) {
                                    const avatarURL = CIDToSubdomainURL(avatarAddress);
                                    if (IsValidURL(avatarURL)) {
                                        avatarStr = avatarURL;
                                    }
                                }
                            }
                        }
                    }
                    let description: string | null = null;
                    if (result.resultType == "profile") {
                        console.log("[DEBUG] Fetching description for profile:", address, "with name:", name);
                        let response = await HttpGetJson("/profile/description/" + blockchain + "/" + address);
                        if (response[0] === 200) {
                            if (response[1] && response[1].description && response[1].description.length > 0) {
                                description = response[1].description;
                                console.log("[DEBUG] Database description found:", description ? description.substring(0, 50) + "..." : "empty");
                            }
                        } else if (response[0] !== 200) {
                            console.warn("Failed to fetch profile description:", response[0], blockchain, address, response[1]);
                        }
                        if (!description || description.length === 0) {
                            try {
                                description = await WalletGetDescription(result.blockchain, result.address);
                                console.log("[DEBUG] ENS description fetch result:", description ? description.substring(0, 50) + "..." : "empty");
                            } catch (e) {
                                console.warn("Failed to get ENS description:", e);
                            }
                        }
                    }
                    globalProfileCache.set(key, {name, avatar: avatarStr || null, description, address, blockchain});
                }
                const profileData = globalProfileCache.get(key) as ProfileData;

                pendingCards.forEach(profileDiv => {
                    const profileAddress = profileDiv.querySelector('.profileCardAddressInput') as HTMLInputElement;
                    const profileBlockchain = profileDiv.querySelector('.profileCardBlockchain') as HTMLInputElement;
                    if (profileAddress && profileBlockchain) {
                        const profileKey = profileBlockchain.value + profileAddress.value;
                        if (profileKey === key) {
                            const nameDiv = profileDiv.querySelector('.profileCardName') as HTMLDivElement;
                            const avatarImg = profileDiv.querySelector('img.profileCardAvatar') as HTMLImageElement;
                            const descriptionDiv = profileDiv.querySelector('.profileCardDescription') as HTMLDivElement;
                            if (nameDiv) nameDiv.textContent = profileData.name || "Unknown";
                            if (avatarImg) {
                                const defaultPath = "/static/image/avatar.png";
                                if (profileData.avatar) {
                                    const avatarUrl = XSSSanitizeUrl(profileData.avatar);
                                    avatarImg.onerror = () => {
                                        avatarImg.src = defaultPath;
                                        avatarImg.onerror = null;
                                    };
                                    avatarImg.src = avatarUrl;
                                } else {
                                    avatarImg.src = defaultPath;
                                }
                            }
                            if (descriptionDiv) descriptionDiv.textContent = profileData.description || "";
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
                            if (authorElement) authorElement.textContent = profileData.name || "Unknown";
                            if (avatarElement) {
                                const defaultPath = "/static/image/avatar.png";
                                if (profileData.avatar) {
                                    const avatarUrl = XSSSanitizeUrl(profileData.avatar);
                                    avatarElement.onerror = () => {
                                        avatarElement.src = defaultPath;
                                        avatarElement.onerror = null;
                                    };
                                    avatarElement.src = avatarUrl;
                                } else {
                                    avatarElement.src = defaultPath;
                                }
                            }
                        }
                    }
                });
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
            const hasValue = DOM.searchInput.value.length > 0;
            DOM.searchClearBtn.style.display = hasValue ? "flex" : "none";
            handleSearch().then();
        };
        const debounceHandler = debounce(handleInput, 2000);
        ["keyup", "cut", "paste"].forEach(event => DOM.searchInput.addEventListener(event, debounceHandler, false));
        DOM.searchInput.addEventListener("input", () => {
            const hasValue = DOM.searchInput.value.length > 0;
            DOM.searchClearBtn.style.display = hasValue ? "flex" : "none";
        });
        DOM.searchClearBtn.addEventListener("click", () => {
            DOM.searchInput.value = "";
            DOM.searchClearBtn.style.display = "none";
            DOM.resultsDiv.replaceChildren();
            DOM.resultsDiv.style.display = "none";
            DOM.followersFeedDiv.style.display = "block";
        });

        /* --- Initialize page variables and start loading --- */
        DOM.searchInput.value = "";
        DOM.resultsDiv.style.display = "none"; // Initialize followers feed on page load
        DOM.followersFeedDiv.style.display = "block";
        ShowNotifications(); // Load notifications in background - don't block page loading
        loadFollowersFeed().then();
    }
})();