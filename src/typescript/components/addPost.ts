window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/addPost.scss";
import {IsValidIpfsCid, IsValidURL, XSSSanitizeUrl, XSSSanitizeValue} from "../util/security";
import {GetAddress, GetChain, GetWallet, WalletGetAvatar, WalletGetName, WalletSubmitPost, WalletSubmitPostAttachTx} from "../util/blockchain/wallet";
import {CreateLocalPost, FinalizeFiles, UploadFile} from "../util/files";
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
            savedPostCheckbox: document.getElementById("savedPostCheckbox") as HTMLInputElement,
            tinymceSpinner: document.getElementById("tinymceSpinner")! as HTMLDivElement,
            attachmentDiv: document.getElementById("postAttachDiv")! as HTMLDivElement,
            userAddress: document.getElementById("userAddress") as HTMLInputElement | null,
            userBlockchain: document.getElementById("userBlockchain") as HTMLInputElement | null,
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
        function shouldCreatePublicPost(): boolean {
            if (isGatewayMode()) {
                return true;
            }
            return !DOM.savedPostCheckbox || DOM.savedPostCheckbox.checked;
        }
        function showGatewayUploadDialog() {
            hideModal();
            ShowDialogModalHTML(
                "To attach a file to a post, you need to host your own YourPlace server.<br><br>" +
                "<a href=\"https://yourplace.network/download\" target=\"_blank\" rel=\"noopener noreferrer\">Download YourPlace</a>"
            );
        }
        function showGatewayHotlinkDialog(url: string) {
            if (gatewayHotlinkDialogUrls.has(url)) return;
            gatewayHotlinkDialogUrls.add(url);
            ShowDialogModalHTML(
                "This image is hosted on a 3rd party server, which does not allow hotlinking.<br>" +
                "Please download the YourPlace Server at " +
                "<a href=\"https://yourplace.network/download\" target=\"_blank\" rel=\"noopener noreferrer\">https://yourplace.network/download</a> " +
                "to host and upload it yourself"
            );
        }

        let tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipTriggerList.map(function (tooltipTriggerEl) {return new window.bootstrap.Tooltip(tooltipTriggerEl, {delay: {show: 1500, hide: 0}});});
        if (DOM.savedPostCheckbox) {
            DOM.savedPostCheckbox.disabled = isGatewayMode();
        }

        let addPostModal = new window.bootstrap.Modal(DOM.addPostModal, {});
        interface inlineMediaData {
            cid: string;
            fileName: string;
            mimeType: string;
            size: string;
            blobUrl: string;
        }
        let inlineMediaMap: Map<string, inlineMediaData> = new Map();
        interface fileData {
            cid: string;
            fileName: string;
            mimeType: string;
            size: string;
        }
        interface importedExternalImageData {
            cid: string;
            fileName: string;
            mimeType: string;
            size: string;
        }
        interface externalImageState {
            allowsEmbedding: boolean;
            importedFile?: importedExternalImageData;
            ipfsUrl?: string;
        }
        let binaryAttachments: fileData[] = [];
        let externalImageMap: Map<string, externalImageState> = new Map();
        let externalImageRequestMap: Map<string, Promise<externalImageState>> = new Map();
        let externalImageProcessingPromise: Promise<void> = Promise.resolve();
        let gatewayHotlinkDialogUrls: Set<string> = new Set();
        let linkPreviewRenderVersion = 0;
        let removedAttachments: string[] = [];
        let tinymceInitialized = false;
        let tinymceInitPromise: Promise<void> | null = null;

        function hasMatchingWalletIdentity(): boolean {
            const activeAddress = GetAddress();
            const activeBlockchain = GetChain();
            if (!activeAddress || !activeBlockchain) {
                return false;
            }
            const expectedAddress = DOM.userAddress?.value || "";
            const expectedBlockchain = DOM.userBlockchain?.value || "";
            if (expectedAddress !== "" && activeAddress !== expectedAddress) {
                return false;
            }
            if (expectedBlockchain !== "" && activeBlockchain !== expectedBlockchain) {
                return false;
            }
            return true;
        }

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
        function parseDetachedHtml(html: string): HTMLElement {
            const parser = new DOMParser();
            const doc = parser.parseFromString(html, "text/html");
            doc.body.querySelectorAll("script, style, form, input, button, textarea, svg").forEach(element => {
                element.remove();
            });
            doc.body.querySelectorAll("*").forEach(element => {
                Array.from(element.attributes).forEach(attribute => {
                    const attributeName = attribute.name.toLowerCase();
                    if (attributeName.startsWith("on")) {
                        element.removeAttribute(attribute.name);
                        return;
                    }
                    if ((attributeName === "href" || attributeName === "src") && XSSSanitizeUrl(attribute.value) === "#") {
                        element.removeAttribute(attribute.name);
                    }
                });
            });
            return doc.body;
        }
        function extractUrlsFromHtml(html: string): string[] {
            const tempDiv = parseDetachedHtml(html);
            const seenUrls = new Set<string>();
            const urls: string[] = [];
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
        function replaceElementWithText(element: Element, value: string) {
            if (value === "") {
                element.remove();
                return;
            }
            element.replaceWith(element.ownerDocument.createTextNode(value));
        }
        function flattenInlineMediaElements(root: HTMLElement) {
            root.querySelectorAll("img[src]").forEach(image => {
                const src = image.getAttribute("src")?.trim() || "";
                replaceElementWithText(image, ` ${src} `);
            });
            root.querySelectorAll("video").forEach(video => {
                const src = video.getAttribute("src")?.trim() || video.querySelector("source[src]")?.getAttribute("src")?.trim() || "";
                replaceElementWithText(video, ` ${src} `);
            });
        }
        function flattenAutoFormattedContent(html: string, flattenInlineMedia: boolean = false): string {
            const tempDiv = parseDetachedHtml(html);
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
            if (flattenInlineMedia) {
                flattenInlineMediaElements(tempDiv);
            }
            return tempDiv.innerHTML;
        }
        function isThirdPartyExternalImageUrl(url: string): boolean {
            if (!url.startsWith("https://") || !IsValidURL(url)) {
                return false;
            }
            try {
                const parsedUrl = new URL(url);
                return parsedUrl.origin !== window.location.origin;
            } catch (error) {
                return false;
            }
        }
        function extractExternalImageUrlsFromHtml(html: string): string[] {
            const tempDiv = parseDetachedHtml(html);
            const seenUrls = new Set<string>();
            const urls: string[] = [];
            tempDiv.querySelectorAll("img[src]").forEach(image => {
                const src = image.getAttribute("src")?.trim() || "";
                if (!src || !isThirdPartyExternalImageUrl(src) || seenUrls.has(src)) {
                    return;
                }
                seenUrls.add(src);
                urls.push(src);
            });
            return urls;
        }
        function replaceExternalImageUrlsInHtml(html: string, replacements: Map<string, string>): string {
            if (replacements.size === 0) {
                return html;
            }
            const tempDiv = parseDetachedHtml(html);
            tempDiv.querySelectorAll("img[src]").forEach(image => {
                const src = image.getAttribute("src");
                if (!src) {
                    return;
                }
                const replacement = replacements.get(src);
                if (replacement) {
                    image.setAttribute("src", replacement);
                }
            });
            return tempDiv.innerHTML;
        }
        async function canEmbedExternalImage(url: string): Promise<boolean> {
            return new Promise(resolve => {
                const sanitizedUrl = XSSSanitizeUrl(url);
                if (sanitizedUrl === "#") {
                    resolve(false);
                    return;
                }
                const testImage = new Image();
                let finished = false;
                const timeoutId = window.setTimeout(() => finish(false), 8000);
                const finish = (result: boolean) => {
                    if (finished) return;
                    finished = true;
                    window.clearTimeout(timeoutId);
                    testImage.onload = null;
                    testImage.onerror = null;
                    resolve(result);
                };
                testImage.onload = () => finish(true);
                testImage.onerror = () => finish(false);
                testImage.decoding = "async";
                testImage.src = sanitizedUrl;
            });
        }
        async function importExternalImage(url: string, csrfToken: string): Promise<importedExternalImageData | null> {
            const [status, data] = await HttpPostJson("/files/fetchExternal", {url}, csrfToken);
            if (status !== 200 || !data?.data || data.data.length === 0) {
                return null;
            }
            return {
                cid: data.data[0].cid,
                fileName: data.data[0].fileName,
                mimeType: data.data[0].mimeType,
                size: String(data.data[0].size || 0)
            };
        }
        async function resolveExternalImage(url: string, csrfToken: string, showGatewayDialog: boolean = false): Promise<externalImageState> {
            const cached = externalImageMap.get(url);
            if (cached) {
                if (!cached.allowsEmbedding && isGatewayMode() && showGatewayDialog) {
                    showGatewayHotlinkDialog(url);
                }
                return cached;
            }
            const inFlight = externalImageRequestMap.get(url);
            if (inFlight) {
                return inFlight;
            }
            const request = (async () => {
                const allowsEmbedding = await canEmbedExternalImage(url);
                if (allowsEmbedding) {
                    const state = {allowsEmbedding: true};
                    externalImageMap.set(url, state);
                    return state;
                }
                if (isGatewayMode()) {
                    if (showGatewayDialog) {
                        showGatewayHotlinkDialog(url);
                    }
                    const state = {allowsEmbedding: false};
                    externalImageMap.set(url, state);
                    return state;
                }
                const importedFile = await importExternalImage(url, csrfToken);
                if (!importedFile) {
                    return {allowsEmbedding: false};
                }
                const state = {
                    allowsEmbedding: false,
                    importedFile: importedFile
                };
                externalImageMap.set(url, state);
                return state;
            })().finally(() => {
                externalImageRequestMap.delete(url);
            });
            externalImageRequestMap.set(url, request);
            return request;
        }
        async function processExternalImagesInEditor(editor: any) {
            const csrfToken = DOM.csrfToken.value;
            const externalUrls = extractExternalImageUrlsFromHtml(editor.getContent());
            for (const externalUrl of externalUrls) {
                await resolveExternalImage(externalUrl, csrfToken, true);
            }
        }
        function queueExternalImageProcessing(editor: any) {
            externalImageProcessingPromise = externalImageProcessingPromise.then(async () => {
                await processExternalImagesInEditor(editor);
            }).catch(error => {
                console.log("[addPost] Error preflighting external image:", error);
            });
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
                const tinymce = await import("tinymce/tinymce");
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
                        console.log("[addPost] Created blob URL:", blobUrl, "cid:", uploadedFile.cid);
                        inlineMediaMap.set(blobUrl, {
                            cid: uploadedFile.cid,
                            fileName: uploadedFile.fileName,
                            mimeType: uploadedFile.mimeType,
                            size: String(uploadedFile.size || renamedFile.size),
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
                                queueExternalImageProcessing(editor);
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
                            queueExternalImageProcessing(editor);
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
            avatarImg.src = "/static/image/avatar.svg";
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
            externalImageProcessingPromise = Promise.resolve();
            gatewayHotlinkDialogUrls.clear();
        }
        async function showModal() {
            addPostModal.show();
            if (!tinymceInitialized) {
                DOM.tinymceSpinner.style.display = "flex";
                // TinyMCE is large; keep it off the initial page load and initialize when the composer opens.
                await initTinyMCE();
            }
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
            let payload = flattenAutoFormattedContent((window as any).tinymce.get("addPostText")?.getContent() || "");
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
            const blockchain = GetChain();
            const publicPost = shouldCreatePublicPost();
            if (publicPost && !hasMatchingWalletIdentity()) {
                DOM.submitPostButton.disabled = false;
                ShowToastWithDelay("Switch your wallet back to this account before publishing", 5000);
                return;
            }
            const attachmentCids = new Set<string>();
            const attachments: string[][] = [];
            await externalImageProcessingPromise;
            console.log("[addPost] inlineMediaMap size:", inlineMediaMap.size);
            for (const [blobUrl, mediaData] of inlineMediaMap) {
                console.log("[addPost] Processing media:", blobUrl, mediaData);
                let cidString = mediaData.cid;
                if (publicPost) {
                    let cid = await AddFileToIPFS(mediaData.cid, csrfToken);
                    console.log("[addPost] AddFileToIPFS returned:", cid);
                    cidString = cid?.toString() || "";
                }
                if (cidString === undefined || !IsValidIpfsCid(cidString)) {
                    console.log("[addPost] Invalid CID, showing error");
                    ShowToastWithDelay("Failed to prepare attachment", 5000);
                    DOM.submitPostButton.disabled = false;
                    return;
                }
                attachmentCids.add(cidString);
                attachments.push([cidString, mediaData.mimeType, mediaData.size, mediaData.fileName]);
                const replacementUrl = publicPost ? `ipfs://${cidString}` : `/files/download/${encodeURIComponent(cidString)}`;
                console.log("[addPost] Replacing", blobUrl, "with", replacementUrl);
                payload = payload.split(blobUrl).join(replacementUrl);
            }
            console.log("[addPost] Final payload after IPFS replacement:", payload);
            const externalImageReplacements = new Map<string, string>();
            for (const externalUrl of extractExternalImageUrlsFromHtml(payload)) {
                const externalState = await resolveExternalImage(externalUrl, csrfToken, true);
                if (externalState.allowsEmbedding) {
                    continue;
                }
                if (isGatewayMode()) {
                    DOM.submitPostButton.disabled = false;
                    return;
                }
                if (!externalState.importedFile) {
                    ShowToastWithDelay("Failed to import external image", 5000);
                    DOM.submitPostButton.disabled = false;
                    return;
                }
                let cidString = externalState.importedFile.cid;
                if (publicPost) {
                    const cid = await AddFileToIPFS(externalState.importedFile.cid, csrfToken);
                    cidString = cid?.toString() || "";
                }
                if (!IsValidIpfsCid(cidString)) {
                    ShowToastWithDelay("Failed to prepare external image", 5000);
                    DOM.submitPostButton.disabled = false;
                    return;
                }
                attachmentCids.add(cidString);
                attachments.push([cidString, externalState.importedFile.mimeType, externalState.importedFile.size, externalState.importedFile.fileName]);
                const resolvedUrl = publicPost ? `ipfs://${cidString}` : `/files/download/${encodeURIComponent(cidString)}`;
                externalImageReplacements.set(externalUrl, resolvedUrl);
            }
            if (externalImageReplacements.size > 0) {
                payload = replaceExternalImageUrlsInHtml(payload, externalImageReplacements);
            }
            if (publicPost) {
                payload = flattenAutoFormattedContent(payload, true);
            }
            console.log("[addPost] Final payload after external image processing:", payload);
            const filteredAttachments = binaryAttachments.filter(f => !removedAttachments.includes(f.fileName));
            console.log("[addPost] Binary attachments:", filteredAttachments.length);
            for (const file of filteredAttachments) {
                let cidString = file.cid;
                if (publicPost) {
                    let cid = await AddFileToIPFS(file.cid, csrfToken);
                    cidString = cid?.toString() || "";
                }
                if (!IsValidIpfsCid(cidString)) {
                    ShowToastWithDelay("Failed to prepare attachment", 5000);
                    DOM.submitPostButton.disabled = false;
                    return;
                }
                attachmentCids.add(cidString);
                attachments.push([cidString, file.mimeType, file.size, file.fileName]);
            }
            let success = false;
            if (publicPost) {
                if (attachments.length > 0) {
                    const txHash = await WalletSubmitPostAttachTx(payload, attachments);
                    if (!txHash || !blockchain) {
                        DOM.submitPostButton.disabled = false;
                        ShowToastWithDelay("Failed to submit post", 5000);
                        return;
                    }
                    const finalizeResponse = await FinalizeFiles(Array.from(attachmentCids), "public", "post_attachment", csrfToken, txHash, blockchain);
                    if (finalizeResponse[0] !== 200) {
                        DOM.submitPostButton.disabled = false;
                        ShowToastWithDelay("Failed to finalize post attachments", 5000);
                        return;
                    }
                    success = true;
                } else {
                    console.log("[addPost] Calling WalletSubmitPost");
                    success = await WalletSubmitPost(payload);
                }
            } else {
                const createResponse = await CreateLocalPost(payload, Array.from(attachmentCids), csrfToken);
                success = createResponse[0] === 200;
            }
            DOM.submitPostButton.disabled = false;
            if (!success) {
                console.log("[addPost] Post submission failed - wallet not connected or invalid");
                ShowToastWithDelay("Failed to submit post", 5000);
                return;
            }
            console.log("[addPost] Post submitted successfully");
            hideModal();
            if (publicPost) {
                ShowToastWithDelay("Your post should show up shortly. Please wait for it to spread through the network.", 10000);
            } else {
                ShowToastWithDelay("Saved privately on your server.", 5000);
            }
            DOM.spiceometerText.innerText = "";
            let crossPostEnabled = publicPost && await XcomIsCrossPostEnabled();
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
                cid: uploadedFile.cid,
                fileName: uploadedFile.fileName,
                mimeType: uploadedFile.mimeType,
                size: String(uploadedFile.size || file.size),
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
                cid: uploadedFile.cid,
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
        document.addEventListener("focusin", (e) => {
            if (e.target instanceof Element && e.target.closest(".emojiPickerPopup, .tox-tinymce-aux, .moxman-window, .tam-assetmanager-root") !== null) {
                e.stopImmediatePropagation();
            }
        });
        checkDraftParam().then();
    }
})();
