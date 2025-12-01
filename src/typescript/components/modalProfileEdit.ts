window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/modalProfileEdit.scss";
import {LogError, LogInfo} from "../util/log";
import {UploadFile} from "../util/files";
import {WalletSetAvatar, WalletSetBanner, WalletSetDescription, WalletSetLocation, WalletSetName, WalletSetWebsite} from "../util/blockchain/wallet";
import DOMPurify from "dompurify";
import {ShowToastWithDelay} from "./toast";
import {ShowDialogModalHTML} from "./modalDialog";

export async function showProfileEditModal() {
    let DOM = {
        avatarPreview: document.getElementById("avatarPreview")! as HTMLImageElement,
        profileBanner: document.getElementById("profileBanner")! as HTMLImageElement,
        bannerPreview: document.getElementById("bannerPreview")! as HTMLImageElement,
        inputBirthday: document.getElementById("inputBirthday")! as HTMLInputElement,
        inputDescription: document.getElementById("inputDescription")! as HTMLTextAreaElement,
        inputUsername: document.getElementById("inputUsername")! as HTMLInputElement,
        inputLocation: document.getElementById("inputLocation")! as HTMLInputElement,
        inputWebsite: document.getElementById("inputWebsite")! as HTMLInputElement,
        profileAvatar: document.getElementById("profileAvatar")! as HTMLImageElement,
        profileDescription: document.getElementById("profileDescription")! as HTMLDivElement,
        profileName: document.getElementById("profileName")! as HTMLDivElement,
        profileLocation: document.getElementById("profileLocation")! as HTMLDivElement,
        profileWebsite: document.getElementById("profileWebsite")! as HTMLAnchorElement,
        modalProfileEdit: document.getElementById("modalProfileEdit")! as HTMLDivElement,
        saveProfileBtn: document.getElementById("saveProfileBtn")! as HTMLButtonElement,
    }
    if (DOM.profileAvatar.src != "/static/image/avatar.png") {
        DOM.avatarPreview.src = DOMPurify.sanitize(DOM.profileAvatar.src);
    }
    if (DOM.profileBanner.src != "/static/image/banner.jpg") {
        DOM.bannerPreview.src = DOMPurify.sanitize(DOM.profileBanner.src);
    }
    if (DOM.profileName.innerText != "Anonymous") {
        DOM.inputUsername.value = DOMPurify.sanitize(DOM.profileName.innerText.split(".")[0]);
    }
    if (DOM.profileDescription.innerText != "Could be anyone") {
        DOM.inputDescription.value = DOMPurify.sanitize(DOM.profileDescription.innerText);
    }
    if (DOM.profileLocation.innerText != "City, State") {
        DOM.inputLocation.value = DOMPurify.sanitize(DOM.profileLocation.innerText);
    }
    if (DOM.profileWebsite.innerText != "https://unknown") {
        DOM.inputWebsite.value = DOMPurify.sanitize(DOM.profileWebsite.innerText);
    }
    const modal = new window.bootstrap.Modal(DOM.modalProfileEdit, {});
    DOM.modalProfileEdit.addEventListener("shown.bs.modal", () => {
        let tooltipTriggerList = [].slice.call(DOM.modalProfileEdit.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipTriggerList.map(function (tooltipTriggerEl: HTMLElement) {return new window.bootstrap.Tooltip(tooltipTriggerEl, {delay: {show: 1500, hide: 0}});});
    }, {once: true});
    modal.show();
}

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            avatarPreview: document.getElementById("avatarPreview")! as HTMLImageElement,
            profileBanner: document.getElementById("profileBanner")! as HTMLImageElement,
            bannerPreview: document.getElementById("bannerPreview")! as HTMLImageElement,
            btnDescriptionSave: document.getElementById("btnDescriptionSave")! as HTMLButtonElement,
            btnLocationSave: document.getElementById("btnLocationSave")! as HTMLButtonElement,
            btnUsernameSave: document.getElementById("btnUsernameSave")! as HTMLButtonElement,
            btnWebsiteSave: document.getElementById("btnWebsiteSave")! as HTMLButtonElement,
            csrfToken: document.getElementById("csrfToken")! as HTMLInputElement,
            inputAvatar: document.getElementById("inputAvatar")! as HTMLInputElement,
            inputBanner: document.getElementById("inputBanner")! as HTMLInputElement,
            inputDescription: document.getElementById("inputDescription")! as HTMLTextAreaElement,
            inputUsername: document.getElementById("inputUsername")! as HTMLInputElement,
            inputLocation: document.getElementById("inputLocation")! as HTMLInputElement,
            inputWebsite: document.getElementById("inputWebsite")! as HTMLInputElement,
            profileAvatar: document.getElementById("profileAvatar")! as HTMLImageElement,
            profileDescription: document.getElementById("profileDescription")! as HTMLDivElement,
            profileName: document.getElementById("profileName")! as HTMLDivElement,
            profileLocation: document.getElementById("profileLocation")! as HTMLDivElement,
            profileWebsite: document.getElementById("profileWebsite")! as HTMLAnchorElement,
            modalProfileEdit: document.getElementById("modalProfileEdit")! as HTMLDivElement,
            gatewayMode: document.getElementById("gatewayMode") as HTMLInputElement,
            injectedBlockchain: document.getElementById("injectedBlockchain") as HTMLInputElement,
            avatarLabel: document.querySelector('label[for="inputAvatar"]') as HTMLLabelElement,
            bannerLabel: document.querySelector('label[for="inputBanner"]') as HTMLLabelElement,
        }
        function getModalInstance() {
            return window.bootstrap.Modal.getInstance(DOM.modalProfileEdit) || new window.bootstrap.Modal(DOM.modalProfileEdit, {});
        }
        function isGatewayMode(): boolean {
            const hostname = window.location.hostname;
            const isLocalhost = hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '[::1]';
            return DOM.gatewayMode && DOM.gatewayMode.value === "true" && !isLocalhost;
        }
        function showGatewayAvatarMessage() {
            let message = "To change your avatar, you must:<br><br>";
            message += "• <a href='https://yourplace.network/download' target='_blank'>Download the YourPlace app</a> to host files yourself<br><br>";
            DOM.modalProfileEdit.addEventListener("hidden.bs.modal", () => {
                ShowDialogModalHTML(message);
            }, {once: true});
            getModalInstance().hide();
        }
        function showGatewayBannerMessage() {
            let message = "To change your banner, you need to <a href='https://yourplace.network/download' target='_blank'>download the YourPlace app</a> to host files yourself.";
            DOM.modalProfileEdit.addEventListener("hidden.bs.modal", () => {
                ShowDialogModalHTML(message);
            }, {once: true});
            getModalInstance().hide();
        }
        function hideModalAndShowToast() {
            DOM.modalProfileEdit.addEventListener("hidden.bs.modal", () => {
                const modalBackdrops = document.querySelectorAll(".modal-backdrop");
                modalBackdrops.forEach(backdrop => backdrop.remove());
                ShowToastWithDelay("Your profile update should show up shortly. Please wait for it to spread through the network.", 10000);
            }, {once: true});
            getModalInstance().hide();
        }

        async function updateAvatar() {
            let file = DOM.inputAvatar.files![0];
            let result = await UploadFile(file, DOM.csrfToken.value); // send file to server
            if (result[0] == 200 && result[1].status == "success" && result[1].data && result[1].data.length > 0) {
                try {
                    let success = await WalletSetAvatar("ipfs://" + result[1].data[0].cid);
                    if (success) hideModalAndShowToast();
                    return;
                } catch (e) {
                    LogError("Failed to set avatar: " + e);
                }
            }
            LogError("Failed to upload avatar: " + result[1].status);
        }
        async function updateBanner() {
            let file = DOM.inputBanner.files![0];
            DOM.bannerPreview.src = URL.createObjectURL(file);
            let result = await UploadFile(file, DOM.csrfToken.value);
            // File upload responses contain arrays now. response[1].data[i] is one file data object. Look in files.go
            if (result[0] == 200) {
                if (result[1].status == "success") {
                    try {
                        let success = await WalletSetBanner("ipfs://" + result[1].cid);
                        if (success) hideModalAndShowToast();
                    } catch (e) {
                        LogError("Failed to set banner" + e);
                    }
                }
            }
        }
        async function updateName() {
            let name = DOM.inputUsername.value;
            try {
                let success = await WalletSetName(name);
                if (success) hideModalAndShowToast();
            } catch (e) {
                LogError("failed to set username: " + e)
            }
        }
        async function updateDescription() {
            let description = DOM.inputDescription.value;
            try {
                let success = await WalletSetDescription(description);
                if (success) hideModalAndShowToast();
            } catch (e) {
                LogError("Failed to set description" + e);
            }
        }
        async function updateLocation() {
            let location = DOM.inputLocation.value;
            try {
                let success = await WalletSetLocation(location);
                if (success) hideModalAndShowToast();
            } catch (e) {
                LogError("Failed to set location" + e);
            }
        }
        async function updateWebsite() {
            let website = DOM.inputWebsite.value;
            try {
                let success = await WalletSetWebsite(website);
                if (success) hideModalAndShowToast();
            } catch (e) {
                LogError("Failed to set website" + e);
            }
        }
        if (DOM.avatarLabel) {
            DOM.avatarLabel.addEventListener("click", (e) => {
                if (isGatewayMode()) {
                    e.preventDefault();
                    e.stopPropagation();
                    showGatewayAvatarMessage();
                }
            });
        }
        DOM.inputAvatar.addEventListener("click", (e) => {
            if (isGatewayMode()) {
                e.preventDefault();
                e.stopPropagation();
                showGatewayAvatarMessage();
            }
        });
        DOM.inputAvatar.addEventListener("change", () => {
            if (isGatewayMode()) return;
            let file = DOM.inputAvatar.files![0];
            DOM.avatarPreview.src = URL.createObjectURL(file);
            updateAvatar().then();
        })
        if (DOM.bannerLabel) {
            DOM.bannerLabel.addEventListener("click", (e) => {
                if (isGatewayMode()) {
                    e.preventDefault();
                    e.stopPropagation();
                    showGatewayBannerMessage();
                }
            });
        }
        DOM.inputBanner.addEventListener("click", (e) => {
            if (isGatewayMode()) {
                e.preventDefault();
                e.stopPropagation();
                showGatewayBannerMessage();
            }
        });
        DOM.inputBanner.addEventListener("change", () => {
            if (isGatewayMode()) return;
            let file = DOM.inputBanner.files![0];
            DOM.bannerPreview.src = URL.createObjectURL(file)
            updateBanner().then();
        });
        DOM.modalProfileEdit.addEventListener("hidden.bs.modal", () => {
            //window.PageReloadCallback();
        });
        DOM.btnUsernameSave.addEventListener("click", updateName);
        DOM.btnDescriptionSave.addEventListener("click", updateDescription);
        DOM.btnLocationSave.addEventListener("click", updateLocation);
        DOM.btnWebsiteSave.addEventListener("click", updateWebsite);
    }
})();
