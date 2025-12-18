import { HttpGetJson } from "../util/network";
import { SanitizeXSS } from "../util/security";

const POST_URL_REGEX = /\/post\/(base|algorand)\/0x[a-fA-F0-9]+/g;
export function DetectPostUrl(text: string): string | null {
    const match = text.match(POST_URL_REGEX);
    if (match && match.length > 0) {
        return match[0];
    }
    return null;
}
export async function CreatePostPreviewCard(postUrl: string): Promise<HTMLDivElement | null> {
    const urlParts = postUrl.split('/');
    if (urlParts.length < 4) return null;
    const blockchain = urlParts[2];
    const txHash = urlParts[3];
    try {
        const response = await HttpGetJson(`/post/data/${blockchain}/${txHash}`);
        if (response[0] !== 200 || !response[1] || !response[1].post) {
            return null;
        }
        const post = response[1].post;
        return renderPreviewCard(post, postUrl);
    } catch (e) {
        console.error("Failed to fetch post for preview:", e);
        return null;
    }
}
function renderPreviewCard(post: any, postUrl: string): HTMLDivElement {
    const card = document.createElement("div");
    card.classList.add("postPreviewCard");
    card.addEventListener("click", () => {
        window.open(postUrl, '_blank');
    });
    const header = document.createElement("div");
    header.classList.add("previewHeader");
    const avatar = document.createElement("img");
    avatar.classList.add("previewAvatar");
    avatar.src = post.avatarSrc || "/static/image/avatar.png";
    avatar.alt = "avatar";
    avatar.crossOrigin = "anonymous";
    avatar.referrerPolicy = "no-referrer";
    header.appendChild(avatar);
    const authorSpan = document.createElement("span");
    authorSpan.classList.add("previewAuthor");
    authorSpan.textContent = post.author || truncateAddress(post.address);
    header.appendChild(authorSpan);
    const dateSpan = document.createElement("span");
    dateSpan.classList.add("previewDate");
    dateSpan.textContent = formatTimestamp(post.timestamp);
    header.appendChild(dateSpan);
    card.appendChild(header);
    const textDiv = document.createElement("div");
    textDiv.classList.add("previewText");
    const truncatedText = truncateText(post.payload, 200);
    textDiv.innerHTML = SanitizeXSS(truncatedText);
    card.appendChild(textDiv);
    if (post.attachments && post.attachments.length > 0) {
        const mediaDiv = document.createElement("div");
        mediaDiv.classList.add("previewMedia");
        const firstAttachment = post.attachments[0];
        const mimeType = firstAttachment[1] || "";
        if (mimeType.startsWith("image/")) {
            const img = document.createElement("img");
            img.src = firstAttachment[0];
            img.alt = "attachment";
            img.crossOrigin = "anonymous";
            img.referrerPolicy = "no-referrer";
            mediaDiv.appendChild(img);
            card.appendChild(mediaDiv);
        }
    }
    return card;
}
function truncateAddress(address: string): string {
    if (!address) return "Anonymous";
    if (address.length <= 13) return address;
    return address.substring(0, 6) + "..." + address.substring(address.length - 4);
}
function truncateText(text: string, maxLength: number): string {
    if (!text) return "";
    const strippedText = text.replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim();
    if (strippedText.length <= maxLength) return text;
    return strippedText.substring(0, maxLength) + "...";
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
export async function ProcessPostContentForPreviews(contentDiv: HTMLElement): Promise<void> {
    const text = contentDiv.textContent || "";
    const postUrls = text.match(POST_URL_REGEX);
    if (!postUrls || postUrls.length === 0) return;
    const processedUrls = new Set<string>();
    for (const url of postUrls) {
        if (processedUrls.has(url)) continue;
        processedUrls.add(url);
        const previewCard = await CreatePostPreviewCard(url);
        if (previewCard) {
            contentDiv.appendChild(previewCard);
        }
    }
}
