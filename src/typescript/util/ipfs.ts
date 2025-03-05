import {CID} from "multiformats/cid";
import {IsValidIpfsCid} from "./security";
import {HttpPostJson} from "./network";
import {LogError} from "./log";

export async function AddFileToIPFS(filePath: string, csrfToken: string): Promise<CID | null> {
    let response = await HttpPostJson("/ipfs/add", {"filePath": filePath}, csrfToken);
    if (response[0] === 200) {
        return stringToCID(response[1].cid);
    }
    LogError("Failed to add file to IPFS: " + response[1].status);
    return null;
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