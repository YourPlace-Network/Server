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
import {XSSSanitizeUrl, XSSSanitizeValue} from "../util/security";

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
            followingNum: document.getElementById("followingNum")! as HTMLDivElement,
            followersNum: document.getElementById("followersNum")! as HTMLDivElement,
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
            let requestedAddress = DOM.injectedAddress.value;
            LogInfo("address: " + requestedAddress);
            let requestedBlockchain = DOM.injectedBlockchain.value;
            if (!requestedAddress || !IsValidAddress(requestedAddress, requestedBlockchain)) {
                LogError("profile.ts updateProfile() - invalid address");
                return;
            }
            try {
                await Promise.allSettled([
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
                ]);
                guestView().then(); // post rendering, non-concurrent, view changes
            } catch (error) {
                LogError("profile.ts updateProfile() - error: " + error);
            }
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
        async function guestView() {
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
            let avatar = "";
            if (DOM.profileAvatar.src.endsWith("/static/image/avatar.png")) { // If you detect the default avatar
                let avatarURL = await WalletGetAvatar(blockchain, address);// get the avatar from the blockchain
                if (avatarURL != null && avatarURL.toString().length > 5) {// If the avatar is not null and has a length greater than 5
                    avatar = avatarURL.toString(); // set the avatar to the blockchain avatar
                } else {
                    // Get the avatar from the server database
                    let response = await HttpGetJson("/profile/avatar/" + blockchain + "/" + address);
                    if (response[0] === 200) {
                        if (!response[1] || response[1].avatarAddress.length < 1) {
                            return;
                        }
                        const ipfsPort = parseInt(window.location.port, 10) + 2;
                        avatar = response[1].avatarAddress.replace('ipfs://', `${window.location.protocol}//${window.location.hostname}:${ipfsPort}/ipfs/`);
                    }
                }
                // Set the avatar
                DOM.profileAvatar.src = XSSSanitizeUrl(avatar); // Set the main avatar
                const avatarImages = document.querySelectorAll("img.postCardAvatar");
                avatarImages.forEach((img: Element) => {
                    if (img instanceof HTMLImageElement) {
                            img.src = XSSSanitizeUrl(avatar); // Set the post avatars
                    }
                });
            }
        }
        async function renderProfileBanner(blockchain: string, address: string) {
            const url = `/profile/banner/${blockchain}/${address}`;
            const response = await HttpGetJson(url);
            console.log(response[1]);

            if (response[0] === 200) {
                const ipfsPort = parseInt(window.location.port, 10) + 2;
                const ipfsAddress = response[1].bannerAddress;

                if (ipfsAddress.startsWith("ipfs://")) {
                    // Extract the CID, preserving case sensitivity
                    const cid = ipfsAddress.substring(7).split("/")[0];
                    const pathPart = ipfsAddress.substring(7 + cid.length);

                    // Create URL without using subdomain format - use path-based approach instead
                    const pathUrl = `${window.location.protocol}//${window.location.hostname}:${ipfsPort}/ipfs/${cid}${pathPart}`;

                    console.log("Using path-based gateway URL:", pathUrl);
                    DOM.profileBanner.src = pathUrl;

                    DOM.profileBanner.onerror = (e) => {
                        console.error("Failed to load image:", e);
                    };
                }
            }

            console.log("renderProfileBanner finished");
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
            let response = await HttpGetJson("/profile/joineddate/" + blockchain + "/" + address);
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
            let selectedAddress = DOM.profileAddressFull.value;
            if (!selectedAddress || !IsValidAddress(selectedAddress)) {
                return;
            }
            WalletFollowUser(selectedAddress).then();
        });

        init().then();
    }
})();
