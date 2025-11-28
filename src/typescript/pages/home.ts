window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/home.scss";
import "../components/addPost";
import {preloadTinyMCE} from "../components/addPost";
import "../components/menu";
import {CreatePostCard, CreateProfileCard} from "../util/domFactory";
import {HttpGetJson} from "../util/network";
import {XSSSanitizeUrl} from "../util/security";
import {WalletGetAvatar, WalletGetDescription, WalletGetName, WalletGetAddressFromName, GetAddress, GetChain} from "../util/blockchain/wallet";
import {CIDToSubdomainURL, getIpfsAvatarUrl} from "../util/ipfs";
import {IsGatewayMode} from "../util/miscellaneous";
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
            followersFeedSection: document.getElementById("followersFeedSection")! as HTMLDivElement,
            feedTopBtn: document.getElementById("feedTopBtn")! as HTMLButtonElement,
            isCookieAuthenticated: document.getElementById("isCookieAuthenticated")! as HTMLInputElement,
            discoverSection: document.getElementById("discoverSection")! as HTMLDivElement,
            discoverRandomRow: document.getElementById("discoverRandomRow")! as HTMLDivElement,
            discoverFollowersRow: document.getElementById("discoverFollowersRow")! as HTMLDivElement,
            discoverPostsRow: document.getElementById("discoverPostsRow")! as HTMLDivElement,
        }
        const FEED_PAGE_SIZE = 25;
        let feedOffset = 0;
        let feedLoading = false;
        let feedHasMore = true;
        let feedNewestTimestamp: number | null = null;
        const loadedTxHashes = new Set<string>();
        type user = { // address is the map key
            blockchain: string,
            avatar: string,
            name: string,
        }

        async function loadFollowersFeed(mode: "initial" | "more" | "refresh" = "initial") {
            if (DOM.isCookieAuthenticated.value !== "true") {
                DOM.followersFeedSection.style.display = "none";
                return;
            }
            if (feedLoading) return;
            if (mode === "more" && !feedHasMore) return;
            feedLoading = true;
            try {
                const userAddress = GetAddress();
                const userBlockchain = GetChain();
                if (!userAddress || !userBlockchain) {
                    DOM.followersFeedSection.style.display = "none";
                    return;
                }
                if (mode === "initial") {
                    DOM.followersFeedDiv.replaceChildren();
                    feedOffset = 0;
                    feedHasMore = true;
                    feedNewestTimestamp = null;
                    loadedTxHashes.clear();
                }
                const requestOffset = mode === "refresh" ? 0 : feedOffset;
                let resp = await HttpGetJson(`/feed/${userBlockchain}/${userAddress}?limit=${FEED_PAGE_SIZE + 1}&offset=${requestOffset}`);
                if (resp[0] !== 200 || !resp[1].posts) {
                    return;
                }
                let posts: any[] = resp[1].posts;
                // Check if there are more posts (we requested one extra)
                if (mode !== "refresh") {
                    feedHasMore = posts.length > FEED_PAGE_SIZE;
                    if (feedHasMore) {
                        posts = posts.slice(0, FEED_PAGE_SIZE);
                    }
                    feedOffset += posts.length;
                }
                // Filter out already loaded posts
                posts = posts.filter(post => !loadedTxHashes.has(post.txHash));
                if (posts.length === 0) {
                    return;
                }
                // Track loaded posts and update newest timestamp
                posts.forEach(post => {
                    loadedTxHashes.add(post.txHash);
                    if (feedNewestTimestamp === null || post.timestamp > feedNewestTimestamp) {
                        feedNewestTimestamp = post.timestamp;
                    }
                });
                const pendingCards: HTMLDivElement[] = [];
                // Create all cards with placeholder data
                for (let i = 0; i < posts.length; i++) {
                    posts[i].author = "Loading...";
                    posts[i].avatarSrc = "/static/image/avatar.png";
                    let postDiv = await CreatePostCard(posts[i]);
                    pendingCards.push(postDiv);
                }
                // Add cards to DOM in correct order
                if (mode === "refresh") {
                    // Insert new posts at top in correct order (newest first)
                    const firstChild = DOM.followersFeedDiv.firstChild;
                    for (let i = pendingCards.length - 1; i >= 0; i--) {
                        DOM.followersFeedDiv.insertBefore(pendingCards[i], firstChild);
                    }
                } else {
                    for (const card of pendingCards) {
                        DOM.followersFeedDiv.appendChild(card);
                    }
                }
                // Update alternating stripes based on DOM position
                const feedChildren = DOM.followersFeedDiv.children;
                for (let i = 0; i < feedChildren.length; i++) {
                    if (i % 2 === 0) {
                        feedChildren[i].classList.add("shaded");
                    } else {
                        feedChildren[i].classList.remove("shaded");
                    }
                }
                // Update top button state
                DOM.feedTopBtn.disabled = DOM.followersFeedDiv.children.length === 0;
                // Fetch profile data for each unique author
                const profilePromises = posts.map(async post => {
                    let blockchain = post.blockchain;
                    let address = post.address;
                    let key = blockchain + address;
                    let cached = globalProfileCache.get(key);
                    // Invalidate cache if avatar is missing or stale ipfs:// URL
                    if (cached !== null) {
                        const cachedAvatar = (cached as ProfileData).avatar;
                        if (!cachedAvatar || cachedAvatar === "" || cachedAvatar.startsWith("ipfs://")) {
                            cached = null;
                        }
                    }
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
                        if (IsGatewayMode()) {
                            avatarStr = await getIpfsAvatarUrl(blockchain, address);
                        } else {
                            try {
                                avatarStr = await WalletGetAvatar(blockchain, address);
                            } catch (e) {
                                console.warn("Failed to get ENS avatar:", e);
                            }
                            if (!avatarStr || avatarStr === "") {
                                avatarStr = await getIpfsAvatarUrl(blockchain, address);
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
                                if (authorElement) authorElement.textContent = profileData.name || "Anonymous";
                                if (avatarElement) {
                                    const defaultPath = "/static/image/avatar.png";
                                    if (profileData.avatar) {
                                        let avatarSrc = profileData.avatar;
                                        if (avatarSrc.startsWith("ipfs://")) {
                                            avatarSrc = CIDToSubdomainURL(avatarSrc) || defaultPath;
                                        }
                                        const avatarUrl = XSSSanitizeUrl(avatarSrc);
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
            } finally {
                feedLoading = false;
            }
        }
        async function loadDiscoverProfiles() {
            try {
                let resp = await HttpGetJson("/discover");
                if (resp[0] !== 200) {
                    console.error("Failed to load discover profiles:", resp[0], resp[1]);
                    return;
                }
                const data = resp[1];
                await populateDiscoverRow(DOM.discoverRandomRow, data.random || []);
                await populateDiscoverRow(DOM.discoverFollowersRow, data.byFollowers || []);
                await populateDiscoverRow(DOM.discoverPostsRow, data.byPosts || []);
            } catch (error) {
                console.error("Error loading discover profiles:", error);
            }
        }
        async function populateDiscoverRow(rowElement: HTMLDivElement, profiles: any[]) {
            const columns = rowElement.querySelectorAll(".discoverCol");
            for (let i = 0; i < Math.min(profiles.length, columns.length); i++) {
                const profile = profiles[i];
                const col = columns[i] as HTMLDivElement;
                col.innerHTML = "";
                profile.name = "Loading...";
                profile.avatarSrc = "/static/image/avatar.png";
                profile.description = "";
                const profileCard = await CreateProfileCard(profile);
                col.appendChild(profileCard);
                fetchAndUpdateProfileCard(profileCard, profile.blockchain, profile.address);
            }
        }
        async function fetchAndUpdateProfileCard(profileCard: HTMLDivElement, blockchain: string, address: string) {
            const key = blockchain + address;
            let cached = globalProfileCache.get(key);
            // Invalidate cache if avatar is missing or stale ipfs:// URL
            if (cached !== null) {
                const cachedAvatar = (cached as ProfileData).avatar;
                if (!cachedAvatar || cachedAvatar === "" || cachedAvatar.startsWith("ipfs://")) {
                    cached = null;
                }
            }
            if (cached === null) {
                let name: string | null = null;
                try {
                    name = await WalletGetName(blockchain, address);
                } catch (e) {
                    console.warn("Failed to get ENS name:", e);
                }
                if (name === null || name.length === 0) {
                    let response = await HttpGetJson("/profile/name/" + blockchain + "/" + address);
                    if (response[0] === 200 && response[1] && response[1].name && response[1].name.length > 0) {
                        name = response[1].name;
                    }
                }
                let avatarStr: string | null = null;
                if (IsGatewayMode()) {
                    avatarStr = await getIpfsAvatarUrl(blockchain, address);
                } else {
                    try {
                        avatarStr = await WalletGetAvatar(blockchain, address);
                    } catch (e) {
                        console.warn("Failed to get ENS avatar:", e);
                    }
                    if (!avatarStr || avatarStr === "") {
                        avatarStr = await getIpfsAvatarUrl(blockchain, address);
                    }
                }
                globalProfileCache.set(key, {name, avatar: avatarStr || null, description: null, address, blockchain});
                cached = globalProfileCache.get(key);
            }
            const profileData = cached as ProfileData;
            const nameDiv = profileCard.querySelector('.profileCardName') as HTMLDivElement;
            const avatarImg = profileCard.querySelector('img.profileCardAvatar') as HTMLImageElement;
            if (nameDiv) nameDiv.textContent = profileData.name || "Anonymous";
            if (avatarImg) {
                const defaultPath = "/static/image/avatar.png";
                if (profileData.avatar) {
                    let avatarSrc = profileData.avatar;
                    if (avatarSrc.startsWith("ipfs://")) {
                        avatarSrc = CIDToSubdomainURL(avatarSrc) || defaultPath;
                    }
                    const avatarUrl = XSSSanitizeUrl(avatarSrc);
                    avatarImg.onerror = () => {
                        avatarImg.src = defaultPath;
                        avatarImg.onerror = null;
                    };
                    avatarImg.src = avatarUrl;
                } else {
                    avatarImg.src = defaultPath;
                }
            }
        }
        async function handleSearch() {
            DOM.resultsDiv.replaceChildren();
            DOM.followersFeedSection.style.display = "none";
            DOM.discoverSection.style.display = "none";
            let query = DOM.searchInput.value;
            if (query.length <= 0) {
                DOM.followersFeedSection.style.display = "block";
                DOM.discoverSection.style.display = "block";
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
                const normalizedQuery = query.toLowerCase();
                console.log("[DEBUG] Attempting direct ENS resolution for:", normalizedQuery);
                ensPromises.push((async () => {
                    const address = await WalletGetAddressFromName("base", normalizedQuery);
                    if (address) {
                        console.log("[DEBUG] Direct ENS resolution SUCCESS:", normalizedQuery, "->", address);
                        return {
                            resultType: "profile",
                            address: address,
                            blockchain: "base",
                            ensName: normalizedQuery
                        };
                    } else {
                        console.log("[DEBUG] Direct ENS resolution FAILED for:", normalizedQuery);
                    }
                    return null;
                })());
            } else if (!query.includes(".")) {
                ensSuffixes.forEach(suffix => {
                    const ensName = query.toLowerCase() + suffix;
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
                let cached = globalProfileCache.get(key);
                // Invalidate cache if avatar is missing or stale ipfs:// URL
                if (cached !== null) {
                    const cachedAvatar = (cached as ProfileData).avatar;
                    if (!cachedAvatar || cachedAvatar === "" || cachedAvatar.startsWith("ipfs://")) {
                        cached = null;
                    }
                }
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
                        if (IsGatewayMode()) {
                            avatarStr = await getIpfsAvatarUrl(blockchain, address);
                        } else {
                            try {
                                avatarStr = await WalletGetAvatar(blockchain, address);
                                console.log("[DEBUG] Avatar fetch result:", avatarStr ? "found" : "empty");
                            } catch (e) {
                                console.warn("Failed to get ENS avatar:", e);
                            }
                            if (!avatarStr || avatarStr === "") {
                                avatarStr = await getIpfsAvatarUrl(blockchain, address);
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
                            if (nameDiv) nameDiv.textContent = profileData.name || "Anonymous";
                            if (avatarImg) {
                                const defaultPath = "/static/image/avatar.png";
                                if (profileData.avatar) {
                                    let avatarSrc = profileData.avatar;
                                    if (avatarSrc.startsWith("ipfs://")) {
                                        avatarSrc = CIDToSubdomainURL(avatarSrc) || defaultPath;
                                    }
                                    const avatarUrl = XSSSanitizeUrl(avatarSrc);
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
                            if (authorElement) authorElement.textContent = profileData.name || "Anonymous";
                            if (avatarElement) {
                                const defaultPath = "/static/image/avatar.png";
                                if (profileData.avatar) {
                                    let avatarSrc = profileData.avatar;
                                    if (avatarSrc.startsWith("ipfs://")) {
                                        avatarSrc = CIDToSubdomainURL(avatarSrc) || defaultPath;
                                    }
                                    const avatarUrl = XSSSanitizeUrl(avatarSrc);
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
            DOM.followersFeedSection.style.display = "block";
            DOM.discoverSection.style.display = "block";
        });
        DOM.feedTopBtn.addEventListener("click", () => {
            DOM.followersFeedDiv.scrollTo({ top: 0, behavior: "smooth" });
        });
        // Infinite scroll - load more when near bottom
        DOM.followersFeedDiv.addEventListener("scroll", () => {
            const { scrollTop, scrollHeight, clientHeight } = DOM.followersFeedDiv;
            if (scrollTop + clientHeight >= scrollHeight - 100) {
                loadFollowersFeed("more").then();
            }
        });

        /* --- Initialize page variables and start loading --- */
        DOM.searchInput.value = "";
        DOM.resultsDiv.style.display = "none";
        DOM.followersFeedSection.style.display = "block";
        DOM.discoverSection.style.display = "block";
        ShowNotifications(); // Load notifications in background - don't block page loading
        loadFollowersFeed("initial").then();
        loadDiscoverProfiles().then();
        preloadTinyMCE(); // Preload TinyMCE in background after page loads
        // Auto-refresh feed every 60 seconds to fetch new posts
        setInterval(() => {
            if (DOM.followersFeedSection.style.display !== "none") {
                loadFollowersFeed("refresh").then();
            }
        }, 60000);
    }
})();