window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/home.scss";
import "../components/scrollTop";
import "../../scss/components/scrollTop.scss";
import "../components/addPost";
import "../components/menu";
import {CreatePostCard} from "../components/postCard";
import {CreateProfileCard, FetchAndUpdateProfileCard} from "../components/profileCard";
import {HttpGetJson} from "../util/network";
import {IsValidAlgoAddress, IsValidAlgoTxId, XSSSanitizeUrl} from "../util/security";
import {WalletGetAvatar, WalletGetCachedAvatar, WalletGetCachedName, WalletGetDescription, WalletGetName, GetAddress, GetChain} from "../util/blockchain/wallet";
import {baseGetEnsAddress} from "../util/blockchain/base";
import {algoGetNfdAddress} from "../util/blockchain/algorand";
import {ethereumGetEnsAddress} from "../util/blockchain/ethereum";
import {ApplyIpfsImageLoadPolicy, CIDToSubdomainURL, getIpfsAvatarUrl} from "../util/ipfs";
import {IsGatewayMode} from "../util/miscellaneous";
import {ShowNotifications} from "../util/notifications";
import {ShowDialogModalHTML} from "../components/modalDialog";
import {CreateXcomCard} from "../components/xcomOEmbedCard";

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
            discoverRandomRefreshBtn: document.getElementById("discoverRandomRefreshBtn")! as HTMLElement,
            discoverRandomRow: document.getElementById("discoverRandomRow")! as HTMLDivElement,
            discoverFollowersRow: document.getElementById("discoverFollowersRow")! as HTMLDivElement,
            discoverPostsRow: document.getElementById("discoverPostsRow")! as HTMLDivElement,
            feedRefreshBtn: document.getElementById("feedRefreshBtn")! as HTMLElement,
        }
        const FEED_PAGE_SIZE = 5;
        const SEARCH_POSTS_PAGE_SIZE = 25;
        const SEARCH_PROFILES_VISIBLE = 5;
        let feedOffset = 0;
        let feedLoading = false;
        let feedHasMore = true;
        let feedNewestTimestamp: number | null = null;
        let searchPostsOffset = 0;
        let searchPostsHasMore = false;
        let searchPostsLoading = false;
        let currentSearchQuery = "";
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
                        item.data.avatarSrc = "/static/image/avatar.svg";
                        let postDiv = await CreatePostCard(item.data);
                        pendingCards.push(postDiv);
                    } else {
                        const createdAt = new Date(item.data.created_at);
                        const postDiv = await CreateXcomCard({
                            date: createdAt.toLocaleDateString(undefined, {month: 'short', day: 'numeric', year: 'numeric', hour: 'numeric', minute: '2-digit', hour12: true}),
                            postUrl: `https://x.com/${item.data.username}/status/${item.data.id}`,
                            text: item.data.text,
                            username: item.data.username,
                        });
                        pendingCards.push(postDiv);
                    }
                }
                if (mode === "refresh") {
                    const firstChild = DOM.followersFeedDiv.firstChild;
                    for (const card of pendingCards) {
                        DOM.followersFeedDiv.insertBefore(card, firstChild);
                    }
                } else {
                    for (const card of pendingCards) {
                        DOM.followersFeedDiv.appendChild(card);
                    }
                }
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
                                    const defaultPath = "/static/image/avatar.svg";
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
                                        ApplyIpfsImageLoadPolicy(avatarElement, avatarUrl);
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
                const data = getDiscoverProfiles(resp[1]);
                await populateDiscoverRow(DOM.discoverRandomRow, data.random || []);
                await populateDiscoverRow(DOM.discoverFollowersRow, data.byFollowers || []);
                await populateDiscoverRow(DOM.discoverPostsRow, data.byPosts || []);
            } catch (error) {
                console.error("Error loading discover profiles:", error);
            }
        }
        async function loadRandomProfiles() {
            try {
                let resp = await HttpGetJson("/discover/random");
                if (resp[0] !== 200) {
                    console.error("Failed to load random profiles:", resp[0], resp[1]);
                    return;
                }
                const data = getDiscoverProfiles(resp[1]);
                await populateDiscoverRow(DOM.discoverRandomRow, data.random || []);
            } catch (error) {
                console.error("Error loading random profiles:", error);
            }
        }
        function getDiscoverProfiles(data: any) {
            if (!data || typeof data !== "object") {
                return {
                    random: [],
                    byFollowers: [],
                    byPosts: [],
                };
            }
            return {
                random: Array.isArray(data.random) ? data.random : [],
                byFollowers: Array.isArray(data.byFollowers) ? data.byFollowers : [],
                byPosts: Array.isArray(data.byPosts) ? data.byPosts : [],
            };
        }
        async function populateDiscoverRow(rowElement: HTMLDivElement, profiles: any[]) {
            const columns = rowElement.querySelectorAll(".discoverCol");
            for (let i = 0; i < Math.min(profiles.length, columns.length); i++) {
                const profile = profiles[i];
                const col = columns[i] as HTMLDivElement;
                col.innerHTML = "";
                profile.name = WalletGetCachedName(profile.blockchain, profile.address) || "Loading...";
                profile.avatarSrc = WalletGetCachedAvatar(profile.blockchain, profile.address) || "/static/image/avatar.svg";
                profile.description = "";
                const profileCard = await CreateProfileCard(profile);
                col.appendChild(profileCard);
                FetchAndUpdateProfileCard(profileCard, profile.blockchain, profile.address);
            }
        }
        function updateCardAvatar(element: HTMLImageElement, avatarStr: string | null) {
            const defaultPath = "/static/image/avatar.svg";
            if (avatarStr) {
                let avatarSrc = avatarStr;
                if (avatarSrc.startsWith("ipfs://")) {
                    avatarSrc = CIDToSubdomainURL(avatarSrc) || defaultPath;
                }
                const avatarUrl = XSSSanitizeUrl(avatarSrc);
                element.onerror = () => {
                    element.src = defaultPath;
                    element.onerror = null;
                };
                ApplyIpfsImageLoadPolicy(element, avatarUrl);
                element.src = avatarUrl;
            } else {
                element.src = defaultPath;
            }
        }
        async function hydrateCards(cards: HTMLElement[], results: any[]) {
            const profileCardMap = new Map<string, HTMLElement[]>();
            const postCardMap = new Map<string, HTMLElement[]>();
            for (const card of cards) {
                const profileAddr = card.querySelector('.profileCardAddressInput') as HTMLInputElement;
                const profileChain = card.querySelector('.profileCardBlockchain') as HTMLInputElement;
                if (profileAddr && profileChain) {
                    const key = profileChain.value + profileAddr.value;
                    if (!profileCardMap.has(key)) profileCardMap.set(key, []);
                    profileCardMap.get(key)!.push(card);
                }
                const postAddr = card.querySelector('.postCardAddress') as HTMLInputElement;
                const postChain = card.querySelector('.postCardBlockchain') as HTMLInputElement;
                if (postAddr && postChain) {
                    const key = postChain.value + postAddr.value;
                    if (!postCardMap.has(key)) postCardMap.set(key, []);
                    postCardMap.get(key)!.push(card);
                }
            }
            const uniqueKeys = new Set<string>();
            const uniqueAuthors: {blockchain: string, address: string, isProfile: boolean}[] = [];
            for (const result of results) {
                const key = result.blockchain + result.address;
                if (!uniqueKeys.has(key)) {
                    uniqueKeys.add(key);
                    uniqueAuthors.push({blockchain: result.blockchain, address: result.address, isProfile: result.resultType === "profile"});
                }
            }
            const profilePromises = uniqueAuthors.map(async (author) => {
                const key = author.blockchain + author.address;
                const [name, ipfsAvatar, description] = await Promise.all([
                    WalletGetName(author.blockchain, author.address),
                    getIpfsAvatarUrl(author.blockchain, author.address),
                    author.isProfile ? WalletGetDescription(author.blockchain, author.address) : Promise.resolve(null)
                ]);
                let avatarStr = ipfsAvatar;
                if (!avatarStr || avatarStr === "") {
                    avatarStr = await WalletGetAvatar(author.blockchain, author.address);
                }
                const pCards = profileCardMap.get(key);
                if (pCards) {
                    for (const card of pCards) {
                        const nameDiv = card.querySelector('.profileCardName') as HTMLDivElement;
                        const avatarImg = card.querySelector('img.profileCardAvatar') as HTMLImageElement;
                        const descriptionDiv = card.querySelector('.profileCardDescription') as HTMLDivElement;
                        if (nameDiv) nameDiv.textContent = name || "Anonymous";
                        if (avatarImg) updateCardAvatar(avatarImg, avatarStr);
                        if (descriptionDiv) descriptionDiv.textContent = description || "";
                    }
                }
                const ptCards = postCardMap.get(key);
                if (ptCards) {
                    for (const card of ptCards) {
                        const authorEl = card.querySelector('.postCardAuthor') as HTMLElement;
                        const avatarEl = card.querySelector('img.postCardAvatar') as HTMLImageElement;
                        if (authorEl) authorEl.textContent = name || "Anonymous";
                        if (avatarEl) updateCardAvatar(avatarEl, avatarStr);
                    }
                }
            });
            await Promise.all(profilePromises);
        }
        async function handleSearch() {
            DOM.resultsDiv.replaceChildren();
            DOM.followersFeedSection.style.display = "none";
            DOM.discoverSection.style.display = "none";
            searchPostsOffset = 0;
            searchPostsHasMore = false;
            searchPostsLoading = false;
            let query = DOM.searchInput.value;
            if (query.length < 3) {
                if (DOM.followersFeedDiv.children.length > 0) {
                    DOM.followersFeedSection.style.display = "block";
                }
                DOM.discoverSection.style.display = "block";
                return;
            }
            currentSearchQuery = query;
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
            let algoTxIdPostPromise: Promise<[number, any]> | null = null;
            if (IsValidAlgoTxId(algoQuery)) {
                algoTxIdPostPromise = HttpGetJson("/post/data/algorand/" + algoQuery);
            }
            const [resp, algoTxIdResp] = await Promise.all([
                HttpGetJson(`/s?q=${query}&limit=${SEARCH_POSTS_PAGE_SIZE + 1}&offset=0`),
                algoTxIdPostPromise
            ]);
            DOM.resultsDiv.replaceChildren();
            let profiles: any[] = [];
            let posts: any[] = [];
            searchPostsHasMore = false;
            if (resp[0] === 200 && resp[1]) {
                profiles = resp[1].profiles || [];
                posts = resp[1].posts || [];
                searchPostsHasMore = resp[1].hasMorePosts || false;
            } else if (resp[0] !== 200) {
                console.error("Search failed with status:", resp[0], "Response:", resp[1]);
            }
            if (algoTxIdResp && algoTxIdResp[0] === 200 && algoTxIdResp[1] && algoTxIdResp[1].post) {
                let post = algoTxIdResp[1].post;
                post.resultType = "post";
                posts.unshift(post);
            }
            if (IsValidAlgoAddress(algoQuery)) {
                profiles.unshift({resultType: "profile", blockchain: "algorand", address: algoQuery});
            }
            if (profiles.length === 0) {
                let lowerQuery = query.toLowerCase().trim();
                let ensQuery = "";
                let ethEnsQuery = "";
                let nfdQuery = "";
                if (lowerQuery.endsWith(".base.eth")) {
                    ensQuery = lowerQuery;
                } else if (lowerQuery.endsWith(".eth")) {
                    ethEnsQuery = lowerQuery;
                } else if (lowerQuery.endsWith(".algo")) {
                    nfdQuery = lowerQuery;
                } else if (!lowerQuery.includes(".")) {
                    ensQuery = lowerQuery + ".base.eth";
                    ethEnsQuery = lowerQuery + ".eth";
                    nfdQuery = lowerQuery + ".algo";
                }
                const [ensAddress, ethEnsAddress, nfdAddress] = await Promise.all([
                    ensQuery !== "" ? baseGetEnsAddress(ensQuery) : Promise.resolve(""),
                    ethEnsQuery !== "" ? ethereumGetEnsAddress(ethEnsQuery) : Promise.resolve(""),
                    nfdQuery !== "" ? algoGetNfdAddress(nfdQuery) : Promise.resolve("")
                ]);
                if (nfdAddress && nfdAddress !== "") {
                    profiles.unshift({resultType: "profile", blockchain: "algorand", address: nfdAddress});
                }
                if (ethEnsAddress && ethEnsAddress !== "") {
                    profiles.unshift({resultType: "profile", blockchain: "ethereum", address: ethEnsAddress});
                }
                if (ensAddress && ensAddress !== "") {
                    profiles.unshift({resultType: "profile", blockchain: "base", address: ensAddress});
                }
            }
            const seenProfiles = new Set<string>();
            profiles = profiles.filter(p => {
                const key = p.blockchain + p.address;
                if (seenProfiles.has(key)) return false;
                seenProfiles.add(key);
                return true;
            });
            if (profiles.length === 0 && posts.length === 0) {
                let noResultsDiv = document.createElement("div");
                noResultsDiv.className = "no-results-dropdown";
                noResultsDiv.textContent = "No results found";
                DOM.resultsDiv.appendChild(noResultsDiv);
                return;
            }
            const allCards: HTMLElement[] = [];
            const allResults: any[] = [];
            if (profiles.length > 0) {
                let profilesLabel = document.createElement("div");
                profilesLabel.className = "searchSectionLabel";
                profilesLabel.textContent = "Profiles";
                DOM.resultsDiv.appendChild(profilesLabel);
                let searchProfilesDiv = document.createElement("div");
                searchProfilesDiv.id = "searchProfilesDiv";
                DOM.resultsDiv.appendChild(searchProfilesDiv);
                for (let i = 0; i < profiles.length; i++) {
                    profiles[i].resultType = "profile";
                    profiles[i].name = WalletGetCachedName(profiles[i].blockchain, profiles[i].address) || "Loading...";
                    profiles[i].avatarSrc = WalletGetCachedAvatar(profiles[i].blockchain, profiles[i].address) || "/static/image/avatar.svg";
                    let profileDiv = await CreateProfileCard(profiles[i]);
                    if (i >= SEARCH_PROFILES_VISIBLE) {
                        profileDiv.style.display = "none";
                        profileDiv.classList.add("searchProfileHidden");
                    }
                    allCards.push(profileDiv);
                    allResults.push(profiles[i]);
                    searchProfilesDiv.appendChild(profileDiv);
                }
                if (profiles.length > SEARCH_PROFILES_VISIBLE) {
                    let loadMoreBtn = document.createElement("div");
                    loadMoreBtn.className = "loadMoreBtn";
                    loadMoreBtn.textContent = `Show ${profiles.length - SEARCH_PROFILES_VISIBLE} more profiles`;
                    loadMoreBtn.addEventListener("click", () => {
                        const hidden = searchProfilesDiv.querySelectorAll(".searchProfileHidden");
                        hidden.forEach(el => {
                            (el as HTMLElement).style.display = "";
                            el.classList.remove("searchProfileHidden");
                        });
                        loadMoreBtn.remove();
                    });
                    searchProfilesDiv.appendChild(loadMoreBtn);
                }
            }
            if (posts.length > 0 || searchPostsHasMore) {
                let postsLabel = document.createElement("div");
                postsLabel.className = "searchSectionLabel";
                postsLabel.textContent = "Posts";
                DOM.resultsDiv.appendChild(postsLabel);
                let searchPostsDiv = document.createElement("div");
                searchPostsDiv.id = "searchPostsDiv";
                DOM.resultsDiv.appendChild(searchPostsDiv);
                for (let i = 0; i < posts.length; i++) {
                    posts[i].author = "Loading...";
                    posts[i].avatarSrc = "/static/image/avatar.svg";
                    let postDiv = await CreatePostCard(posts[i]);
                    allCards.push(postDiv);
                    allResults.push(posts[i]);
                    searchPostsDiv.appendChild(postDiv);
                }
                searchPostsOffset = posts.length;
                if (searchPostsHasMore) {
                    setupSearchPostsObserver(searchPostsDiv);
                }
            }
            await hydrateCards(allCards, allResults);
        }
        async function loadMoreSearchPosts() {
            if (searchPostsLoading || !searchPostsHasMore) return;
            searchPostsLoading = true;
            try {
                let searchPostsDiv = document.getElementById("searchPostsDiv");
                if (!searchPostsDiv) return;
                const resp = await HttpGetJson(`/s?q=${currentSearchQuery}&limit=${SEARCH_POSTS_PAGE_SIZE + 1}&offset=${searchPostsOffset}`);
                if (resp[0] !== 200 || !resp[1]) return;
                let posts: any[] = resp[1].posts || [];
                searchPostsHasMore = resp[1].hasMorePosts || false;
                const newCards: HTMLElement[] = [];
                const existingCount = searchPostsDiv.children.length;
                for (let i = 0; i < posts.length; i++) {
                    posts[i].author = "Loading...";
                    posts[i].avatarSrc = "/static/image/avatar.svg";
                    let postDiv = await CreatePostCard(posts[i]);
                    newCards.push(postDiv);
                    searchPostsDiv.appendChild(postDiv);
                }
                searchPostsOffset += posts.length;
                if (searchPostsHasMore) {
                    setupSearchPostsObserver(searchPostsDiv);
                }
                await hydrateCards(newCards, posts);
            } catch (error) {
                console.error("Error loading more search posts:", error);
            } finally {
                searchPostsLoading = false;
            }
        }
        let searchPostsObserver: IntersectionObserver | null = null;
        function setupSearchPostsObserver(searchPostsDiv: HTMLElement) {
            if (searchPostsObserver) {
                searchPostsObserver.disconnect();
            }
            searchPostsObserver = new IntersectionObserver((entries) => {
                for (const entry of entries) {
                    if (entry.isIntersecting && searchPostsHasMore && !searchPostsLoading) {
                        loadMoreSearchPosts().then();
                    }
                }
            }, {rootMargin: "100px"});
            const lastPost = searchPostsDiv.lastElementChild;
            if (lastPost) {
                searchPostsObserver.observe(lastPost);
            }
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
        const debounceHandler = debounce(handleInput, 700);
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
            if (searchPostsObserver) {
                searchPostsObserver.disconnect();
                searchPostsObserver = null;
            }
            searchPostsOffset = 0;
            searchPostsHasMore = false;
            searchPostsLoading = false;
            currentSearchQuery = "";
            if (DOM.followersFeedDiv.children.length > 0) {
                DOM.followersFeedSection.style.display = "block";
            }
            DOM.discoverSection.style.display = "block";
        });
        DOM.discoverRandomRefreshBtn.addEventListener("click", () => {
            loadRandomProfiles().then();
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
        setInterval(() => {
            if (DOM.followersFeedSection.style.display !== "none") {
                loadFollowersFeed("refresh").then();
            }
        }, 60000); // Auto-refresh feed every 60 seconds to fetch new posts
    }
})();
