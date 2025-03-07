import {CID} from "multiformats/cid";
import {IsValidIpfsCid} from "./security";
import {HttpPostJson} from "./network";
import {LogError, LogInfo} from "./log";

export async function AddFileToIPFS(filePath: string, csrfToken: string): Promise<CID | null> {
    let response = await HttpPostJson("/ipfs/add", {"filePath": filePath}, csrfToken);
    if (response[0] === 200) {
        return stringToCID(response[1].cid);
    }
    LogError("Failed to add file to IPFS: " + response[1].status);
    return null;
}
export function CIDToSubdomainURL(cid: string): string {
    const IPFS_PREFIX = "ipfs://";
    let url = "";
    if (cid.startsWith(IPFS_PREFIX)) {
        cid = cid.substring(IPFS_PREFIX.length);
    }
    if (!IsValidIpfsCid(cid)) {
        return url;
    }
    try {
        const parsedCid = CID.parse(cid);
        const cidv1 = parsedCid.version === 0 ? parsedCid.toV1().toString() : parsedCid.toString();
        url = "http://" + cidv1 + ".ipfs.localhost:42426";
    } catch (error) {
        LogError("Invalid CID when trying to convert to subdomain syntax: " + error)
    }
    LogInfo("CID: " + cid + " to Subdomain URL: " + url);
    return url.trim();
}

/* --- Helper Functions --- */
export function stringToCID(cid: string): CID {
    if (!IsValidIpfsCid(cid)) throw new Error("Invalid CID");
    return CID.parse(cid);
}
async function iterableToBlobArray(asyncIterable: AsyncIterable<Uint8Array>): Promise<BlobPart[]> {
    const blobParts: BlobPart[] = [];
    for await (const chunk of asyncIterable) {
        blobParts.push(chunk);
    }
    return blobParts;
}