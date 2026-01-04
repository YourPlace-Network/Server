window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/modalLogin.scss";
import {ShowDialogModal, ShowDialogModalHTML, ShowDialogModalWithCallback} from "./modalDialog";
import {ConnectWallet, ReconnectWallet, SetAddress, SetChain, SetWallet, WalletLogin} from "../util/blockchain/wallet";
import {basePrefetchLoginNonce} from "../util/blockchain/base";
import {hasLocalWalletEthereum, localWalletEthereumConnect, localWalletEthereumCreate} from "../util/blockchain/localWallet";
import {IsGatewayMode} from "../util/miscellaneous";

let modal: bootstrap.Modal;
let DOM: {
    coinbaseWalletBtn: HTMLButtonElement;
    csrfToken: string;
    localWalletBtn: HTMLButtonElement;
    metaMaskWalletBtn: HTMLButtonElement;
    modalDialog: HTMLDivElement;
    modalDialogOkBtn: HTMLButtonElement;
    noWalletBtn: HTMLButtonElement;
    peraWalletBtn: HTMLButtonElement;
};

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
export function ConfigureModalLoginForLocalWallet(hasLocalWallet: boolean) {
    if (hasLocalWallet) {
        DOM.localWalletBtn.style.display = "flex";
        DOM.noWalletBtn.style.display = "none";
    } else {
        DOM.localWalletBtn.style.display = "none";
        DOM.noWalletBtn.style.display = "flex";
    }
}

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        ReconnectWallet().then();
        basePrefetchLoginNonce().then();
        modal = new window.bootstrap.Modal("#loginModal", {});
        DOM = {
            coinbaseWalletBtn: document.getElementById("coinbaseWalletBtn")! as HTMLButtonElement,
            csrfToken: (document.getElementById("csrfToken")! as HTMLInputElement).value,
            localWalletBtn: document.getElementById("localWalletBtn")! as HTMLButtonElement,
            metaMaskWalletBtn: document.getElementById("metaMaskWalletBtn")! as HTMLButtonElement,
            modalDialog: document.getElementById("modalDialog")! as HTMLDivElement,
            modalDialogOkBtn: document.getElementsByClassName("yp-modal-btn")[0]! as HTMLButtonElement,
            noWalletBtn: document.getElementById("noWalletBtn")! as HTMLButtonElement,
            peraWalletBtn: document.getElementById("peraWalletBtn")! as HTMLButtonElement,
        }

        async function handleLocalWallet() {
            HideModalLogin();
            const address = await localWalletEthereumConnect();
            if (!address || address === "") {
                ShowDialogModal("Failed to connect to your wallet");
                return;
            }
            SetWallet("localwalletethereum");
            SetChain("base");
            SetAddress(address);
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
        async function handleNoWallet() {
            HideModalLogin();
            const address = await localWalletEthereumCreate();
            if (!address || address === "") {
                ShowDialogModal("Failed to create wallet");
                return;
            }
            SetWallet("localwalletethereum");
            SetChain("base");
            SetAddress(address);
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
        }
        function connectWalletDispatcher(wallet: string) {
            if (wallet === "local") {
                return async function(e: Event) {
                    e.stopImmediatePropagation();
                    e.preventDefault();
                    await handleLocalWallet();
                };
            }
            if (wallet === "none") {
                return async function(e: Event) {
                    e.stopImmediatePropagation();
                    e.preventDefault();
                    await handleNoWallet();
                };
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
                    if (loginResult === "wallet_not_deployed" || loginResult === "popup_blocked") {
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
        DOM.localWalletBtn.addEventListener("click", connectWalletDispatcher("local"));
        DOM.noWalletBtn.addEventListener("click", connectWalletDispatcher("none"));
        DOM.peraWalletBtn.addEventListener("click", connectWalletDispatcher("pera"));
    }
})();
