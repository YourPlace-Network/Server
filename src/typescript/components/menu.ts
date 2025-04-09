import "bootstrap/dist/js/bootstrap.bundle";
import "../../scss/components/menu.scss";
import {GetAddress, GetChain, WalletGetAvatar} from "../util/blockchain/wallet";
import {XSSSanitizeUrl} from "../util/security";
import {CIDToSubdomainURL} from "../util/ipfs";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            htmlMenu: document.getElementById("htmlMenu")! as HTMLButtonElement,
            offcanvas: document.querySelectorAll('.offcanvas')! as NodeListOf<Element>,
            menuLoginBtn: document.getElementById("menuLoginBtn")! as HTMLButtonElement,
            menuAvatar: document.getElementById("menuAvatar")! as HTMLImageElement,
            menuSettingsLink: document.getElementById("menuSettingsLink")! as HTMLAnchorElement,
            isCookieAuthenticated: document.getElementById("isCookieAuthenticated")! as HTMLInputElement,
            gatewayMode: document.getElementById("gatewayMode")! as HTMLInputElement,
        }

        function loginEvent(e: Event) {
            e.preventDefault();
            e.stopPropagation();
            window.location.replace("/login");
        }
        function logoutEvent(e: Event) {
            e.preventDefault();
            e.stopPropagation();
            window.location.replace("/logout");
        }
        async function toggleLoginBtn() {
            if (!DOM.isCookieAuthenticated) {
                return;
            }
            if (DOM.isCookieAuthenticated.value === "true") {
                DOM.menuLoginBtn.innerText = "Logout";
                DOM.menuLoginBtn.removeEventListener("click", loginEvent);
                DOM.menuLoginBtn.addEventListener("click", logoutEvent);
                DOM.menuLoginBtn.style.marginLeft = "3.3em";
            } else {
                DOM.menuLoginBtn.innerText = "Login";
                DOM.menuLoginBtn.removeEventListener("click", logoutEvent);
                DOM.menuLoginBtn.addEventListener("click", loginEvent);
                DOM.menuLoginBtn.style.marginLeft = "4em";
            }
        }
        async function toggleAvatarBtn() {
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
            }
        }

        DOM.menuLoginBtn.addEventListener("click", loginEvent);
        DOM.htmlMenu.addEventListener("click", (e) => {
            e.preventDefault();
            e.stopPropagation();
            toggleAvatarBtn().then();
            toggleLoginBtn().then();
        });
        DOM.htmlMenu.addEventListener("focusin", (e) => {
            //e.preventDefault();
            //e.stopPropagation();
            console.log("focusin");
            if (DOM.gatewayMode.value == "true") {
                DOM.menuSettingsLink.style.display = "none";
            } else {
                DOM.menuSettingsLink.style.display = "block";
            }
        });

        toggleAvatarBtn().then();
        toggleLoginBtn().then();
    }
})();