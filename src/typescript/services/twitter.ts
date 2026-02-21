import {PersistentCache} from "../util/cache";
import {LogError, LogInfo} from "../util/log";
import {HttpGetJson, HttpPostJson} from "../util/network";
import {XSSSanitizeOEmbed, XSSSanitizeUrl, XSSSanitizeValue} from "../util/security";

const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000;
const twitterOEmbedCache = new PersistentCache("twitter_oembed", SEVEN_DAYS_MS);
const twitterUrlRegex = /^https:\/\/(?:www\.)?(twitter\.com|x\.com)\/([a-zA-Z0-9_]+)\/status\/(\d+)/;

interface TwitterOEmbedData {
    author_name: string;
    author_url: string;
    html: string;
    url: string;
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
export function isTwitterStatusUrl(url: string): boolean {
    return twitterUrlRegex.test(url);
}
async function fetchTwitterOEmbed(url: string): Promise<TwitterOEmbedData | null> {
    const [status, data] = await HttpGetJson(`/services/twitter/oembed?url=${encodeURIComponent(url)}`);
    if (status === 200 && data) {
        return data as TwitterOEmbedData;
    }
    return null;
}
function createTwitterEmbedCard(data: TwitterOEmbedData, originalUrl: string): HTMLDivElement {
    const username = extractUsernameFromUrl(data.author_url);
    const tweetText = extractTextFromOEmbedHtml(data.html);
    const postDiv = document.createElement("div");
    const avatarDiv = document.createElement("div");
    const avatarImg = document.createElement("img");
    const postHeaderDiv = document.createElement("div");
    const postAuthorLink = document.createElement("a");
    const postAuthor = document.createElement("b");
    const postUsername = document.createElement("span");
    const postTextDiv = document.createElement("div");
    postDiv.classList.add("postCard", "xcomPostCard");
    postDiv.style.marginLeft = "1em";
    avatarDiv.classList.add("postCardAvatar", "clickable");
    avatarDiv.addEventListener("click", () => {
        window.open(XSSSanitizeUrl(data.author_url), "_blank");
    });
    avatarImg.classList.add("postCardAvatar");
    avatarImg.src = "/static/image/x.svg";
    postHeaderDiv.classList.add("postCardHeaderDiv");
    postAuthorLink.classList.add("postCardAuthorLink");
    postAuthorLink.href = XSSSanitizeUrl(originalUrl);
    postAuthorLink.target = "_blank";
    postAuthor.classList.add("postCardAuthor");
    postAuthor.textContent = XSSSanitizeValue(data.author_name || username);
    postUsername.classList.add("postCardUsername");
    postUsername.textContent = " @" + XSSSanitizeValue(username);
    postTextDiv.classList.add("postCardTextDiv");
    postTextDiv.innerHTML = XSSSanitizeOEmbed(tweetText);
    avatarDiv.appendChild(avatarImg);
    postDiv.appendChild(avatarDiv);
    postAuthorLink.appendChild(postAuthor);
    postAuthorLink.appendChild(postUsername);
    postHeaderDiv.appendChild(postAuthorLink);
    postDiv.appendChild(postHeaderDiv);
    postDiv.appendChild(postTextDiv);
    return postDiv;
}
export async function TwitterEmbed(url: string): Promise<HTMLDivElement | null> {
    if (!isTwitterStatusUrl(url)) {
        return null;
    }
    const cached = twitterOEmbedCache.get<TwitterOEmbedData>(url);
    if (cached) {
        return createTwitterEmbedCard(cached, url);
    }
    const data = await fetchTwitterOEmbed(url);
    if (!data) {
        return null;
    }
    twitterOEmbedCache.set(url, data);
    return createTwitterEmbedCard(data, url);
}

export async function XcomIsCrossPostEnabled(): Promise<boolean> {
    try {
        const response = await HttpGetJson("/settings/services/xcom/crosspost");
        if (response[0] !== 200) {
            return false;
        }
        return response[1].enabled === true;
    } catch (error: any) {
        LogError("X.com CrossPost Check Error: " + error);
        return false;
    }
}
export async function XcomCrossPost(text: string, csrfToken: string): Promise<boolean> {
    try {
        const response = await HttpPostJson("/services/xcom/post", {text: text}, csrfToken);
        if (response[0] !== 200) {
            LogError("X.com CrossPost Error: " + response[1].status);
            return false;
        }
        LogInfo("X.com CrossPost Success");
        return true;
    } catch (error: any) {
        LogError("X.com CrossPost Error: " + error);
        return false;
    }
}