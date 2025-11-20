import "bootstrap/dist/js/bootstrap.bundle";
import "../../scss/components/menu.scss";
import {DisconnectWallet, GetAddress, GetChain, WalletGetAvatar, WalletIsConnected} from "../util/blockchain/wallet";
import {XSSSanitizeUrl} from "../util/security";
import {CIDToSubdomainURL} from "../util/ipfs";

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
            menuSettingsLink: document.getElementById("menuSettingsLink")! as HTMLAnchorElement,
            menuPlacesLink: document.getElementById("menuPlacesLink")! as HTMLAnchorElement,
            isCookieAuthenticated: document.getElementById("isCookieAuthenticated")! as HTMLInputElement,
            gatewayMode: document.getElementById("gatewayMode")! as HTMLInputElement,
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
                DOM.menuLoginBtn.innerText = "Login";
                return;
            }
            if (DOM.isCookieAuthenticated.value === "true") {
                DOM.menuLoginBtn.innerText = "Logout";
            } else {
                DOM.menuLoginBtn.innerText = "Login";
            }
        }
        async function toggleAvatarBtn() {
            let isAuthenticated = DOM.isCookieAuthenticated && DOM.isCookieAuthenticated.value === "true";
            if (!isAuthenticated) {
                DOM.menuAvatar.src = "/static/image/avatar.png";
                return;
            }
            let blockchain = GetChain();
            let address = GetAddress();
            if (blockchain && address) {
                let avatar = await WalletGetAvatar(blockchain, address);
                if (avatar) {
                    let cidURL = CIDToSubdomainURL(avatar.toString());
                    if (cidURL) {
                        DOM.menuAvatar.src = cidURL;
                    } else {
                        DOM.menuAvatar.src = XSSSanitizeUrl(avatar.toString());
                    }
                }
            } else {
                DOM.menuAvatar.src = "/static/image/avatar.png";
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
            console.log("focusin");
            if (DOM.gatewayMode.value === "true" && !isLocalhost()) {
                DOM.menuSettingsLink.style.display = "none";
                DOM.menuPlacesLink.href = `${window.location.protocol}//${window.location.host}/`;
            } else {
                DOM.menuSettingsLink.style.display = "block";
                DOM.menuPlacesLink.href = "/";
            }
        });

        await syncAuthState();
        toggleAvatarBtn().then();
        toggleLoginBtn().then();
    }
})();