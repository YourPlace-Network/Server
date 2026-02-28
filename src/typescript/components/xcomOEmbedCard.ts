import "../../scss/components/xcomOEmbedCard.scss";
import {PersistentCache} from "../util/cache";
import {HttpGetJson} from "../util/network";
import {XSSSanitizeOEmbed, XSSSanitizeUrl, XSSSanitizeValue} from "../util/security";

const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000;
const twitterOEmbedCache = new PersistentCache("twitter_oembed", SEVEN_DAYS_MS);
const twitterMediaSuffixRegex = /\/(photo|video)\/\d+\/?(?:[?#].*)?$/;
const twitterUrlRegex = /^https:\/\/(?:www\.)?(twitter\.com|x\.com)\/([a-zA-Z0-9_]+)\/status\/(\d+)\/?(?:[?#].*)?$/;

interface MediaItem {
    type: string;
    url: string;
}

interface TwitterOEmbedData {
    author_name: string;
    author_url: string;
    html: string;
    media_urls?: MediaItem[];
    url: string;
}

export interface XcomCardData {
    date?: string;
    mediaUrls?: MediaItem[];
    postUrl: string;
    text: string;
    textIsHtml?: boolean;
    username: string;
}

export async function CreateXcomCard(data: XcomCardData, depth: number = 0): Promise<HTMLDivElement> {
    const cardDiv = document.createElement("div");
    const avatarImg = document.createElement("img");
    const headerDiv = document.createElement("div");
    const textDiv = document.createElement("div");
    const usernameLink = document.createElement("a");
    cardDiv.classList.add("xcomCard");
    avatarImg.classList.add("xcomCardAvatar");
    avatarImg.src = "/static/image/x.svg";
    headerDiv.classList.add("xcomCardHeaderDiv");
    usernameLink.classList.add("xcomCardUsernameLink");
    usernameLink.href = data.postUrl;
    usernameLink.target = "_blank";
    usernameLink.rel = "noopener noreferrer";
    usernameLink.textContent = "@" + XSSSanitizeValue(data.username);
    textDiv.classList.add("xcomCardTextDiv");
    if (data.textIsHtml) {
        textDiv.innerHTML = XSSSanitizeOEmbed(data.text);
    } else {
        textDiv.textContent = data.text;
    }
    const currentStatusId = extractStatusId(data.postUrl);
    const links = textDiv.querySelectorAll("a");
    const nestedCards: HTMLDivElement[] = [];
    for (const link of Array.from(links)) {
        const href = link.getAttribute("href") || "";
        const statusUrl = normalizeTwitterUrl(href);
        if (!statusUrl) { continue; }
        if (extractStatusId(statusUrl) === currentStatusId) {
            link.remove();
            continue;
        }
        if (depth < 1) {
            const nestedCard = await XcomOEmbedCard(statusUrl, depth + 1);
            if (nestedCard) {
                link.remove();
                nestedCards.push(nestedCard);
                continue;
            }
        }
        link.remove();
    }
    if (data.mediaUrls && data.mediaUrls.length > 0) {
        for (const item of data.mediaUrls) {
            const sanitizedUrl = XSSSanitizeUrl(item.url);
            if (sanitizedUrl === "#") { continue; }
            const img = document.createElement("img");
            img.classList.add("xcomCardMedia");
            img.src = sanitizedUrl;
            const linkWrapper = document.createElement("a");
            linkWrapper.href = data.postUrl;
            linkWrapper.target = "_blank";
            linkWrapper.rel = "noopener noreferrer";
            linkWrapper.appendChild(img);
            if (item.type === "video") {
                const playIcon = document.createElement("i");
                playIcon.classList.add("bi", "bi-play-fill", "xcomCardPlayIcon");
                linkWrapper.appendChild(playIcon);
                const container = document.createElement("div");
                container.classList.add("xcomCardMediaContainer");
                container.appendChild(linkWrapper);
                textDiv.appendChild(container);
            } else {
                textDiv.appendChild(linkWrapper);
            }
        }
    }
    for (const nestedCard of nestedCards) {
        textDiv.appendChild(nestedCard);
    }
    cardDiv.appendChild(avatarImg);
    headerDiv.appendChild(usernameLink);
    if (data.date) {
        const dateDiv = document.createElement("div");
        dateDiv.classList.add("xcomCardDate");
        dateDiv.textContent = data.date;
        headerDiv.appendChild(dateDiv);
    }
    cardDiv.appendChild(headerDiv);
    cardDiv.appendChild(textDiv);
    return cardDiv;
}
function normalizeTwitterUrl(url: string): string | null {
    if (twitterMediaSuffixRegex.test(url)) {
        return null;
    }
    if (twitterUrlRegex.test(url)) {
        return url;
    }
    return null;
}
function extractStatusId(url: string): string {
    const match = url.match(twitterUrlRegex);
    return match ? match[3] : "";
}
function extractTextFromOEmbedHtml(html: string): string {
    const parser = new DOMParser();
    const doc = parser.parseFromString(html, "text/html");
    const blockquote = doc.querySelector("blockquote");
    if (blockquote) {
        const paragraph = blockquote.querySelector("p");
        if (paragraph) {
            return paragraph.innerHTML;
        }
    }
    return "";
}
function extractUsernameFromUrl(authorUrl: string): string {
    const match = authorUrl.match(/(?:twitter\.com|x\.com)\/([a-zA-Z0-9_]+)/);
    return match ? match[1] : "";
}
async function fetchTwitterOEmbed(url: string): Promise<TwitterOEmbedData | null> {
    const [status, data] = await HttpGetJson(`/services/twitter/oembed?url=${encodeURIComponent(url)}`);
    if (status === 200 && data) {
        return data as TwitterOEmbedData;
    }
    return null;
}
export async function XcomOEmbedCard(url: string, depth: number = 0): Promise<HTMLDivElement | null> {
    if (!twitterUrlRegex.test(url)) {
        return null;
    }
    let data: TwitterOEmbedData | null = twitterOEmbedCache.get<TwitterOEmbedData>(url);
    if (!data) {
        data = await fetchTwitterOEmbed(url);
        if (!data) {
            return null;
        }
        twitterOEmbedCache.set(url, data);
    }
    const username = extractUsernameFromUrl(data.author_url);
    const tweetText = extractTextFromOEmbedHtml(data.html);
    return CreateXcomCard({
        mediaUrls: data.media_urls,
        postUrl: url,
        text: tweetText,
        textIsHtml: true,
        username: username,
    }, depth);
}
