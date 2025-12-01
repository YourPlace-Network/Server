window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/modalLogin.scss";
import {ShowDialogModal, ShowDialogModalHTML} from "./modalDialog";
import {ConnectWallet, ReconnectWallet, WalletLogin} from "../util/blockchain/wallet";
import {IsGatewayMode} from "../util/miscellaneous";

let modal: bootstrap.Modal;

export function ShowModalLogin() {
    modal.show();
}
export function HideModalLogin() {
    modal.hide();
    let loginModal = document.getElementById("loginModal")! as HTMLDivElement;
    loginModal.style.display = "none";
    document.querySelectorAll(".modal-backdrop").forEach(el => el.remove());
    document.body.classList.remove("modal-open");
}

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        ReconnectWallet().then();
        modal = new window.bootstrap.Modal("#loginModal", {});
        let DOM = {
            noWalletBtn: document.getElementById("noWalletBtn")! as HTMLButtonElement,
            coinbaseWalletBtn: document.getElementById("coinbaseWalletBtn")! as HTMLButtonElement,
            csrfToken: (document.getElementById("csrfToken")! as HTMLInputElement).value,
            metaMaskWalletBtn: document.getElementById("metaMaskWalletBtn")! as HTMLButtonElement,
            modalDialog: document.getElementById("modalDialog")! as HTMLDivElement,
            modalDialogOkBtn: document.getElementsByClassName("yp-modal-btn")[0]! as HTMLButtonElement,
            peraWalletBtn: document.getElementById("peraWalletBtn")! as HTMLButtonElement,
        }

        function connectWalletDispatcher(wallet: string) {
            if (wallet === "none") {
                if (IsGatewayMode()) {
                    ShowDialogModalHTML("A wallet holds your ID on YourPlace. If you don't have one, download the <a href=\"https://yourplace.network/downloaod#noWallet\" target=\"_blank\">YourPlace Server</a> and try again to create a wallet.");
                    let modalElement = document.getElementById("modalDialog")!;
                    let handleDialogClose = function() {
                        window.location.href = "/download#noWallet";
                    };
                    modalElement.addEventListener("hidden.bs.modal", handleDialogClose, {once: true});
                    return () => {};
                } else {

                }
            }

            return async function(e: Event) {
                e.stopImmediatePropagation();
                e.preventDefault();
                HideModalLogin();
                let status = await ConnectWallet(wallet);
                if (status !== "success") {
                    ShowDialogModal(status);
                    return;
                }
                if (window.location.pathname !== "/setup") {
                    let loginResult = await WalletLogin();
                    if (loginResult === "wallet_not_deployed") {
                        return;
                    }
                    if (loginResult !== "success") {
                        window.location.href = "/login";
                        return;
                    }
                }
                if (typeof window.LoginCallback === "function") {
                    window.LoginCallback(status);
                }
            }
        }

        DOM.coinbaseWalletBtn.addEventListener("click", connectWalletDispatcher("cbwalletbase"));
        DOM.peraWalletBtn.addEventListener("click", connectWalletDispatcher("pera"));
        // DOM.noWalletBtn.addEventListener("click", connectWalletDispatcher("none")); // Disabled
    }
})();
