import {isValidAddress} from "algosdk";
import {CID} from "multiformats/cid";
import {isAddress} from "web3-validator";
import DOMPurify from "dompurify";

export function IsValidBlockchain(chain: string): boolean {
    const validChains = ["algo", "base", "eth", "sol"];
    return validChains.includes(chain);
}
export function IsValidAlgoAddress(address: string): boolean {
    return isValidAddress(address);
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
export function IsValidHttpUrl(url: string): boolean {
    try {
        let urlObj = new URL(url);
        if (urlObj.protocol === "https:") { // allow external HTTPS links
            return true;
        }
        if (urlObj.protocol === "http:" && urlObj.hostname === "localhost") { // allow localhost HTTP links
            return true;
        }
    } catch (error) {
        return false;
    }
    return false;
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
    if (IsValidHttpUrl(href)) {
        return href;
    }
    return "#";
}
export function XSSSanitizeTextUrl(payload: string): string {
    const config = {
        ALLOWED_TAGS: ["a"],
        ALLOWED_ATTR: ["href", "target"],
        ADD_ATTR: ["target"],
        SANITIZE_DOM: true,
    };
    // Add a hook to validate href attributes on links
    DOMPurify.addHook("beforeSanitizeAttributes", (node) => {
        if (node.nodeName === "A" && node.hasAttribute("href")) {
            const href = node.getAttribute("href");
            if (href && !IsValidHttpUrl(href)) {
                node.remove();
            }
            // Add target="_blank" for external links
            if (href && IsValidHttpUrl(href)) {
                node.setAttribute("target", "_blank");
            }
        }
    });
    const sanitized = DOMPurify.sanitize(payload, config) as string;
    // Clean up by removing the hook
    DOMPurify.removeHook("beforeSanitizeAttributes");
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
            "hr","span","img","iframe"],
        ALLOWED_ATTR: [
            "style","src","width","height","frameborder","allowfullscreen",
            "allow","loading","credentialless","class"],
        ADD_ATTR: ["target"],
        FORBID_TAGS: ["script","style","form","input","button","textarea","svg"],
        FORBID_ATTR: ["onerror","onload","onclick","onmouseover","onmouseout"],
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
    DOMPurify.addHook("uponSanitizeAttribute", function(node: Element, data: any) {
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

    const sanitized = DOMPurify.sanitize(html, config) as string;

    DOMPurify.removeHook("uponSanitizeElement");
    DOMPurify.removeHook("beforeSanitizeAttributes");
    return sanitized;
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
export async function HashString (input: string): Promise<string> { //TODO: Use UUID instead for element IDs
    const buffer = new TextEncoder().encode(input); // generates hash buffer from string
    const hash = await crypto.subtle.digest("SHA-256", buffer); // hashes the buffer
    const hashArray = Array.from(new Uint8Array(hash)); // converts hash to Uint8Array
    const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join(""); // converts array to hex string
    return hashHex;
}
