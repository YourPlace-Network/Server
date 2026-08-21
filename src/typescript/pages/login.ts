import {InitTooltips} from "../util/bootstrap";

window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import {DisableDialogModalExit, DisableDialogModalOkBtn, HideDialogModal, ShowDialogModal} from "../components/modalDialog";
import "../../scss/pages/login.scss";
import {ConfigureModalLoginForLocalWallet, HideModalLogin, ShowModalLogin} from "../components/modalLogin";
import {GetAddress, IsValidAddress} from "../util/blockchain/wallet";
import {hasLocalWalletEthereum} from "../util/blockchain/localWallet";
import {LogError, LogInfo} from "../util/log";
import {useRedirect} from "../util/redirect";
import {Sleep} from "../util/time";

declare global { // Extend the window interface with public callback objects
    interface Window {
        LoginCallback: (status: string) => void;
        DisconnectWalletCallback: () => void;
    }
}

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            loginModal: document.getElementById("loginModal")! as HTMLDivElement,
            csrfToken: (document.getElementById("csrfToken")! as HTMLInputElement).value,
            refreshBtn: document.getElementById("refreshBtn")! as HTMLButtonElement,
        }

        // --------- Callback Functions --------- //
        window.LoginCallback = async function(status: string) {
            try {
                let address = GetAddress();
                if (!address || !IsValidAddress(address)) {
                    throw new Error("Invalid address after login callback");
                }
                DisableDialogModalExit();
                DisableDialogModalOkBtn();
                ShowDialogModal("Success! 👍 Logging You In Now...");
                await Sleep(3000);
                await Sleep(500); // Allow delay for cookie/context to be set
                if (!await useRedirect()) {
                    LogInfo("No redirect, defaulting to /p/");
                    window.location.href = "/p/";
                    return;
                }
            } catch(error) {
                LogError("LoginCallback Error: " + error);
                ShowDialogModal("Login Failed: " + error);
            }
        }
        window.DisconnectWalletCallback = function() {
            HideModalLogin();
            HideDialogModal();
            ShowModalLogin();
        }

        // --------- Event Handlers --------- //
        DOM.loginModal.addEventListener("hide.bs.modal", e => { // Prevent the modal from being hidden
            e.stopPropagation();
            e.preventDefault();
        });
        DOM.refreshBtn.addEventListener("click", () => {
            document.cookie.split(";").forEach(cookie => {
                const name = cookie.split("=")[0].trim();
                document.cookie = name + "=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/";
            });
            const localWallets: Record<string, string> = {};
            const redirect = localStorage.getItem("yp_redirect");
            for (let i = 0; i < localStorage.length; i++) {
                const key = localStorage.key(i);
                if (key?.startsWith("yp_local_wallet_")) {
                    localWallets[key] = localStorage.getItem(key)!;
                }
            }
            localStorage.clear();
            sessionStorage.clear();
            for (const [key, value] of Object.entries(localWallets)) {
                localStorage.setItem(key, value);
            }
            if (redirect) {
                localStorage.setItem("yp_redirect", redirect);
            }
            window.location.reload();
        });

        // --------- Running Functions --------- //
        document.cookie.split(";").forEach(cookie => {
            const name = cookie.split("=")[0].trim();
            document.cookie = name + "=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/";
        });
        const localWallets: Record<string, string> = {};
        const redirect = localStorage.getItem("yp_redirect");
        for (let i = 0; i < localStorage.length; i++) {
            const key = localStorage.key(i);
            if (key?.startsWith("yp_local_wallet_")) {
                localWallets[key] = localStorage.getItem(key)!;
            }
        }
        localStorage.clear();
        sessionStorage.clear();
        for (const [key, value] of Object.entries(localWallets)) {
            localStorage.setItem(key, value);
        }
        if (redirect) {
            localStorage.setItem("yp_redirect", redirect);
        }
        InitTooltips();
        ConfigureModalLoginForLocalWallet(hasLocalWalletEthereum());
        ShowModalLogin();
    }
})();
