window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/addPost.scss";
import {IsValidIpfsCid, XSSSanitizeUrl, XSSSanitizeValue} from "../util/security";
import {GetAddress, GetChain, GetWallet, WalletGetAvatar, WalletGetName, WalletSubmitPost, WalletSubmitPostAttach} from "../util/blockchain/wallet";
import {UploadFile} from "../util/files";
import {AddFileToIPFS, getIpfsAvatarUrl} from "../util/ipfs";
import {HttpPostJson} from "../util/network";
import {AIGetSpiciness, AIIsEnabled} from "../services/ai";
import {ShowToastWithDelay} from "./toast";
import {ShowDialogModalHTML} from "./modalDialog";
import {CreateAttachmentPreview} from "../util/domFactory";
import {OEmbedCard} from "./oEmbedCard";
import {XcomCrossPost, XcomIsCrossPostEnabled} from "../services/xcom";
import {XcomOEmbedCard} from "./xcomOEmbedCard";
import {setupTinyMCEEmojiButton} from "../util/emojiPicker";
// TinyMCE will be lazy loaded when needed
let tinymceModulePromise: Promise<any> | null = null;

async function preloadTinyMCE() {
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
            addPostLinkPreview: document.getElementById("addPostLinkPreview")! as HTMLDivElement,
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
        let linkPreviewRenderVersion = 0;
        let removedAttachments: string[] = [];
        let tinymceInitialized = false;
        let tinymceInitPromise: Promise<void> | null = null;

        function formatUrlDisplayText(url: string): string {
            return url.replace(/^https:\/\/(www\.)?/, "").replace(/[?#].*$/, "");
        }
        function createImageEmbed(url: string): HTMLImageElement | null {
            const imageRegex = /^https:\/\/.*\.(jpg|jpeg|gif|webp|png|svg)$/i;
            if (!imageRegex.test(url)) {
                return null;
            }
            const image = document.createElement("img");
            image.classList.add("postCardEmbeddedImage");
            image.crossOrigin = "anonymous";
            image.referrerPolicy = "no-referrer";
            image.src = XSSSanitizeUrl(url);
            return image;
        }
        function createYoutubeEmbed(url: string): HTMLIFrameElement | null {
            const youtubeRegex = /^https:\/\/((?:www\.)?youtube\.com\/watch\?v=|youtu\.be\/)([a-zA-Z0-9_-]{11})(?:[?&].*)?$/;
            const match = url.match(youtubeRegex);
            if (!match) {
                return null;
            }
            const iframe = document.createElement("iframe");
            iframe.classList.add("postCardEmbeddedIframe");
            iframe.src = XSSSanitizeUrl(`https://www.youtube-nocookie.com/embed/${match[2]}`);
            iframe.allow = "encrypted-media; picture-in-picture";
            iframe.allowFullscreen = true;
            iframe.setAttribute("loading", "lazy");
            iframe.setAttribute("credentialless", "");
            return iframe;
        }
        function extractUrlsFromNode(node: Node, urls: string[], seenUrls: Set<string>) {
            if (node.nodeType === Node.ELEMENT_NODE) {
                const element = node as HTMLElement;
                if (element.tagName === "A") {
                    const href = element.getAttribute("href");
                    if (href && href.startsWith("https://") && !seenUrls.has(href)) {
                        seenUrls.add(href);
                        urls.push(href);
                    }
                    return;
                }
                Array.from(element.childNodes).forEach(childNode => {
                    extractUrlsFromNode(childNode, urls, seenUrls);
                });
                return;
            }
            if (node.nodeType !== Node.TEXT_NODE) {
                return;
            }
            const textContent = node.textContent || "";
            const matches = textContent.match(/https:\/\/[^\s"<>]+/g);
            if (!matches) {
                return;
            }
            for (const match of matches) {
                if (!seenUrls.has(match)) {
                    seenUrls.add(match);
                    urls.push(match);
                }
            }
        }
        function extractUrlsFromHtml(html: string): string[] {
            const tempDiv = document.createElement("div");
            const seenUrls = new Set<string>();
            const urls: string[] = [];
            tempDiv.innerHTML = html;
            Array.from(tempDiv.childNodes).forEach(node => {
                extractUrlsFromNode(node, urls, seenUrls);
            });
            return urls;
        }
        function linkifyTextNode(textNode: Text): boolean {
            const text = textNode.textContent || "";
            const matches = Array.from(text.matchAll(/https:\/\/[^\s"<>]+/g));
            if (matches.length === 0) {
                return false;
            }
            const doc = textNode.ownerDocument;
            const fragment = doc.createDocumentFragment();
            let cursor = 0;
            for (const match of matches) {
                const url = match[0];
                const index = match.index || 0;
                if (index > cursor) {
                    fragment.appendChild(doc.createTextNode(text.slice(cursor, index)));
                }
                const anchor = doc.createElement("a");
                anchor.href = XSSSanitizeUrl(url);
                anchor.rel = "noopener noreferrer";
                anchor.target = "_blank";
                anchor.textContent = formatUrlDisplayText(url);
                fragment.appendChild(anchor);
                cursor = index + url.length;
            }
            if (cursor < text.length) {
                fragment.appendChild(doc.createTextNode(text.slice(cursor)));
            }
            textNode.parentNode?.replaceChild(fragment, textNode);
            return true;
        }
        function normalizeEditorLinks(editor: any) {
            const body = editor.getBody() as HTMLElement | null;
            if (!body) return;
            const nodeFilter = body.ownerDocument.defaultView?.NodeFilter || window.NodeFilter;
            const walker = body.ownerDocument.createTreeWalker(body, nodeFilter.SHOW_TEXT, {
                acceptNode(node) {
                    const parentElement = node.parentElement;
                    if (!parentElement || parentElement.closest("a")) {
                        return nodeFilter.FILTER_REJECT;
                    }
                    if ((node.textContent || "").includes("https://")) {
                        return nodeFilter.FILTER_ACCEPT;
                    }
                    return nodeFilter.FILTER_REJECT;
                }
            });
            const textNodes: Text[] = [];
            let currentNode = walker.nextNode();
            while (currentNode) {
                textNodes.push(currentNode as Text);
                currentNode = walker.nextNode();
            }
            let didMutate = false;
            for (const textNode of textNodes) {
                didMutate = linkifyTextNode(textNode) || didMutate;
            }
            if (didMutate) {
                editor.nodeChanged();
            }
        }
        function flattenAutoLinkedUrls(html: string): string {
            const tempDiv = document.createElement("div");
            tempDiv.innerHTML = html;
            const anchors = tempDiv.querySelectorAll("a[href]");
            anchors.forEach(anchor => {
                const href = anchor.getAttribute("href");
                const text = anchor.textContent?.trim() || "";
                if (!href || !href.startsWith("https://")) {
                    return;
                }
                if (text === href || text === formatUrlDisplayText(href)) {
                    anchor.replaceWith(document.createTextNode(href));
                }
            });
            return tempDiv.innerHTML;
        }
        async function createLinkPreviewEmbed(url: string): Promise<HTMLElement | null> {
            const imageEmbed = createImageEmbed(url);
            if (imageEmbed) {
                return imageEmbed;
            }
            const youtubeEmbed = createYoutubeEmbed(url);
            if (youtubeEmbed) {
                return youtubeEmbed;
            }
            const xcomEmbed = await XcomOEmbedCard(url);
            if (xcomEmbed) {
                return xcomEmbed;
            }
            return OEmbedCard(url);
        }
        async function renderLinkPreview(editor: any) {
            const renderVersion = ++linkPreviewRenderVersion;
            DOM.addPostLinkPreview.innerHTML = "";
            const urls = extractUrlsFromHtml(editor.getContent());
            if (urls.length === 0) {
                return;
            }
            const previewDiv = document.createElement("div");
            previewDiv.classList.add("postCardEmbedDiv");
            for (const url of urls) {
                const embed = await createLinkPreviewEmbed(url);
                if (renderVersion !== linkPreviewRenderVersion) {
                    return;
                }
                if (embed) {
                    previewDiv.appendChild(embed);
                }
            }
            if (renderVersion === linkPreviewRenderVersion && previewDiv.children.length > 0) {
                DOM.addPostLinkPreview.appendChild(previewDiv);
            }
        }

        async function initTinyMCE() {
            if (tinymceInitialized) return;
            if (tinymceInitPromise) return tinymceInitPromise;
            tinymceInitPromise = (async () => {
                const tinymce = tinymceModulePromise ? await tinymceModulePromise : await import("tinymce/tinymce");
                const isMobile = window.innerWidth < 768;
                await tinymce.default.init({
                    selector: "#addPostText",
                    plugins: "autolink code table lists",
                    toolbar: isMobile
                        ? "emojipicker forecolor backcolor | formatting"
                        : "emojipicker forecolor backcolor | bold italic underline strikethrough | bullist numlist",
                    toolbar_groups: isMobile ? {
                        formatting: {
                            icon: 'more-drawer',
                            items: 'bold italic underline strikethrough | bullist numlist'
                        }
                    } : {},
                    toolbar_mode: "sliding",
                    menubar: false,
                    statusbar: true,
                    elementpath: false,
                    min_height: 200,
                    base_url: "/static/tinymce",
                    resize: true,
                    branding: false,
                    license_key: "gpl",
                    paste_data_images: true,
                    automatic_uploads: true,
                    content_css: "/static/css/tinymce.css",
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
                        setupTinyMCEEmojiButton(editor);
                        editor.on("input", function() {
                            debounceHandler();
                            debounceLinkPreviewHandler();
                        });
                        editor.on("change", function() {
                            debounceHandler();
                            debounceLinkPreviewHandler();
                        });
                        editor.on("PastePostProcess", function() {
                            window.setTimeout(() => {
                                normalizeEditorLinks(editor);
                                debounceLinkPreviewHandler();
                            }, 0);
                        });
                        editor.on("drop", function(e: any) {
                            const dragEvent = e as DragEvent;
                            if (dragEvent.dataTransfer) {
                                handleDroppedMedia(e, dragEvent);
                            }
                        });
                        editor.on("SetContent", function() {
                            debounceLinkPreviewHandler();
                        });
                    }
                });
                tinymceInitialized = true;
                addAvatarToToolbar().then();
            })();
            return tinymceInitPromise;
        }
        async function addAvatarToToolbar() {
            const modalBody = DOM.addPostModal.querySelector(".modal-body") as HTMLElement;
            if (!modalBody) return;
            const existing = modalBody.querySelector("#addPostAvatarBtn");
            if (existing) existing.remove();
            const blockchain = GetChain();
            const address = GetAddress();
            if (!blockchain || !address) return;
            const avatarLink = document.createElement("a");
            avatarLink.id = "addPostAvatarBtn";
            avatarLink.href = `/p/${encodeURIComponent(blockchain)}/${encodeURIComponent(address)}`;
            avatarLink.title = "Posting as Anonymous";
            const avatarImg = document.createElement("img");
            avatarImg.src = "/static/image/avatar.png";
            avatarImg.alt = "Profile";
            avatarImg.width = 28;
            avatarImg.height = 28;
            avatarLink.appendChild(avatarImg);
            modalBody.appendChild(avatarLink);
            let avatarUrl: string | null = await getIpfsAvatarUrl(blockchain, address);
            if (!avatarUrl) {
                avatarUrl = await WalletGetAvatar(blockchain, address);
            }
            if (avatarUrl) {
                avatarImg.src = XSSSanitizeUrl(avatarUrl);
            }
            const name = await WalletGetName(blockchain, address);
            avatarLink.title = `Posting as ${name && name.length > 0 ? name : "Anonymous"}`;
        }
        function hideModal() {
            linkPreviewRenderVersion++;
            DOM.spiceometerText.innerText = "";
            addPostModal.hide();
            const modalBackdrops = document.querySelectorAll(".modal-backdrop");
            modalBackdrops.forEach(backdrop => backdrop.remove());
            if (tinymceInitialized) {
                (window as any).tinymce.get("addPostText")?.setContent("");
            }
            DOM.addPostLinkPreview.innerHTML = "";
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
            let payload = flattenAutoLinkedUrls((window as any).tinymce.get("addPostText")?.getContent() || "");
            console.log("[addPost] Payload:", payload);
            if (!payload || payload.trim() === "") {
                console.log("[addPost] Empty payload, hiding modal");
                hideModal();
                return;
            }
            if (!GetWallet()) {
                console.log("[addPost] No wallet connected - redirecting to login page");
                window.location.href = "/login";
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
            let crossPostEnabled = await XcomIsCrossPostEnabled();
            if (crossPostEnabled) {
                let plainText = payload.replace(/<[^>]*>/g, "").trim();
                if (plainText.length > 0) {
                    let crossPostSuccess = await XcomCrossPost(plainText, csrfToken);
                    if (crossPostSuccess) {
                        ShowToastWithDelay("Cross-posted to X.com", 3000);
                    }
                }
            }
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
        const handleLinkPreview = () => {
            const editor = (window as any).tinymce.get("addPostText");
            if (!editor) return;
            renderLinkPreview(editor).then();
        };
        const debounceLinkPreviewHandler = debounce(handleLinkPreview, 250);
        function clickFileInput() {
            if (isGatewayMode()) {
                showGatewayUploadDialog();
                return;
            }
            DOM.fileInput.click();
        }

        async function checkDraftParam() {
            const params = new URLSearchParams(window.location.search);
            const draft = params.get("draft");
            if (!draft) return;
            await showModal();
            const editor = (window as any).tinymce.get("addPostText");
            if (editor) {
                const sanitized = XSSSanitizeValue(draft);
                editor.setContent(`<p>${sanitized.replace(/^ /, "&nbsp;")}</p>`);
                normalizeEditorLinks(editor);
                renderLinkPreview(editor).then();
            }
        }
        DOM.addPostButton.addEventListener("click", showModal);
        DOM.submitPostButton.addEventListener("click", submitPost);
        DOM.uploadFileButton.addEventListener("click", clickFileInput);
        DOM.fileInput.addEventListener("change", uploadFile);
        preloadTinyMCE().then(() => initTinyMCE());
        document.addEventListener("focusin", (e) => {
            if (e.target instanceof Element && e.target.closest(".emojiPickerPopup, .tox-tinymce-aux, .moxman-window, .tam-assetmanager-root") !== null) {
                e.stopImmediatePropagation();
            }
        });
        checkDraftParam().then();
    }
})();
