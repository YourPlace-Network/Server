import {GetAddress, WalletSubmitComment, WalletSubmitCommentAttach} from "../util/blockchain/wallet";
import {ShowDialogModal, ShowDialogModalHTML} from "./modalDialog";
import {UploadFile} from "../util/files";
import {AddFileToIPFS} from "../util/ipfs";
import {IsValidIpfsCid, XSSSanitizeTextUrl} from "../util/security";
import {CreatePostControlsBar} from "./postControls";
import {HttpGetJson} from "../util/network";

interface CommentPreview {
    txHash: string;
    blockchain: string;
    address: string;
    parentTxHash: string;
    parentType: string;
    timestamp: number;
    payload: string;
    author: string;
    avatarSrc: string;
    likeCount: number;
    dislikeCount: number;
    replyCount: number;
}
let commentEditorId = 0;
interface InlineMediaData {
    uuid: string;
    fileName: string;
    mimeType: string;
    size: string;
    blobUrl: string;
}
let inlineMediaMap: Map<string, InlineMediaData[]> = new Map();
const randomColors = ["#FF0000", "#FF4500", "#FF6347", "#FF6B6B", "#FF6F61", "#FF7F50", "#FF8C00", "#FFA500", "#FFD700", "#FFEA00", "#FFEAA7", "#F7DC6F", "#F8B500", "#ADFF2F", "#7FFF00", "#88B04B", "#32CD32", "#00FF00", "#00FA9A", "#00FF7F", "#96CEB4", "#98D8C8", "#4ECDC4", "#40E0D0", "#00CED1", "#00FFFF", "#45B7D1", "#85C1E9", "#1E90FF", "#4169E1", "#0000FF", "#6B5B95", "#8A2BE2", "#9400D3", "#BB8FCE", "#DA70D6", "#DDA0DD", "#FF00FF", "#FF1493", "#FF69B4", "#F7CAC9", "#92A8D1"];

