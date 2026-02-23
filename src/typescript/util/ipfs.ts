import {CID} from "multiformats/cid";
import {IsValidIpfsCid, IsValidURL} from "./security";
import {HttpGetJson, HttpPostJson} from "./network";
import {LogDebug, LogError, LogInfo} from "./log";
import {IsGatewayMode} from "./miscellaneous";

const IPFS_GATEWAY_DEFAULT = "ipfs.io";
let ipfsGatewayCache: string | null = null;

async function getIpfsGateway(): Promise<string> {
    if (ipfsGatewayCache !== null) return ipfsGatewayCache;
    try {
        const response = await HttpGetJson("/settings/content/ipfsGateway");
        if (response[0] === 200 && response[1].gateway) {
            ipfsGatewayCache = response[1].gateway;
            return ipfsGatewayCache as string;
        }
    } catch (error) {
        LogError("Failed to get IPFS gateway setting: " + error);
    }
    return IPFS_GATEWAY_DEFAULT;
}
export async function AddFileToIPFS(fileUUID: string, csrfToken: string): Promise<CID | null> {
    let response = await HttpPostJson("/files/ipfs/add", {"fileUUID": fileUUID}, csrfToken);
    if (response[0] === 200) {
        return stringToCID(response[1].cid);
    }
    LogError("Failed to add file to IPFS: " + response[1].status);
    return null;
}
export async function GetNFTUploadAuth(csrfToken: string): Promise<any> {
    const response = await HttpPostJson("/files/nft/sign", {}, csrfToken);
    if (response[0] === 200) {
        return response[1];
    }
    LogDebug("Failed to get NFT upload auth: " + (response[1].status || response[0]));
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
        if (IsGatewayMode()) {
            const ipfsGateway = ipfsGatewayCache || IPFS_GATEWAY_DEFAULT;
            url = "https://" + ipfsGateway + "/ipfs/" + cidv1;
        } else {
            url = "http://" + cidv1 + IPFS_POSTFIX;
        }
    } catch (error) {
        LogError("Invalid CID when trying to convert to subdomain syntax: " + error)
    }
    return url.trim();
}
export async function CIDToSubdomainURLAsync(cid: string): Promise<string> {
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
        if (IsGatewayMode()) {
            const ipfsGateway = await getIpfsGateway();
            url = "https://" + ipfsGateway + "/ipfs/" + cidv1;
        } else {
            url = "http://" + cidv1 + IPFS_POSTFIX;
        }
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
        const publicGatewayUrl = "https://" + IPFS_GATEWAY_DEFAULT + "/ipfs/" + cidv1;
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

/* --- Image Loading with Timeout --- */
export async function loadImageWithTimeout(url: string, timeoutMs: number = 5000): Promise<boolean> {
    return new Promise((resolve) => {
        const img = new Image();
        img.crossOrigin = "anonymous";
        let timeoutId: number;
        let resolved = false;
        const cleanup = () => {
            if (timeoutId) clearTimeout(timeoutId);
            resolved = true;
            img.onload = null;
            img.onerror = null;
            img.src = "";
        };
        img.onload = () => {
            if (!resolved) {
                cleanup();
                resolve(true);
            }
        };
        img.onerror = () => {
            if (!resolved) {
                cleanup();
                LogError(`Failed to load image: ${url}`);
                resolve(false);
            }
        };
        timeoutId = window.setTimeout(() => {
            if (!resolved) {
                cleanup();
                LogError(`Image load timeout (${timeoutMs}ms): ${url}`);
                resolve(false);
            }
        }, timeoutMs);
        img.src = url;
    });
}
export async function checkIPFSContentExists(cid: string, timeoutMs: number = 3000): Promise<boolean> {
    try {
        const url = CIDToSubdomainURL(cid);
        if (!url) return false;
        // Use HEAD request to check if content exists without downloading
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), timeoutMs);
        try {
            const response = await fetch(url, {
                method: 'HEAD',
                signal: controller.signal
            });
            clearTimeout(timeoutId);
            return response.ok;
        } catch (error) {
            clearTimeout(timeoutId);
            LogError(`IPFS content check failed for ${cid}: ${error}`);
            return false;
        }
    } catch (error) {
        LogError(`IPFS content existence check error: ${error}`);
        return false;
    }
}

/* --- IPFS Avatar Caching --- */
export async function getIpfsAvatarUrl(blockchain: string, address: string): Promise<string | null> {
    try {
        const response = await HttpGetJson("/profile/avatar/" + blockchain + "/" + address);
        if (response[0] === 200 && response[1] && response[1].avatarAddress) {
            const avatarCid = response[1].avatarAddress.trim();
            if (avatarCid.length > 0) {
                if (avatarCid.startsWith("ipfs://")) {
                    const converted = CIDToSubdomainURL(avatarCid);
                    if (converted) return converted;
                } else if (IsValidURL(avatarCid)) {
                    return avatarCid;
                }
                const avatarURL = CIDToSubdomainURL(avatarCid);
                if (IsValidURL(avatarURL)) {
                    return avatarURL;
                }
            }
        }
    } catch (error) {
        LogError("Failed to get IPFS avatar: " + error);
    }
    return null;
}

/* --- Helper Functions --- */
export function stringToCID(cid: string): CID {
    if (!IsValidIpfsCid(cid)) throw new Error("Invalid CID");
    return CID.parse(cid);
}
export async function UploadToIPFSService(file: File, csrfToken: string): Promise<string | null> {
    const auth = await GetNFTUploadAuth(csrfToken);
    if (!auth) return null;
    try {
        if (auth.type === "pinata") {
            return await uploadToPinata(file, auth.uploadUrl);
        } else if (auth.type === "ipfs") {
            return await uploadToIPFSNode(file, auth.uploadUrl, auth.key);
        }
        LogDebug("Unknown pinning service type: " + auth.type);
        return null;
    } catch (error) {
        LogDebug("UploadToIPFSService failed: " + error);
        return null;
    }
}
async function uploadToIPFSNode(file: File, url: string, key: string): Promise<string | null> {
    const formData = new FormData();
    formData.append("file", file);
    const response = await fetch(url, {
        method: "POST",
        headers: {"Authorization": "Bearer " + key},
        body: formData,
    });
    if (!response.ok) {
        LogDebug("IPFS node upload failed with status: " + response.status);
        return null;
    }
    const result = await response.json();
    if (result.Hash && IsValidIpfsCid(result.Hash)) {
        return result.Hash;
    }
    LogDebug("IPFS node upload returned invalid response");
    return null;
}
async function uploadToPinata(file: File, signedUrl: string): Promise<string | null> {
    const formData = new FormData();
    formData.append("file", file);
    const response = await fetch(signedUrl, {
        method: "POST",
        body: formData,
    });
    if (!response.ok) {
        LogDebug("Pinata upload failed with status: " + response.status);
        return null;
    }
    const result = await response.json();
    const cid = result.data?.cid || result.IpfsHash;
    if (cid && IsValidIpfsCid(cid)) {
        return cid;
    }
    LogDebug("Pinata upload returned invalid response");
    return null;
}
async function iterableToBlobArray(asyncIterable: AsyncIterable<Uint8Array>): Promise<BlobPart[]> {
    const blobParts: BlobPart[] = [];
    for await (const chunk of asyncIterable) {
        blobParts.push(new Uint8Array(chunk));
    }
    return blobParts;
}