window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/addPost.scss";
import {IsValidIpfsCid} from "../util/security";
import {WalletSubmitPost, WalletSubmitPostAttach} from "../util/blockchain/wallet";
import {HttpGetJson} from "../util/network";
import {CreateAttachmentPreview} from "../util/domFactory";
import {UploadFile} from "../util/files";
import {AddFileToIPFS} from "../util/ipfs";
import {AIGetSpiciness, AIIsEnabled} from "../services/ai";
import {ShowToastWithDelay} from "./toast";
// TinyMCE will be lazy loaded when needed

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
            csrfToken: document.getElementById("csrfToken") as HTMLInputElement,
            attachmentDiv: document.getElementById("postAttachDiv")! as HTMLDivElement,
            gatewayMode: document.getElementById("gatewayModeAddPost") as HTMLInputElement,
        }

        function isLocalhost(): boolean {
            const hostname = window.location.hostname;
            return hostname === 'localhost' ||
                   hostname === '127.0.0.1' ||
                   hostname === '[::1]';
        }

        function disableUploadInGatewayMode() {
            if (DOM.gatewayMode && DOM.gatewayMode.value === "true" && !isLocalhost()) {
                DOM.uploadFileButton.disabled = true;
                DOM.uploadFileButton.style.opacity = "0.5";
                DOM.uploadFileButton.style.cursor = "not-allowed";
                DOM.uploadFileButton.setAttribute("data-bs-original-title", "File uploads are disabled on this gateway.<br>Download your own server at <a href='https://yourplace.network/download' target='_blank' style='color: #fff; text-decoration: underline;'>yourplace.network/download</a> to enable file uploading");
            }
        }

        let tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipTriggerList.map(function (tooltipTriggerEl) {return new window.bootstrap.Tooltip(tooltipTriggerEl, {delay: {show: 1500, hide: 0}});});

        disableUploadInGatewayMode();
        let postObj = {postText: "", fileHash: "", status: "", cid: "", extension: ""}
        let addPostModal = new window.bootstrap.Modal(DOM.addPostModal, {});
        let uploadedFiles: fileData[] = [];
        let removedFiles: string[] = [];
        interface fileData {
            uuid: string;
            pathOnDisk: string;
            mimeType: string;
            fileName: string;
            size: string;
            fileUrl?: string;
        }
        let tinymceLoaded = false;
        let tinymceLoadingPromise: Promise<void> | null = null;

        async function initTinyMCE() {
            if (tinymceLoaded) return;
            if (tinymceLoadingPromise) return tinymceLoadingPromise;
            tinymceLoadingPromise = (async () => {
                const tinymce = await import("tinymce/tinymce");
                await tinymce.default.init({
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
                tinymceLoaded = true;
            })();
            return tinymceLoadingPromise;
        }

        function hideModal() {
            DOM.spiceometerText.innerText = "";
            addPostModal.hide();
            // Ensure the modal backdrop is removed
            const modalBackdrops = document.querySelectorAll(".modal-backdrop");
            modalBackdrops.forEach(backdrop => {
                backdrop.remove();
            })
            if (tinymceLoaded) {
                (window as any).tinymce.get("addPostText")?.setContent("");
            }
        }
        async function showModal() {
            addPostModal.show();
            await initTinyMCE();
            (window as any).tinymce.get("addPostText")?.focus();
            enableSpiceometer().then();
        }

        async function submitPost() {
            if (!tinymceLoaded) return;

            let payload = (window as any).tinymce.get("addPostText")?.getContent();
            if (!payload || payload.trim() === "") {
                hideModal();
                return;
            }
            stripRemovedAttachments();
            if (Array.isArray(uploadedFiles) && uploadedFiles.length > 0) {
                postObj.postText = payload;
                await prepareAttachedPost();
            } else {
                await WalletSubmitPost(payload);
            }
            clearPostObj();
            hideModal();
            ShowToastWithDelay("Your post should show up shortly. Please wait for it to spread through the network.", 10000);
            DOM.spiceometerText.innerText = "";
            (window as any).tinymce.get("addPostText")?.setContent("");
        }
        async function prepareAttachedPost() {
            let csrfToken = DOM.csrfToken.value;
            for (let i = 0; i < uploadedFiles.length; i++){
                let cid = await AddFileToIPFS(uploadedFiles[i].uuid, csrfToken);
                let cidString = cid?.toString()
                if (cidString === undefined || !IsValidIpfsCid(cidString)) {
                    return
                }
                uploadedFiles[i].fileUrl = "ipfs://" + cidString;
            }
            let attachments: string[][] = [];
            for (let i = 0; i < uploadedFiles.length; i++){
                let file = uploadedFiles[i];
                let url = file.fileUrl;
                let mimeType = file.mimeType;
                if (mimeType == null) {return}
                let size = file.size;
                let fileName = file.fileName;
                if (typeof url === "string" && mimeType !== "" && size !== "" && fileName !== ""){
                    let attachment = [url, mimeType, size, fileName];
                    attachments.push(attachment);
                } else return
            }
            WalletSubmitPostAttach(postObj.postText, attachments);
            uploadedFiles = [];
            removedFiles = [];
        }
        function clearPostObj() {
            postObj.postText = "";
            postObj.cid = "";
            postObj.fileHash = "";
            postObj.status = "";
        }
        async function uploadFile() {
            // todo: Needs better tinymce integration. Allow for future drag & drop upload too
            let fileInput = DOM.fileInput;
            let fileList = fileInput.files;
            if (fileList == null) {
                return;
            }
            let csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
            // Show some loading indication
            DOM.uploadFileButton.disabled = true;
            DOM.submitPostButton.disabled = true;
            for (const file of fileList) {
                const previewElement = await CreateAttachmentPreview(file);
                const fileNameElement = previewElement.querySelector(".fileNameSpan")! as HTMLSpanElement;
                const removeButton = previewElement.querySelector(".removeButton") as HTMLButtonElement;
                const fileName = fileNameElement.textContent!;
                removeButton.addEventListener("click", function () {
                    removedFiles.push(fileName);
                    previewElement.remove();
                })
                DOM.attachmentDiv.appendChild(previewElement);
            }
            let [status, data] = await UploadFile(fileList, csrfToken);
            uploadedFiles.push(...data.data);
            // Reset UI after upload
            DOM.uploadFileButton.disabled = false;
            DOM.submitPostButton.disabled = false;
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
            if (!tinymceLoaded) return;
            let quote = (window as any).tinymce.get("addPostText")?.getContent();
            if (!quote) return;
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
        function stripRemovedAttachments() {
            uploadedFiles = uploadedFiles.filter(file => !removedFiles?.includes(file.fileName));
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
            DOM.fileInput.click();
        }

        DOM.addPostButton.addEventListener("click", showModal);
        DOM.submitPostButton.addEventListener("click", submitPost);
        DOM.uploadFileButton.addEventListener("click", clickFileInput);
        DOM.fileInput.addEventListener("change", uploadFile);
        document.addEventListener("focusin", (e) => { // Prevent Bootstrap dialog from blocking focusin, thus breaking tinymce
            if (e.target instanceof Element && e.target.closest(".tox-tinymce-aux, .moxman-window, .tam-assetmanager-root") !== null) {
                e.stopImmediatePropagation();
            }
        });
    }
})();