function getRandomColor(): string {
    return randomColors[Math.floor(Math.random() * randomColors.length)];
}
export function ShowAddCommentUI(parentTxHash: string, blockchain: string, onSuccess?: () => void, commentButton?: HTMLElement): HTMLDivElement {
    const icon = commentButton?.querySelector("i") as HTMLElement | null;
    const originalColor = icon?.style.color || "";
    if (icon) {
        icon.style.color = getRandomColor();
    }
    const container = document.createElement("div");
    container.classList.add("addCommentUIContainer");
    const editorId = `commentEditor_${commentEditorId++}`;
    const editorDiv = document.createElement("div");
    editorDiv.id = editorId;
    editorDiv.classList.add("commentEditorDiv");
    container.appendChild(editorDiv);
    const actionsDiv = document.createElement("div");
    actionsDiv.classList.add("addCommentActions");
    const cancelBtn = document.createElement("button");
    cancelBtn.classList.add("btn", "btn-secondary", "btn-sm");
    cancelBtn.textContent = "Cancel";
    cancelBtn.addEventListener("click", () => {
        destroyEditor(editorId);
        if (icon) {
            icon.style.color = originalColor;
        }
        container.remove();
    });
    actionsDiv.appendChild(cancelBtn);
    const submitBtn = document.createElement("button");
    submitBtn.classList.add("btn", "btn-primary", "btn-sm");
    submitBtn.textContent = "Post Comment";
    submitBtn.addEventListener("click", async () => {
        const address = GetAddress();
        if (!address) {
            ShowDialogModal("Please connect your wallet to comment");
            return;
        }
        const content = getEditorContent(editorId);
        if (!content || content.trim().length === 0) {
            ShowDialogModal("Please enter a comment");
            return;
        }
        submitBtn.disabled = true;
        submitBtn.textContent = "Posting...";
        try {
            const mediaList = inlineMediaMap.get(editorId) || [];
            if (mediaList.length > 0) {
                const csrfToken = (document.getElementById("csrfToken") as HTMLInputElement)?.value || "";
                const attachments: string[][] = [];
                for (const media of mediaList) {
                    const cid = await AddFileToIPFS(media.uuid, csrfToken);
                    const cidString = cid?.toString();
                    if (cidString === undefined || !IsValidIpfsCid(cidString)) {
                        ShowDialogModal("Failed to upload attachment to IPFS");
                        submitBtn.disabled = false;
                        submitBtn.textContent = "Post Comment";
                        return;
                    }
                    const ipfsUrl = `ipfs://${cidString}`;
                    attachments.push([ipfsUrl, media.mimeType, media.size, media.fileName]);
                }
                await WalletSubmitCommentAttach(parentTxHash, content, attachments);
            } else {
                await WalletSubmitComment(parentTxHash, content);
            }
            destroyEditor(editorId);
            inlineMediaMap.delete(editorId);
            if (icon) {
                icon.style.color = originalColor;
            }
            if (onSuccess) {
                onSuccess();
            }
            container.remove();
        } catch (e) {
            console.error("Failed to submit comment:", e);
            ShowDialogModal("Failed to submit comment. Please try again.");
            submitBtn.disabled = false;
            submitBtn.textContent = "Post Comment";
        }
    });
    actionsDiv.appendChild(submitBtn);
    container.appendChild(actionsDiv);
    setTimeout(() => {
        initCommentEditor(editorId);
    }, 100);
    const threadPreview = document.createElement("div");
    threadPreview.classList.add("commentThreadPreview");
    container.appendChild(threadPreview);
    loadThreadPreview(threadPreview, parentTxHash, blockchain);
    return container;
}
function initCommentEditor(editorId: string): void {
    if (typeof window.tinymce === 'undefined') {
        const editorDiv = document.getElementById(editorId);
        if (editorDiv) {
            const textarea = document.createElement("textarea");
            textarea.id = `${editorId}_fallback`;
            textarea.classList.add("form-control");
            textarea.rows = 3;
            textarea.placeholder = "Write a comment...";
            editorDiv.appendChild(textarea);
        }
        return;
    }
    let DOM = {
        csrfToken: document.getElementById("csrfToken")! as HTMLInputElement,
        gatewayMode: document.getElementById("gatewayModeAddComment")! as HTMLInputElement,

    };
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
        ShowDialogModalHTML(
            "To attach a file to a post, you need to host your own YourPlace server.<br><br>" +
            "<a href=\"https://yourplace.network/download\" target=\"_blank\" rel=\"noopener noreferrer\">Download YourPlace</a>"
        );
    }
    window.tinymce.init({
        selector: `#${editorId}`,
        plugins: "code table emoticons lists",
        toolbar: "emoticons forecolor backcolor | bold italic underline strikethrough | bullist numlist",
        menubar: false,
        statusbar: true,
        resize: true,
        height: 150,
        branding: false,
        paste_data_images: true,
        automatic_uploads: true,
        placeholder: "Write a comment...",
        base_url: "/static/tinymce",
        license_key: "gpl",
        content_css: "/static/css/tinymce.css",
        content_style: "img, video { max-width: 100%; height: auto; }",
        images_upload_handler: async (blobInfo: any) => {
            console.log("[addComment] images_upload_handler called", blobInfo);
            if (isGatewayMode()) {
                console.log("[addComment] Gateway mode, showing dialog");
                showGatewayUploadDialog();
                throw new Error("Gateway mode - uploads disabled");
            }
            const file = blobInfo.blob();
            const fileName = blobInfo.filename() || `pasted-${Date.now()}.${file.type.split("/")[1] || "png"}`;
            console.log("[addComment] Uploading file:", fileName, file.type, file.size);
            const renamedFile = new File([file], fileName, {type: file.type});
            const csrfToken = DOM.csrfToken.value;
            const [status, data] = await UploadFile(renamedFile, csrfToken);
            console.log("[addComment] Upload response:", status, data);
            if (!data.data || data.data.length === 0) {
                console.log("[addComment] Upload failed");
                throw new Error("Failed to upload file");
            }
            const uploadedFile = data.data[0];
            const blobUrl = URL.createObjectURL(renamedFile);
            console.log("[addComment] Created blob URL:", blobUrl, "uuid:", uploadedFile.uuid);
            const mediaData = {
                uuid: uploadedFile.uuid,
                fileName: uploadedFile.fileName,
                mimeType: uploadedFile.mimeType,
                size: uploadedFile.size || renamedFile.size.toString(),
                blobUrl: blobUrl
            };
            const existing = inlineMediaMap.get(editorId) || [];
            existing.push(mediaData);
            inlineMediaMap.set(editorId, existing);
            console.log("[addComment] inlineMediaMap now has", existing.length, "entries for", editorId);
            return blobUrl;
        },
        setup: (editor: any) => {
            editor.on("init", () => {
                inlineMediaMap.set(editorId, []);
            });
        }
    });
}
function getEditorContent(editorId: string): string {
    if (typeof window.tinymce !== 'undefined') {
        const editor = window.tinymce.get(editorId);
        if (editor) {
            return editor.getContent();
        }
    }
    const fallback = document.getElementById(`${editorId}_fallback`) as HTMLTextAreaElement;
    if (fallback) {
        return fallback.value;
    }
    return "";
}
function destroyEditor(editorId: string): void {
    if (typeof window.tinymce !== 'undefined') {
        const editor = window.tinymce.get(editorId);
        if (editor) {
            editor.remove();
        }
    }
}
async function loadThreadPreview(container: HTMLElement, parentTxHash: string, blockchain: string): Promise<void> {
    try {
        const [commentsRes, countRes] = await Promise.all([
            HttpGetJson(`/comments/${blockchain}/${parentTxHash}?limit=3&offset=0`),
            HttpGetJson(`/comments/${blockchain}/${parentTxHash}/count`)
        ]);
        const count = countRes[0] === 200 && countRes[1]?.count ? countRes[1].count : 0;
        if (count > 0) {
            const countLabel = document.createElement("div");
            countLabel.classList.add("threadPreviewCount");
            countLabel.textContent = `${count} comment${count === 1 ? "" : "s"}`;
            container.appendChild(countLabel);
        }
        if (commentsRes[0] !== 200 || !commentsRes[1]?.comments) return;
        const comments = commentsRes[1].comments as CommentPreview[];
        for (const comment of comments) {
            const el = createPreviewCommentElement(comment, blockchain);
            container.appendChild(el);
        }
    } catch (e) {
        console.error("Failed to load thread preview:", e);
    }
}
function createPreviewCommentElement(comment: CommentPreview, blockchain: string): HTMLDivElement {
    const commentDiv = document.createElement("div");
    commentDiv.classList.add("previewCommentItem");
    const headerDiv = document.createElement("div");
    headerDiv.classList.add("previewCommentHeader");
    const avatarImg = document.createElement("img");
    avatarImg.classList.add("previewCommentAvatar");
    avatarImg.src = comment.avatarSrc || "/static/image/avatar.png";
    avatarImg.alt = "avatar";
    avatarImg.crossOrigin = "anonymous";
    avatarImg.referrerPolicy = "no-referrer";
    headerDiv.appendChild(avatarImg);
    const authorSpan = document.createElement("span");
    authorSpan.classList.add("previewCommentAuthor");
    authorSpan.textContent = comment.author || comment.address.substring(0, 10) + "...";
    headerDiv.appendChild(authorSpan);
    const dateSpan = document.createElement("span");
    dateSpan.classList.add("previewCommentDate");
    dateSpan.textContent = formatPreviewTimestamp(comment.timestamp);
    headerDiv.appendChild(dateSpan);
    commentDiv.appendChild(headerDiv);
    const contentWrapper = document.createElement("div");
    contentWrapper.classList.add("previewCommentContentWrapper");
    const contentDiv = document.createElement("div");
    contentDiv.classList.add("previewCommentContent", "collapsed");
    contentDiv.innerHTML = XSSSanitizeTextUrl(comment.payload);
    contentWrapper.appendChild(contentDiv);
    const expandBtn = document.createElement("span");
    expandBtn.classList.add("previewExpandBtn");
    expandBtn.textContent = "show more";
    expandBtn.style.display = "none";
    expandBtn.addEventListener("click", () => {
        const isCollapsed = contentDiv.classList.contains("collapsed");
        if (isCollapsed) {
            contentDiv.classList.remove("collapsed");
            expandBtn.textContent = "show less";
        } else {
            contentDiv.classList.add("collapsed");
            expandBtn.textContent = "show more";
        }
    });
    contentWrapper.appendChild(expandBtn);
    commentDiv.appendChild(contentWrapper);
    setTimeout(() => {
        if (contentDiv.scrollHeight > contentDiv.clientHeight) {
            expandBtn.style.display = "inline";
        }
    }, 10);
    const controlsBar = CreatePostControlsBar({
        txHash: comment.txHash,
        blockchain: comment.blockchain,
        targetType: 'comment',
        initialLikes: comment.likeCount,
        initialDislikes: comment.dislikeCount,
        initialComments: comment.replyCount,
        onCommentClick: () => {
            window.location.href = `/post/${comment.blockchain}/${comment.txHash}`;
        },
        onRepostClick: () => {
            window.open(`/post/${comment.blockchain}/${comment.txHash}`, '_blank');
        }
    });
    controlsBar.classList.add("previewControlsBar");
    commentDiv.appendChild(controlsBar);
    return commentDiv;
}
function formatPreviewTimestamp(timestamp: number): string {
    const now = Date.now() / 1000;
    const diff = now - timestamp;
    if (diff < 60) return "now";
    if (diff < 3600) return `${Math.floor(diff / 60)}m`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
    if (diff < 604800) return `${Math.floor(diff / 86400)}d`;
    return new Date(timestamp * 1000).toLocaleDateString();
}

declare global {
    interface Window {
        tinymce: any;
    }
}
