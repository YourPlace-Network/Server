window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/global.scss"
import "../../scss/pages/profile.scss";
import "../components/addPost";
import {preloadTinyMCE} from "../components/addPost";
import "../components/mintNFT";
import "../components/modalDialog";
import "../components/scrollTop";
import "../components/menu";
import {LogError, LogInfo} from "../util/log";
import {HttpGetJson} from "../util/network";
import {showProfileEditModal} from "../components/modalProfileEdit";
import {FetchPosts} from "../components/post";
import {ShowNotifications} from "../util/notifications";
import {GetAddress, GetChain, GetWallet, IsValidAddress, WalletBurnCollectible, WalletFollowUser, WalletGetAvatar, WalletGetCollectibles, WalletGetDescription, WalletGetExplorerAddressLink, WalletGetName, WalletGetTransferFeeEstimate, WalletSendPostNudge, WalletTransferCollectible, WalletUnfollowUser} from "../util/blockchain/wallet";
import type {CollectibleData} from "../util/blockchain/wallet";
import {CreateCollectibleCard, CreatePostCard, getBlockchainIconPath, getBlockchainUrl, processTextWithTags} from "../util/domFactory";
import {IsValidURL, IsValidIpfsCid, IsValidBaseAddress, IsValidAlgoAddress, XSSSanitizeUrl, XSSSanitizeValue} from "../util/security";
import {CIDToSubdomainURL, loadImageWithTimeout, getIpfsAvatarUrl} from "../util/ipfs";
import {IsGatewayMode} from "../util/miscellaneous";
import {ShowDialogModalWithCallback} from "../components/modalDialog";
import {ShowToast} from "../components/toast";

