import {AddFileToIPFS} from "../util/ipfs";

window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/addPost.scss";
import {WalletSubmitPost, WalletSubmitPostAttach} from "../util/blockchain/wallet";
import {HttpGetJson} from "../util/network";
import {UploadFile} from "../util/files";
import {Sleep} from "../util/time";
import {AIGetSpiciness, AIIsEnabled} from "../services/ai";
import tinymce from "tinymce/tinymce";
import {lookup} from "mime-types";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            addPostModal: document.getElementById("addPostModal")! as HTMLDivElement,
            addPostText: document.getElementById("addPostText")! as HTMLTextAreaElement,
            addPostButton: document.getElementById("addPostButton")! as HTMLButtonElement,
            submitPostButton: document.getElementById("submitPostButton")! as HTMLButtonElement,
            uploadFileButton: document.getElementById("btnUploadFile")! as HTMLButtonElement,
            fileInput: document.getElementById("file")! as HTMLInputElement,
            spiceometerDiv: document.getElementById("spiceometerDiv")! as HTMLDivElement,
            spiceometerText: document.getElementById("spiceometerText")! as HTMLDivElement,
        }
        let tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipTriggerList.map(function (tooltipTriggerEl) {return new window.bootstrap.Tooltip(tooltipTriggerEl, {delay: {show: 1500, hide: 0}});});
        let postObj = {postText: "", fileHash: "", status: "", cid: "", extension: ""}
        let addPostModal = new window.bootstrap.Modal(DOM.addPostModal, {});
        let uploadedFiles: fileData[] = [];
        interface fileData {
            uuid: string;
            path: string;
            extension: string;
            encodedUnsafeName: string;
            size: string;
            ipfs?: string;
        }

        tinymce.init({
            selector: "#addPostText",
            plugins: "code table lists emoticons",
            toolbar: "styles | forecolor backcolor emoticons | bullist numlist | code",
            toolbar_mode: "sliding",
            menubar: false,
            statusbar: true,
            min_height: 200,
            base_url: "/static/tinymce",
            resize: true,
            branding: false,
            license_key: "gpl",
            //paste_data_images: false, // todo
            setup: function(editor) {
                editor.on("input", function() {
                    debounceHandler();
                });
                editor.on("change", function() {
                    debounceHandler();
                });
            }
        });

        function hideModal() {
            DOM.spiceometerText.innerText = "";
            addPostModal.hide();
            // Ensure the modal backdrop is removed
            const modalBackdrops = document.querySelectorAll(".modal-backdrop");
            modalBackdrops.forEach(backdrop => {
                backdrop.remove();
            })
            tinymce.get("addPostText")!.setContent("");
        }
        function showModal() {
            addPostModal.show();
            tinymce.get("addPostText")!.focus();
            enableSpiceometer().then();
        }

        async function submitPost() {
            let payload = tinymce.get("addPostText")!.getContent();
            if (!payload || payload.trim() === "") {
                hideModal();
                return;
            }
            console.log("post payload");
            console.log(payload);
            // If there is a file attached, upload it first
            if (Array.isArray(uploadedFiles) && uploadedFiles.length > 0) {
                console.log("prepare attached if triggered")
                postObj.postText = payload;
                await prepareAttachedPost();
            } else {
                await WalletSubmitPost(payload);
            }
            hideModal();
            DOM.spiceometerText.innerText = "";
            tinymce.get("addPostText")!.setContent("");
        }
        async function prepareAttachedPost() {
            await ipfsUpload();
            let attachments: string[][] = [];
            for (let i = 0; i < uploadedFiles.length; i++){
                let file = uploadedFiles[i];
                let ipfs = file.ipfs;
                let mimeType = lookup(file.extension);
                let size = file.size
                if (typeof ipfs === 'string' && typeof mimeType === 'string' && size != ""){
                    let attachment = [ipfs, mimeType, size];
                    attachments.push(attachment);
                } else return
            }
            await WalletSubmitPostAttach(postObj.postText, attachments);
            clearPostObj();
            uploadedFiles = [];
        }
        function clearPostObj() {
            postObj.postText = "";
            postObj.cid = "";
            postObj.fileHash = "";
            postObj.status = "";
        }
        async function ipfsUpload(){
            let csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
            for (let i = 0; i <uploadedFiles.length; i++){
                let cid = await AddFileToIPFS(uploadedFiles[i].path, csrfToken);
                uploadedFiles[i].ipfs = "ipfs://" + cid?.toString();
            }
        }
        async function uploadFile() {
            console.log("upload triggered");
            // todo: not used right now. Needs better tinymce integration. Allow for future drag & drop upload too
            let fileInput = DOM.fileInput;
            let fileList = fileInput.files;
            if (fileList == null) {
                return;
            }
            let csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
            // Show some loading indication
            DOM.uploadFileButton.disabled = true;
            DOM.uploadFileButton.textContent = "Uploading...";

            let [status, data] = await UploadFile(fileList, csrfToken);
            uploadedFiles = data.data;
            console.log(uploadedFiles);

            // Reset UI after upload
            DOM.uploadFileButton.disabled = false;
            DOM.uploadFileButton.textContent = "Attach File";
        }
        async function checkVideoStatus(fileHash: string) {
            let [status, data] = await HttpGetJson("/files/checkvideo/" + fileHash);
            return data
        }
        async function enableSpiceometer() {
            let isEnabled = await AIIsEnabled();
            if (isEnabled) {
                DOM.spiceometerText.innerText = "";
                DOM.spiceometerDiv.style.display = "block";
            } else {
                DOM.spiceometerText.innerText = "";
                DOM.spiceometerDiv.style.display = "none";
            }
        }
        async function checkSpiciness() {
            let quote = tinymce.get("addPostText")!.getContent();
            // Strip HTML tags for spiciness check
            quote = quote.replace(/<[^>]*>/g, "");
            if (quote.length < 3) {
                DOM.spiceometerText.innerText = "";
                return;
            }
            let spiciness = await AIGetSpiciness(quote);
            if (spiciness == -1) { // error
                DOM.spiceometerText.innerText = "";
                return;
            }
            let chilies = "";
            for (let i = 0; i < spiciness; i++) {
                chilies += "🌶️";
            }
            DOM.spiceometerText.innerText = chilies;
        }
        function debounce<T extends (...args: any[]) => void>(func: T, delay: number): (...args: Parameters<T>) => void {
            let timeoutId: number;
            return (...args: Parameters<T>) => {
                // Clear the spiceometer text here when the function is actually called
                DOM.spiceometerText.innerText = "";
                window.clearTimeout(timeoutId);
                timeoutId = window.setTimeout(() => {
                    func(...args);
                }, delay);
            };
        }
        const handleInput = () => {
            checkSpiciness().then();
        };
        const debounceHandler = debounce(handleInput, 2000);
        function clickFileInput() {
            DOM.fileInput.click()
        }

        DOM.addPostButton.addEventListener("click", showModal);
        DOM.submitPostButton.addEventListener("click", submitPost);
        DOM.uploadFileButton.addEventListener("click", clickFileInput);
        DOM.fileInput.addEventListener("change", uploadFile)
        document.addEventListener("focusin", (e) => { // Prevent Bootstrap dialog from blocking focusin, thus breaking tinymce
            if (e.target instanceof Element && e.target.closest(".tox-tinymce-aux, .moxman-window, .tam-assetmanager-root") !== null) {
                e.stopImmediatePropagation();
            }
        });
    }
})();
