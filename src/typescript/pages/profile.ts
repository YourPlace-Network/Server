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
import {GetToasts} from "../components/toast";
import {GetAddress, WalletGetExplorerAddressLink, IsValidAddress, WalletGetAvatar, WalletGetName, WalletGetDescription, WalletGetLocation, WalletGetWebsite, WalletSendPostNudge, WalletFollowUser} from "../util/blockchain/wallet";
import {CreatePostCard} from "../util/domFactory";
import {IsValidURL, IsValidIpfsCid, XSSSanitizeUrl, XSSSanitizeValue} from "../util/security";
import {CIDToSubdomainURL, GetIPFSFile} from "../util/ipfs";

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

        // --------- Page Functions --------- //
        async function init() {
            let tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
            tooltipTriggerList.map(function (tooltipTriggerEl) {return new window.bootstrap.Tooltip(tooltipTriggerEl, {delay: {show: 1500, hide: 0}});}); // enable tooltips
            await updateProfile();
            await GetToasts();
            copiedTooltip = new window.bootstrap.Tooltip(DOM.profileAddressCopy, {title: "Copied", trigger: "manual", placement: "right"});
        }
        async function updateProfile() {
            console.log("test");
            let requestedAddress = DOM.injectedAddress.value;
            LogInfo("address: " + requestedAddress);
            let requestedBlockchain = DOM.injectedBlockchain.value;
            if (!requestedAddress || !IsValidAddress(requestedAddress, requestedBlockchain)) {
                LogError("profile.ts updateProfile() - invalid address");
                return;
            }
            await allSettledWithTimeout([
                displayPosts(requestedBlockchain, requestedAddress),
                renderProfileAddress(requestedAddress),
                renderProfileName(requestedBlockchain, requestedAddress),
                renderProfileAvatar(requestedBlockchain, requestedAddress),
                renderProfileBanner(requestedBlockchain, requestedAddress),
                renderProfileDescription(requestedBlockchain, requestedAddress),
                renderProfileLocation(requestedBlockchain, requestedAddress),
                renderProfileWebsite(requestedBlockchain, requestedAddress),
                renderProfileBirthdate(requestedBlockchain, requestedAddress),
                renderProfileJoinedDate(requestedBlockchain, requestedAddress),
                renderProfileFollowerCount(requestedBlockchain, requestedAddress),
            ], 60000);
            renderGuestView().then();
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

        // --------- Profile Render --------- //
        async function renderProfileAddress(address: string) {
            let truncatedAddress = truncateAddress(address);
            DOM.profileAddressFull.value = XSSSanitizeValue(address);
            DOM.profileAddress.textContent = truncatedAddress;
            DOM.profileAddressLink.href = XSSSanitizeUrl(WalletGetExplorerAddressLink(address));
        }
        async function renderProfileName(blockchain: string, address: string) {
            let name = await WalletGetName(blockchain, address);
            if (name === null || name.length === 0) {
                let response = await HttpGetJson("/profile/name/" + blockchain + "/" + address);
                if (response[0] === 200) {
                    if (!response[1] || response[1].name.length < 1) {
                        return;
                    }
                    name = response[1].name;
                }
            }
            if (typeof name === "string") {
                DOM.profileName.textContent = name;
            }
            const avatarAuthors = document.querySelectorAll("b.postCardAuthor");
            avatarAuthors.forEach((b: Element) => {
                if (b instanceof HTMLElement) {
                    if (typeof name === "string") {
                        b.textContent = name;
                    } // Set the post authors
                }
            });
        }
        async function renderProfileAvatar(blockchain: string, address: string) {
            let avatarURL = await WalletGetAvatar(blockchain, address); // get the avatar from the blockchain
            if (IsValidURL(avatarURL)) {
                DOM.profileAvatar.src = XSSSanitizeUrl(avatarURL);
                populatePostCards();
                return;
            } else if (IsValidIpfsCid(avatarURL)) {
                avatarURL = CIDToSubdomainURL(avatarURL);
                DOM.profileAvatar.src = XSSSanitizeUrl(avatarURL);
                return;
            }
        }
        async function renderProfileBanner(blockchain: string, address: string) {
            const response = await HttpGetJson("/profile/banner/" + blockchain + "/" + address);
            if (response[0] === 200 && response[1]?.bannerAddress) {
                let bannerAddress = response[1].bannerAddress;
                if (bannerAddress.startsWith("ipfs://")) {
                    bannerAddress = CIDToSubdomainURL(bannerAddress);
                    if (bannerAddress) {
                        const img = new Image();
                        img.crossOrigin = "anonymous";
                        img.onload = () => {
                            DOM.profileBanner.src = img.src;
                        };
                        img.onerror = () => {
                            LogError("Failed to load banner image");
                        };
                        img.src = bannerAddress; // Start the loading process
                    } else {
                        LogError("Invalid banner address");
                    }
                } else if (IsValidURL(bannerAddress)) {
                    DOM.profileBanner.src = XSSSanitizeUrl(bannerAddress);
                }
            }
        }
        async function renderProfileDescription(blockchain: string, address: string) {
            let description = await WalletGetDescription(blockchain, address);
            if (description != null && description.length > 0) {
                DOM.profileDescription.textContent = description;
            } else {
                let response = await HttpGetJson("/profile/description/" + blockchain + "/" + address);
                if (response[0] === 200) {
                    if (!response[1] || response[1].description.length < 1) {
                        return;
                    }
                    DOM.profileDescription.textContent = response[1].description;
                }
            }
        }
        async function renderProfileLocation(blockchain: string, address: string) {
            let location = await WalletGetLocation(blockchain, address);
            if (location != null && location.length > 0) {
                DOM.profileLocation.textContent = location;
            } else {
                let response = await HttpGetJson("/profile/location/" + blockchain + "/" + address);
                if (response[0] === 200) {
                    if (!response[1] || response[1].location.length < 1) {
                        return;
                    }
                    DOM.profileLocation.textContent = response[1].location;
                }
            }
        }
        async function renderProfileWebsite(blockchain: string, address: string) {
            let website = await WalletGetWebsite(blockchain, address);
            if (website != null && website.href.length > 0) {
                DOM.profileWebsite.href = XSSSanitizeUrl(website.href);
                DOM.profileWebsite.textContent = website.hostname;
            } else {
                let response = await HttpGetJson("/profile/website/" + blockchain + "/" + address);
                if (response[0] === 200) {
                    if (!response[1] || response[1].website.length < 1) {
                        return;
                    }
                    let website = response[1].website;
                    let url: string
                    try {
                        const check = new URL(website);
                        url = website;
                    } catch {
                        url = "https://" + website;
                    }
                    DOM.profileWebsite.href = XSSSanitizeUrl(url);
                    DOM.profileWebsite.textContent = website;
                }
            }
        }
        async function renderProfileBirthdate(blockchain: string, address: string) {
            let response = await HttpGetJson("/profile/birthdate/" + blockchain + "/" + address);
            if (response[0] === 200) {
                let birthdate = response[1].birthdate;
                if (!birthdate || birthdate == 0) {
                    return;
                }
                let birthdateformatted = new Date(birthdate * 1000).toLocaleDateString(undefined, {
                    month: 'short',
                    day: 'numeric',
                    year: 'numeric'
                });
                DOM.profileBirthdate.textContent = birthdateformatted.toString();
            }
        }
        async function renderProfileJoinedDate(blockchain: string, address: string) {
            let response = await HttpGetJson("/profile/joinedDate/" + blockchain + "/" + address);
            if (response[0] === 200) {
                let joinedDate = response[1].joinedDate;
                if (!joinedDate || joinedDate == 0) {
                    return;
                }
                let joineddateformatted = new Date(joinedDate * 1000).toLocaleDateString(undefined, {
                    month: 'short',
                    day: 'numeric',
                    year: 'numeric'
                });
                DOM.profileJoined.textContent = joineddateformatted;
            }
        }
        async function renderProfileFollowerCount(blockchain: string, address: string) {
            let response = await HttpGetJson("/profile/followerCount/" + blockchain + "/" + address);
            if (response[0] === 200) {
                let followerCount = response[1].followerCount;
                DOM.followerCount.textContent = followerCount;
            }
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

        // --------- Helper Functions --------- //
        function truncateAddress(address: string) {
            let length = address.length;
            let first = address.slice(0, 6);
            let middle = "...";
            let endIndex = length - 6;
            let end = address.slice(endIndex, length);
            return first + middle + end;
        }
        function populatePostCards() {
            let avatarURL = DOM.profileAvatar.src;
            // Update post avatars
            const avatarImages = document.querySelectorAll("img.postCardAvatar");
            avatarImages.forEach((img: Element) => {
                if (img instanceof HTMLImageElement) {
                    img.src = XSSSanitizeUrl(avatarURL);
                }
            });
        }
        const timeoutPromise = (timeoutMs: number): Promise<never> => {
            return new Promise((_, reject) => {
                setTimeout(() => {
                    reject(new Error("Promise timed out"));
                }, timeoutMs);
            });
        };
        const allSettledWithTimeout = async <T>(promises: Promise<T>[], timeoutMs: number): Promise<PromiseSettledResult<T>[]> => {
            try {
                return await Promise.race([
                    Promise.allSettled(promises),
                    timeoutPromise(timeoutMs).then(() => {
                        throw new Error(`Operation timed out after ${timeoutMs}ms`);
                    })
                ]);
            } catch (error) {
                // If timeout occurs, return partially settled promises
                console.error("Operation timed out:", error);
                // Return array of settled results, with pending ones marked as rejected with timeout reason
                return promises.map((_, index) => ({
                    status: "rejected" as const,
                    reason: new Error(`Promise #${index} timed out after ${timeoutMs}ms`)
                }));
            }
        };

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
        DOM.followBtn.addEventListener("click", function() {
            let toAddress = DOM.profileAddressFull.value;
            if (!toAddress || !IsValidAddress(toAddress)) {
                return;
            }
            let toBlockchain = DOM.injectedBlockchain.value;
            WalletFollowUser(toAddress, toBlockchain).then();
        });

        init().then();
    }
})();
