window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/modalLogin.scss";
import {ShowDialogModal, ShowDialogModalHTML, ShowDialogModalWithCallback} from "./modalDialog";
import {ConnectWallet, ReconnectWallet, SetAddress, SetChain, SetWallet, WalletLogin} from "../util/blockchain/wallet";
import {hasLocalWalletEthereum, localWalletEthereumConnect, localWalletEthereumCreate} from "../util/blockchain/localWallet";
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

        async function handleNoWallet() {
            HideModalLogin();
            let address: string;
            let isNewWallet = false;
            if (hasLocalWalletEthereum()) {
                address = await localWalletEthereumConnect();
            } else {
                isNewWallet = true;
                address = await localWalletEthereumCreate();
            }
            if (!address || address === "") {
                ShowDialogModal("Failed to create wallet");
                return;
            }
            SetWallet("localwalletethereum");
            SetChain("base");
            SetAddress(address);
            if (isNewWallet) {
                ShowDialogModalWithCallback(
                    "Your wallet has been created and a backup file has been downloaded. Keep this file safe - you'll need it to recover your account if you clear your browser data.",
                    async () => {
                        if (window.location.pathname !== "/setup") {
                            let loginResult = await WalletLogin();
                            if (loginResult !== "success") {
                                ShowDialogModal("Failed to login: " + loginResult);
                                return;
                            }
                        }
                        if (typeof window.LoginCallback === "function") {
                            window.LoginCallback("success");
                        }
                    }
                );
                return;
            }
            if (window.location.pathname !== "/setup") {
                let loginResult = await WalletLogin();
                if (loginResult !== "success") {
                    ShowDialogModal("Failed to login: " + loginResult);
                    return;
                }
            }
            if (typeof window.LoginCallback === "function") {
                window.LoginCallback("success");
            }
        }
        function connectWalletDispatcher(wallet: string) {
            if (wallet === "none") {
                if (IsGatewayMode()) {
                    return async function(e: Event) {
                        e.stopImmediatePropagation();
                        e.preventDefault();
                        await handleNoWallet();
                    };
                } else {
                    return async function(e: Event) {
                        e.stopImmediatePropagation();
                        e.preventDefault();
                        await handleNoWallet();
                    };
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
        DOM.noWalletBtn.addEventListener("click", connectWalletDispatcher("none"));
        DOM.peraWalletBtn.addEventListener("click", connectWalletDispatcher("pera"));
    }
})();
