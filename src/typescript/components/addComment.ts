import { GetAddress, WalletSubmitComment, WalletSubmitCommentAttach } from "../util/blockchain/wallet";
import { ShowDialogModal } from "./modalDialog";

let commentEditorId = 0;
let inlineMediaMap: Map<string, string[][]> = new Map();
export function ShowAddCommentUI(parentTxHash: string, blockchain: string, onSuccess?: () => void): HTMLDivElement {
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
            const attachments = inlineMediaMap.get(editorId) || [];
            if (attachments.length > 0) {
                await WalletSubmitCommentAttach(parentTxHash, content, attachments);
            } else {
                await WalletSubmitComment(parentTxHash, content);
            }
            destroyEditor(editorId);
            inlineMediaMap.delete(editorId);
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
    window.tinymce.init({
        selector: `#${editorId}`,
        plugins: "emoticons lists",
        toolbar: "emoticons | bold italic | bullist numlist",
        menubar: false,
        statusbar: false,
        height: 150,
        placeholder: "Write a comment...",
        base_url: "/static/tinymce",
        license_key: "gpl",
        content_css: "/static/css/tinymce-content.css",
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
declare global {
    interface Window {
        tinymce: any;
    }
}
