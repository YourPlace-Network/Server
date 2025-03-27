import DOMPurify from "dompurify";
import {IsValidAlgoAddress} from "../util/security";

let nfdJSON: null | any = null;

export async function NfdGetVanityAddress(address: string): Promise<string> {
    if (nfdJSON !== null) {
        return nfdJSON[address][0].name;
    }
    let response = await fetch("https://api.nf.domains/nfd/v2/address?address=" + address + "&view=full");
    nfdJSON = await response.json();
    return DOMPurify.sanitize(nfdJSON[address][0].name);
}
export async function NfdGetAvatar(address: string): Promise<URL | null> {
    if (nfdJSON !== null) {
        return new URL(DOMPurify.sanitize(nfdJSON[address][0].properties.userDefined.avatar));
    }
    if (!IsValidAlgoAddress(address)) return null;
    let response = await fetch("https://api.nf.domains/nfd/v2/address?address=" + address + "&view=full");
    nfdJSON = await response.json();
    let urlString = DOMPurify.sanitize(nfdJSON[address][0].properties.userDefined.avatar);
    if (urlString === "") return null;
    return new URL(urlString);
}
export async function NfdGetBanner(address: string): Promise<string> {
    if (nfdJSON !== null) {
        return nfdJSON[address][0].properties.userDefined.banner;
    }
    let response = await fetch("https://api.nf.domains/nfd/v2/address?address=" + address + "&view=full");
    nfdJSON = await response.json();
    return DOMPurify.sanitize(nfdJSON[address][0].properties.userDefined.banner);
}