window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/underground.scss";
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
import {ShowNotifications} from "../util/notifications";
import {CreateXcomCard} from "../components/xcomOEmbedCard";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function initializeAsciiArt() {
        const DOM = {
            accent: document.getElementById("homeAsciiAccent")! as HTMLSpanElement,
            art: document.getElementById("homeAsciiArt")! as HTMLPreElement,
            structure: document.getElementById("homeAsciiStructure")! as HTMLSpanElement,
        };
        const logo = new Image();
        const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
        let cellAspect = 0.61;
        let columns = 1;
        let frame = 0;
        let lastFrame = 0;
        let lastMotionFrame = 0;
        let logoAspect = 1;
        let logoHeight = 1;
        let logoPixels: Uint8ClampedArray | null = null;
        let logoWidth = 192;
        let pageHidden = false;
        let phase = 0;
        let pointerPosition: {x: number, y: number} | null = null;
        let pointerX = 0;
        let pointerY = 0;
        let rows = 1;
        let targetX = 0;
        let targetY = 0;
        let visible = false;

        function animate(timestamp: number) {
            const easing = 1 - Math.exp(-Math.min(timestamp - lastMotionFrame, 50) / 90);
            frame = 0;
            lastMotionFrame = timestamp;
            pointerX += (targetX - pointerX) * easing;
            pointerY += (targetY - pointerY) * easing;
            updateMotion();
            if (timestamp - lastFrame >= 50) {
                phase += Math.min(timestamp - lastFrame, 50) * 0.00018;
                lastFrame = timestamp;
                render();
            }
            frame = window.requestAnimationFrame(animate);
        }
        function grain(x: number, y: number) {
            const value = Math.sin(x * 127.1 + y * 311.7) * 43758.5453;
            return value - Math.floor(value);
        }
        function loadLogo() {
            const canvas = document.createElement("canvas");
            const context = canvas.getContext("2d");
            if (!context) return;
            logoAspect = logo.naturalWidth / logo.naturalHeight;
            logoHeight = Math.round(logoWidth / logoAspect);
            canvas.width = logoWidth;
            canvas.height = logoHeight;
            context.drawImage(logo, 0, 0, logoWidth, logoHeight);
            logoPixels = context.getImageData(0, 0, logoWidth, logoHeight).data;
            resize();
            updateAnimation();
        }
        function noise(x: number, y: number) {
            const left = Math.floor(x);
            const top = Math.floor(y);
            const dx = x - left;
            const dy = y - top;
            const sx = dx * dx * (3 - 2 * dx);
            const sy = dy * dy * (3 - 2 * dy);
            const a = grain(left, top);
            const b = grain(left + 1, top);
            const c = grain(left, top + 1);
            const d = grain(left + 1, top + 1);
            return a + (b - a) * sx + (c - a) * sy + (a - b - c + d) * sx * sy;
        }
        function render() {
            if (!logoPixels) return;
            const width = Math.max(1, Math.min(columns - 4, (rows - 2) * logoAspect / cellAspect));
            const height = width * cellAspect / logoAspect;
            const left = (columns - width) / 2;
            const top = (rows - height) / 2;
            let accent = "";
            let structure = "";
            for (let row = 0; row < rows; row++) {
                for (let column = 0; column < columns; column++) {
                    const y = (row + 0.5 - top) / height;
                    const x = (column + 0.5 - left) / width;
                    const pixelX = Math.floor(x * logoWidth);
                    const pixelY = Math.floor(y * logoHeight);
                    const pixel = (pixelY * logoWidth + pixelX) * 4;
                    let accentBlock = " ";
                    let structureBlock = " ";
                    if (pixelX >= 0 && pixelX < logoWidth && pixelY >= 0 && pixelY < logoHeight && logoPixels[pixel + 3] > 100) {
                        const corrosion = noise(x * 14 + phase * 1.4 + pointerX * 3, y * 14 - phase + pointerY * 3);
                        const block = corrosion > 0.76 ? "░" : corrosion > 0.63 ? "▒" : corrosion > 0.48 ? "▓" : "█";
                        if (logoPixels[pixel + 2] > logoPixels[pixel]) accentBlock = block;
                        else structureBlock = block;
                    }
                    accent += accentBlock;
                    structure += structureBlock;
                }
                if (row < rows - 1) {
                    accent += "\n";
                    structure += "\n";
                }
            }
            DOM.accent.textContent = accent;
            DOM.structure.textContent = structure;
        }
        function resize() {
            const styles = window.getComputedStyle(DOM.art);
            cellAspect = parseFloat(styles.fontSize) * 0.61 / parseFloat(styles.lineHeight);
            columns = Math.max(1, Math.min(120, Math.floor(DOM.art.clientWidth / (parseFloat(styles.fontSize) * 0.61))));
            rows = Math.max(1, Math.min(32, Math.floor(DOM.art.clientHeight / parseFloat(styles.lineHeight))));
            updatePointer();
            render();
        }
        function updateAnimation() {
            window.cancelAnimationFrame(frame);
            frame = 0;
            if (reducedMotion.matches) {
                pointerX = pointerY = targetX = targetY = phase = 0;
                updateMotion();
                render();
            }
            if (logoPixels && !reducedMotion.matches && visible && !document.hidden && !pageHidden) {
                lastFrame = lastMotionFrame = performance.now();
                frame = window.requestAnimationFrame(animate);
            }
        }
        function updateMotion() {
            DOM.art.style.setProperty("--home-ascii-x", `${(pointerX * 8).toFixed(2)}px`);
            DOM.art.style.setProperty("--home-ascii-y", `${(pointerY * 6).toFixed(2)}px`);
        }
        function updatePointer() {
            if (reducedMotion.matches) return;
            if (!pointerPosition) {
                targetX = targetY = 0;
                return;
            }
            const bounds = DOM.art.getBoundingClientRect();
            const distanceX = pointerPosition.x - (bounds.left + bounds.width / 2);
            const distanceY = pointerPosition.y - (bounds.top + bounds.height / 2);
            targetX = Math.max(-1, Math.min(1, distanceX * 0.15 / 24));
            targetY = Math.max(-1, Math.min(1, distanceY * 0.15 / 18));
        }

        document.addEventListener("pointermove", event => {
            if (event.pointerType !== "mouse" || reducedMotion.matches || !visible) return;
            pointerPosition = {x: event.clientX, y: event.clientY};
            updatePointer();
        }, {passive: true});
        document.documentElement.addEventListener("pointerleave", () => { pointerPosition = null; updatePointer(); });
        document.addEventListener("visibilitychange", updateAnimation);
        reducedMotion.addEventListener("change", updateAnimation);
        window.addEventListener("pagehide", () => { pageHidden = true; updateAnimation(); });
        window.addEventListener("pageshow", () => { pageHidden = false; updateAnimation(); });
        window.addEventListener("scroll", updatePointer, {passive: true});
        new ResizeObserver(resize).observe(DOM.art);
        new IntersectionObserver(entries => {
            visible = entries[0].isIntersecting;
            updateAnimation();
        }).observe(DOM.art);
        logo.addEventListener("load", loadLogo);
        logo.src = "/static/image/yourplace-logo.svg";
        resize();
        updateAnimation();
    }

    function main() {
        let DOM = {
            discoverCategoryBtn: document.getElementById("discoverCategoryBtn")! as HTMLButtonElement,
            discoverFollowersRow: document.getElementById("discoverFollowersRow")! as HTMLDivElement,
            discoverPostsRow: document.getElementById("discoverPostsRow")! as HTMLDivElement,
            discoverRandomRefreshBtn: document.getElementById("discoverRandomRefreshBtn")! as HTMLButtonElement,
            discoverRandomRow: document.getElementById("discoverRandomRow")! as HTMLDivElement,
            discoverRetryBtn: document.getElementById("discoverRetryBtn")! as HTMLButtonElement,
            discoverSection: document.getElementById("discoverSection")! as HTMLDivElement,
            discoverStatus: document.getElementById("discoverStatus")! as HTMLDivElement,
            discoverTab: document.getElementById("discoverTab")! as HTMLButtonElement,
            feedDiscoverBtn: document.getElementById("feedDiscoverBtn")! as HTMLButtonElement,
            feedRefreshBtn: document.getElementById("feedRefreshBtn")! as HTMLButtonElement,
            feedRetryBtn: document.getElementById("feedRetryBtn")! as HTMLButtonElement,
            feedStatus: document.getElementById("feedStatus")! as HTMLDivElement,
            followersFeedDiv: document.getElementById("followersFeedDiv")! as HTMLDivElement,
            followersFeedSection: document.getElementById("followersFeedSection")! as HTMLDivElement,
            homeDiscovery: document.getElementById("homeDiscovery")! as HTMLDivElement,
            isCookieAuthenticated: document.getElementById("isCookieAuthenticated")! as HTMLInputElement,
            publicLoadMoreBtn: document.getElementById("publicLoadMoreBtn")! as HTMLButtonElement,
            publicPostsDiv: document.getElementById("publicPostsDiv")! as HTMLDivElement,
            publicRefreshBtn: document.getElementById("publicRefreshBtn")! as HTMLButtonElement,
            publicStatus: document.getElementById("publicStatus")! as HTMLDivElement,
            resultsDiv: document.getElementById("resultsDiv")! as HTMLDivElement,
            searchClearBtn: document.getElementById("searchClearBtn")! as HTMLButtonElement,
            searchInput: document.getElementById("searchInput")! as HTMLInputElement,
        }
        const authenticated = DOM.isCookieAuthenticated.value === "true";
        const FEED_PAGE_SIZE = 5;
        const PUBLIC_PAGE_SIZE = 12;
        const SEARCH_POSTS_PAGE_SIZE = 25;
        const SEARCH_PROFILES_VISIBLE = 5;
        let feedOffset = 0;
        let feedLoading = false;
        let feedHasMore = true;
        let feedObserver: IntersectionObserver | null = null;
        let profileCategory = "random";
        let profilesLoading = false;
        let publicHasMore = true;
        let publicInitialized = false;
        let publicLoading = false;
        let publicOffset = 0;
        let searchGeneration = 0;
        let searchTimeout: number;
        let searchPostsObserver: IntersectionObserver | null = null;
        let selectedFeed = authenticated ? "feed" : "discover";
        let searchPostsOffset = 0;
        let searchPostsHasMore = false;
        let searchPostsLoading = false;
        let currentSearchQuery = "";
        const publicPostKeys = new Set<string>();
        const loadedTxHashes = new Set<string>();
        const loadedXcomIds = new Set<string>();
        async function handleSearch() {
            const generation = ++searchGeneration;
            const query = DOM.searchInput.value;
            searchPostsObserver?.disconnect();
            DOM.resultsDiv.replaceChildren();
            DOM.resultsDiv.hidden = query.length < 3;
            DOM.homeDiscovery.hidden = query.length >= 3;
            searchPostsOffset = 0;
            searchPostsHasMore = false;
            searchPostsLoading = false;
            currentSearchQuery = query;
            if (query.length < 3) {
                if (selectedFeed === "feed") setupFeedObserver();
                return;
            }
            let algoQuery = query.trim();
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
                HttpGetJson(`/s?q=${encodeURIComponent(query)}&limit=${SEARCH_POSTS_PAGE_SIZE + 1}&offset=0`),
                algoTxIdPostPromise
            ]);
            if (generation !== searchGeneration) return;
            DOM.resultsDiv.replaceChildren();
            let profiles: any[] = [];
            let posts: any[] = [];
            searchPostsHasMore = false;
            if (resp[0] === 200 && resp[1]) {
                profiles = resp[1].profiles || [];
                posts = resp[1].posts || [];
                searchPostsHasMore = resp[1].hasMorePosts || false;
            } else if (resp[0] !== 200) {
                throw new Error("Could not search");
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
                let nameQuery = query.trim();
                let ensQuery = "";
                let ethEnsQuery = "";
                let nfdQuery = "";
                if (nameQuery.endsWith(".base.eth")) {
                    ensQuery = nameQuery;
                } else if (nameQuery.endsWith(".eth")) {
                    ethEnsQuery = nameQuery;
                } else if (nameQuery.endsWith(".algo")) {
                    nfdQuery = nameQuery;
                } else if (!nameQuery.includes(".")) {
                    ensQuery = nameQuery + ".base.eth";
                    ethEnsQuery = nameQuery + ".eth";
                    nfdQuery = nameQuery + ".algo";
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
            if (generation !== searchGeneration) return;
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
                    if (generation !== searchGeneration) return;
                    if (i >= SEARCH_PROFILES_VISIBLE) {
                        profileDiv.style.display = "none";
                        profileDiv.classList.add("searchProfileHidden");
                    }
                    allCards.push(profileDiv);
                    allResults.push(profiles[i]);
                    searchProfilesDiv.appendChild(profileDiv);
                }
                if (profiles.length > SEARCH_PROFILES_VISIBLE) {
                    let loadMoreBtn = document.createElement("button");
                    loadMoreBtn.type = "button";
                    loadMoreBtn.className = "btn homeButton loadMoreBtn";
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
                    if (generation !== searchGeneration) return;
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
            await Promise.allSettled(profilePromises);
        }
        async function loadDiscoverProfiles(randomOnly = false) {
            if (profilesLoading) return;
            profilesLoading = true;
            DOM.discoverRandomRefreshBtn.disabled = true;
            DOM.discoverRetryBtn.hidden = true;
            DOM.discoverStatus.textContent = "Loading profiles…";
            try {
                const resp = await HttpGetJson(randomOnly ? "/discover/random" : "/discover");
                if (resp[0] !== 200 || !resp[1]) throw new Error("Could not load profiles");
                const data = resp[1];
                await populateDiscoverRow(DOM.discoverRandomRow, Array.isArray(data.random) ? data.random : []);
                if (!randomOnly) {
                    await populateDiscoverRow(DOM.discoverFollowersRow, Array.isArray(data.byFollowers) ? data.byFollowers : []);
                    await populateDiscoverRow(DOM.discoverPostsRow, Array.isArray(data.byPosts) ? data.byPosts : []);
                }
                DOM.discoverStatus.textContent = "";
            } catch (error) {
                DOM.discoverStatus.textContent = "Profiles could not be loaded. Try again.";
                DOM.discoverRetryBtn.hidden = false;
                DOM.discoverSection.querySelectorAll(".homeSkeleton").forEach(card => card.remove());
            } finally {
                profilesLoading = false;
                DOM.discoverRandomRefreshBtn.disabled = false;
            }
        }
        async function loadFollowersFeed(mode: "initial" | "more" | "refresh" = "initial") {
            if (DOM.isCookieAuthenticated.value !== "true") {
                return;
            }
            if (feedLoading) return;
            if (mode === "more" && !feedHasMore) return;
            feedLoading = true;
            DOM.feedRefreshBtn.disabled = true;
            DOM.feedRetryBtn.hidden = true;
            DOM.feedDiscoverBtn.hidden = true;
            DOM.feedStatus.textContent = mode === "refresh" ? "" : "Loading your feed…";
            try {
                const userAddress = GetAddress();
                const userBlockchain = GetChain();
                if (!userAddress || !userBlockchain) {
                    DOM.feedStatus.textContent = "Connect your wallet to load your feed.";
                    DOM.feedRetryBtn.hidden = false;
                    return;
                }
                if (mode === "initial") {
                    feedObserver?.disconnect();
                    DOM.followersFeedDiv.replaceChildren();
                    showSkeletons(DOM.followersFeedDiv, 2);
                    feedOffset = 0;
                    feedHasMore = true;
                    loadedTxHashes.clear();
                    loadedXcomIds.clear();
                }
                const requestOffset = mode === "refresh" ? 0 : feedOffset;
                const [feedResp, xcomPosts] = await Promise.all([
                    HttpGetJson(`/feed/${userBlockchain}/${userAddress}?limit=${FEED_PAGE_SIZE + 1}&offset=${requestOffset}`),
                    mode === "initial" ? loadXcomTimeline() : Promise.resolve([])
                ]);
                if (feedResp[0] !== 200) throw new Error("Could not load feed");
                let posts: any[] = feedResp[1]?.posts || [];
                if (mode !== "refresh") {
                    feedHasMore = posts.length > FEED_PAGE_SIZE;
                    if (feedHasMore) {
                        posts = posts.slice(0, FEED_PAGE_SIZE);
                    }
                }
                const pageLength = posts.length;
                posts = posts.filter(post => !loadedTxHashes.has(post.blockchain + ":" + post.txHash));
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
                if (feedItems.length === 0 && (mode === "initial" || DOM.followersFeedDiv.children.length === 0)) {
                    DOM.followersFeedDiv.replaceChildren();
                    DOM.feedStatus.textContent = "Follow people to build your feed.";
                    DOM.feedDiscoverBtn.hidden = false;
                    return;
                }
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
                if (mode !== "refresh") feedOffset += pageLength;
                DOM.followersFeedDiv.querySelectorAll(".homeSkeleton").forEach(card => card.remove());
                posts.forEach(post => loadedTxHashes.add(post.blockchain + ":" + post.txHash));
                DOM.feedStatus.textContent = feedHasMore ? "" : "You’re all caught up.";
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
                void hydrateCards(pendingCards, posts);
                setupFeedObserver();
            } catch (error) {
                DOM.followersFeedDiv.querySelectorAll(".homeSkeleton").forEach(card => card.remove());
                DOM.feedStatus.textContent = "Your feed could not be loaded. Try again.";
                DOM.feedRetryBtn.hidden = false;
            } finally {
                feedLoading = false;
                DOM.feedRefreshBtn.disabled = false;
            }
        }
        async function loadMoreSearchPosts() {
            if (searchPostsLoading || !searchPostsHasMore) return;
            searchPostsLoading = true;
            const generation = searchGeneration;
            try {
                let searchPostsDiv = document.getElementById("searchPostsDiv");
                if (!searchPostsDiv) return;
                const resp = await HttpGetJson(`/s?q=${encodeURIComponent(currentSearchQuery)}&limit=${SEARCH_POSTS_PAGE_SIZE + 1}&offset=${searchPostsOffset}`);
                if (generation !== searchGeneration) return;
                if (resp[0] !== 200 || !resp[1]) throw new Error("Could not load more posts");
                let posts: any[] = resp[1].posts || [];
                searchPostsHasMore = resp[1].hasMorePosts || false;
                const newCards: HTMLElement[] = [];
                for (let i = 0; i < posts.length; i++) {
                    posts[i].author = "Loading...";
                    posts[i].avatarSrc = "/static/image/avatar.svg";
                    let postDiv = await CreatePostCard(posts[i]);
                    if (generation !== searchGeneration) return;
                    newCards.push(postDiv);
                    searchPostsDiv.appendChild(postDiv);
                }
                searchPostsOffset += posts.length;
                if (searchPostsHasMore) {
                    setupSearchPostsObserver(searchPostsDiv);
                }
                await hydrateCards(newCards, posts);
            } catch (error) {
                if (generation === searchGeneration) showSearchError(() => { void loadMoreSearchPosts(); });
            } finally {
                if (generation === searchGeneration) searchPostsLoading = false;
            }
        }
        async function loadPublicPosts(reset = false) {
            if (publicLoading || (!reset && !publicHasMore)) return;
            publicLoading = true;
            DOM.publicRefreshBtn.disabled = true;
            DOM.publicLoadMoreBtn.disabled = true;
            DOM.publicStatus.textContent = "Loading public posts…";
            if (reset) {
                publicOffset = 0;
                publicHasMore = true;
                publicPostKeys.clear();
                DOM.publicPostsDiv.replaceChildren();
                showSkeletons(DOM.publicPostsDiv, 2);
            }
            try {
                const resp = await HttpGetJson(`/discover/posts?limit=${PUBLIC_PAGE_SIZE}&offset=${publicOffset}`);
                if (resp[0] !== 200 || !Array.isArray(resp[1]?.posts)) throw new Error("Could not load public posts");
                const posts: any[] = resp[1].posts;
                const freshPosts = posts.filter(post => !publicPostKeys.has(post.blockchain + ":" + post.txHash));
                const cards: HTMLElement[] = [];
                for (const post of freshPosts) {
                    post.author = WalletGetCachedName(post.blockchain, post.address) || "Anonymous";
                    post.avatarSrc = WalletGetCachedAvatar(post.blockchain, post.address) || "/static/image/avatar.svg";
                    cards.push(await CreatePostCard(post));
                }
                DOM.publicPostsDiv.querySelectorAll(".homeSkeleton").forEach(card => card.remove());
                DOM.publicPostsDiv.append(...cards);
                freshPosts.forEach(post => publicPostKeys.add(post.blockchain + ":" + post.txHash));
                publicOffset += posts.length;
                publicHasMore = resp[1].hasMore === true;
                publicInitialized = true;
                DOM.publicStatus.textContent = publicPostKeys.size === 0 ? "No public posts yet. Explore people while the network catches up." : publicHasMore ? "" : "You’re all caught up.";
                DOM.publicLoadMoreBtn.textContent = "Load more ↓";
                DOM.publicLoadMoreBtn.hidden = !publicHasMore;
                void hydrateCards(cards, freshPosts);
            } catch (error) {
                DOM.publicPostsDiv.querySelectorAll(".homeSkeleton").forEach(card => card.remove());
                DOM.publicStatus.textContent = "Public posts could not be loaded. Try again.";
                DOM.publicLoadMoreBtn.textContent = "Retry public posts";
                DOM.publicLoadMoreBtn.hidden = false;
            } finally {
                publicLoading = false;
                DOM.publicRefreshBtn.disabled = false;
                DOM.publicLoadMoreBtn.disabled = false;
            }
        }

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
        async function populateDiscoverRow(rowElement: HTMLDivElement, profiles: any[]) {
            const columns: HTMLElement[] = [];
            for (const profile of profiles.slice(0, 5)) {
                const col = document.createElement("div");
                const linkLabel = document.createElement("span");
                col.className = "discoverCol";
                profile.name = WalletGetCachedName(profile.blockchain, profile.address) || "Anonymous";
                profile.avatarSrc = WalletGetCachedAvatar(profile.blockchain, profile.address) || "/static/image/avatar.svg";
                profile.description = "";
                const profileCard = await CreateProfileCard(profile);
                linkLabel.className = "homeProfileLink";
                linkLabel.textContent = "View profile ↗";
                profileCard.appendChild(linkLabel);
                col.appendChild(profileCard);
                columns.push(col);
                void FetchAndUpdateProfileCard(profileCard, profile.blockchain, profile.address).catch(() => {});
            }
            rowElement.replaceChildren(...columns);
            if (columns.length === 0) {
                const empty = document.createElement("p");
                empty.className = "homeStatus";
                empty.textContent = "No profiles to show yet.";
                rowElement.appendChild(empty);
            }
        }

        function runSearch() {
            const request = handleSearch();
            const generation = searchGeneration;
            void request.catch(() => {
                if (generation !== searchGeneration) return;
                DOM.resultsDiv.replaceChildren();
                showSearchError(runSearch);
            });
        }
        function selectProfileCategory(category: string) {
            profileCategory = category;
            DOM.discoverRandomRow.hidden = category !== "random";
            DOM.discoverFollowersRow.hidden = category !== "followers";
            DOM.discoverPostsRow.hidden = category !== "posts";
            DOM.discoverRandomRefreshBtn.hidden = category !== "random";
            document.querySelectorAll<HTMLButtonElement>("[data-discover-category]").forEach(button => {
                const active = button.dataset.discoverCategory === category;
                button.classList.toggle("active", active);
                button.setAttribute("aria-pressed", String(active));
                if (active) DOM.discoverCategoryBtn.textContent = button.textContent;
            });
        }
        function setupFeedObserver() {
            if (feedObserver) {
                feedObserver.disconnect();
            }
            feedObserver = new IntersectionObserver((entries) => {
                for (const entry of entries) {
                    if (entry.isIntersecting && selectedFeed === "feed" && !DOM.homeDiscovery.hidden && feedHasMore && !feedLoading) {
                        loadFollowersFeed("more").then();
                    }
                }
            }, { rootMargin: "100px" });
            const lastPost = DOM.followersFeedDiv.lastElementChild;
            if (lastPost) {
                feedObserver.observe(lastPost);
            }
        }
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
        function showSearchError(retry: () => void) {
            const message = document.createElement("p");
            const button = document.createElement("button");
            const error = document.createElement("div");
            searchPostsObserver?.disconnect();
            DOM.resultsDiv.querySelector(".homeSearchError")?.remove();
            error.className = "homeSearchError";
            error.setAttribute("role", "status");
            message.textContent = "Search could not be loaded. Try again.";
            button.type = "button";
            button.className = "btn homeButton";
            button.textContent = "Retry search";
            button.addEventListener("click", () => {
                error.remove();
                retry();
            });
            error.append(message, button);
            DOM.resultsDiv.appendChild(error);
        }
        function showSkeletons(container: HTMLElement, count: number) {
            for (let i = 0; i < count; i++) {
                const skeleton = document.createElement("div");
                skeleton.className = "homeSkeleton";
                skeleton.setAttribute("aria-hidden", "true");
                container.appendChild(skeleton);
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

        DOM.searchInput.addEventListener("input", () => {
            window.clearTimeout(searchTimeout);
            searchGeneration++;
            searchPostsObserver?.disconnect();
            DOM.searchClearBtn.hidden = DOM.searchInput.value.length === 0;
            if (DOM.searchInput.value.length < 3) {
                runSearch();
            } else {
                searchTimeout = window.setTimeout(runSearch, 700);
            }
        });
        DOM.searchInput.addEventListener("keydown", event => {
            if (event.key === "Enter") {
                window.clearTimeout(searchTimeout);
                runSearch();
            }
        });
        DOM.searchClearBtn.addEventListener("click", () => {
            window.clearTimeout(searchTimeout);
            DOM.searchInput.value = "";
            DOM.searchClearBtn.hidden = true;
            runSearch();
            DOM.searchInput.focus();
        });
        document.querySelectorAll<HTMLButtonElement>("[data-discover-category]").forEach(button => {
            button.addEventListener("click", () => selectProfileCategory(button.dataset.discoverCategory || "random"));
        });
        document.querySelectorAll<HTMLButtonElement>("[data-bs-toggle='tab']").forEach(button => {
            button.addEventListener("shown.bs.tab", () => {
                selectedFeed = button.id === "feedTab" ? "feed" : "discover";
                feedObserver?.disconnect();
                if (selectedFeed === "discover" && !publicInitialized) void loadPublicPosts(true);
                if (selectedFeed === "feed") setupFeedObserver();
            });
        });
        DOM.discoverRandomRefreshBtn.addEventListener("click", () => { void loadDiscoverProfiles(true); });
        DOM.discoverRetryBtn.addEventListener("click", () => { void loadDiscoverProfiles(); });
        DOM.feedRefreshBtn.addEventListener("click", () => { void loadFollowersFeed("initial"); });
        DOM.feedRetryBtn.addEventListener("click", () => { void loadFollowersFeed("initial"); });
        DOM.feedDiscoverBtn.addEventListener("click", () => DOM.discoverTab.click());
        DOM.publicRefreshBtn.addEventListener("click", () => { void loadPublicPosts(true); });
        DOM.publicLoadMoreBtn.addEventListener("click", () => { void loadPublicPosts(); });
        DOM.searchInput.value = "";
        initializeAsciiArt();
        document.getElementById("htmlMenu")?.setAttribute("aria-label", "Open menu");
        selectProfileCategory(profileCategory);
        showSkeletons(DOM.discoverRandomRow, 3);
        void ShowNotifications();
        if (authenticated) void loadFollowersFeed("initial");
        else void loadPublicPosts(true);
        void loadDiscoverProfiles();
        setInterval(() => {
            if (selectedFeed === "feed" && !DOM.homeDiscovery.hidden && !document.hidden) {
                void loadFollowersFeed("refresh");
            }
        }, 60000);
    }
})();
