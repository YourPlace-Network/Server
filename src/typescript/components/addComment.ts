import {GetAddress, WalletSubmitComment, WalletSubmitCommentAttach} from "../util/blockchain/wallet";
import {ShowDialogModal, ShowDialogModalHTML} from "./modalDialog";
import {UploadFile} from "../util/files";
import {AddFileToIPFS, CIDToSubdomainURL} from "../util/ipfs";
import {IsValidIpfsCid, XSSSanitizeTextUrl} from "../util/security";
import {CreatePostControlsBar} from "./postControls";
import {HttpGetJson} from "../util/network";
import {setupTinyMCEEmojiButton} from "../util/emojiPicker";
import {formatTimestamp} from "../util/time";

export interface Comment {
    address: string;
    attachments?: string[][];
    author: string;
    avatarSrc: string;
    blockchain: string;
    dislikeCount: number;
    likeCount: number;
    parentTxHash: string;
    payload: string;
    replyCount: number;
    timestamp: number;
    txHash: string;
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
    const submitHandler = async (setButtonState: (disabled: boolean, text: string) => void) => {
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
        setButtonState(true, "Posting...");
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
                        setButtonState(false, "Post");
                        return;
                    }
                    const ipfsUrl = `ipfs://${cidString}`;
                    attachments.push([ipfsUrl, media.mimeType, media.size, media.fileName]);
                }
                await WalletSubmitCommentAttach(parentTxHash, content, attachments);
            } else {
                await WalletSubmitComment(parentTxHash, content);
            }
            if (commentButton) {
                const countSpan = commentButton.querySelector(".count");
                if (countSpan) {
                    const currentCount = parseInt(countSpan.textContent || "0", 10);
                    countSpan.textContent = (currentCount + 1).toString();
                }
                commentButton.classList.add("active");
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
            setButtonState(false, "Post");
        }
    };
    setTimeout(() => {
        initCommentEditor(editorId, submitHandler);
    }, 100);
    const threadPreview = document.createElement("div");
    threadPreview.classList.add("commentThreadPreview");
    container.appendChild(threadPreview);
    loadThreadPreview(threadPreview, parentTxHash, blockchain);
    return container;
}
function initCommentEditor(editorId: string, onSubmit?: (setButtonState: (disabled: boolean, text: string) => void) => void): void {
    if (typeof window.tinymce === 'undefined') {
        const editorDiv = document.getElementById(editorId);
        if (editorDiv) {
            const textarea = document.createElement("textarea");
            textarea.id = `${editorId}_fallback`;
            textarea.classList.add("form-control");
            textarea.rows = 1;
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
    const isMobile = window.innerWidth < 768;
    window.tinymce.init({
        selector: `#${editorId}`,
        plugins: "code table lists",
        toolbar: isMobile
            ? "emojipicker forecolor backcolor | formatting | postcomment"
            : "emojipicker forecolor backcolor | bold italic underline strikethrough | bullist numlist | postcomment",
        toolbar_groups: isMobile ? {
            formatting: {
                icon: 'more-drawer',
                items: 'bold italic underline strikethrough | bullist numlist'
            }
        } : {},
        menubar: false,
        statusbar: true,
        elementpath: false,
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
            setupTinyMCEEmojiButton(editor);
            if (onSubmit) {
                editor.ui.registry.addButton('postcomment', {
                    text: 'Post',
                    onAction: () => {
                        const setButtonState = (disabled: boolean, text: string) => {
                            const btn = editor.editorContainer?.querySelector('.tox-tbtn--postcomment, button[data-mce-name="postcomment"]');
                            if (btn) {
                                btn.disabled = disabled;
                                const textSpan = btn.querySelector('.tox-tbtn__select-label');
                                if (textSpan) textSpan.textContent = text;
                            }
                        };
                        onSubmit(setButtonState);
                    }
                });
            }
            editor.on("init", () => {
                inlineMediaMap.set(editorId, []);
                if (editor.editorContainer) {
                    editor.editorContainer.classList.add('commentTinyMCE');
                }
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
            HttpGetJson(`/comments/${blockchain}/${parentTxHash}?limit=3&offset=0&sort=likes`),
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
        const comments = commentsRes[1].comments as Comment[];
        for (const comment of comments) {
            const el = createPreviewCommentElement(comment, blockchain);
            container.appendChild(el);
        }
    } catch (e) {
        console.error("Failed to load thread preview:", e);
    }
}
function createPreviewCommentElement(comment: Comment, blockchain: string): HTMLDivElement {
    const commentDiv = document.createElement("div");
    commentDiv.classList.add("previewCommentItem");
    commentDiv.dataset.address = comment.address;
    commentDiv.dataset.blockchain = comment.blockchain;
    commentDiv.dataset.parenttxhash = comment.parentTxHash;
    commentDiv.dataset.txhash = comment.txHash;
    const headerDiv = document.createElement("div");
    headerDiv.classList.add("previewCommentHeader");
    const profileUrl = `/p/${comment.blockchain}/${comment.address}`;
    const avatarLink = document.createElement("a");
    avatarLink.href = profileUrl;
    avatarLink.classList.add("previewCommentAvatarLink");
    const avatarImg = document.createElement("img");
    avatarImg.classList.add("previewCommentAvatar");
    let avatarSrc = comment.avatarSrc || "/static/image/avatar.png";
    if (avatarSrc.startsWith("ipfs://")) {
        avatarSrc = CIDToSubdomainURL(avatarSrc) || "/static/image/avatar.png";
    }
    avatarImg.src = avatarSrc;
    avatarImg.alt = "avatar";
    avatarImg.crossOrigin = "anonymous";
    avatarImg.referrerPolicy = "no-referrer";
    avatarLink.appendChild(avatarImg);
    headerDiv.appendChild(avatarLink);
    const authorLink = document.createElement("a");
    authorLink.href = profileUrl;
    authorLink.classList.add("previewCommentAuthorLink");
    authorLink.textContent = comment.author || comment.address.substring(0, 10) + "...";
    headerDiv.appendChild(authorLink);
    const dateSpan = document.createElement("span");
    dateSpan.classList.add("previewCommentDate");
    dateSpan.textContent = formatTimestamp(comment.timestamp);
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
    const addCommentContainer = document.createElement("div");
    addCommentContainer.classList.add("addCommentContainer");
    const controlsBar = CreatePostControlsBar({
        txHash: comment.txHash,
        blockchain: comment.blockchain,
        targetType: 'comment',
        initialLikes: comment.likeCount,
        initialDislikes: comment.dislikeCount,
        initialComments: comment.replyCount,
        onCommentClick: () => {
            const commentBtn = controlsBar.querySelector(".comment") as HTMLElement;
            const commentIcon = commentBtn?.querySelector("i") as HTMLElement | null;
            if (addCommentContainer.classList.contains("expanded")) {
                addCommentContainer.classList.remove("expanded");
                addCommentContainer.innerHTML = "";
                if (commentIcon) {
                    commentIcon.style.color = "";
                }
            } else {
                addCommentContainer.classList.add("expanded");
                const commentUI = ShowAddCommentUI(comment.txHash, comment.blockchain, () => {
                    addCommentContainer.classList.remove("expanded");
                    addCommentContainer.innerHTML = "";
                }, commentBtn);
                addCommentContainer.appendChild(commentUI);
            }
        },
        onRepostClick: () => {
            window.open(`/post/${comment.blockchain}/${comment.txHash}`, '_blank');
        }
    });
    controlsBar.classList.add("previewControlsBar");
    commentDiv.appendChild(controlsBar);
    commentDiv.appendChild(addCommentContainer);
    return commentDiv;
}
declare global {
    interface Window {
        tinymce: any;
    }
}
