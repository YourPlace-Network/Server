import "bootstrap/dist/js/bootstrap.bundle";
import "../../scss/components/menu.scss";
import {DisconnectWallet, GetAddress, GetChain, WalletGetAvatar, WalletIsConnected} from "../util/blockchain/wallet";
import {XSSSanitizeUrl} from "../util/security";
import {getIpfsAvatarUrl} from "../util/ipfs";
import {IsGatewayMode} from "../util/miscellaneous";

declare global {
    interface Window {
        DisconnectWalletCallback: () => void;
    }
}

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    async function main() {
        let DOM = {
            htmlMenu: document.getElementById("htmlMenu")! as HTMLButtonElement,
            offcanvas: document.querySelectorAll('.offcanvas')! as NodeListOf<Element>,
            menuLoginBtn: document.getElementById("menuLoginBtn")! as HTMLButtonElement,
            menuAvatar: document.getElementById("menuAvatar")! as HTMLImageElement,
            menuAvatarLink: document.getElementById("menuAvatarLink")! as HTMLAnchorElement,
            menuSettingsLink: document.getElementById("menuSettingsLink")! as HTMLAnchorElement,
            menuDownloadLink: document.getElementById("menuDownloadLink")! as HTMLAnchorElement,
            menuPlacesLink: document.getElementById("menuPlacesLink")! as HTMLAnchorElement,
            isCookieAuthenticated: document.getElementById("isCookieAuthenticated") as HTMLInputElement | null,
            gatewayMode: document.getElementById("gatewayMode") as HTMLInputElement | null,
            userAddress: document.getElementById("userAddress") as HTMLInputElement | null,
            userBlockchain: document.getElementById("userBlockchain") as HTMLInputElement | null,
        }

        window.DisconnectWalletCallback = function() {}

        async function syncAuthState() {
            let isAuthenticated = DOM.isCookieAuthenticated && DOM.isCookieAuthenticated.value === "true";
            let walletConnected = await WalletIsConnected();
            if (!isAuthenticated && walletConnected) {
                await DisconnectWallet();
            }
        }

        function handleLoginLogoutClick(e: Event) {
            e.preventDefault();
            e.stopPropagation();
            let isAuthenticated = DOM.isCookieAuthenticated && DOM.isCookieAuthenticated.value === "true";
            if (isAuthenticated) {
                window.location.replace("/logout");
            } else {
                window.location.replace("/login");
            }
        }
        async function toggleLoginBtn() {
            if (!DOM.isCookieAuthenticated) {
                DOM.menuLoginBtn.innerHTML = '🔑&nbsp;<span class="menuLoginText">Login</span>';
                DOM.menuLoginBtn.style.width = "5.5em";
                return;
            }
            if (DOM.isCookieAuthenticated.value === "true") {
                DOM.menuLoginBtn.innerHTML = '🔑&nbsp;<span class="menuLoginText">Logout</span>';
                DOM.menuLoginBtn.style.width = "6em";
            } else {
                DOM.menuLoginBtn.innerHTML = '🔑&nbsp;<span class="menuLoginText">Login</span>';
                DOM.menuLoginBtn.style.width = "5.5em";
            }
        }
        async function toggleAvatarBtn() {
            let isAuthenticated = DOM.isCookieAuthenticated && DOM.isCookieAuthenticated.value === "true";
            if (!isAuthenticated) {
                DOM.menuAvatar.src = "/static/image/avatar.png";
                DOM.menuAvatarLink.href = "/login";
                return;
            }
            let blockchain = DOM.userBlockchain?.value || GetChain();
            let address = DOM.userAddress?.value || GetAddress();
            if (blockchain && address) {
                DOM.menuAvatarLink.href = `/p/${blockchain}/${address}`;
                let avatar: string | null = null;
                avatar = await getIpfsAvatarUrl(blockchain, address);
                if (!avatar) {
                    avatar = await WalletGetAvatar(blockchain, address);
                }
                if (avatar) {
                    DOM.menuAvatar.src = XSSSanitizeUrl(avatar);
                } else {
                    DOM.menuAvatar.src = "/static/image/avatar.png";
                }
            } else {
                DOM.menuAvatar.src = "/static/image/avatar.png";
                DOM.menuAvatarLink.href = "/login";
            }
        }

        function isLocalhost(): boolean {
            const hostname = window.location.hostname;
            return hostname === 'localhost' ||
                   hostname === '127.0.0.1' ||
                   hostname === '[::1]';
        }

        DOM.menuLoginBtn.addEventListener("click", handleLoginLogoutClick);
        DOM.htmlMenu.addEventListener("click", async (e) => {
            e.preventDefault();
            e.stopPropagation();
            await syncAuthState();
            toggleAvatarBtn().then();
            toggleLoginBtn().then();
        });
        DOM.htmlMenu.addEventListener("focusin", (e) => {
            //e.preventDefault();
            //e.stopPropagation();
            if (DOM.gatewayMode?.value === "true" && !isLocalhost()) {
                DOM.menuDownloadLink.style.display = "block";
                DOM.menuSettingsLink.style.display = "none";
                DOM.menuPlacesLink.href = `${window.location.protocol}//${window.location.host}/`;
            } else {
                DOM.menuDownloadLink.style.display = "none";
                DOM.menuSettingsLink.style.display = "block";
                DOM.menuPlacesLink.href = "/";
            }
        });

        await syncAuthState();
        toggleAvatarBtn().then();
        toggleLoginBtn().then();
    }
})();