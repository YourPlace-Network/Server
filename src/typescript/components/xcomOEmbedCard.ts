import "../../scss/components/xcomOEmbedCard.scss";
import {PersistentCache} from "../util/cache";
import {HttpGetJson} from "../util/network";
import {XSSSanitizeOEmbed, XSSSanitizeValue} from "../util/security";

const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000;
const twitterOEmbedCache = new PersistentCache("twitter_oembed", SEVEN_DAYS_MS);
const twitterUrlRegex = /^https:\/\/(?:www\.)?(twitter\.com|x\.com)\/([a-zA-Z0-9_]+)\/status\/(\d+)/;

interface TwitterOEmbedData {
    author_name: string;
    author_url: string;
    html: string;
    url: string;
}

export interface XcomCardData {
    date?: string;
    postUrl: string;
    text: string;
    textIsHtml?: boolean;
    username: string;
}

export function CreateXcomCard(data: XcomCardData): HTMLDivElement {
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
export async function XcomOEmbedCard(url: string): Promise<HTMLDivElement | null> {
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
        postUrl: url,
        text: tweetText,
        textIsHtml: true,
        username: username,
    });
}
