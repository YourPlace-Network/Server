import "../../scss/components/oEmbedCard.scss";
import {PersistentCache} from "../util/cache";
import {HttpGetJson} from "../util/network";
import {XSSSanitizeUrl, XSSSanitizeValue} from "../util/security";

const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000;
const oEmbedCache = new PersistentCache("generic_oembed", SEVEN_DAYS_MS);

interface OEmbedData {
    author_name?: string;
    author_url?: string;
    description?: string;
    provider_name?: string;
    provider_url?: string;
    thumbnail_url?: string;
    title?: string;
    type?: string;
}

function CreateOEmbedCard(data: OEmbedData, sourceUrl: string): HTMLDivElement | null {
    if (!data.title && !data.thumbnail_url) {
        return null;
    }
    const cardDiv = document.createElement("div");
    const contentDiv = document.createElement("div");
    cardDiv.classList.add("oEmbedCard");
    contentDiv.classList.add("oEmbedCardContent");
    cardDiv.addEventListener("click", (e) => {
        const target = e.target as HTMLElement;
        if (target.closest("a")) { return; }
        window.open(sourceUrl, "_blank", "noopener,noreferrer");
    });
    if (data.thumbnail_url) {
        const sanitizedUrl = XSSSanitizeUrl(data.thumbnail_url);
        if (sanitizedUrl !== "#") {
            const thumbnail = document.createElement("img");
            thumbnail.classList.add("oEmbedCardThumbnail");
            thumbnail.crossOrigin = "anonymous";
            thumbnail.referrerPolicy = "no-referrer";
            thumbnail.src = sanitizedUrl;
            thumbnail.onerror = () => { thumbnail.remove(); };
            cardDiv.appendChild(thumbnail);
        }
    }
    const providerName = data.provider_name || new URL(sourceUrl).hostname;
    const providerDiv = document.createElement("div");
    providerDiv.classList.add("oEmbedCardProvider");
    providerDiv.textContent = XSSSanitizeValue(providerName);
    contentDiv.appendChild(providerDiv);
    if (data.title) {
        const titleDiv = document.createElement("div");
        titleDiv.classList.add("oEmbedCardTitle");
        titleDiv.textContent = XSSSanitizeValue(data.title);
        contentDiv.appendChild(titleDiv);
    }
    if (data.description) {
        const descDiv = document.createElement("div");
        descDiv.classList.add("oEmbedCardDescription");
        descDiv.textContent = XSSSanitizeValue(data.description);
        contentDiv.appendChild(descDiv);
    } else if (data.author_name) {
        const authorDiv = document.createElement("div");
        authorDiv.classList.add("oEmbedCardDescription");
        authorDiv.textContent = XSSSanitizeValue(data.author_name);
        contentDiv.appendChild(authorDiv);
    }
    cardDiv.appendChild(contentDiv);
    return cardDiv;
}
export async function OEmbedCard(url: string): Promise<HTMLDivElement | null> {
    let data: OEmbedData | null = oEmbedCache.get<OEmbedData>(url);
    if (!data) {
        const [status, response] = await HttpGetJson(`/services/oembed?url=${encodeURIComponent(url)}`);
        if (status !== 200 || !response) {
            return null;
        }
        data = response as OEmbedData;
        oEmbedCache.set(url, data);
    }
    return CreateOEmbedCard(data, url);
}
