import { HttpGetJson } from "../util/network";
import { CreatePostControlsBar } from "./postControls";
import { ShowAddCommentUI } from "./addComment";
import { XSSSanitizeTextUrl } from "../util/security";

export interface Comment {
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
export interface CommentThreadOptions {
    parentTxHash: string;
    blockchain: string;
    maxDepth?: number;
}
const MAX_INDENT_DEPTH = 4;
export function CreateCommentThread(options: CommentThreadOptions): HTMLDivElement {
    const threadDiv = document.createElement("div");
    threadDiv.classList.add("commentThread");
    threadDiv.dataset.parenttxhash = options.parentTxHash;
    threadDiv.dataset.blockchain = options.blockchain;
    threadDiv.dataset.maxdepth = (options.maxDepth || MAX_INDENT_DEPTH).toString();
    loadComments(threadDiv, options.parentTxHash, options.blockchain, 0);
    return threadDiv;
}
async function loadComments(container: HTMLElement, parentTxHash: string, blockchain: string, depth: number, limit: number = 50, offset: number = 0): Promise<void> {
    try {
        const response = await HttpGetJson(`/comments/${blockchain}/${parentTxHash}?limit=${limit}&offset=${offset}`);
        if (response[0] === 200 && response[1] && response[1].comments) {
            const comments = response[1].comments as Comment[];
            for (const comment of comments) {
                const commentElement = createCommentElement(comment, depth, blockchain);
                container.appendChild(commentElement);
            }
        }
    } catch (e) {
        console.error("Failed to load comments:", e);
    }
}
function createCommentElement(comment: Comment, depth: number, blockchain: string): HTMLDivElement {
    const commentDiv = document.createElement("div");
    commentDiv.classList.add("commentItem");
    commentDiv.dataset.txhash = comment.txHash;
    commentDiv.dataset.depth = Math.min(depth, MAX_INDENT_DEPTH).toString();
    const headerDiv = document.createElement("div");
    headerDiv.classList.add("commentHeader");
    if (comment.replyCount > 0) {
        const toggleBtn = document.createElement("span");
        toggleBtn.classList.add("commentToggle", "collapsed");
        const chevron = document.createElement("i");
        chevron.classList.add("bi", "bi-chevron-down");
        toggleBtn.appendChild(chevron);
        toggleBtn.addEventListener("click", () => {
            toggleReplies(commentDiv, comment.txHash, blockchain, depth + 1);
        });
        headerDiv.appendChild(toggleBtn);
    }
    const avatarImg = document.createElement("img");
    avatarImg.classList.add("commentAvatar");
    avatarImg.src = comment.avatarSrc || "/static/image/avatar.png";
    avatarImg.alt = "avatar";
    avatarImg.crossOrigin = "anonymous";
    avatarImg.referrerPolicy = "no-referrer";
    headerDiv.appendChild(avatarImg);
    const authorSpan = document.createElement("span");
    authorSpan.classList.add("commentAuthor");
    authorSpan.textContent = comment.author || comment.address.substring(0, 10) + "...";
    headerDiv.appendChild(authorSpan);
    const dateSpan = document.createElement("span");
    dateSpan.classList.add("commentDate");
    dateSpan.textContent = formatTimestamp(comment.timestamp);
    headerDiv.appendChild(dateSpan);
    commentDiv.appendChild(headerDiv);
    const contentDiv = document.createElement("div");
    contentDiv.classList.add("commentContent");
    contentDiv.innerHTML = XSSSanitizeTextUrl(comment.payload);
    commentDiv.appendChild(contentDiv);
    const controlsBar = CreatePostControlsBar({
        txHash: comment.txHash,
        blockchain: comment.blockchain,
        targetType: 'comment',
        initialLikes: comment.likeCount,
        initialDislikes: comment.dislikeCount,
        initialComments: comment.replyCount,
        onCommentClick: () => {
            toggleAddCommentUI(commentDiv, comment.txHash, blockchain);
        },
        onRepostClick: () => {
            const postUrl = `/post/${comment.blockchain}/${comment.txHash}`;
            window.open(postUrl, '_blank');
        }
    });
    commentDiv.appendChild(controlsBar);
    const addCommentContainer = document.createElement("div");
    addCommentContainer.classList.add("addCommentContainer");
    commentDiv.appendChild(addCommentContainer);
    const repliesDiv = document.createElement("div");
    repliesDiv.classList.add("commentReplies", "collapsed");
    commentDiv.appendChild(repliesDiv);
    return commentDiv;
}
function toggleReplies(commentDiv: HTMLElement, txHash: string, blockchain: string, depth: number): void {
    const toggle = commentDiv.querySelector(".commentToggle");
    const repliesDiv = commentDiv.querySelector(".commentReplies");
    if (!toggle || !repliesDiv) return;
    const isCollapsed = toggle.classList.contains("collapsed");
    if (isCollapsed) {
        toggle.classList.remove("collapsed");
        repliesDiv.classList.remove("collapsed");
        if (repliesDiv.children.length === 0) {
            loadComments(repliesDiv as HTMLElement, txHash, blockchain, depth);
        }
    } else {
        toggle.classList.add("collapsed");
        repliesDiv.classList.add("collapsed");
    }
}
function toggleAddCommentUI(commentDiv: HTMLElement, parentTxHash: string, blockchain: string): void {
    const container = commentDiv.querySelector(".addCommentContainer");
    if (!container) return;
    const isExpanded = container.classList.contains("expanded");
    if (isExpanded) {
        container.classList.remove("expanded");
        container.innerHTML = "";
    } else {
        container.classList.add("expanded");
        const addCommentUI = ShowAddCommentUI(parentTxHash, blockchain, () => {
            container.classList.remove("expanded");
            container.innerHTML = "";
            const repliesDiv = commentDiv.querySelector(".commentReplies");
            if (repliesDiv) {
                repliesDiv.innerHTML = "";
                const depth = parseInt(commentDiv.dataset.depth || "0");
                loadComments(repliesDiv as HTMLElement, parentTxHash, blockchain, depth + 1);
            }
        });
        container.appendChild(addCommentUI);
    }
}
function formatTimestamp(timestamp: number): string {
    const now = Date.now() / 1000;
    const diff = now - timestamp;
    if (diff < 60) {
        return "just now";
    } else if (diff < 3600) {
        const mins = Math.floor(diff / 60);
        return `${mins}m ago`;
    } else if (diff < 86400) {
        const hours = Math.floor(diff / 3600);
        return `${hours}h ago`;
    } else if (diff < 604800) {
        const days = Math.floor(diff / 86400);
        return `${days}d ago`;
    } else {
        const date = new Date(timestamp * 1000);
        return date.toLocaleDateString();
    }
}
export function ExpandCommentThread(threadDiv: HTMLDivElement): void {
    const toggles = threadDiv.querySelectorAll(".commentToggle.collapsed");
    toggles.forEach(toggle => {
        (toggle as HTMLElement).click();
    });
}
export function CollapseCommentThread(threadDiv: HTMLDivElement): void {
    const toggles = threadDiv.querySelectorAll(".commentToggle:not(.collapsed)");
    toggles.forEach(toggle => {
        (toggle as HTMLElement).click();
    });
}
export async function FetchComments(blockchain: string, parentTxHash: string, limit: number = 50, offset: number = 0): Promise<Comment[]> {
    try {
        const response = await HttpGetJson(`/comments/${blockchain}/${parentTxHash}?limit=${limit}&offset=${offset}`);
        if (response[0] === 200 && response[1] && response[1].comments) {
            return response[1].comments as Comment[];
        }
    } catch (e) {
        console.error("Failed to fetch comments:", e);
    }
    return [];
}
