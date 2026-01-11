window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/home.scss";
import "../components/scrollTop";
import "../../scss/components/scrollTop.scss";
import "../components/addPost";
import {preloadTinyMCE} from "../components/addPost";
import "../components/menu";
import {CreatePostCard, CreateProfileCard} from "../util/domFactory";
import {HttpGetJson} from "../util/network";
import {IsValidAlgoAddress, IsValidAlgoTxId, XSSSanitizeUrl} from "../util/security";
import {WalletGetAvatar, WalletGetDescription, WalletGetName, GetAddress, GetChain} from "../util/blockchain/wallet";
import {baseGetEnsAddress} from "../util/blockchain/base";
import {algoGetNfdAddress} from "../util/blockchain/algorand";
import {CIDToSubdomainURL, getIpfsAvatarUrl} from "../util/ipfs";
import {IsGatewayMode} from "../util/miscellaneous";
import {ShowNotifications} from "../util/notifications";
import {ShowDialogModalHTML} from "../components/modalDialog";
import {CreateXcomPostCard} from "../util/domFactory";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            searchInput: document.getElementById("searchInput")! as HTMLInputElement,
            searchClearBtn: document.getElementById("searchClearBtn")! as HTMLButtonElement,
            resultsDiv: document.getElementById("resultsDiv")! as HTMLDivElement,
            followersFeedDiv: document.getElementById("followersFeedDiv")! as HTMLDivElement,
            followersFeedSection: document.getElementById("followersFeedSection")! as HTMLDivElement,
            isCookieAuthenticated: document.getElementById("isCookieAuthenticated")! as HTMLInputElement,
            discoverSection: document.getElementById("discoverSection")! as HTMLDivElement,
            discoverRandomRow: document.getElementById("discoverRandomRow")! as HTMLDivElement,
            discoverRandomLabel: document.getElementById("discoverRandomLabel")! as HTMLHeadingElement,
            discoverFollowersRow: document.getElementById("discoverFollowersRow")! as HTMLDivElement,
            discoverFollowersLabel: document.getElementById("discoverFollowersLabel")! as HTMLHeadingElement,
            discoverPostsRow: document.getElementById("discoverPostsRow")! as HTMLDivElement,
            discoverPostsLabel: document.getElementById("discoverPostsLabel")! as HTMLHeadingElement,
            feedRefreshBtn: document.getElementById("feedRefreshBtn")! as HTMLElement,
        }
        const FEED_PAGE_SIZE = 5;
        let feedOffset = 0;
        let feedLoading = false;
        let feedHasMore = true;
        let feedNewestTimestamp: number | null = null;
        const loadedTxHashes = new Set<string>();
        const loadedXcomIds = new Set<string>();

        async function loadXcomTimeline(): Promise<any[]> {
            try {
                let resp = await HttpGetJson("/services/xcom/timeline");
                if (resp[0] === 200 && resp[1].posts) {
                    return resp[1].posts.filter((post: any) => !loadedXcomIds.has(post.id));
                }
            } catch (error) {
                console.error("Error loading X.com timeline:", error);
            }
            return [];
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
                    loadedXcomIds.clear();
                }
                const requestOffset = mode === "refresh" ? 0 : feedOffset;
                const [feedResp, xcomPosts] = await Promise.all([
                    HttpGetJson(`/feed/${userBlockchain}/${userAddress}?limit=${FEED_PAGE_SIZE + 1}&offset=${requestOffset}`),
                    mode === "initial" ? loadXcomTimeline() : Promise.resolve([])
                ]);
                if (feedResp[0] !== 200 || !feedResp[1].posts) {
                    return;
                }
                let posts: any[] = feedResp[1].posts;
                if (mode !== "refresh") {
                    feedHasMore = posts.length > FEED_PAGE_SIZE;
                    if (feedHasMore) {
                        posts = posts.slice(0, FEED_PAGE_SIZE);
                    }
                    feedOffset += posts.length;
                }
                posts = posts.filter(post => !loadedTxHashes.has(post.txHash));
                posts.forEach(post => {
                    loadedTxHashes.add(post.txHash);
                    if (feedNewestTimestamp === null || post.timestamp > feedNewestTimestamp) {
                        feedNewestTimestamp = post.timestamp;
                    }
                });
                interface FeedItem {
                    type: "native" | "xcom";
                    timestamp: number;
                    data: any;
                }
                let feedItems: FeedItem[] = posts.map(post => ({
                    type: "native" as const,
                    timestamp: post.timestamp * 1000,
                    data: post
                }));
                if (mode === "initial" && xcomPosts.length > 0) {
                    for (const xpost of xcomPosts) {
                        loadedXcomIds.add(xpost.id);
                        feedItems.push({
                            type: "xcom",
                            timestamp: new Date(xpost.created_at).getTime(),
                            data: xpost
                        });
                    }
                    feedItems.sort((a, b) => b.timestamp - a.timestamp);
                }
                if (feedItems.length === 0 && mode === "initial") {
                    DOM.followersFeedSection.style.display = "none";
                    return;
                }
                DOM.followersFeedSection.style.display = "block";
                const pendingCards: HTMLDivElement[] = [];
                for (const item of feedItems) {
                    if (item.type === "native") {
                        item.data.author = "Loading...";
                        item.data.avatarSrc = "/static/image/avatar.png";
                        let postDiv = await CreatePostCard(item.data);
                        pendingCards.push(postDiv);
                    } else {
                        let postDiv = await CreateXcomPostCard(item.data);
                        pendingCards.push(postDiv);
                    }
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
                // Fetch profile data for each unique author
                const profilePromises = posts.map(async post => {
                    let blockchain = post.blockchain;
                    let address = post.address;
                    let key = blockchain + address;
                    let name: string | null = await WalletGetName(blockchain, address);
                    let avatarStr: string | null = null;
                    avatarStr = await getIpfsAvatarUrl(blockchain, address);
                    if (!avatarStr || avatarStr === "") {
                        avatarStr = await WalletGetAvatar(blockchain, address);
                    }
                    // Update all posts for this profile
                    pendingCards.forEach(postDiv => {
                        const postAddress = postDiv.querySelector('.postCardAddress') as HTMLInputElement;
                        const postBlockchain = postDiv.querySelector('.postCardBlockchain') as HTMLInputElement;
                        if (postAddress && postBlockchain) {
                            const postKey = postBlockchain.value + postAddress.value;
                            if (postKey === key) {
                                const authorElement = postDiv.querySelector('.postCardAuthor') as HTMLElement;
                                const avatarElement = postDiv.querySelector('img.postCardAvatar') as HTMLImageElement;
                                if (authorElement) authorElement.textContent = name || "Anonymous";
                                if (avatarElement) {
                                    const defaultPath = "/static/image/avatar.png";
                                    if (avatarStr) {
                                        let avatarSrc = avatarStr;
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
                setupFeedObserver();
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
                DOM.discoverRandomLabel.style.visibility = "visible";
                await populateDiscoverRow(DOM.discoverFollowersRow, data.byFollowers || []);
                DOM.discoverFollowersLabel.style.visibility = "visible";
                await populateDiscoverRow(DOM.discoverPostsRow, data.byPosts || []);
                DOM.discoverPostsLabel.style.visibility = "visible";
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
            let name: string | null = await WalletGetName(blockchain, address);
            let avatarStr: string | null = null;
            avatarStr = await getIpfsAvatarUrl(blockchain, address);
            if (!avatarStr || avatarStr === "") {
                avatarStr = await WalletGetAvatar(blockchain, address);
            }
            const nameDiv = profileCard.querySelector('.profileCardName') as HTMLDivElement;
            const avatarImg = profileCard.querySelector('img.profileCardAvatar') as HTMLImageElement;
            if (nameDiv) nameDiv.textContent = name || "Anonymous";
            if (avatarImg) {
                const defaultPath = "/static/image/avatar.png";
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
                if (DOM.followersFeedDiv.children.length > 0) {
                    DOM.followersFeedSection.style.display = "block";
                }
                DOM.discoverSection.style.display = "block";
                return;
            }
            let algoQuery = query.trim();
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
            let ensQuery = query.toLowerCase().trim();
            if (!ensQuery.endsWith(".base.eth")) {
                ensQuery = ensQuery + ".base.eth";
            }
            let nfdQuery = query.toLowerCase().trim();
            if (!nfdQuery.endsWith(".algo")) {
                nfdQuery = nfdQuery + ".algo";
            }
            let algoTxIdPostPromise: Promise<[number, any]> | null = null;
            if (IsValidAlgoTxId(algoQuery)) {
                algoTxIdPostPromise = HttpGetJson("/post/data/algorand/" + algoQuery);
            }
            const [resp, ensAddress, nfdAddress, algoTxIdResp] = await Promise.all([
                HttpGetJson("/s/?q=" + query),
                baseGetEnsAddress(ensQuery),
                algoGetNfdAddress(nfdQuery),
                algoTxIdPostPromise
            ]);
            DOM.resultsDiv.replaceChildren();
            let results: any[] = [];
            if (resp[0] === 200 && resp[1] && resp[1].results !== null) {
                results = resp[1].results;
            } else if (resp[0] !== 200) {
                console.error("Search failed with status:", resp[0], "Response:", resp[1]);
            }
            if (algoTxIdResp && algoTxIdResp[0] === 200 && algoTxIdResp[1] && algoTxIdResp[1].post) {
                let post = algoTxIdResp[1].post;
                post.resultType = "post";
                results.unshift(post);
            }
            if (IsValidAlgoAddress(algoQuery)) {
                results.unshift({resultType: "profile", blockchain: "algorand", address: algoQuery});
            }
            if (nfdAddress && nfdAddress !== "") {
                results.unshift({resultType: "profile", blockchain: "algorand", address: nfdAddress});
            }
            if (ensAddress && ensAddress !== "") {
                results.unshift({resultType: "profile", blockchain: "base", address: ensAddress});
            }
            const seenProfiles = new Set<string>();
            results = results.filter(result => {
                if (result.resultType === "profile") {
                    const key = result.blockchain + result.address;
                    if (seenProfiles.has(key)) {
                        return false;
                    }
                    seenProfiles.add(key);
                }
                return true;
            });
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
                    results[i].name = "Loading...";
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
                let name: string | null = await WalletGetName(blockchain, address);
                let avatarStr: string | null = null;
                avatarStr = await getIpfsAvatarUrl(blockchain, address);
                if (!avatarStr || avatarStr === "") {
                    avatarStr = await WalletGetAvatar(blockchain, address);
                }
                let description: string | null = null;
                if (result.resultType == "profile") {
                    description = await WalletGetDescription(result.blockchain, result.address);
                }
                pendingCards.forEach(profileDiv => {
                    const profileAddress = profileDiv.querySelector('.profileCardAddressInput') as HTMLInputElement;
                    const profileBlockchain = profileDiv.querySelector('.profileCardBlockchain') as HTMLInputElement;
                    if (profileAddress && profileBlockchain) {
                        const profileKey = profileBlockchain.value + profileAddress.value;
                        if (profileKey === key) {
                            const nameDiv = profileDiv.querySelector('.profileCardName') as HTMLDivElement;
                            const avatarImg = profileDiv.querySelector('img.profileCardAvatar') as HTMLImageElement;
                            const descriptionDiv = profileDiv.querySelector('.profileCardDescription') as HTMLDivElement;
                            if (nameDiv) nameDiv.textContent = name || "Anonymous";
                            if (avatarImg) {
                                const defaultPath = "/static/image/avatar.png";
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
                                    avatarImg.src = avatarUrl;
                                } else {
                                    avatarImg.src = defaultPath;
                                }
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
                            if (authorElement) authorElement.textContent = name || "Anonymous";
                            if (avatarElement) {
                                const defaultPath = "/static/image/avatar.png";
                                if (avatarStr) {
                                    let avatarSrc = avatarStr;
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
            if (DOM.followersFeedDiv.children.length > 0) {
                DOM.followersFeedSection.style.display = "block";
            }
            DOM.discoverSection.style.display = "block";
        });
        DOM.feedRefreshBtn.addEventListener("click", () => {
            loadFollowersFeed("initial").then();
        });
        let feedObserver: IntersectionObserver | null = null;
        function setupFeedObserver() {
            if (feedObserver) {
                feedObserver.disconnect();
            }
            feedObserver = new IntersectionObserver((entries) => {
                for (const entry of entries) {
                    if (entry.isIntersecting && feedHasMore && !feedLoading) {
                        loadFollowersFeed("more").then();
                    }
                }
            }, { rootMargin: "100px" });
            const lastPost = DOM.followersFeedDiv.lastElementChild;
            if (lastPost) {
                feedObserver.observe(lastPost);
            }
        }

        /* --- Initialize page variables and start loading --- */
        DOM.searchInput.value = "";
        DOM.resultsDiv.style.display = "none";
        DOM.followersFeedSection.style.display = "none";
        DOM.discoverSection.style.display = "block";
        ShowNotifications().then(); // Load notifications in background - don't block page loading
        loadFollowersFeed("initial").then();
        loadDiscoverProfiles().then();
        preloadTinyMCE().then(); // Preload TinyMCE in background after page loads
        setInterval(() => {
            if (DOM.followersFeedSection.style.display !== "none") {
                loadFollowersFeed("refresh").then();
            }
        }, 60000); // Auto-refresh feed every 60 seconds to fetch new posts
    }
})();