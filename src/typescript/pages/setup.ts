window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import {
    DisableDialogModalExit,
    DisableDialogModalOkBtn, EnableDialogModalExit, EnableDialogModalOkBtn, HideDialogModal,
    ShowDialogModal,
    ShowDialogModalHTML
} from "../components/modalDialog";
import {HideModalLogin, ShowModalLogin} from "../components/modalLogin";
import {DisconnectWallet, GetAddress, GetChain, GetWallet, IsValidAddress, TruncateAddress} from "../util/blockchain/wallet";
import {LogError, LogInfo} from "../util/log";
import {HttpPostJson} from "../util/network";
import "../../scss/pages/setup.scss";
import "../components/modalDialog";
import flatpickr from "flatpickr";
import "flatpickr/dist/flatpickr.min.css";
require("flatpickr/dist/themes/material_blue.css");

declare global { // Extend the window interface with public objects
    interface Window {
        LoginCallback: (status: string) => void;
        DisconnectWalletCallback: () => void;
    }
}

window.DisconnectWalletCallback = function() {};

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            address: document.getElementById("address")! as HTMLDivElement,
            addressTruncated: document.getElementById("addressTruncated")! as HTMLDivElement,
            arrowLeft: document.getElementById("arrowLeft")! as HTMLDivElement,
            birthDate: document.getElementById("birthDate")! as HTMLInputElement,
            birthDateEpoch: document.getElementById("birthDateEpoch")! as HTMLInputElement,
            calendarIcon: document.getElementById("calendarIcon")! as HTMLElement,
            btnLogin: document.getElementById("btnLogin")! as HTMLButtonElement,
            buttonFour: document.getElementById("buttonFour")! as HTMLButtonElement,
            buttonOne: document.getElementById("buttonOne")! as HTMLButtonElement,
            buttonTwo: document.getElementById("buttonTwo")! as HTMLButtonElement,
            buttonThree: document.getElementById("buttonThree")! as HTMLButtonElement,
            checkOne: document.getElementById("checkOne")! as HTMLDivElement,
            checkTwo: document.getElementById("checkTwo")! as HTMLDivElement,
            checkThree: document.getElementById("checkThree")! as HTMLDivElement,
            checkFour: document.getElementById("checkFour")! as HTMLDivElement,
            collapseOne: document.getElementById("collapseOne")! as HTMLDivElement,
            collapseTwo: document.getElementById("collapseTwo")! as HTMLDivElement,
            collapseThree: document.getElementById("collapseThree")! as HTMLDivElement,
            collapseFour: document.getElementById("collapseFour")! as HTMLDivElement,
            csrfToken: document.getElementById("csrfToken")! as HTMLInputElement,
            imgAppStoreCBWallet: document.getElementById("imgAppstoreCBWallet")! as HTMLImageElement,
            imgPlayStoreCBWallet: document.getElementById("imgPlaystoreCBWallet")! as HTMLImageElement,
            installSpinner: document.getElementById("installSpinner")! as HTMLElement,
            startInstallBtn: document.getElementById("startInstallBtn")! as HTMLButtonElement,
            startInstallBtnText: document.getElementById("startInstallBtnText")! as HTMLSpanElement,
            uploadDirectoryValue: document.getElementById("uploadDirectoryValue")! as HTMLInputElement,
        };
        let stepOneComplete = false;
        let stepTwoComplete = false;
        let stepThreeComplete = false;
        let tooltipTriggerList = [].slice.call(document.querySelectorAll("[data-bs-toggle=\"tooltip\"]"));
        tooltipTriggerList.map(function (tooltipTriggerEl) {
            return new window.bootstrap.Tooltip(tooltipTriggerEl)
        });
        DisconnectWallet().then();
        const minAge = new Date();
        minAge.setFullYear(minAge.getFullYear() - 13);
        const birthDatePicker = flatpickr("#birthDate", {dateFormat: "Y-m-d", allowInput: true, altInput: true, altFormat: "F j, Y", maxDate: minAge, minDate: "1900-01-01", enableTime: false,
            onChange: function (selectedDates: Date[], dateStr: string, instance) {
                const epochMs = selectedDates[0].getTime();
                const epochSeconds = Math.floor(epochMs / 1000);
                DOM.birthDateEpoch.value = epochSeconds.toString();
            },
            onClose: stepFour,
        });

        // --------- Page Functions --------- //
        function updateConnected(address: string) {
            DOM.addressTruncated.textContent = TruncateAddress(address)!;
            DOM.checkTwo.style.display = "inline-block";
            DOM.address.style.display = "inline-block";
        }
        function stepOne() {
            DOM.checkOne.style.display = "inline-block";
            stepOneComplete = true;
            DOM.buttonOne.click();
            DOM.buttonTwo.click();
        }
        function stepTwo() {
            DOM.checkTwo.style.display = "inline-block";
            stepTwoComplete = true;
            DOM.buttonTwo.click();
            DOM.buttonThree.click();
        }
        function stepFour() {
            DOM.checkThree.style.display = "inline-block";
            stepThreeComplete = true;
            if (stepThreeComplete && stepTwoComplete && stepOneComplete) {
                DOM.startInstallBtn.disabled = false;
                DOM.arrowLeft.style.display = "block";
            }
            DOM.buttonThree.click();
            DOM.buttonFour.click();
        }
        function openStore(name: string) {
            let url;
            if (name == "playstorebase") {
                url = "https://play.google.com/store/apps/details?id=org.toshi";
            } else if (name == "appstorebase") {
                url = "https://apps.apple.com/us/app/coinbase-wallet-nfts-crypto/id1278383455";
            } else {
                return;
            }
            if (self != top) {
                window.parent.postMessage(name, "*");
            } else {
                window.open(url, "_blank");
            }
        }
        async function startInstall() {
            LogInfo("Starting Install");
            DisableDialogModalOkBtn();
            DisableDialogModalExit();
            ShowDialogModalHTML("Please wait while we install YourPlace<br>This may take a few minutes<br>You'll be sent home when we're done");
            if (!GetAddress()) {
                ShowDialogModal("Please Connect Your Wallet");
                stepTwo();
                return;
            }
            DOM.checkFour.style.display = "inline-block";
            DOM.startInstallBtn.disabled = true;
            DOM.startInstallBtnText.textContent = "Installing...";
            DOM.installSpinner.style.display = "inline-block";
            DOM.arrowLeft.style.display = "none";
            let payload = {
                address: GetAddress()!,
                birthdate: DOM.birthDateEpoch.value,
                uploadDirectory: DOM.uploadDirectoryValue.value,
                wallet: GetWallet()!,
                blockchain: GetChain()!,
            };
            HttpPostJson("/setup", payload, DOM.csrfToken.value).then(); // fire off the setup request, and don't expect a response
            awaitInstallation().then(async (bool) => {
                if (!bool) {
                    DOM.startInstallBtn.disabled = false;
                    DOM.installSpinner.style.display = "none";
                    DOM.arrowLeft.style.display = "block";
                    DOM.startInstallBtnText.textContent = "Start Install";
                    HideModalLogin();
                    HideDialogModal();
                    EnableDialogModalOkBtn();
                    EnableDialogModalExit();
                    ShowDialogModal("Installation Failed. Please try again.");
                    return;
                } else {
                    LogInfo("Installation Complete - redirecting");
                    window.close(); // the server will open itself to the home page after coming back up
                }
            });
        }
        async function awaitInstallation(): Promise<boolean> {
            await new Promise(resolve => setTimeout(resolve, 3000)); // Wait 3 seconds for the server to shut down
            // Poll the server /ping until it is alive, then redirect to the home page
            const timeoutSeconds = 600; // ~ 10 minutes
            const startTime = Date.now();
            LogInfo("Waiting for the server to come back online...");
            while (true) {
                // Check if timeout exceeded
                if ((Date.now() - startTime) / 1000 > timeoutSeconds) {
                    LogError("Installation Timed Out");
                    return false;
                }
                try {
                    // Create an abort controller to handle request timeouts
                    const controller = new AbortController();
                    const timeoutId = setTimeout(() => controller.abort(), 2000); // 2 second timeout
                    const response = await fetch("/ping", {
                        method: "GET",
                        signal: controller.signal,
                        cache: "no-cache"
                    });
                    clearTimeout(timeoutId);
                    if (response.ok) {
                        LogInfo("Server is back online");
                        return true;
                    }
                } catch (error) { // Server not alive yet, keep polling
                    await new Promise(resolve => setTimeout(resolve, 2000));
                    continue;
                }
                await new Promise(resolve => setTimeout(resolve, 2000));
            }
        }

        // --------- Exported Functions --------- //
        window.LoginCallback = function (status: string) {
            let address = GetAddress();
            if (address == null || !IsValidAddress(address)) {
                ShowModalLogin();
                return;
            }
            updateConnected(address);
            HideModalLogin();
            stepTwo();
        }

        // --------- Event Handlers --------- //
        DOM.imgAppStoreCBWallet.addEventListener("click", function () {
            openStore("appstorebase");
            stepOne();
        });
        DOM.imgPlayStoreCBWallet.addEventListener("click", function () {
            openStore("playstorebase");
            stepOne();
        });
        DOM.btnLogin.addEventListener("click", function () {
            ShowModalLogin();
        });
        DOM.startInstallBtn.addEventListener("click", startInstall);
        DOM.calendarIcon.addEventListener("click", function() {
            if (Array.isArray(birthDatePicker)) {
                birthDatePicker[0].open();
            } else {
                birthDatePicker.open();
            }
        });
        DOM.collapseOne.addEventListener("hidden.bs.collapse", function () {
            DOM.checkOne.style.display = "inline-block";
            stepOneComplete = true;
        });
        DOM.collapseTwo.addEventListener("shown.bs.collapse", function () {
            DOM.checkOne.style.display = "inline-block";
            stepOneComplete = true;
            DOM.btnLogin.click();
        })
        DOM.collapseFour.addEventListener("shown.bs.collapse", function () {
            if (stepOneComplete && stepTwoComplete && stepThreeComplete) {
                DOM.startInstallBtn.disabled = false;
                DOM.arrowLeft.style.display = "block";
            }
        });
    }
})();
