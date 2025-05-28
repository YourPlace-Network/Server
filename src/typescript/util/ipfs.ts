import {CID} from "multiformats/cid";
import {IsValidIpfsCid} from "./security";
import {HttpPostJson} from "./network";
import {LogError, LogInfo} from "./log";

const IPFS_PUBLIC_GATEWAY = "dweb.link";

export async function AddFileToIPFS(fileUUID: string, csrfToken: string): Promise<CID | null> {
    let response = await HttpPostJson("/ipfs/add", {"fileUUID": fileUUID}, csrfToken);
    if (response[0] === 200) {
        return stringToCID(response[1].cid);
    }
    LogError("Failed to add file to IPFS: " + response[1].status);
    return null;
}
export function CIDToSubdomainURL(cid: string): string {
    const IPFS_PREFIX = "ipfs://";
    const IPFS_POSTFIX = ".ipfs.localhost:42426";
    let url = "";
    if (cid.startsWith(IPFS_PREFIX)) {
        cid = cid.substring(IPFS_PREFIX.length);
        if (cid.endsWith(IPFS_POSTFIX)) {
            cid = cid.substring(0, (cid.length - IPFS_POSTFIX.length));
        }
    }
    if (!IsValidIpfsCid(cid)) {
        return url;
    }
    try {
        const parsedCid = CID.parse(cid);
        const cidv1 = parsedCid.version === 0 ? parsedCid.toV1().toString() : parsedCid.toString();
        url = "http://" + cidv1 + IPFS_POSTFIX;
    } catch (error) {
        LogError("Invalid CID when trying to convert to subdomain syntax: " + error)
    }
    return url.trim();
}
export function GetIPFSFile(cid: string): Promise<Blob> {
    if (!IsValidIpfsCid(cid)) {
        throw new Error("Invalid IPFS CID");
    }
    // Remove ipfs:// prefix if present
    const IPFS_PREFIX = "ipfs://";
    if (cid.startsWith(IPFS_PREFIX)) {
        cid = cid.substring(IPFS_PREFIX.length);
    }
    try {
        // Parse the CID and ensure we have a v1 CID for subdomain gateways
        const parsedCid = CID.parse(cid);
        const cidv1 = parsedCid.version === 0 ? parsedCid.toV1().toString() : parsedCid.toString();

        // Set up our gateway URLs
        const publicGatewayUrl = "https://" + cidv1 + ".ipfs." + IPFS_PUBLIC_GATEWAY;
        const localGatewayUrl = "http://" + cidv1 + ".ipfs.localhost:42426";

        LogInfo("Fetching IPFS file from public gateway: " + publicGatewayUrl);
        LogInfo("Fetching IPFS file from local gateway: " + localGatewayUrl);

        // Create fetch promises for both gateways
        const fetchPublic = fetch(publicGatewayUrl).then(response => {
            if (!response.ok) {
                throw new Error("Public gateway returned status: " + response.status);
            }
            return response.blob();
        }).catch(error => {
            LogError("Error fetching from public gateway: " + error);
            throw error;
        });

        const fetchLocal = fetch(localGatewayUrl).then(response => {
            if (!response.ok) {
                throw new Error("Local gateway returned status: " + response.status);
            }
            return response.blob();
        }).catch(error => {
            LogError("Error fetching from local gateway: " + error.message);
            throw error;
        });

        // Race the promises - return whichever completes first
        console.log("Race starting");
        return Promise.race([fetchPublic, fetchLocal]);
    } catch (error: any) {
        LogError("Failed to fetch IPFS file with CID " + cid + ": " + error);
        throw new Error("Failed to fetch IPFS file: " + error);
    }
}

async function downloadFromIpfs(cid: string): Promise<Blob> {
    const ipfsNodeUrl = "http://localhost:42425";
    try {
        // First, get the file size to determine if we need chunking
        const statResponse = await fetch(ipfsNodeUrl, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({
                jsonrpc: "2.0",
                id: 1,
                method: "files.stat",
                params: [`/ipfs/${cid}`]
            }),
        });

        if (!statResponse.ok) {
            throw new Error(`IPFS stat request failed with status: ${statResponse.status}`);
        }

        const statData = await statResponse.json();

        if (statData.error) {
            throw new Error(`IPFS stat error: ${statData.error.message}`);
        }

        // Make the cat request to get the file content
        const catResponse = await fetch(ipfsNodeUrl, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({
                jsonrpc: "2.0",
                id: 2,
                method: "cat",
                params: [cid]
            }),
        });

        if (!catResponse.ok) {
            throw new Error(`IPFS cat request failed with status: ${catResponse.status}`);
        }

        const catData = await catResponse.json();

        if (catData.error) {
            throw new Error(`IPFS cat error: ${catData.error.message}`);
        }

        // The result is a base64-encoded string of the file content. We need to decode it to get the binary data
        const binaryString = atob(catData.result);
        const bytes = new Uint8Array(binaryString.length);

        for (let i = 0; i < binaryString.length; i++) {
            bytes[i] = binaryString.charCodeAt(i);
        }

        // Create a Blob from the binary data. We don't specify the MIME type as it may vary based on the file
        return new Blob([bytes]);
    } catch (error) {
        console.error("Error downloading file from IPFS:", error);
        throw error;
    }
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