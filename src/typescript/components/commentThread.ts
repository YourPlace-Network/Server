import { HttpGetJson } from "../util/network";
import { CreatePostControlsBar } from "./postControls";
import type { Comment } from "./addComment";
import { ShowAddCommentUI } from "./addComment";
import { CIDToSubdomainURL } from "../util/ipfs";
import { IsValidURL, XSSSanitizeTinyMCEHtml, XSSSanitizeUrl } from "../util/security";
import { XcomOEmbedCard } from "./xcomOEmbedCard";
import { formatTimestamp } from "../util/time";
import { OEmbedCard } from "./oEmbedCard";
import { ProcessPostContentForPreviews } from "./postPreviewCard";
import { processTextWithTags } from "../util/domFactory";

const MAX_INDENT_DEPTH = 4;
const PAGE_SIZE = 5;
export type CommentSort = "dislikes" | "likes" | "reactions" | "recent";
export interface CommentThreadOptions {
    blockchain: string;
    initialPage?: number;
    maxDepth?: number;
    onPageChange?: (page: number) => void;
    parentTxHash: string;
    sort?: CommentSort;
}
interface PaginationState {
    currentPage: number;
    hasMore: boolean;
    loading: boolean;
    pages: Comment[][];
    sort: CommentSort;
}

const pageChangeCallbacks: WeakMap<HTMLElement, (page: number) => void> = new WeakMap();
const paginationStates: WeakMap<HTMLElement, PaginationState> = new WeakMap();

