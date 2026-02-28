import "../../scss/components/xcomOEmbedCard.scss";
import {PersistentCache} from "../util/cache";
import {HttpGetJson} from "../util/network";
import {XSSSanitizeOEmbed, XSSSanitizeValue} from "../util/security";

const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000;
const twitterOEmbedCache = new PersistentCache("twitter_oembed", SEVEN_DAYS_MS);
const twitterMediaSuffixRegex = /\/(photo|video)\/\d+\/?(?:[?#].*)?$/;
const twitterUrlRegex = /^https:\/\/(?:www\.)?(twitter\.com|x\.com)\/([a-zA-Z0-9_]+)\/status\/(\d+)\/?(?:[?#].*)?$/;

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
    const links = textDiv.querySelectorAll("a");
    for (const link of Array.from(links)) {
        const href = link.getAttribute("href") || "";
        const statusUrl = normalizeTwitterUrl(href);
        if (!statusUrl) { continue; }
        if (depth < 1) {
            const nestedCard = await XcomOEmbedCard(statusUrl, depth + 1);
            if (nestedCard) {
                link.replaceWith(nestedCard);
                continue;
            }
        }
        link.remove();
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
    if (twitterUrlRegex.test(url)) {
        return url;
    }
    const stripped = url.replace(twitterMediaSuffixRegex, "");
    if (twitterUrlRegex.test(stripped)) {
        return stripped;
    }
    return null;
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
        postUrl: url,
        text: tweetText,
        textIsHtml: true,
        username: username,
    }, depth);
}
