window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/addPost.scss";
import {IsValidIpfsCid} from "../util/security";
import {GetWallet, WalletSubmitPost, WalletSubmitPostAttach} from "../util/blockchain/wallet";
import {ShowModalLogin} from "./modalLogin";
import {UploadFile} from "../util/files";
import {AddFileToIPFS} from "../util/ipfs";
import {HttpPostJson} from "../util/network";
import {AIGetSpiciness, AIIsEnabled} from "../services/ai";
import {ShowToastWithDelay} from "./toast";
import {ShowDialogModalHTML} from "./modalDialog";
import {CreateAttachmentPreview} from "../util/domFactory";
// TinyMCE will be lazy loaded when needed
let tinymceModulePromise: Promise<any> | null = null;

export async function preloadTinyMCE() {
    if (tinymceModulePromise) return tinymceModulePromise;
    tinymceModulePromise = import("tinymce/tinymce");
    return tinymceModulePromise;
}

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
            gatewayMode: document.getElementById("gatewayModeAddPost") as HTMLInputElement,
            tinymceSpinner: document.getElementById("tinymceSpinner")! as HTMLDivElement,
            attachmentDiv: document.getElementById("postAttachDiv")! as HTMLDivElement,
        }

        function isLocalhost(): boolean {
            const hostname = window.location.hostname;
            return hostname === 'localhost' ||
                   hostname === '127.0.0.1' ||
                   hostname === '[::1]';
        }
        function isGatewayMode(): boolean {
            return DOM.gatewayMode && DOM.gatewayMode.value === "true" && !isLocalhost();
        }
        function showGatewayUploadDialog() {
            hideModal();
            ShowDialogModalHTML(
                "To attach a file to a post, you need to host your own YourPlace server.<br><br>" +
                "<a href=\"https://yourplace.network/download\" target=\"_blank\" rel=\"noopener noreferrer\">Download YourPlace</a>"
            );
        }

        let tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipTriggerList.map(function (tooltipTriggerEl) {return new window.bootstrap.Tooltip(tooltipTriggerEl, {delay: {show: 1500, hide: 0}});});

        let addPostModal = new window.bootstrap.Modal(DOM.addPostModal, {});
        interface inlineMediaData {
            uuid: string;
            fileName: string;
            mimeType: string;
            blobUrl: string;
        }
        let inlineMediaMap: Map<string, inlineMediaData> = new Map();
        interface fileData {
            uuid: string;
            fileName: string;
            mimeType: string;
            size: string;
        }
        let binaryAttachments: fileData[] = [];
        let removedAttachments: string[] = [];
        let tinymceInitialized = false;
        let tinymceInitPromise: Promise<void> | null = null;

        async function initTinyMCE() {
            if (tinymceInitialized) return;
            if (tinymceInitPromise) return tinymceInitPromise;
            tinymceInitPromise = (async () => {
                const tinymce = tinymceModulePromise ? await tinymceModulePromise : await import("tinymce/tinymce");
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
                    paste_data_images: true,
                    automatic_uploads: true,
                    content_style: "img, video { max-width: 100%; height: auto; }",
                    images_upload_handler: async (blobInfo: any) => {
                        console.log("[addPost] images_upload_handler called", blobInfo);
                        if (isGatewayMode()) {
                            console.log("[addPost] Gateway mode, showing dialog");
                            showGatewayUploadDialog();
                            throw new Error("Gateway mode - uploads disabled");
                        }
                        const file = blobInfo.blob();
                        const fileName = blobInfo.filename() || `pasted-${Date.now()}.${file.type.split("/")[1] || "png"}`;
                        console.log("[addPost] Uploading file:", fileName, file.type, file.size);
                        const renamedFile = new File([file], fileName, {type: file.type});
                        const csrfToken = DOM.csrfToken.value;
                        const [status, data] = await UploadFile(renamedFile, csrfToken);
                        console.log("[addPost] Upload response:", status, data);
                        if (!data.data || data.data.length === 0) {
                            console.log("[addPost] Upload failed");
                            throw new Error("Failed to upload file");
                        }
                        const uploadedFile = data.data[0];
                        const blobUrl = URL.createObjectURL(renamedFile);
                        console.log("[addPost] Created blob URL:", blobUrl, "uuid:", uploadedFile.uuid);
                        inlineMediaMap.set(blobUrl, {
                            uuid: uploadedFile.uuid,
                            fileName: uploadedFile.fileName,
                            mimeType: uploadedFile.mimeType,
                            blobUrl: blobUrl
                        });
                        console.log("[addPost] inlineMediaMap now has", inlineMediaMap.size, "entries");
                        return blobUrl;
                    },
                    setup: function(editor: any) {
                        editor.on("input", function() {
                            debounceHandler();
                        });
                        editor.on("change", function() {
                            debounceHandler();
                        });
                        editor.on("drop", function(e: any) {
                            const dragEvent = e as DragEvent;
                            if (dragEvent.dataTransfer) {
                                handleDroppedMedia(e, dragEvent);
                            }
                        });
                    }
                });
                tinymceInitialized = true;
            })();
            return tinymceInitPromise;
        }
        function hideModal() {
            DOM.spiceometerText.innerText = "";
            addPostModal.hide();
            const modalBackdrops = document.querySelectorAll(".modal-backdrop");
            modalBackdrops.forEach(backdrop => backdrop.remove());
            if (tinymceInitialized) {
                (window as any).tinymce.get("addPostText")?.setContent("");
            }
            inlineMediaMap.forEach(data => URL.revokeObjectURL(data.blobUrl));
            inlineMediaMap.clear();
            binaryAttachments = [];
            removedAttachments = [];
            DOM.attachmentDiv.innerHTML = "";
        }
        async function showModal() {
            addPostModal.show();
            if (!tinymceInitialized) {
                DOM.tinymceSpinner.style.display = "flex";
            }
            await initTinyMCE();
            DOM.tinymceSpinner.style.display = "none";
            (window as any).tinymce.get("addPostText")?.focus();
            enableSpiceometer().then();
        }
        async function submitPost() {
            console.log("[addPost] submitPost called");
            if (!tinymceInitialized) {
                console.log("[addPost] TinyMCE not initialized, returning");
                return;
            }
            let payload = (window as any).tinymce.get("addPostText")?.getContent();
            console.log("[addPost] Payload:", payload);
            if (!payload || payload.trim() === "") {
                console.log("[addPost] Empty payload, hiding modal");
                hideModal();
                return;
            }
            if (!GetWallet()) {
                console.log("[addPost] No wallet connected - setting pending callback and showing login modal");
                ShowModalLogin();
                return;
            }
            DOM.submitPostButton.disabled = true;
            let csrfToken = DOM.csrfToken.value;
            console.log("[addPost] inlineMediaMap size:", inlineMediaMap.size);
            // Replace blob URLs with IPFS URLs for inline media
            for (const [blobUrl, mediaData] of inlineMediaMap) {
                console.log("[addPost] Processing media:", blobUrl, mediaData);
                let cid = await AddFileToIPFS(mediaData.uuid, csrfToken);
                console.log("[addPost] AddFileToIPFS returned:", cid);
                let cidString = cid?.toString();
                if (cidString === undefined || !IsValidIpfsCid(cidString)) {
                    console.log("[addPost] Invalid CID, showing error");
                    ShowToastWithDelay("Failed to upload file to IPFS", 5000);
                    DOM.submitPostButton.disabled = false;
                    return;
                }
                let ext = mediaData.fileName.split(".").pop() || "";
                let ipfsUrl = `ipfs://${cidString}.${ext}`;
                console.log("[addPost] Replacing", blobUrl, "with", ipfsUrl);
                payload = payload.split(blobUrl).join(ipfsUrl);
            }
            console.log("[addPost] Final payload after IPFS replacement:", payload);
            // Handle external image URLs - fetch via server proxy, upload to IPFS
            const externalImageRegex = /<img[^>]+src=["'](https?:\/\/[^"']+)["'][^>]*>/gi;
            let match;
            const processedUrls = new Set<string>();
            while ((match = externalImageRegex.exec(payload)) !== null) {
                const fullTag = match[0];
                const externalUrl = match[1];
                if (processedUrls.has(externalUrl)) continue;
                processedUrls.add(externalUrl);
                console.log("[addPost] Found external image:", externalUrl);
                try {
                    const [status, data] = await HttpPostJson("/files/fetch-external", {url: externalUrl}, csrfToken);
                    console.log("[addPost] Fetch external response:", status, data);
                    if (status !== 200 || !data.data || data.data.length === 0) {
                        console.log("[addPost] Failed to fetch external image via proxy");
                        continue;
                    }
                    const uploadedFile = data.data[0];
                    const cid = await AddFileToIPFS(uploadedFile.uuid, csrfToken);
                    const cidString = cid?.toString();
                    if (cidString === undefined || !IsValidIpfsCid(cidString)) {
                        console.log("[addPost] Failed to add external image to IPFS");
                        continue;
                    }
                    const ext = uploadedFile.fileName.split(".").pop() || "";
                    const ipfsUrl = `ipfs://${cidString}.${ext}`;
                    console.log("[addPost] Converted external image to IPFS:", ipfsUrl);
                    payload = payload.split(externalUrl).join(ipfsUrl);
                } catch (error) {
                    console.log("[addPost] Error processing external image:", error);
                }
            }
            console.log("[addPost] Final payload after external image processing:", payload);
            const filteredAttachments = binaryAttachments.filter(f => !removedAttachments.includes(f.fileName));
            console.log("[addPost] Binary attachments:", filteredAttachments.length);
            let success: boolean;
            if (filteredAttachments.length > 0) {
                let attachments: string[][] = [];
                for (const file of filteredAttachments) {
                    let cid = await AddFileToIPFS(file.uuid, csrfToken);
                    let cidString = cid?.toString();
                    if (cidString === undefined || !IsValidIpfsCid(cidString)) {
                        ShowToastWithDelay("Failed to upload attachment to IPFS", 5000);
                        DOM.submitPostButton.disabled = false;
                        return;
                    }
                    let ipfsUrl = `ipfs://${cidString}`;
                    attachments.push([ipfsUrl, file.mimeType, file.size, file.fileName]);
                }
                console.log("[addPost] Calling WalletSubmitPostAttach");
                success = await WalletSubmitPostAttach(payload, attachments);
            } else {
                console.log("[addPost] Calling WalletSubmitPost");
                success = await WalletSubmitPost(payload);
            }
            DOM.submitPostButton.disabled = false;
            if (!success) {
                console.log("[addPost] Post submission failed - wallet not connected or invalid");
                ShowToastWithDelay("Failed to submit post", 5000);
                return;
            }
            console.log("[addPost] Post submitted successfully");
            hideModal();
            ShowToastWithDelay("Your post should show up shortly. Please wait for it to spread through the network.", 10000);
            DOM.spiceometerText.innerText = "";
            if (typeof window.PostSubmitCallback === "function") {
                window.PostSubmitCallback();
            }
        }
        async function uploadFile() {
            let fileInput = DOM.fileInput;
            let fileList = fileInput.files;
            if (fileList == null) return;
            let csrfToken = DOM.csrfToken.value;
            DOM.uploadFileButton.disabled = true;
            DOM.submitPostButton.disabled = true;
            for (const file of fileList) {
                if (file.type.startsWith("image/") || file.type.startsWith("video/")) {
                    await insertMediaIntoEditor(file, csrfToken);
                } else {
                    await addBinaryAttachment(file, csrfToken);
                }
            }
            DOM.uploadFileButton.disabled = false;
            DOM.submitPostButton.disabled = false;
            fileInput.value = "";
        }
        async function insertMediaIntoEditor(file: File, csrfToken: string) {
            const editor = (window as any).tinymce.get("addPostText");
            if (!editor) return;
            let [status, data] = await UploadFile(file, csrfToken);
            if (!data.data || data.data.length === 0) {
                ShowToastWithDelay("Failed to upload file", 5000);
                return;
            }
            const uploadedFile = data.data[0];
            const blobUrl = URL.createObjectURL(file);
            inlineMediaMap.set(blobUrl, {
                uuid: uploadedFile.uuid,
                fileName: uploadedFile.fileName,
                mimeType: uploadedFile.mimeType,
                blobUrl: blobUrl
            });
            if (file.type.startsWith("image/")) {
                editor.insertContent(`<img src="${blobUrl}" alt="${file.name}">`);
            } else if (file.type.startsWith("video/")) {
                editor.insertContent(`<video src="${blobUrl}" controls></video>`);
            }
        }
        async function addBinaryAttachment(file: File, csrfToken: string) {
            let [status, data] = await UploadFile(file, csrfToken);
            if (!data.data || data.data.length === 0) {
                ShowToastWithDelay("Failed to upload file", 5000);
                return;
            }
            const uploadedFile = data.data[0];
            binaryAttachments.push({
                uuid: uploadedFile.uuid,
                fileName: uploadedFile.fileName,
                mimeType: uploadedFile.mimeType,
                size: uploadedFile.size
            });
            const previewElement = await CreateAttachmentPreview(file);
            const fileNameElement = previewElement.querySelector(".fileNameSpan")! as HTMLSpanElement;
            const removeButton = previewElement.querySelector(".removeButton") as HTMLButtonElement;
            const fileName = fileNameElement.textContent!;
            removeButton.addEventListener("click", function () {
                removedAttachments.push(fileName);
                previewElement.remove();
            });
            DOM.attachmentDiv.appendChild(previewElement);
        }
        async function handleDroppedMedia(tinymceEvent: any, e: DragEvent) {
            const dataTransfer = e.dataTransfer;
            if (!dataTransfer || !dataTransfer.files || dataTransfer.files.length === 0) return;
            const files = dataTransfer.files;
            const mediaFiles: File[] = [];
            const otherFiles: File[] = [];
            for (let i = 0; i < files.length; i++) {
                const file = files[i];
                if (file.type.startsWith("image/") || file.type.startsWith("video/")) {
                    mediaFiles.push(file);
                } else {
                    otherFiles.push(file);
                }
            }
            if (mediaFiles.length === 0 && otherFiles.length === 0) return;
            tinymceEvent.preventDefault();
            if (isGatewayMode()) {
                showGatewayUploadDialog();
                return;
            }
            let csrfToken = DOM.csrfToken.value;
            DOM.uploadFileButton.disabled = true;
            DOM.submitPostButton.disabled = true;
            for (const file of mediaFiles) {
                await insertMediaIntoEditor(file, csrfToken);
            }
            for (const file of otherFiles) {
                await addBinaryAttachment(file, csrfToken);
            }
            DOM.uploadFileButton.disabled = false;
            DOM.submitPostButton.disabled = false;
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
            if (!tinymceInitialized) return;
            let quote = (window as any).tinymce.get("addPostText")?.getContent();
            if (!quote) return;
            quote = quote.replace(/<[^>]*>/g, "");
            if (quote.length < 3) {
                DOM.spiceometerText.innerText = "";
                return;
            }
            let spiciness = await AIGetSpiciness(quote);
            if (spiciness == -1) {
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
                DOM.spiceometerText.innerText = "";
                window.clearTimeout(timeoutId);
                timeoutId = window.setTimeout(() => {
                    func(...args);
                }, delay);
            };
        }
        const handleInput = () => { checkSpiciness().then(); };
        const debounceHandler = debounce(handleInput, 2000);
        function clickFileInput() {
            if (isGatewayMode()) {
                showGatewayUploadDialog();
                return;
            }
            DOM.fileInput.click();
        }

        DOM.addPostButton.addEventListener("click", showModal);
        DOM.submitPostButton.addEventListener("click", submitPost);
        DOM.uploadFileButton.addEventListener("click", clickFileInput);
        DOM.fileInput.addEventListener("change", uploadFile);
        document.addEventListener("focusin", (e) => {
            if (e.target instanceof Element && e.target.closest(".tox-tinymce-aux, .moxman-window, .tam-assetmanager-root") !== null) {
                e.stopImmediatePropagation();
            }
        });
    }
})();