function createCommentImageEmbed(url: string): HTMLImageElement | null {
    const imageRegex = /^https:\/\/.*\.(jpg|jpeg|gif|webp|png|svg)$/i;
    if (!imageRegex.test(url)) {
        return null;
    }
    const img = document.createElement("img");
    img.classList.add("commentEmbeddedImage");
    img.crossOrigin = "anonymous";
    img.referrerPolicy = "no-referrer";
    img.src = XSSSanitizeUrl(url);
    return img;
}
function createCommentYoutubeEmbed(url: string): HTMLIFrameElement | null {
    const youtubeRegex = /^https:\/\/((?:www\.)?youtube\.com\/watch\?v=|youtu\.be\/)([a-zA-Z0-9_-]{11})(?:[?&].*)?$/;
    const match = url.match(youtubeRegex);
    if (!match) {
        return null;
    }
    const iframe = document.createElement("iframe");
    iframe.classList.add("commentEmbeddedIframe");
    iframe.src = XSSSanitizeUrl(`https://www.youtube-nocookie.com/embed/${match[2]}`);
    iframe.allow = "encrypted-media; picture-in-picture";
    iframe.allowFullscreen = true;
    iframe.setAttribute("loading", "lazy");
    iframe.setAttribute("credentialless", "");
    return iframe;
}
function normalizeCommentMediaSource(url: string | null): string | null {
    if (!url) {
        return null;
    }
    if (url.startsWith("ipfs://")) {
        return CIDToSubdomainURL(url) || null;
    }
    if (!IsValidURL(url)) {
        return null;
    }
    const sanitizedUrl = XSSSanitizeUrl(url);
    return sanitizedUrl === "#" ? null : sanitizedUrl;
}
function styleCommentBodyMedia(contentDiv: HTMLElement): void {
    const images = contentDiv.querySelectorAll("img");
    images.forEach((img) => {
        const src = normalizeCommentMediaSource(img.getAttribute("src"));
        if (!src) {
            img.remove();
            return;
        }
        img.src = src;
        img.classList.add("commentBodyImage");
        (img as HTMLImageElement).crossOrigin = "anonymous";
        (img as HTMLImageElement).referrerPolicy = "no-referrer";
    });
    const videos = contentDiv.querySelectorAll("video");
    videos.forEach((video) => {
        const src = normalizeCommentMediaSource(video.getAttribute("src"));
        if (src) {
            video.setAttribute("src", src);
        }
        video.querySelectorAll("source").forEach((source) => {
            const sourceSrc = normalizeCommentMediaSource(source.getAttribute("src"));
            if (!sourceSrc) {
                source.remove();
                return;
            }
            source.setAttribute("src", sourceSrc);
        });
        if (!video.getAttribute("src") && video.querySelectorAll("source").length === 0) {
            video.remove();
            return;
        }
        video.classList.add("commentInlineVideo");
        video.setAttribute("controls", "");
    });
    const iframes = contentDiv.querySelectorAll("iframe");
    iframes.forEach((iframe) => {
        iframe.classList.add("commentEmbeddedIframe");
    });
}
async function renderCommentBody(comment: Comment): Promise<HTMLDivElement> {
    const contentDiv = document.createElement("div");
    contentDiv.classList.add("commentContent");
    contentDiv.innerHTML = XSSSanitizeTinyMCEHtml(comment.payload);
    contentDiv.querySelectorAll("p").forEach((p) => {
        if (p.textContent?.trim() === "" && !p.querySelector("iframe, img, video")) {
            p.remove();
        }
    });
    styleCommentBodyMedia(contentDiv);
    processTextWithTags(contentDiv);
    await ProcessPostContentForPreviews(contentDiv);
    return contentDiv;
}
async function renderCommentEmbeds(comment: Comment): Promise<HTMLDivElement | null> {
    const urlRegex = /(https:\/\/[^\s"<>]+)/g;
    const embedUrls = comment.payload.replace(/<[^>]*>/g, " ").match(urlRegex);
    if (!embedUrls) {
        return null;
    }
    const embedDiv = document.createElement("div");
    embedDiv.classList.add("commentEmbeds");
    for (const url of embedUrls) {
        const imageEmbed = createCommentImageEmbed(url);
        if (imageEmbed) {
            embedDiv.appendChild(imageEmbed);
            continue;
        }
        const youtubeEmbed = createCommentYoutubeEmbed(url);
        if (youtubeEmbed) {
            embedDiv.appendChild(youtubeEmbed);
            continue;
        }
        const xcomEmbed = await XcomOEmbedCard(url);
        if (xcomEmbed) {
            embedDiv.appendChild(xcomEmbed);
            continue;
        }
        const oEmbedCard = await OEmbedCard(url);
        if (oEmbedCard) {
            embedDiv.appendChild(oEmbedCard);
        }
    }
    return embedDiv.childElementCount > 0 ? embedDiv : null;
}
function createCommentAttachmentVideo(fileUrl: string): HTMLVideoElement {
    const video = document.createElement("video");
    video.classList.add("commentAttachmentVideo");
    video.controls = true;
    video.preload = "metadata";
    video.src = XSSSanitizeUrl(fileUrl);
    return video;
}
function createPaginationControls(container: HTMLElement, parentTxHash: string, blockchain: string, depth: number, sort: CommentSort): HTMLDivElement {
    const controls = document.createElement("div");
    controls.classList.add("commentPaginationControls");
    const upArrow = document.createElement("button");
    upArrow.classList.add("commentPaginationBtn", "commentPaginationUp");
    upArrow.innerHTML = '<i class="bi bi-chevron-up"></i>';
    upArrow.title = "Previous comments";
    upArrow.style.display = "none";
    upArrow.addEventListener("click", () => navigatePage(container, parentTxHash, blockchain, depth, -1, sort));
    controls.appendChild(upArrow);
    const downArrow = document.createElement("button");
    downArrow.classList.add("commentPaginationBtn", "commentPaginationDown");
    downArrow.innerHTML = '<i class="bi bi-chevron-down"></i>';
    downArrow.title = "More comments";
    downArrow.style.display = "none";
    downArrow.addEventListener("click", () => navigatePage(container, parentTxHash, blockchain, depth, 1, sort));
    controls.appendChild(downArrow);
    return controls;
}
async function createCommentElement(comment: Comment, depth: number, blockchain: string, sort: CommentSort): Promise<HTMLDivElement> {
    const commentDiv = document.createElement("div");
    commentDiv.classList.add("commentItem");
    commentDiv.dataset.address = comment.address;
    commentDiv.dataset.blockchain = comment.blockchain;
    commentDiv.dataset.depth = Math.min(depth, MAX_INDENT_DEPTH).toString();
    commentDiv.dataset.parenttxhash = comment.parentTxHash;
    commentDiv.dataset.txhash = comment.txHash;
    const headerDiv = document.createElement("div");
    headerDiv.classList.add("commentHeader");
    if (comment.replyCount > 0) {
        if (depth >= MAX_INDENT_DEPTH - 1) {
            const viewMoreLink = document.createElement("a");
            viewMoreLink.href = `/post/${comment.blockchain}/${comment.txHash}`;
            viewMoreLink.classList.add("commentViewMore");
            viewMoreLink.innerHTML = '<i class="bi bi-box-arrow-up-right"></i>';
            viewMoreLink.title = "View full thread";
            headerDiv.appendChild(viewMoreLink);
        } else {
            const toggleBtn = document.createElement("span");
            toggleBtn.classList.add("commentToggle", "collapsed");
            const chevron = document.createElement("i");
            chevron.classList.add("bi", "bi-chevron-down");
            toggleBtn.appendChild(chevron);
            toggleBtn.addEventListener("click", () => {
                toggleReplies(commentDiv, comment.txHash, blockchain, depth + 1, sort);
            });
            headerDiv.appendChild(toggleBtn);
        }
    }
    const profileUrl = `/p/${comment.blockchain}/${comment.address}`;
    const avatarLink = document.createElement("a");
    avatarLink.href = profileUrl;
    avatarLink.classList.add("commentAvatarLink");
    const avatarImg = document.createElement("img");
    avatarImg.classList.add("commentAvatar");
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
    authorLink.classList.add("commentAuthorLink");
    authorLink.textContent = comment.author || comment.address.substring(0, 10) + "...";
    headerDiv.appendChild(authorLink);
    const dateSpan = document.createElement("span");
    dateSpan.classList.add("commentDate");
    dateSpan.textContent = formatTimestamp(comment.timestamp);
    headerDiv.appendChild(dateSpan);
    commentDiv.appendChild(headerDiv);
    const contentDiv = await renderCommentBody(comment);
    commentDiv.appendChild(contentDiv);
    const embedDiv = await renderCommentEmbeds(comment);
    if (embedDiv) {
        commentDiv.appendChild(embedDiv);
    }
    if (comment.attachments && comment.attachments.length > 0) {
        const attachmentDiv = document.createElement("div");
        attachmentDiv.classList.add("commentAttachments");
        for (const attachment of comment.attachments) {
            const fileUrl = normalizeCommentMediaSource(attachment[0]);
            const mimeType = attachment[1];
            const fileName = attachment[3];
            if (!fileUrl) continue;
            if (mimeType.startsWith("image/")) {
                const img = document.createElement("img");
                img.classList.add("commentAttachmentImage");
                img.src = XSSSanitizeUrl(fileUrl);
                img.alt = fileName || "attachment";
                img.crossOrigin = "anonymous";
                img.referrerPolicy = "no-referrer";
                attachmentDiv.appendChild(img);
            } else if (mimeType.startsWith("video/")) {
                const video = createCommentAttachmentVideo(fileUrl);
                attachmentDiv.appendChild(video);
            } else {
                const link = document.createElement("a");
                link.classList.add("commentAttachmentLink");
                link.href = XSSSanitizeUrl(fileUrl);
                link.download = fileName || "attachment";
                link.target = "_blank";
                link.rel = "noopener noreferrer";
                link.textContent = fileName || "Download attachment";
                attachmentDiv.appendChild(link);
            }
        }
        commentDiv.appendChild(attachmentDiv);
    }
    const controlsBar = CreatePostControlsBar({
        blockchain: comment.blockchain,
        initialComments: comment.replyCount,
        initialDislikes: comment.dislikeCount,
        initialLikes: comment.likeCount,
        onCommentClick: () => {
            toggleAddCommentUI(commentDiv, comment.txHash, blockchain, sort);
        },
        onRepostClick: () => {
            const postUrl = `/post/${comment.blockchain}/${comment.txHash}`;
            window.open(postUrl, '_blank');
        },
        targetType: 'comment',
        txHash: comment.txHash,
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
async function loadInitialPages(container: HTMLElement, parentTxHash: string, blockchain: string, sort: CommentSort, targetPage: number): Promise<void> {
    for (let p = 0; p <= targetPage; p++) {
        await loadCommentsPage(container, parentTxHash, blockchain, 0, p, sort);
        const state = paginationStates.get(container);
        if (state && !state.hasMore && p < targetPage) break;
    }
}
async function loadCommentsPage(container: HTMLElement, parentTxHash: string, blockchain: string, depth: number, page: number, sort: CommentSort): Promise<void> {
    let state = paginationStates.get(container);
    if (!state) {
        state = { currentPage: 0, hasMore: true, loading: false, pages: [], sort };
        paginationStates.set(container, state);
    }
    if (state.loading) return;
    if (page < state.pages.length) {
        await renderPage(container, parentTxHash, blockchain, depth, page, sort);
        return;
    }
    if (!state.hasMore) return;
    state.loading = true;
    try {
        const offset = state.pages.length * PAGE_SIZE;
        const response = await HttpGetJson(`/comments/${blockchain}/${parentTxHash}?limit=${PAGE_SIZE}&offset=${offset}&sort=${sort}`);
        if (response[0] === 200 && response[1] && response[1].comments) {
            const comments = response[1].comments as Comment[];
            state.hasMore = comments.length === PAGE_SIZE;
            if (comments.length === 0) {
                updatePaginationArrows(container, state);
                return;
            }
            state.pages.push(comments);
            await renderPage(container, parentTxHash, blockchain, depth, page, sort);
        }
    } catch (e) {
        console.error("Failed to load comments:", e);
    } finally {
        state.loading = false;
    }
}
function updatePaginationArrows(container: HTMLElement, state: PaginationState): void {
    const paginationControls = container.querySelector(".commentPaginationControls") as HTMLElement;
    if (!paginationControls) return;
    const upArrow = paginationControls.querySelector(".commentPaginationUp") as HTMLElement;
    const downArrow = paginationControls.querySelector(".commentPaginationDown") as HTMLElement;
    if (upArrow) {
        upArrow.style.display = state.currentPage > 0 ? "flex" : "none";
    }
    if (downArrow) {
        downArrow.style.display = state.hasMore || state.currentPage < state.pages.length - 1 ? "flex" : "none";
    }
}
async function navigatePage(container: HTMLElement, parentTxHash: string, blockchain: string, depth: number, direction: number, sort: CommentSort): Promise<void> {
    const state = paginationStates.get(container);
    if (!state) return;
    const newPage = state.currentPage + direction;
    if (newPage < 0) return;
    await loadCommentsPage(container, parentTxHash, blockchain, depth, newPage, sort);
}
async function renderPage(container: HTMLElement, parentTxHash: string, blockchain: string, depth: number, page: number, sort: CommentSort): Promise<void> {
    const state = paginationStates.get(container);
    if (!state || page >= state.pages.length) return;
    state.currentPage = page;
    const commentsContainer = container.querySelector(".commentsContainer") as HTMLElement;
    if (!commentsContainer) return;
    commentsContainer.innerHTML = "";
    const comments = state.pages[page];
    for (const comment of comments) {
        const commentElement = await createCommentElement(comment, depth, blockchain, sort);
        commentsContainer.appendChild(commentElement);
    }
    updatePaginationArrows(container, state);
    const callback = pageChangeCallbacks.get(container);
    if (callback) callback(page);
}
function toggleAddCommentUI(commentDiv: HTMLElement, parentTxHash: string, blockchain: string, sort: CommentSort): void {
    const container = commentDiv.querySelector(".addCommentContainer");
    const commentBtn = commentDiv.querySelector(".postControlItem.comment") as HTMLElement;
    const commentIcon = commentBtn?.querySelector("i") as HTMLElement | null;
    if (!container) return;
    const isExpanded = container.classList.contains("expanded");
    if (isExpanded) {
        container.classList.remove("expanded");
        container.innerHTML = "";
        if (commentIcon) {
            commentIcon.style.color = "";
        }
    } else {
        container.classList.add("expanded");
        const addCommentUI = ShowAddCommentUI(parentTxHash, blockchain, () => {
            container.classList.remove("expanded");
            container.innerHTML = "";
            const repliesDiv = commentDiv.querySelector(".commentReplies") as HTMLElement | null;
            if (repliesDiv) {
                paginationStates.delete(repliesDiv);
                repliesDiv.innerHTML = "";
                const depth = parseInt(commentDiv.dataset.depth || "0");
                initializeRepliesContainer(repliesDiv, parentTxHash, blockchain, depth + 1, sort);
            }
        }, commentBtn);
        container.appendChild(addCommentUI);
    }
}
function toggleReplies(commentDiv: HTMLElement, txHash: string, blockchain: string, depth: number, sort: CommentSort): void {
    const toggle = commentDiv.querySelector(".commentToggle");
    const repliesDiv = commentDiv.querySelector(".commentReplies");
    if (!toggle || !repliesDiv) return;
    const isCollapsed = toggle.classList.contains("collapsed");
    if (isCollapsed) {
        toggle.classList.remove("collapsed");
        repliesDiv.classList.remove("collapsed");
        if (!repliesDiv.querySelector(".commentsContainer")) {
            initializeRepliesContainer(repliesDiv as HTMLElement, txHash, blockchain, depth, sort);
        }
    } else {
        toggle.classList.add("collapsed");
        repliesDiv.classList.add("collapsed");
    }
}
function initializeRepliesContainer(container: HTMLElement, parentTxHash: string, blockchain: string, depth: number, sort: CommentSort): void {
    container.innerHTML = "";
    const commentsContainer = document.createElement("div");
    commentsContainer.classList.add("commentsContainer");
    container.appendChild(commentsContainer);
    const paginationControls = createPaginationControls(container, parentTxHash, blockchain, depth, sort);
    container.appendChild(paginationControls);
    loadCommentsPage(container, parentTxHash, blockchain, depth, 0, sort);
}

export function CollapseCommentThread(threadDiv: HTMLDivElement): void {
    const toggles = threadDiv.querySelectorAll(".commentToggle:not(.collapsed)");
    toggles.forEach(toggle => {
        (toggle as HTMLElement).click();
    });
}
export function CreateCommentThread(options: CommentThreadOptions): HTMLDivElement {
    const initialPage = options.initialPage || 0;
    const sort = options.sort || "likes";
    const threadDiv = document.createElement("div");
    threadDiv.classList.add("commentThread");
    threadDiv.dataset.blockchain = options.blockchain;
    threadDiv.dataset.maxdepth = (options.maxDepth || MAX_INDENT_DEPTH).toString();
    threadDiv.dataset.parenttxhash = options.parentTxHash;
    threadDiv.dataset.sort = sort;
    if (options.onPageChange) {
        pageChangeCallbacks.set(threadDiv, options.onPageChange);
    }
    const commentsContainer = document.createElement("div");
    commentsContainer.classList.add("commentsContainer");
    threadDiv.appendChild(commentsContainer);
    const paginationControls = createPaginationControls(threadDiv, options.parentTxHash, options.blockchain, 0, sort);
    threadDiv.appendChild(paginationControls);
    loadInitialPages(threadDiv, options.parentTxHash, options.blockchain, sort, initialPage);
    return threadDiv;
}
export function ExpandCommentThread(threadDiv: HTMLDivElement): void {
    const toggles = threadDiv.querySelectorAll(".commentToggle.collapsed");
    toggles.forEach(toggle => {
        (toggle as HTMLElement).click();
    });
}
export async function FetchComments(blockchain: string, parentTxHash: string, limit: number = PAGE_SIZE, offset: number = 0, sort: CommentSort = "likes"): Promise<Comment[]> {
    try {
        const response = await HttpGetJson(`/comments/${blockchain}/${parentTxHash}?limit=${limit}&offset=${offset}&sort=${sort}`);
        if (response[0] === 200 && response[1] && response[1].comments) {
            return response[1].comments as Comment[];
        }
    } catch (e) {
        console.error("Failed to fetch comments:", e);
    }
    return [];
}
