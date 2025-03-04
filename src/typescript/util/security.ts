import {isValidAddress} from "algosdk";
import {CID} from "multiformats/cid";
import {isAddress} from "web3-validator";
import * as DOMPurify from "dompurify";

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
    const config: DOMPurify.Config = {
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
    const config: DOMPurify.Config = {
        ALLOWED_TAGS: [
            "p","div","h1","h2","h3","h4","h5","h6","ul","ol","li",
            "blockquote","pre","code","em","i","strong","b","s",
            "sub","sup","table","thead","tbody","tr","th","td","br",
            "hr","span","img","iframe"],
        ALLOWED_ATTR: [
            "style","src","width","height","frameborder","allowfullscreen",
            "allow","loading","credentialless"],
        ADD_ATTR: ["target"],
        FORBID_TAGS: ["script","style","form","input","button","textarea","svg"],
        FORBID_ATTR: ["onerror","onload","onclick","onmouseover","onmouseout"],
        SANITIZE_DOM: true,
        FORCE_BODY: true,
        IN_PLACE: true,
        KEEP_CONTENT: false,
        RETURN_DOM: false,
        RETURN_DOM_FRAGMENT: false,
        RETURN_DOM_IMPORT: false,
        RETURN_TRUSTED_TYPE: false,
        ALLOW_UNKNOWN_PROTOCOLS: false,
        ALLOW_ARIA_ATTR: false,
        ALLOW_DATA_ATTR: false,
    };
    // Set up a hook to filter style attributes
    DOMPurify.addHook("beforeSanitizeAttributes", (node) => {
        if (node.hasAttribute("style")) {
            const style = node.getAttribute("style");
            if (style) {
                // Create whitelist of allowed CSS properties and their valid patterns
                const allowedStyles: Record<string, RegExp> = {
                    "text-align": /^\s*(left|right|center|justify)\s*$/i,
                    "color": /^(#[0-9a-f]{3,6}|rgba?\([0-9, .]+\))$/i,
                    "background-color": /^(#[0-9a-f]{3,6}|rgba?\([0-9, .]+\))$/i,
                    "font-weight": /^(normal|bold|[1-9]00)$/i,
                    "text-decoration": /^(none|underline|line-through)$/i,
                    "margin": /^[0-9]+(px|em|rem|%)( [0-9]+(px|em|rem|%))*$/i,
                    "padding": /^[0-9]+(px|em|rem|%)( [0-9]+(px|em|rem|%))*$/i
                };
                // Filter style properties
                const styleArray = style.split(";").map(s => s.trim());
                const safeStyles = styleArray
                    .map(s => {
                        const [prop, value] = s.split(":");
                        if (!prop || !value) return null;
                        const trimmedProp = prop.trim().toLowerCase();
                        const trimmedValue = value.trim().toLowerCase();
                        // Check if property is allowed to value matches pattern
                        if (allowedStyles[trimmedProp] &&
                            allowedStyles[trimmedProp].test(trimmedValue)) {
                            return `${trimmedProp}:${trimmedValue}`;
                        }
                        return null;
                    }).filter(s => s != null).join(";");
                if (safeStyles) {
                    node.setAttribute("style", safeStyles);
                } else {
                    node.removeAttribute("style");
                }
            } else {
                node.removeAttribute("style");
            }
        }
        // Handle iframe src attributes - only allow YouTube
        if (node.nodeName === "IFRAME" && node.hasAttribute("src")) {
            const src = node.getAttribute("src");
            if (src && !IsValidYoutubeUrl(src)) { // If not a valid YouTube URL, remove the entire iframe
                node.parentNode?.removeChild(node);
            }
        }
    });
    const sanitized = DOMPurify.sanitize(html, config) as string;
    DOMPurify.removeHook("beforeSanitizeAttributes");
    return sanitized;
}
