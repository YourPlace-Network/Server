window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/test.scss";
import "../components/menu";
import "../components/addPost";
import {LogError, LogInfo} from "../util/log";
import {UploadFile} from "../util/files";
import type {App, Account, FeedItem} from "../services/twitter";
import {ProfileService} from "../services/twitter";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            avatarPreview: document.getElementById("avatarPreview")! as HTMLImageElement,
            inputAvatar: document.getElementById("inputAvatar")! as HTMLInputElement,
            csrfToken: document.getElementById("csrfToken")! as HTMLInputElement,
        }
        
        async function updateAvatar() {
            let file = DOM.inputAvatar.files![0];
            let result = await UploadFile(file, DOM.csrfToken.value); // send file to server
            if (result[0] == 200) {
                if (result[1].status == "success") {
                    LogInfo("success");
                    console.log(result);
                    return;
                    // todo
                    /*try {
                        await WalletSetAvatar("ipfs://" + result[1].cid);
                    } catch (e) {
                        LogError("Failed to set avatar: " + e);
                    }*/
                }
            }
            LogError("Failed to upload avatar: " + result[1].status);
        }

        DOM.inputAvatar.addEventListener("change", () => {
            let file = DOM.inputAvatar.files![0];
            DOM.avatarPreview.src = URL.createObjectURL(file);
            updateAvatar().then();
        })
    }
})();