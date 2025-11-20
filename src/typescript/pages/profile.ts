window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/global.scss"
import "../../scss/pages/profile.scss";
import "../components/addPost";
import "../components/modalDialog";
import "../components/scrollTop";
import "../components/menu";
import {LogError, LogInfo} from "../util/log";
import {HttpGetJson} from "../util/network";
import {showProfileEditModal} from "../components/modalProfileEdit";
import {FetchPosts} from "../components/post";
import {ShowNotifications} from "../util/notifications";
import {GetAddress, WalletGetExplorerAddressLink, IsValidAddress, WalletGetAvatar, WalletGetName, WalletSendPostNudge, WalletFollowUser, WalletUnfollowUser, GetChain} from "../util/blockchain/wallet";
import {CreatePostCard} from "../util/domFactory";
import {IsValidURL, IsValidIpfsCid, XSSSanitizeUrl, XSSSanitizeValue} from "../util/security";
import {CIDToSubdomainURL, loadImageWithTimeout} from "../util/ipfs";
import PersistentCache from "../util/cache";

declare global { // Extend the window interface with public objects
    interface Window {
        LoginCallback: (status: string) => void;
        PageReloadCallback: () => void;
        DisconnectWalletCallback: () => void;
    }
}

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            addPostButton: document.getElementById("addPostButton")! as HTMLButtonElement,
            avatarPreview: document.getElementById("avatarPreview")! as HTMLImageElement,
            bannerPreview: document.getElementById("bannerPreview")! as HTMLImageElement,
            btnFiles: document.getElementById("btnFiles")! as HTMLButtonElement,
            btnNFTs: document.getElementById("btnNFTs")! as HTMLButtonElement,
            btnSearch: document.getElementById("btnSearch")! as HTMLButtonElement,
            btnPosts: document.getElementById("btnPosts")! as HTMLButtonElement,
            btnComments: document.getElementById("btnComments")! as HTMLButtonElement,
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
            profileWebsite: document.getElementById("profileWebsite")! as HTMLAnchorElement,
            profileBirthdate: document.getElementById("profileBirthdate")! as HTMLDivElement,
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
        }
        let copiedTooltip: any;
        let isFollowing = false;
        const profileCache = new PersistentCache({
            defaultTtl: 3600000, // 60 minutes
            keyPrefix: "profile_"
        });

        // --------- Page Functions --------- //
        async function init() {
            let tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
            tooltipTriggerList.map(function (tooltipTriggerEl) {return new window.bootstrap.Tooltip(tooltipTriggerEl, {delay: {show: 1500, hide: 0}});}); // enable tooltips
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
            const profileKey = `${requestedBlockchain}_${requestedAddress}`;
            const cached = profileCache.get(profileKey);
            if (cached && cached.profileData) {
                await displayPosts(requestedBlockchain, requestedAddress);
                await renderProfileFromCache(cached.profileData, requestedBlockchain, requestedAddress);
                await renderGuestView();
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
                    profileCache.set(profileKey, { profileData: response[1].profileData });
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
        async function displayPosts(blockchain: string, address: string) { // adds posts to the DOM
            let posts = await FetchPosts(blockchain, address);
            if (!posts || posts.length == 0) {
                DOM.postsNum.textContent = "0";
                return;
            }
            DOM.postsNum.textContent = String(posts.length);
            for (let i = 0; i < posts.length; i++) {
                let postDiv = await CreatePostCard(posts[i]);
                if (i % 2 === 0) {
                    postDiv.classList.add("shaded");
                }
                DOM.contentDiv.appendChild(postDiv);
            }
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
            await renderProfileNameFromData(profileData.name, blockchain, address);
            await renderProfileAvatarFromData(profileData.avatarAddress, blockchain, address);
            await renderProfileDescriptionFromData(profileData.description);
            await renderProfileLocationFromData(profileData.location);
            await renderProfileWebsiteFromData(profileData.website);
            await renderProfileBirthdateFromData(profileData.birthdate);
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
            DOM.profileAddressFull.value = XSSSanitizeValue(address);
            DOM.profileAddress.textContent = truncatedAddress;
            DOM.profileAddressLink.href = XSSSanitizeUrl(WalletGetExplorerAddressLink(address));
        }
        async function renderGuestView() {
            // Edit the profile view, depending on if the viewer is the owner of the profile or not
            const placeHolderNudgeHandler = () => {
                WalletSendPostNudge(DOM.injectedAddress.value).then();
            };
            const placeHolderAddPostHandler = () => {
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
                DOM.emptyContentDivPlaceHolder.style.display = "block";
            }
        }

        // --------- Exported Functions --------- //
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
        window.DisconnectWalletCallback = function () {
            LogInfo("profile.ts DisconnectWalletCallback() stub - redirecting to logout");
            window.location.href = "/logout";
        }

        // --------- Cache Render Helper Functions --------- //
        async function renderProfileNameFromData(cachedName: string, blockchain: string, address: string) {
            // First try ENS lookup (same as original renderProfileName)
            let name = await WalletGetName(blockchain, address);
            if (name === null || name.length === 0) {
                // Fall back to cached database name
                name = cachedName && cachedName.length > 0 ? cachedName : truncateAddress(address);
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
        async function renderProfileAvatarFromData(avatarAddress: string, blockchain: string, address: string) {
            let avatarURL = avatarAddress;
            if (!avatarAddress) { // If no cached avatar, try blockchain lookup
                avatarURL = await WalletGetAvatar(blockchain, address);
            }
            DOM.profileAvatar.src = "/static/image/avatar.png"; // Clear any existing avatar loading to prevent memory leaks
            if (IsValidURL(avatarURL)) {
                const success = await loadImageWithTimeout(avatarURL, 3000);
                if (success) {
                    DOM.profileAvatar.src = XSSSanitizeUrl(avatarURL);
                } else {
                    LogError(`Avatar failed to load, using default: ${avatarURL}`);
                }
            } else if (IsValidIpfsCid(avatarURL)) {
                const ipfsURL = CIDToSubdomainURL(avatarURL);
                if (ipfsURL) {
                    const success = await loadImageWithTimeout(ipfsURL, 3000);
                    if (success) {
                        DOM.profileAvatar.src = XSSSanitizeUrl(ipfsURL);
                    } else {
                        LogError(`IPFS avatar failed to load, using default: ${ipfsURL}`);
                    }
                }
            }
        }
        async function renderProfileDescriptionFromData(description: string) {
            if (description && description.length > 0) {
                DOM.profileDescription.textContent = description;
            }
        }
        async function renderProfileLocationFromData(location: string) {
            if (location && location.length > 0) {
                DOM.profileLocation.textContent = location;
                DOM.profileLocation.parentElement?.classList.remove("hidden");
            } else {
                DOM.profileLocation.parentElement?.classList.add("hidden");
            }
        }
        async function renderProfileWebsiteFromData(website: string) {
            if (website && website.length > 0) {
                try {
                    const url = new URL(website.startsWith('http') ? website : `https://${website}`);
                    DOM.profileWebsite.href = XSSSanitizeUrl(url.href);
                    DOM.profileWebsite.textContent = url.hostname;
                } catch {
                    DOM.profileWebsite.href = XSSSanitizeUrl(`https://${website}`);
                    DOM.profileWebsite.textContent = website;
                }
                DOM.profileWebsite.parentElement?.parentElement?.classList.remove("hidden");
            } else {
                DOM.profileWebsite.parentElement?.parentElement?.classList.add("hidden");
            }
        }
        async function renderProfileBirthdateFromData(birthdate: number) {
            if (birthdate && birthdate > 0) {
                const birthdateFormatted = new Date(birthdate * 1000).toLocaleDateString(undefined, {
                    month: 'short',
                    day: 'numeric',
                    year: 'numeric'
                });
                DOM.profileBirthdate.textContent = birthdateFormatted;
                DOM.profileBirthdate.parentElement?.classList.remove("hidden");
            } else {
                DOM.profileBirthdate.parentElement?.classList.add("hidden");
            }
        }
        async function renderProfileJoinedDateFromData(joinedDate: number) {
            if (joinedDate && joinedDate > 0) {
                const joinedDateFormatted = new Date(joinedDate * 1000).toLocaleDateString(undefined, {
                    month: 'short',
                    day: 'numeric',
                    year: 'numeric'
                });
                DOM.profileJoined.textContent = joinedDateFormatted;
                DOM.profileJoined.parentElement?.classList.remove("hidden");
            } else {
                DOM.profileJoined.parentElement?.classList.add("hidden");
            }
        }
        async function renderProfileFollowerCountFromData(followerCount: number) {
            DOM.followerCount.textContent = followerCount ? followerCount.toString() : "0";
        }
        async function renderProfileFollowingCountFromData(followingCount: number) {
            DOM.followingCount.textContent = followingCount ? followingCount.toString() : "0";
        }
        async function renderProfileBannerFromData(bannerAddress: string) {
            if (bannerAddress && bannerAddress.length > 0) {
                let bannerURL = bannerAddress;
                if (bannerAddress.startsWith("ipfs://")) {
                    bannerURL = CIDToSubdomainURL(bannerAddress);
                }
                if (IsValidURL(bannerURL)) {
                    const success = await loadImageWithTimeout(bannerURL, 3000);
                    if (success) {
                        DOM.profileBanner.src = XSSSanitizeUrl(bannerURL);
                    } else {
                        DOM.profileBanner.src = "/static/image/banner.jpg";
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
        DOM.btnPosts.addEventListener("click", function () {
            window.location.href = "/p/";
        });
        DOM.btnSearch.addEventListener("click", function () {
            window.location.href = "/";
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

        init().then();
    }
})();
