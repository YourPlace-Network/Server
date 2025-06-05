window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import {DisableDialogModalExit, DisableDialogModalOkBtn, HideDialogModal, ShowDialogModal} from "../components/modalDialog";
import "../../scss/pages/login.scss";
import {HideModalLogin, ShowModalLogin} from "../components/modalLogin";
import {GetAddress, IsValidAddress} from "../util/blockchain/wallet";
import {LogError, LogInfo} from "../util/log";
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

                const queryParams = new URLSearchParams(window.location.search);
                let redirect = queryParams.get("redirect");

                if (!redirect) {
                    LogInfo("No redirect");
                    window.location.replace("/");
                    return;
                }

                if (!redirect.endsWith("/")) {
                    redirect += "/";
                } // Add trailing slash if needed
                await Sleep(500); // Allow delay for cookie/context to be set

                // Handle all redirects
                let redir: any;
                if (redirect.startsWith("/p/")) {
                    console.log("Redirecting to: " + redirect);
                    redir = redirect;
                } else if (redirect === "/settings/") {
                    console.log("Redirecting to: " + redirect);
                    redir = "/settings/";
                } else {
                    console.log("Redirecting to: /");
                    redir = "/";
                }
                if (redir !== null) {
                    console.log("Redirecting to: " + redir);
                    window.location.replace(redir);
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

        // --------- Running Functions --------- //
        ShowModalLogin();
    }
})();
