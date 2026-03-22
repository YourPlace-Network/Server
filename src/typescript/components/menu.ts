window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/menu.scss";
import {DisconnectWallet, GetAddress, GetChain, GetWallet, WalletGetAvatar, WalletIsConnected} from "../util/blockchain/wallet";
import {hasLocalWalletEthereum, localWalletEthereumAuthLogin} from "../util/blockchain/localWallet";
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
            bsOffcanvas: new window.bootstrap.Offcanvas("#htmlMenuOffcanvas"),
            htmlMenu: document.getElementById("htmlMenu")! as HTMLButtonElement,
            offcanvas: document.querySelectorAll('.offcanvas')! as NodeListOf<Element>,
            menuLoginBtn: document.getElementById("menuLoginBtn")! as HTMLButtonElement,
            menuAvatar: document.getElementById("menuAvatar")! as HTMLImageElement,
            menuAvatarLink: document.getElementById("menuAvatarLink")! as HTMLAnchorElement,
            menuSettingsLink: document.getElementById("menuSettingsLink")! as HTMLAnchorElement,
            menuDownloadLink: document.getElementById("menuDownloadLink")! as HTMLAnchorElement,
            menuPlacesLink: document.getElementById("menuPlacesLink") as HTMLAnchorElement | null,
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
                if (GetWallet() === "localwalletethereum" && hasLocalWalletEthereum()) {
                    let result = await localWalletEthereumAuthLogin();
                    if (result === "success") {
                        DOM.isCookieAuthenticated!.value = "true";
                        return;
                    }
                }
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
                DOM.menuLoginBtn.innerHTML = '<span class="menuLoginText">Login</span>';
                return;
            }
            if (DOM.isCookieAuthenticated.value === "true") {
                DOM.menuLoginBtn.innerHTML = '<span class="menuLoginText">Logout</span>';
            } else {
                DOM.menuLoginBtn.innerHTML = '<span class="menuLoginText">Login</span>';
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
        let hoverTimeout: ReturnType<typeof setTimeout> | null = null;
        DOM.htmlMenu.addEventListener("mouseenter", () => {
            hoverTimeout = setTimeout(async () => {
                await syncAuthState();
                toggleAvatarBtn().then();
                toggleLoginBtn().then();
                DOM.bsOffcanvas.show();
            }, 300);
        });
        DOM.htmlMenu.addEventListener("mouseleave", () => {
            if (hoverTimeout) { clearTimeout(hoverTimeout); hoverTimeout = null; }
        });
        DOM.htmlMenu.addEventListener("focusin", (e) => {
            //e.preventDefault();
            //e.stopPropagation();
            if (DOM.gatewayMode?.value === "true" && !isLocalhost()) {
                DOM.menuDownloadLink.style.display = "block";
                DOM.menuSettingsLink.style.display = "none";
                if (DOM.menuPlacesLink) { DOM.menuPlacesLink.href = `${window.location.protocol}//${window.location.host}/`; }
            } else {
                DOM.menuDownloadLink.style.display = "none";
                DOM.menuSettingsLink.style.display = "block";
                if (DOM.menuPlacesLink) { DOM.menuPlacesLink.href = "/"; }
            }
        });

        await syncAuthState();
        toggleAvatarBtn().then();
        toggleLoginBtn().then();
    }
})();