declare global {
    interface Window {
        CollectibleMintCallback: () => void;
        DisconnectWalletCallback: () => void;
        LoginCallback: (status: string) => void;
        PageReloadCallback: () => void;
        PostSubmitCallback: () => void;
    }
}

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            addPostButton: document.getElementById("addPostButton")! as HTMLButtonElement,
            avatarPreview: document.getElementById("avatarPreview")! as HTMLImageElement,
            bannerPreview: document.getElementById("bannerPreview")! as HTMLImageElement,
            btnCollectible: document.getElementById("btnCollectible")! as HTMLButtonElement,
            btnComments: document.getElementById("btnComments")! as HTMLButtonElement,
            btnFiles: document.getElementById("btnFiles")! as HTMLButtonElement,
            btnPosts: document.getElementById("btnPosts")! as HTMLButtonElement,
            btnSearch: document.getElementById("btnSearch")! as HTMLButtonElement,
            mintNFTButton: document.getElementById("mintNFTButton")! as HTMLButtonElement,
            profileAddressCopy: document.getElementById("profileAddressCopy")! as HTMLElement,
            csrfToken: (document.getElementById("csrfToken")! as HTMLInputElement).value,
            emptyContentDivPlaceHolder: document.getElementById("emptyContentDivPlaceHolder")! as HTMLDivElement,
            followBtn: document.getElementById("followBtn")! as HTMLButtonElement,
            followBtnLabel: document.getElementById("followBtnLabel")! as HTMLSpanElement,
            followImg: document.getElementById("followImg")! as HTMLElement,
            profileAddressLink: document.getElementById("profileAddressLink")! as HTMLAnchorElement,
            profileAvatar: document.getElementById("profileAvatar")! as HTMLImageElement,
            profileBanner: document.getElementById("profileBanner")! as HTMLImageElement,
            profileName: document.getElementById("profileName")! as HTMLDivElement,
            profileAddress: document.getElementById("profileAddress")! as HTMLDivElement,
            profileAddressFull: document.getElementById("profileAddressFull")! as HTMLInputElement,
            profileNameDomain: document.getElementById("profileNameDomain")! as HTMLDivElement,
            profileEditBtn: document.getElementById("profileEditBtn")! as HTMLButtonElement,
            profileDescription: document.getElementById("profileDescription")! as HTMLDivElement,
            profileLocation: document.getElementById("profileLocation")! as HTMLDivElement,
            profileVertical: document.getElementById("profileVertical")! as HTMLDivElement,
            profileWebsite: document.getElementById("profileWebsite")! as HTMLAnchorElement,
            profileJoined: document.getElementById("profileJoined")! as HTMLDivElement,
            postAvatars: document.getElementsByClassName("postCardAvatar")! as HTMLCollectionOf<HTMLImageElement>,
            postsDiv: document.getElementsByClassName("postCard")! as HTMLCollectionOf<HTMLDivElement>,
            injectedAddress: document.getElementById("injectedAddress")! as HTMLInputElement,
            injectedBlockchain: document.getElementById("injectedBlockchain")! as HTMLInputElement,
            isGuest: document.getElementById("isGuest")! as HTMLInputElement,
            tooltipComingSoon: document.getElementById("comingsoon-tooltip")! as HTMLDivElement,
            contentDiv: document.getElementById("contentDiv")! as HTMLDivElement,
            placeHolderH3: document.querySelector('#emptyContentDivPlaceHolder h3')! as HTMLHeadingElement,
            placeHolderP: document.querySelector('#emptyContentDivPlaceHolder p')! as HTMLParagraphElement,
            placeHolderIcon: document.querySelector('#emptyContentDivPlaceHolder i')! as HTMLElement,
            followingCount: document.getElementById("followingCount")! as HTMLDivElement,
            followerCount: document.getElementById("followerCount")! as HTMLDivElement,
            postsNum: document.getElementById("postsNum")! as HTMLDivElement,
            likesNum: document.getElementById("likesNum")! as HTMLDivElement,
            profileBlockchainIcon: document.getElementById("profileBlockchainIcon")! as HTMLImageElement,
            profileBlockchainLink: document.getElementById("profileBlockchainLink")! as HTMLAnchorElement,
        }
        let activeTab: "posts" | "collectibles" = "posts";
        let copiedTooltip: any;
        let isFollowing = false;
        let lastPostsHash = "";
        let postsHasMore = true;
        let postsLoading = false;
        let displayCollectiblesCallId = 0;
        let postsObserver: IntersectionObserver | null = null;
        let postsOffset = 0;
        let refreshIntervalId: ReturnType<typeof setInterval> | null = null;
        const POSTS_PAGE_SIZE = 20;
        const REFRESH_INTERVAL_MS = 60000;

        // --------- Page Functions --------- //
        async function init() {
            let tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]:not(#modalProfileEdit [data-bs-toggle="tooltip"])'));
            tooltipTriggerList.map(function (tooltipTriggerEl) {return new window.bootstrap.Tooltip(tooltipTriggerEl, {delay: {show: 1500, hide: 0}});}); // enable tooltips
            const blockchainIconPath = getBlockchainIconPath(DOM.injectedBlockchain.value);
            const blockchainUrl = getBlockchainUrl(DOM.injectedBlockchain.value);
            if (blockchainIconPath) {
                DOM.profileBlockchainIcon.src = blockchainIconPath;
                DOM.profileBlockchainIcon.alt = DOM.injectedBlockchain.value;
                DOM.profileBlockchainLink.title = DOM.injectedBlockchain.value;
                if (blockchainUrl) {
                    DOM.profileBlockchainLink.href = blockchainUrl;
                }
            } else {
                DOM.profileBlockchainLink.style.display = "none";
            }
            await updateProfile();
            ShowNotifications().then(); // Load notifications in background - don't block profile loading
            copiedTooltip = new window.bootstrap.Tooltip(DOM.profileAddressCopy, {title: "Copied", trigger: "manual", placement: "right"});
        }
        async function updateProfile() { // "main" method for loading the profile and it's data
            let requestedAddress = DOM.injectedAddress.value;
            let requestedBlockchain = DOM.injectedBlockchain.value;
            if (!requestedAddress || !IsValidAddress(requestedAddress, requestedBlockchain)) {
                LogError("profile.ts updateProfile() - invalid address");
                return;
            }
            // Phase 1: Load critical profile info immediately (address only)
            await renderProfileAddress(requestedAddress);
            // Phase 2: Load profile data in background
            const profileDataPromise = HttpGetJson(`/profile/data/${requestedBlockchain}/${requestedAddress}`);
            // Phase 3: Load posts in parallel (non-blocking)
            const postsPromise = displayPosts(requestedBlockchain, requestedAddress);
            // Handle profile data response
            try {
                const response = await profileDataPromise;
                if (response[0] === 200 && response[1]?.profileData) {
                    await renderProfileFromCache(response[1].profileData, requestedBlockchain, requestedAddress);
                } else {
                    LogError("Failed to fetch profile data - profile will show minimal info");
                }
            } catch (error) {
                LogError("Profile data fetch failed: " + error);
            }
            // Wait for posts to finish loading, then render guest view
            await postsPromise;
            await renderGuestView();
        }
        async function displayPosts(blockchain: string, address: string) {
            postsOffset = 0;
            postsHasMore = true;
            let result = await FetchPosts(blockchain, address, POSTS_PAGE_SIZE + 1, 0);
            if (!result || result.posts.length === 0) {
                DOM.postsNum.textContent = String(result?.totalCount || 0);
                lastPostsHash = "";
                return;
            }
            DOM.postsNum.textContent = String(result.totalCount);
            let posts = result.posts;
            postsHasMore = posts.length > POSTS_PAGE_SIZE;
            if (postsHasMore) {
                posts = posts.slice(0, POSTS_PAGE_SIZE);
            }
            postsOffset = posts.length;
            lastPostsHash = JSON.stringify(posts.map((p: any) => p.txHash));
            for (let i = 0; i < posts.length; i++) {
                let postDiv = await CreatePostCard(posts[i]);
                if (i % 2 === 0) {
                    postDiv.classList.add("shaded");
                }
                DOM.contentDiv.appendChild(postDiv);
            }
            setupPostsObserver(blockchain, address);
        }
        async function loadMorePosts(blockchain: string, address: string) {
            if (postsLoading || !postsHasMore) return;
            postsLoading = true;
            try {
                let result = await FetchPosts(blockchain, address, POSTS_PAGE_SIZE + 1, postsOffset);
                if (!result || result.posts.length === 0) {
                    postsHasMore = false;
                    return;
                }
                let posts = result.posts;
                postsHasMore = posts.length > POSTS_PAGE_SIZE;
                if (postsHasMore) {
                    posts = posts.slice(0, POSTS_PAGE_SIZE);
                }
                for (const post of posts) {
                    let postDiv = await CreatePostCard(post);
                    DOM.contentDiv.appendChild(postDiv);
                }
                postsOffset += posts.length;
                const children = DOM.contentDiv.children;
                for (let i = 0; i < children.length; i++) {
                    if (i % 2 === 0) {
                        children[i].classList.add("shaded");
                    } else {
                        children[i].classList.remove("shaded");
                    }
                }
                setupPostsObserver(blockchain, address);
            } finally {
                postsLoading = false;
            }
        }
        function setupPostsObserver(blockchain: string, address: string) {
            if (postsObserver) {
                postsObserver.disconnect();
            }
            if (!postsHasMore) return;
            postsObserver = new IntersectionObserver((entries) => {
                for (const entry of entries) {
                    if (entry.isIntersecting && postsHasMore && !postsLoading) {
                        loadMorePosts(blockchain, address).then();
                    }
                }
            }, {rootMargin: "100px"});
            const lastPost = DOM.contentDiv.lastElementChild;
            if (lastPost) {
                postsObserver.observe(lastPost);
            }
        }
        async function refreshProfileData() {
            let requestedAddress = DOM.injectedAddress.value;
            let requestedBlockchain = DOM.injectedBlockchain.value;
            if (!requestedAddress || !IsValidAddress(requestedAddress, requestedBlockchain)) {
                return;
            }
            try {
                const profileResponse = await HttpGetJson(`/profile/data/${requestedBlockchain}/${requestedAddress}`);
                if (profileResponse[0] === 200 && profileResponse[1]?.profileData) {
                    const profileData = profileResponse[1].profileData;
                    await renderProfileFromCache(profileData, requestedBlockchain, requestedAddress);
                }
                if (activeTab === "collectibles") return;
                const result = await FetchPosts(requestedBlockchain, requestedAddress, POSTS_PAGE_SIZE + 1, 0);
                if (result) {
                    DOM.postsNum.textContent = String(result.totalCount);
                }
                let posts = result ? result.posts : [];
                const newPostsHash = posts.length > 0 ? JSON.stringify(posts.slice(0, POSTS_PAGE_SIZE).map((p: any) => p.txHash)) : "";
                if (newPostsHash !== lastPostsHash) {
                    lastPostsHash = newPostsHash;
                    const existingPosts = DOM.contentDiv.querySelectorAll('.postCard');
                    existingPosts.forEach(post => post.remove());
                    if (posts.length > 0) {
                        postsHasMore = posts.length > POSTS_PAGE_SIZE;
                        if (postsHasMore) {
                            posts = posts.slice(0, POSTS_PAGE_SIZE);
                        }
                        postsOffset = posts.length;
                        for (let i = 0; i < posts.length; i++) {
                            let postDiv = await CreatePostCard(posts[i]);
                            if (i % 2 === 0) {
                                postDiv.classList.add("shaded");
                            }
                            DOM.contentDiv.appendChild(postDiv);
                        }
                        setupPostsObserver(requestedBlockchain, requestedAddress);
                    } else {
                        postsOffset = 0;
                        postsHasMore = false;
                    }
                }
                await renderGuestView();
            } catch (error) {
                LogError("Profile refresh failed: " + error);
            }
        }
        function startAutoRefresh() {
            if (refreshIntervalId) {
                clearInterval(refreshIntervalId);
            }
            refreshIntervalId = setInterval(refreshProfileData, REFRESH_INTERVAL_MS);
        }
        function stopAutoRefresh() {
            if (refreshIntervalId) {
                clearInterval(refreshIntervalId);
                refreshIntervalId = null;
            }
        }

        // --------- Collectible Functions --------- //
        async function displayCollectibles(blockchain: string, address: string) {
            const callId = ++displayCollectiblesCallId;
            DOM.contentDiv.querySelectorAll(".collectibleGrid").forEach(g => g.remove());
            const collectibles = await WalletGetCollectibles(address, blockchain);
            if (callId !== displayCollectiblesCallId) return;
            if (collectibles.length === 0) {
                DOM.emptyContentDivPlaceHolder.style.display = "flex";
                DOM.emptyContentDivPlaceHolder.classList.remove("clickable");
                DOM.emptyContentDivPlaceHolder.style.cursor = "default";
                DOM.placeHolderIcon.classList.remove("bi-house-add", "bi-envelope-paper-heart");
                DOM.placeHolderIcon.classList.add("bi-gem");
                if (DOM.isGuest.value === "false") {
                    DOM.placeHolderH3.textContent = "Create your first Collectible!";
                    DOM.placeHolderP.textContent = "Upload media and create a unique digital collectible";
                } else {
                    DOM.placeHolderH3.textContent = "No Collectibles yet";
                    DOM.placeHolderP.textContent = "";
                }
                return;
            }
            DOM.emptyContentDivPlaceHolder.style.display = "none";
            const isOwner = DOM.isGuest.value === "false";
            let grid = document.createElement("div");
            grid.classList.add("collectibleGrid");
            for (const data of collectibles) {
                let card = CreateCollectibleCard(data, isOwner);
                grid.appendChild(card);
            }
            DOM.contentDiv.appendChild(grid);
        }
        function switchToCollectiblesTab() {
            activeTab = "collectibles";
            history.replaceState(null, "", window.location.pathname + "#collection");
            DOM.btnCollectible.classList.add("active");
            DOM.btnPosts.classList.remove("active");
            const postCards = DOM.contentDiv.querySelectorAll(".postCard");
            postCards.forEach(p => (p as HTMLElement).style.display = "none");
            DOM.emptyContentDivPlaceHolder.style.display = "none";
            DOM.emptyContentDivPlaceHolder.classList.remove("clickable");
            DOM.emptyContentDivPlaceHolder.style.cursor = "default";
            DOM.addPostButton.style.display = "none";
            if (DOM.isGuest.value === "false") {
                DOM.mintNFTButton.style.display = "block";
            }
            displayCollectibles(DOM.injectedBlockchain.value, DOM.injectedAddress.value);
        }
        function switchToPostsTab() {
            activeTab = "posts";
            history.replaceState(null, "", window.location.pathname);
            DOM.btnPosts.classList.add("active");
            DOM.btnCollectible.classList.remove("active");
            const existingGrid = DOM.contentDiv.querySelector(".collectibleGrid");
            if (existingGrid) existingGrid.remove();
            const postCards = DOM.contentDiv.querySelectorAll(".postCard");
            postCards.forEach(p => (p as HTMLElement).style.display = "");
            DOM.emptyContentDivPlaceHolder.classList.add("clickable");
            DOM.emptyContentDivPlaceHolder.style.cursor = "";
            DOM.mintNFTButton.style.display = "none";
            DOM.addPostButton.style.display = "";
            renderGuestView();
        }

        // --------- Profile Data Helpers --------- //
        async function checkFollowStatus(profileAddress: string, profileBlockchain: string) {
            let userAddress = GetAddress();
            let userBlockchain = GetChain();
            if (!userAddress || !userBlockchain) {
                return false;
            }
            try {
                let response = await HttpGetJson(`/profile/isFollower/${profileBlockchain}/${profileAddress}/${userBlockchain}/${userAddress}`);
                if (response[0] === 200 && response[1]?.isFollower !== undefined) {
                    return response[1].isFollower;
                }
            } catch (error) {
                LogError("Failed to check follow status: " + error);
            }
            return false;
        }
        function updateFollowButton() {
            if (isFollowing) {
                DOM.followBtnLabel.textContent = "Unfollow";
                DOM.followImg.className = "bi-person-dash";
            } else {
                DOM.followBtnLabel.textContent = "Follow";
                DOM.followImg.className = "bi-person-plus";
            }
        }
        async function renderProfileFromCache(profileData: any, blockchain: string, address: string) {
            await renderProfileAddress(address);
            await renderProfileNameFromData(blockchain, address);
            await renderProfileAvatarFromData(blockchain, address);
            await renderProfileDescriptionFromData(profileData.description, blockchain, address);
            await renderProfileLocationFromData(profileData.location);
            await renderProfileVerticalFromData(profileData.vertical);
            await renderProfileWebsiteFromData(profileData.website);
            await renderProfileJoinedDateFromData(profileData.joinedDate);
            await renderProfileFollowerCountFromData(profileData.followerCount);
            await renderProfileFollowingCountFromData(profileData.followingCount);
            if (profileData.bannerAddress) {
                await renderProfileBannerFromData(profileData.bannerAddress);
            }
        }

        // --------- Profile Render --------- //
        async function renderProfileAddress(address: string) {
            let truncatedAddress = truncateAddress(address);
            let sanitizedAddress = XSSSanitizeValue(address);
            let explorerLink = XSSSanitizeUrl(WalletGetExplorerAddressLink(address, DOM.injectedBlockchain.value));
            if (DOM.profileAddressFull.value !== sanitizedAddress) {
                DOM.profileAddressFull.value = sanitizedAddress;
            }
            if (DOM.profileAddress.textContent !== truncatedAddress) {
                DOM.profileAddress.textContent = truncatedAddress;
            }
            if (DOM.profileAddressLink.href !== explorerLink) {
                DOM.profileAddressLink.href = explorerLink;
            }
        }
        async function renderGuestView() {
            const placeHolderNudgeHandler = () => {
                if (activeTab !== "posts") return;
                WalletSendPostNudge(DOM.injectedAddress.value).then();
            };
            const placeHolderAddPostHandler = () => {
                if (activeTab !== "posts") return;
                DOM.addPostButton.click();
            };
            let postCount = Number(DOM.postsNum.textContent);
            if (DOM.isGuest.value === "true") {
                isFollowing = await checkFollowStatus(DOM.injectedAddress.value, DOM.injectedBlockchain.value);
                updateFollowButton();
                DOM.profileEditBtn.style.display = "none";
                DOM.followBtn.style.display = "block";
                DOM.placeHolderH3.textContent = "Nothing posted yet";
                DOM.placeHolderP.textContent = "Click to send a nudge! Sometimes friends need a little encouragement to share";
                DOM.placeHolderIcon.classList.remove("bi-house-add");
                DOM.placeHolderIcon.classList.add("bi-envelope-paper-heart");
                DOM.emptyContentDivPlaceHolder.removeEventListener("click", placeHolderAddPostHandler);
                DOM.emptyContentDivPlaceHolder.addEventListener("click", placeHolderNudgeHandler);
            } else if (DOM.isGuest.value === "false") {
                DOM.profileEditBtn.style.display = "block";
                DOM.followBtn.style.display = "none";
                DOM.placeHolderH3.textContent = "Share your first post!";
                DOM.placeHolderP.textContent = "Your amazing thoughts belong here";
                DOM.placeHolderIcon.classList.remove("bi-envelope-paper-heart");
                DOM.placeHolderIcon.classList.add("bi-house-add");
                DOM.emptyContentDivPlaceHolder.removeEventListener("click", placeHolderNudgeHandler);
                DOM.emptyContentDivPlaceHolder.addEventListener("click", placeHolderAddPostHandler);
            }
            if (postCount > 0) {
                DOM.emptyContentDivPlaceHolder.style.display = "none";
            } else {
                DOM.emptyContentDivPlaceHolder.style.display = "flex";
            }
        }

        // --------- Exported Functions --------- //
        window.CollectibleMintCallback = function () {
            if (activeTab === "collectibles") {
                displayCollectibles(DOM.injectedBlockchain.value, DOM.injectedAddress.value);
            }
        }
        window.DisconnectWalletCallback = function () {
            LogInfo("profile.ts DisconnectWalletCallback() stub - redirecting to logout");
            window.location.href = "/logout";
        }
        window.LoginCallback = function (status: string) {
            let address = GetAddress();
            if (address == null || !IsValidAddress(address)) {
                return;
            }
            updateProfile().then();
        }
        window.PageReloadCallback = function () {
            LogInfo("profile.ts PageReloadCallback() stub - reloading page");
            window.location.reload();
        }
        window.PostSubmitCallback = function () {
            LogInfo("profile.ts PostSubmitCallback() - refreshing profile data");
            refreshProfileData().then();
        }

        // --------- Cache Render Helper Functions --------- //
        async function renderProfileNameFromData(blockchain: string, address: string) {
            let name = await WalletGetName(blockchain, address);
            if (!name || name.length === 0) {
                name = "Anonymous";
            }
            if (DOM.profileName.textContent === name) {
                return; // No change, skip DOM updates
            }
            DOM.profileName.textContent = name;
            const updatePostAuthors = () => {
                const avatarAuthors = document.querySelectorAll("b.postCardAuthor");
                if (avatarAuthors.length > 0 && typeof name === "string") {
                    avatarAuthors.forEach((b: Element) => {
                        if (b instanceof HTMLElement) {
                            b.textContent = name;
                        }
                    });
                }
            };
            const waitForPostAuthors = (attempts = 0, maxAttempts = 50) => {
                if (attempts >= maxAttempts) return;
                const avatarAuthors = document.querySelectorAll("b.postCardAuthor");
                if (avatarAuthors.length > 0) {
                    updatePostAuthors();
                } else {
                    setTimeout(() => waitForPostAuthors(attempts + 1, maxAttempts), 100);
                }
            };
            waitForPostAuthors();
        }
        async function renderProfileAvatarFromData(blockchain: string, address: string) {
            let avatarURL = await WalletGetAvatar(blockchain, address);
            if (!avatarURL || avatarURL === "") {
                avatarURL = await getIpfsAvatarUrl(blockchain, address) || "";
            }
            // Convert ipfs:// URLs to HTTP gateway URLs
            if (avatarURL && avatarURL.startsWith("ipfs://")) {
                avatarURL = CIDToSubdomainURL(avatarURL);
            }
            const defaultAvatar = "/static/image/avatar.png";
            let finalAvatarUrl = defaultAvatar;
            if (IsValidURL(avatarURL)) {
                finalAvatarUrl = XSSSanitizeUrl(avatarURL);
            } else if (IsValidIpfsCid(avatarURL)) {
                const ipfsURL = CIDToSubdomainURL(avatarURL);
                if (ipfsURL) {
                    finalAvatarUrl = XSSSanitizeUrl(ipfsURL);
                }
            }
            if (DOM.profileAvatar.src !== finalAvatarUrl && !DOM.profileAvatar.src.endsWith(finalAvatarUrl)) {
                DOM.profileAvatar.onerror = () => {
                    DOM.profileAvatar.src = defaultAvatar;
                    DOM.profileAvatar.onerror = null;
                };
                DOM.profileAvatar.src = finalAvatarUrl;
            }
        }
        async function renderProfileDescriptionFromData(description: string, blockchain: string, address: string) {
            let finalDescription = description;
            if (!description || description.length === 0) {
                try {
                    const ensDescription = await WalletGetDescription(blockchain, address);
                    if (ensDescription && ensDescription.length > 0) {
                        finalDescription = ensDescription;
                    }
                } catch (e) {
                    console.warn("Failed to get ENS description for profile:", e);
                }
            }
            if (finalDescription && finalDescription.length > 0 && DOM.profileDescription.textContent !== finalDescription) {
                DOM.profileDescription.textContent = finalDescription;
                processTextWithTags(DOM.profileDescription);
            }
        }
        async function renderProfileLocationFromData(location: string) {
            if (location && location.length > 0) {
                if (DOM.profileLocation.textContent !== location) {
                    DOM.profileLocation.textContent = location;
                }
                DOM.profileLocation.parentElement?.classList.remove("hidden");
            } else {
                DOM.profileLocation.parentElement?.classList.add("hidden");
            }
        }
        async function renderProfileVerticalFromData(vertical: string) {
            if (vertical && vertical.length > 0) {
                const displayVertical = vertical.charAt(0).toUpperCase() + vertical.slice(1);
                if (DOM.profileVertical.textContent !== displayVertical) {
                    DOM.profileVertical.textContent = displayVertical;
                }
                DOM.profileVertical.parentElement?.classList.remove("hidden");
            } else {
                DOM.profileVertical.parentElement?.classList.add("hidden");
            }
        }
        async function renderProfileWebsiteFromData(website: string) {
            if (website && website.length > 0) {
                let href: string;
                let displayText: string;
                try {
                    const url = new URL(website.startsWith('http') ? website : `https://${website}`);
                    href = XSSSanitizeUrl(url.href);
                    displayText = url.hostname;
                } catch {
                    href = XSSSanitizeUrl(`https://${website}`);
                    displayText = website;
                }
                if (DOM.profileWebsite.href !== href) {
                    DOM.profileWebsite.href = href;
                }
                if (DOM.profileWebsite.textContent !== displayText) {
                    DOM.profileWebsite.textContent = displayText;
                }
                DOM.profileWebsite.parentElement?.parentElement?.classList.remove("hidden");
            } else {
                DOM.profileWebsite.parentElement?.parentElement?.classList.add("hidden");
            }
        }
        async function renderProfileJoinedDateFromData(joinedDate: number) {
            if (joinedDate && joinedDate > 0) {
                const joinedDateFormatted = new Date(joinedDate * 1000).toLocaleDateString(undefined, {
                    month: 'short',
                    day: 'numeric',
                    year: 'numeric'
                });
                if (DOM.profileJoined.textContent !== joinedDateFormatted) {
                    DOM.profileJoined.textContent = joinedDateFormatted;
                }
                DOM.profileJoined.parentElement?.classList.remove("hidden");
            } else {
                DOM.profileJoined.parentElement?.classList.add("hidden");
            }
        }
        async function renderProfileFollowerCountFromData(followerCount: number) {
            const count = followerCount ? followerCount.toString() : "0";
            if (DOM.followerCount.textContent !== count) {
                DOM.followerCount.textContent = count;
            }
        }
        async function renderProfileFollowingCountFromData(followingCount: number) {
            const count = followingCount ? followingCount.toString() : "0";
            if (DOM.followingCount.textContent !== count) {
                DOM.followingCount.textContent = count;
            }
        }
        async function renderProfileBannerFromData(bannerAddress: string) {
            if (bannerAddress && bannerAddress.length > 0) {
                let bannerURL = bannerAddress;
                if (bannerAddress.startsWith("ipfs://")) {
                    bannerURL = CIDToSubdomainURL(bannerAddress);
                }
                if (IsValidURL(bannerURL)) {
                    const success = await loadImageWithTimeout(bannerURL, 10000);
                    if (success) {
                        const sanitizedUrl = XSSSanitizeUrl(bannerURL);
                        if (DOM.profileBanner.src !== sanitizedUrl && !DOM.profileBanner.src.endsWith(sanitizedUrl)) {
                            DOM.profileBanner.src = sanitizedUrl;
                        }
                    } else {
                        if (!DOM.profileBanner.src.endsWith("/static/image/banner.jpg")) {
                            DOM.profileBanner.src = "/static/image/banner.jpg";
                        }
                        LogError(`Banner failed to load, using default`);
                    }
                }
            }
        }

        // --------- Helper Functions --------- //
        function truncateAddress(address: string) {
            let length = address.length;
            let first = address.slice(0, 6);
            let middle = "...";
            let endIndex = length - 6;
            let end = address.slice(endIndex, length);
            return first + middle + end;
        }

        // --------- Event Handlers --------- //
        DOM.btnCollectible.addEventListener("click", function () {
            if (activeTab !== "collectibles") switchToCollectiblesTab();
        });
        DOM.btnPosts.addEventListener("click", function () {
            if (activeTab === "posts") {
                window.location.href = "/p/";
            } else {
                switchToPostsTab();
            }
        });
        DOM.btnSearch.addEventListener("click", function () {
            window.location.href = "/";
        });
        DOM.contentDiv.addEventListener("click", async (e) => {
            const burnBtn = (e.target as HTMLElement).closest(".collectibleBurnBtn");
            if (burnBtn) {
                const card = burnBtn.closest(".collectibleCard") as HTMLDivElement;
                if (!card) return;
                const tokenId = (card.querySelector(".collectibleTokenId") as HTMLInputElement).value;
                const blockchain = (card.querySelector(".collectibleBlockchain") as HTMLInputElement).value;
                ShowDialogModalWithCallback("Are you sure you want to burn this Collectible? This cannot be undone.", async () => {
                    const success = await WalletBurnCollectible(tokenId, blockchain);
                    if (success) {
                        ShowToast("Collectible burned");
                        displayCollectibles(DOM.injectedBlockchain.value, DOM.injectedAddress.value);
                    } else {
                        ShowToast("Failed to burn collectible");
                    }
                });
                return;
            }
            const sendBtn = (e.target as HTMLElement).closest(".collectibleSendBtn");
            if (sendBtn) {
                const card = sendBtn.closest(".collectibleCard") as HTMLDivElement;
                if (!card) return;
                const tokenId = (card.querySelector(".collectibleTokenId") as HTMLInputElement).value;
                const blockchain = (card.querySelector(".collectibleBlockchain") as HTMLInputElement).value;
                const cardName = card.querySelector(".collectibleCardName");
                const cardMedia = card.querySelector(".collectibleMediaElement") as HTMLImageElement | HTMLVideoElement;
                const transferTokenIdInput = document.getElementById("transferNFTTokenId") as HTMLInputElement;
                const transferBlockchainInput = document.getElementById("transferNFTBlockchain") as HTMLInputElement;
                const transferPreviewDiv = document.getElementById("transferNFTPreviewDiv") as HTMLDivElement;
                const transferAddressInput = document.getElementById("transferNFTAddress") as HTMLInputElement;
                const transferFeeEstimate = document.getElementById("transferNFTFeeEstimate") as HTMLSpanElement;
                const transferConfirmBtn = document.getElementById("transferNFTConfirmBtn") as HTMLButtonElement;
                const transferAddressValid = document.getElementById("transferNFTAddressValid") as HTMLSpanElement;
                transferTokenIdInput.value = tokenId;
                transferBlockchainInput.value = blockchain;
                transferAddressInput.value = "";
                transferFeeEstimate.textContent = "--";
                transferConfirmBtn.disabled = true;
                transferAddressValid.textContent = "";
                transferPreviewDiv.innerHTML = "";
                if (cardMedia) {
                    let thumb = document.createElement("img");
                    thumb.src = (cardMedia as HTMLImageElement).src || "";
                    transferPreviewDiv.appendChild(thumb);
                }
                if (cardName) {
                    let nameSpan = document.createElement("span");
                    nameSpan.textContent = cardName.textContent || "";
                    transferPreviewDiv.appendChild(nameSpan);
                }
                if (blockchain === "algorand") {
                    transferAddressInput.placeholder = "ALGO...";
                } else {
                    transferAddressInput.placeholder = "0x...";
                }
                let debounceTimer: ReturnType<typeof setTimeout> | null = null;
                const addressHandler = () => {
                    if (debounceTimer) clearTimeout(debounceTimer);
                    debounceTimer = setTimeout(async () => {
                        const addr = transferAddressInput.value.trim();
                        let valid = false;
                        if (blockchain === "algorand") {
                            valid = IsValidAlgoAddress(addr);
                        } else {
                            valid = IsValidBaseAddress(addr);
                        }
                        if (valid) {
                            transferAddressValid.textContent = "\u2713";
                            transferAddressValid.style.color = "var(--yp-success)";
                            transferConfirmBtn.disabled = false;
                            const fee = await WalletGetTransferFeeEstimate(addr, tokenId, blockchain);
                            transferFeeEstimate.textContent = fee;
                        } else {
                            transferAddressValid.textContent = addr.length > 0 ? "\u2717" : "";
                            transferAddressValid.style.color = "var(--yp-danger)";
                            transferConfirmBtn.disabled = true;
                            transferFeeEstimate.textContent = "--";
                        }
                    }, 500);
                };
                transferAddressInput.removeEventListener("input", addressHandler);
                transferAddressInput.addEventListener("input", addressHandler);
                const confirmHandler = async () => {
                    const toAddress = transferAddressInput.value.trim();
                    transferConfirmBtn.disabled = true;
                    transferConfirmBtn.textContent = "Transferring...";
                    const success = await WalletTransferCollectible(tokenId, toAddress, blockchain);
                    if (success) {
                        ShowToast("Collectible transferred!");
                        const transferModal = window.bootstrap.Modal.getInstance(document.getElementById("modalTransferNFT")!);
                        if (transferModal) transferModal.hide();
                        displayCollectibles(DOM.injectedBlockchain.value, DOM.injectedAddress.value);
                    } else {
                        ShowToast("Failed to transfer collectible");
                    }
                    transferConfirmBtn.disabled = false;
                    transferConfirmBtn.textContent = "Confirm";
                };
                transferConfirmBtn.onclick = confirmHandler;
                const transferModalEl = document.getElementById("modalTransferNFT")!;
                const transferModal = window.bootstrap.Modal.getOrCreateInstance(transferModalEl);
                transferModal.show();
                return;
            }
        });
        DOM.profileEditBtn.addEventListener("click", showProfileEditModal);
        DOM.profileAddressCopy.addEventListener("click", function () {
            let address = GetAddress();
            if (!address || !IsValidAddress(address)) {
                return
            }
            navigator.clipboard.writeText(address).then();
            copiedTooltip.show();
            setTimeout(() => {
                copiedTooltip.hide();
            }, 1000);
        });
        DOM.followBtn.addEventListener("click", async function() {
            if (!GetWallet()) {
                window.location.href = "/login";
                return;
            }
            let toAddress = DOM.profileAddressFull.value;
            if (!toAddress || !IsValidAddress(toAddress)) {
                return;
            }
            let toBlockchain = DOM.injectedBlockchain.value;
            try {
                if (isFollowing) {
                    await WalletUnfollowUser(toAddress, toBlockchain);
                    isFollowing = false;
                } else {
                    await WalletFollowUser(toAddress, toBlockchain);
                    isFollowing = true;
                }
                updateFollowButton();
            } catch (error) {
                LogError("Failed to update follow status: " + error);
            }
        });

        // Cleanup auto-refresh on page unload
        window.addEventListener("beforeunload", stopAutoRefresh);
        document.addEventListener("visibilitychange", () => {
            if (document.hidden) {
                stopAutoRefresh();
            } else {
                startAutoRefresh();
            }
        });

        init().then(() => {
            if (window.location.hash === "#collection") {
                switchToCollectiblesTab();
            }
            preloadTinyMCE();
            startAutoRefresh();
        });
    }
})();
