import {isValidAddress} from "algosdk";
import {CID} from "multiformats/cid";
import {isAddress} from "web3-validator";
import DOMPurify from "dompurify";
import {GetBootstrappedIpfsGateway} from "./ipfs";

function ResolveIpfsUrl(url: string): string {
    const trimmedUrl = url.trim();
    if (trimmedUrl === "") {
        return "";
    }
    let candidate = trimmedUrl;
    if (candidate.startsWith("http://") || candidate.startsWith("https://") || candidate.startsWith("/") || candidate.startsWith("data:")) {
        return "";
    }
    if (candidate.startsWith("ipfs://")) {
        candidate = candidate.substring("ipfs://".length);
    }
    const match = candidate.match(/^([^/?#]+)(.*)$/);
    if (!match) {
        return "";
    }
    try {
        const parsedCid = CID.parse(match[1]);
        const normalizedCid = parsedCid.version === 0 ? parsedCid.toV1().toString() : parsedCid.toString();
        const suffix = match[2] || "";
        const isLocalhost = typeof window !== "undefined" &&
            (window.location.hostname === "127.0.0.1" ||
                window.location.hostname === "localhost" ||
                window.location.hostname.endsWith(".localhost"));
        if (isLocalhost) {
            return `http://${normalizedCid}.ipfs.localhost:42426${suffix}`;
        }
        const configuredGateway = GetBootstrappedIpfsGateway();
        if (configuredGateway !== "") {
            return `https://${configuredGateway}/ipfs/${normalizedCid}${suffix}`;
        }
        return "";
    } catch (error) {
        return "";
    }
}

export function IsValidBlockchain(chain: string): boolean {
    const validChains = ["algorand", "base", "ethereum"];
    return validChains.includes(chain);
}
export function IsValidAlgoAddress(address: string): boolean {
    return isValidAddress(address);
}
export function IsValidAlgoTxId(txId: string): boolean {
    if (txId.length !== 52) {
        return false;
    }
    return /^[A-Z2-7]{52}$/i.test(txId);
}
export function IsValidBaseAddress(address: string): boolean {
    return isAddress(address);
}
export function IsValidIpfsCid(cid: string): boolean {
    const IPFS_PREFIX = "ipfs://";
    if (cid.startsWith(IPFS_PREFIX)) {
        cid = cid.substring(IPFS_PREFIX.length);
    }
    try {
        CID.parse(cid);
        return true;
    } catch (error) {
        return false;
    }
}
export function IsValidDataImageUrl(url: string): boolean {
    const lowerUrl = url.toLowerCase();
    if (!lowerUrl.startsWith("data:image/")) {
        return false;
    }
    const MAX_DATA_URL_LENGTH = 14_000_000;
    if (url.length > MAX_DATA_URL_LENGTH) {
        return false;
    }
    const validImageTypes = [
        "data:image/png",
        "data:image/jpeg",
        "data:image/jpg",
        "data:image/gif",
        "data:image/webp",
        "data:image/svg+xml",
    ];
    if (!validImageTypes.some(type => lowerUrl.startsWith(type))) {
        return false;
    }
    if (!url.includes(',')) {
        return false;
    }
    const commaIndex = url.indexOf(',');
    const metadata = url.substring(0, commaIndex).toLowerCase();
    const validMetadataPattern = /^data:image\/(png|jpeg|jpg|gif|webp|svg\+xml)(;charset=[a-z0-9-]+)?(;base64)?$/;
    if (!validMetadataPattern.test(metadata)) {
        return false;
    }
    return true;
}
export function IsValidURL(url: string): boolean {
    try {
        if (url.startsWith("data:")) {
            return IsValidDataImageUrl(url);
        }
        if (url.startsWith("/")) { // allow relative URLs that start with "/" for local navigation
            if (url.toLowerCase().includes("javascript:") || url.toLowerCase().includes("data:")) {
                return false;
            }
            return true;
        }
        if (url.endsWith(".ipfs.localhost:42426")) { // allow local IPFS node links
            let cid: string = url.substring("ipfs://".length, (url.length - ".ipfs.localhost:42426".length));
            return IsValidIpfsCid(cid);
        } else if (url.startsWith("ipfs://")) { // allow generic IPFS links with CID
            let cid: string = url.substring("ipfs://".length);
            return IsValidIpfsCid(cid);
        }
        let urlObj: URL = new URL(url);
        if (urlObj.protocol === "https:") { // allow external HTTPS links
            return true;
        }
        if (urlObj.protocol === "http:" && (urlObj.hostname === "localhost" || urlObj.hostname.endsWith(".localhost"))) { // allow localhost HTTP links including subdomains
            return true;
        }
    } catch (error) {
        return false;
    }
    return false; // disallow everything else
}
export function IsValidYoutubeUrl(url: string): boolean {
    try {
        const urlObj = new URL(url);
        return (
            urlObj.protocol === "https:" &&
            (urlObj.hostname === "www.youtube.com" ||
            urlObj.hostname === "youtube.com" ||
            urlObj.hostname === "www.youtube-nocookie.com" ||
            urlObj.hostname === "youtube-nocookie.com")
        );
    } catch (error) {
        return false;
    }
}
export function XSSSanitizeUrl(href: string): string {
    const resolvedIpfsUrl = ResolveIpfsUrl(href);
    if (resolvedIpfsUrl !== "") {
        return resolvedIpfsUrl;
    }
    if (IsValidURL(href)) {
        return href;
    }
    return "#";
}
export function XSSSanitizeOEmbed(payload: string): string {
    const config = {
        ALLOWED_TAGS: ["a", "br"],
        ALLOWED_ATTR: ["href", "target", "class"],
        ADD_ATTR: ["target"],
        SANITIZE_DOM: true,
    };
    DOMPurify.addHook("beforeSanitizeAttributes", (node) => {
        if (node.nodeName === "A" && node.hasAttribute("href")) {
            const href = node.getAttribute("href");
            if (href && !IsValidURL(href)) {
                node.remove();
            }
            if (href && IsValidURL(href)) {
                node.setAttribute("target", "_blank");
            }
        }
    });
    const sanitized = DOMPurify.sanitize(payload, config) as string;
    DOMPurify.removeHook("beforeSanitizeAttributes");
    return sanitized;
}
export function XSSSanitizeTextUrl(payload: string): string {
    const config = {
        ALLOWED_TAGS: ["a", "br"],
        ALLOWED_ATTR: ["href", "target", "class"],
        ADD_ATTR: ["target"],
        SANITIZE_DOM: true,
    };
    DOMPurify.addHook("beforeSanitizeAttributes", (node) => { // Add a hook to validate href attributes on links
        if (node.nodeName === "A" && node.hasAttribute("href")) {
            const href = node.getAttribute("href");
            if (href && !IsValidURL(href)) {
                node.remove();
            }
            if (href && IsValidURL(href)) { // Add target="_blank" for external links
                node.setAttribute("target", "_blank");
            }
        }
    });
    const sanitized = DOMPurify.sanitize(payload, config) as string;
    DOMPurify.removeHook("beforeSanitizeAttributes"); // Clean up by removing the hook
    return sanitized;
}
export function XSSSanitizeValue(value: string): string {
    return DOMPurify.sanitize(value, {
        ALLOWED_TAGS: [], // No HTML tags allowed
        ALLOWED_ATTR: [], // No HTML attributes allowed
    }) as string;
}
export function XSSSanitizeTinyMCEHtml(html: string): string {
    const config = {
        ALLOWED_TAGS: [
            "p","div","h1","h2","h3","h4","h5","h6","ul","ol","li",
            "blockquote","pre","code","em","i","strong","b","s",
            "sub","sup","table","thead","tbody","tr","th","td","br",
            "hr","span","img","iframe","video"],
        ALLOWED_ATTR: [
            "style","src","width","height","frameborder","allowfullscreen",
            "allow","loading","credentialless","class","controls","alt"],
        ADD_ATTR: ["target"],
        FORBID_TAGS: ["script","style","form","input","button","textarea","svg"],
        FORBID_ATTR: forbiddenAttributes([]),
        SANITIZE_DOM: true,
        FORCE_BODY: true,
        IN_PLACE: true,
        RETURN_DOM: false,
        RETURN_DOM_FRAGMENT: false,
        RETURN_DOM_IMPORT: false,
        RETURN_TRUSTED_TYPE: false,
        ALLOW_UNKNOWN_PROTOCOLS: false,
        ALLOW_ARIA_ATTR: false,
        ALLOW_DATA_ATTR: false,
    };
    DOMPurify.addHook("uponSanitizeElement", node => sanitizeIframe);
    DOMPurify.addHook("uponSanitizeAttribute", function (node: Element, data: any) {
        const attributes = node.attributes;
        let styleValue: string | null = null;
        let styleAttrName: string | null = null;
        for (let i = 0; i < attributes.length; i++) { // Look for any variant of "style" attribute
            if (attributes[i].name.toLowerCase() === "style") {
                styleValue = attributes[i].value;
                styleAttrName = attributes[i].name;
                node.removeAttribute(styleAttrName); // Remove the attribute regardless of case
                break;
            }
        }
        if (styleValue) { // If style was found, sanitize it
            const sanitizedStyle = sanitizeStyle(styleValue);
            if (sanitizedStyle) { // If there are valid style properties, add back a clean style attribute
                node.setAttribute("style", sanitizedStyle);
            }
        }
    });

    let sanitized = DOMPurify.sanitize(html, config) as string;
    if (typeof document !== "undefined" && sanitized !== "") {
        const template = document.createElement("template");
        template.innerHTML = sanitized;
        template.content.querySelectorAll("[src]").forEach((node) => {
            const src = node.getAttribute("src");
            if (!src) {
                return;
            }
            const resolvedSrc = ResolveIpfsUrl(src);
            if (resolvedSrc !== "") {
                node.setAttribute("src", resolvedSrc);
                return;
            }
            if (src.startsWith("ipfs://")) {
                node.removeAttribute("src");
            }
        });
        sanitized = template.innerHTML;
    }

    DOMPurify.removeHook("uponSanitizeElement");
    DOMPurify.removeHook("beforeSanitizeAttributes");
    return sanitized;
}
function forbiddenAttributes(allowedAttrs: string[]): string[] {
    let forbidAttributes = ["onafterprint","onafterscriptexecute","onanimationcancel","onanimationend","onanimationiteration",
        "onanimationstart","onauxclick","onbeforecopy","onbeforecut","onbeforeinput","onbeforeprint","onbeforescriptexecute",
        "onbeforetoggle","onbeforeunload","onbegin","onblur","oncancel","oncanplay","oncanplaythrough","onchange",
        "onclick","onclose","oncontentvisibilityautostatechange","oncontentvisibilityautostatechange(hidden)",
        "oncontextmenu","oncopy","oncuechange","oncut","ondblclick","ondrag","ondragend","ondragenter","ondragexit",
        "ondragleave","ondragover","ondragstart","ondrop","ondurationchange","onend","onended","onerror","onfocus",
        "onfocus(autofocus)","onfocusin","onfocusout","onformdata","onfullscreenchange","onhashchange","oninput",
        "oninvalid","onkeydown","onkeypress","onkeyup","onload","onloadeddata","onloadedmetadata","onloadstart",
        "onmessage","onmousedown","onmouseenter","onmouseleave","onmousemove","onmouseout","onmouseover","onmouseup",
        "onmousewheel","onmozfullscreenchange","onpagehide","onpageshow","onpaste","onpause","onplay","onplaying",
        "onpointercancel","onpointerdown","onpointerenter","onpointerleave","onpointermove","onpointerout","onpointerover",
        "onpointerrawupdate","onpointerup","onpopstate","onprogress","onratechange","onrepeat","onreset","onresize",
        "onscroll","onscrollend","onscrollsnapchange","onsearch","onseeked","onseeking","onselect","onselectionchange",
        "onselectstart","onshow","onsubmit","onsuspend","ontimeupdate","ontoggle","ontoggle(popover)","ontouchend",
        "ontouchmove","ontouchstart","ontransitioncancel","ontransitionend","ontransitionrun","ontransitionstart",
        "onunhandledrejection","onunload","onvolumechange","onwaiting","onwaiting(loop)","onwebkitanimationend",
        "onwebkitanimationiteration","onwebkitanimationstart","onwebkitfullscreenchange","onwebkitmouseforcechanged",
        "onwebkitmouseforcedown","onwebkitmouseforceup","onwebkitmouseforcewillbegin","onwebkitplaybacktargetavailabilitychanged",
        "onwebkitpresentationmodechanged","onwebkittransitionend","onwebkitwillrevealbottom","onwheel"] as string[];
    for (let i = 0; i < allowedAttrs.length; i++) {
        const attr = allowedAttrs[i].toLowerCase();
        if (forbidAttributes.includes(attr)) {
            forbidAttributes.splice(forbidAttributes.indexOf(attr), 1); // Remove the attribute from the list
        }
    }
    return forbidAttributes;
}
function sanitizeIframe(node: Element, data: any) {
    const allowedIframeURLs: string[] = [
        "https://www.youtube.com/embed/",
        "https://www.youtube-nocookie.com/embed/",
        "https://player.vimeo.com/video/",
        "https://rumble.com/embed/",
    ];
    if (data.tagName.toLowerCase() === "iframe") {
        const attributes = node.attributes; // Get all attributes of the node
        let srcValue: string | null = null;
        for (let i = 0; i < attributes.length; i++) { // Loop through all attributes and find all variations of src
            if (attributes[i].name.toLowerCase() === "src") {
                srcValue = attributes[i].value;
                node.removeAttribute(attributes[i].name); // Remove the attribute regardless of case
                break;
            }
        }
        if (srcValue) { // If src was found, check if it's in the whitelist
            const isAllowed = allowedIframeURLs.some(url => srcValue!.startsWith(url));
            if (isAllowed) { // If allowed, add back a clean src attribute
                node.setAttribute("src", srcValue);
            }
        }
    }
}
function sanitizeStyle(styleAttr: string): string {
    const allowedStyleProperties: string[] = [
        "text-decoration",
        "text-align",
        "color",
        "background-color",
    ];
    if (!styleAttr) return "";
    // Split the style attribute into individual declarations
    const declarations = styleAttr.split(";").filter(Boolean);
    const sanitizedDeclarations: string[] = [];
    for (const declaration of declarations) {
        // Split each declaration into property and value
        const parts = declaration.split(":");
        if (parts.length >= 2) {
            const propertyName = parts[0].toLowerCase().trim();
            const propertyValue = parts.slice(1).join(":").toLowerCase().trim();
            if (propertyName && propertyValue) {
                // Check if the property is in the whitelist
                if (allowedStyleProperties.some(allowed => propertyName.toLowerCase() == allowed.toLowerCase())) {
                    sanitizedDeclarations.push(`${propertyName}: ${propertyValue}`);
                }
            }
        }
    }
    return sanitizedDeclarations.join("; ");
}
function sanitizeEmojis(payload: string): string {
    const emojiRegex = /[\u{1F600}-\u{1F64F}]|[\u{1F300}-\u{1F5FF}]|[\u{1F680}-\u{1F6FF}]|[\u{1F700}-\u{1F77F}]|[\u{1F780}-\u{1F7FF}]|[\u{1F800}-\u{1F8FF}]|[\u{1F900}-\u{1F9FF}]|[\u{1FA00}-\u{1FA6F}]|[\u{1FA70}-\u{1FAFF}]|[\u{2600}-\u{26FF}]|[\u{2700}-\u{27BF}]|[\u{1F1E0}-\u{1F1FF}]|[\u{1F170}-\u{1F251}]/gu;
    const matches = payload.match(emojiRegex);
    return matches ? matches.join("") : "";
}
function sanitizeSingleEmoji(payload: string): string {
    const sanitized = sanitizeEmojis(payload);
    if (sanitized.length === 0) return "";
    // Use spread operator to properly handle multi-byte emojis
    const emojis = [...sanitized];
    return emojis[0];
}
export async function HashString (input: string): Promise<string> { //TODO: Use UUID instead for element IDs
    const buffer = new TextEncoder().encode(input); // generates hash buffer from string
    const hash = await crypto.subtle.digest("SHA-256", buffer); // hashes the buffer
    const hashArray = Array.from(new Uint8Array(hash)); // converts hash to Uint8Array
    const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join(""); // converts array to hex string
    return hashHex;
}
