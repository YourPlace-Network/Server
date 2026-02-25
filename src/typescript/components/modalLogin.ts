window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/modalLogin.scss";
import {DisableDialogModalOkBtn, EnableDialogModalOkBtn, HideDialogModal, ShowDialogModal, ShowDialogModalHTML, ShowDialogModalWithCallback} from "./modalDialog";
import {ConnectWallet, ReconnectWallet, SetAddress, SetChain, SetWallet, WalletLogin} from "../util/blockchain/wallet";
import {basePrefetchLoginNonce} from "../util/blockchain/base";
import {localWalletEthereumCreate, localWalletEthereumImport} from "../util/blockchain/localWallet";
import {IsGatewayMode} from "../util/miscellaneous";

let modal: bootstrap.Modal;
let DOM: {
    coinbaseWalletBtn: HTMLButtonElement;
    csrfToken: string;
    localWalletBtn: HTMLButtonElement;
    localWalletDiv: HTMLDivElement;
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
    DOM.localWalletDiv.style.display = "flex";
    DOM.noWalletBtn.style.display = "none";
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
            localWalletDiv: document.getElementById("localWalletDiv")! as HTMLDivElement,
            metaMaskWalletBtn: document.getElementById("metaMaskWalletBtn")! as HTMLButtonElement,
            modalDialog: document.getElementById("modalDialog")! as HTMLDivElement,
            modalDialogOkBtn: document.getElementsByClassName("yp-modal-btn")[0]! as HTMLButtonElement,
            noWalletBtn: document.getElementById("noWalletBtn")! as HTMLButtonElement,
            peraWalletBtn: document.getElementById("peraWalletBtn")! as HTMLButtonElement,
        }

        async function completeLocalWalletLogin() {
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
        function handleImportWallet() {
            const fileInput = document.createElement("input");
            fileInput.type = "file";
            fileInput.accept = ".json";
            fileInput.addEventListener("change", async () => {
                const file = fileInput.files?.[0];
                if (!file) return;
                try {
                    const walletJson = await file.text();
                    const address = localWalletEthereumImport(walletJson);
                    if (!address) {
                        ShowDialogModal("Invalid wallet backup file");
                        return;
                    }
                    SetWallet("localwalletethereum");
                    SetChain("base");
                    SetAddress(address);
                    await completeLocalWalletLogin();
                } catch (e) {
                    ShowDialogModal("Failed to read wallet file");
                }
            });
            fileInput.click();
        }
        async function handleNewWallet() {
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
                async () => { await completeLocalWalletLogin(); }
            );
        }
        function showWalletChoiceDialog() {
            const content = document.getElementById("modalDialogContent")!;
            content.textContent = "";
            content.style.display = "flex";
            content.style.flexDirection = "column";
            content.style.gap = "2em";
            const newWalletBtn = document.createElement("button");
            newWalletBtn.type = "button";
            newWalletBtn.className = "btn btn-primary btn-login-wallet";
            newWalletBtn.textContent = "New Wallet";
            content.appendChild(newWalletBtn);
            const importWalletBtn = document.createElement("button");
            importWalletBtn.type = "button";
            importWalletBtn.className = "btn btn-primary btn-login-wallet";
            importWalletBtn.textContent = "Import Wallet";
            content.appendChild(importWalletBtn);
            DisableDialogModalOkBtn();
            const element = document.getElementById("modalDialog")!;
            const dialogModal = window.bootstrap.Modal.getOrCreateInstance(element);
            dialogModal.show();
            importWalletBtn.addEventListener("click", () => {
                HideDialogModal();
                EnableDialogModalOkBtn();
                handleImportWallet();
            });
            newWalletBtn.addEventListener("click", async () => {
                HideDialogModal();
                EnableDialogModalOkBtn();
                await handleNewWallet();
            });
        }
        function connectWalletDispatcher(wallet: string) {
            if (wallet === "local") {
                return async function(e: Event) {
                    e.stopImmediatePropagation();
                    e.preventDefault();
                    HideModalLogin();
                    showWalletChoiceDialog();
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
        DOM.metaMaskWalletBtn.addEventListener("click", connectWalletDispatcher("metamaskethereum"));
        DOM.noWalletBtn.addEventListener("click", connectWalletDispatcher("local"));
        DOM.peraWalletBtn.addEventListener("click", connectWalletDispatcher("pera"));
    }
})();
