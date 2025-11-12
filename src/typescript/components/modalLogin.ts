import type {MouseEvent} from "react";

window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/modalLogin.scss";
import {
    DisableDialogModalExit,
    DisableDialogModalOkBtn,
    ShowDialogModal, ShowDialogModalHTML,
    ShowDialogModalHTMLUnsafe
} from "./modalDialog";
import {
    ConnectWallet,
    DisconnectWallet,
    ReconnectWallet,
    WalletIsConnected,
    WalletLogin
} from "../util/blockchain/wallet";
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
            txnlabWalletBtn: document.getElementById("txnlabWalletBtn")! as HTMLButtonElement
        }

        function connectWalletDispatcher(wallet: string) {
            if (wallet === "none") {
                if (IsGatewayMode()) {
                    ShowDialogModalHTML("If you do not have a wallet and can't download any of the supported ones, then you'll need to run your own <a href=\"https://yourplace.network/download\" target=\"_blank\">YourPlace Server</a>, which will allow you to create a wallet.");
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
                    await WalletLogin();
                }
                window.LoginCallback(status);
            }
        }

        DOM.coinbaseWalletBtn.addEventListener("click", connectWalletDispatcher("cbwalletbase"));
        DOM.peraWalletBtn.addEventListener("click", connectWalletDispatcher("pera"));
        DOM.noWalletBtn.addEventListener("click", connectWalletDispatcher("none"));
    }
})();
