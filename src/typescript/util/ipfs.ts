import {CID} from "multiformats/cid";
import {HttpGetJson, HttpPostJson} from "./network";
import {LogDebug, LogError, LogInfo} from "./log";
import {IsGatewayMode} from "./miscellaneous";

const IPFS_GATEWAY_META_NAME = "yp-ipfs-gateway";
let ipfsGatewayCache: string | null = null;
const ipfsResolvedUrlCache = new Map<string, string>();
const ipfsMediaProbeCache = new Map<string, Promise<"file" | "image" | "video">>();

function isValidIpfsCidValue(cid: string): boolean {
    const IPFS_PREFIX = "ipfs://";
    let normalizedCid = cid;
    if (normalizedCid.startsWith(IPFS_PREFIX)) {
        normalizedCid = normalizedCid.substring(IPFS_PREFIX.length);
    }
    try {
        CID.parse(normalizedCid);
        return true;
    } catch (error) {
        return false;
    }
}
function isValidNetworkUrl(url: string): boolean {
    try {
        const urlObj = new URL(url);
        if (urlObj.protocol === "https:") {
            return true;
        }
        return urlObj.protocol === "http:" && (urlObj.hostname === "localhost" || urlObj.hostname.endsWith(".localhost"));
    } catch (error) {
        return false;
    }
}
function initializeIpfsGatewayCache(): void {
    if (ipfsGatewayCache !== null || typeof document === "undefined") {
        return;
    }
    const metaEl = document.querySelector(`meta[name="${IPFS_GATEWAY_META_NAME}"]`) as HTMLMetaElement | null;
    const hiddenEl = document.getElementById("ipfsGateway") as HTMLInputElement | null;
    const bootstrappedGateway = metaEl?.content?.trim() || hiddenEl?.value?.trim() || "";
    if (bootstrappedGateway !== "") {
        ipfsGatewayCache = bootstrappedGateway;
    }
}
export function GetBootstrappedIpfsGateway(): string {
    initializeIpfsGatewayCache();
    return ipfsGatewayCache || "";
}
function normalizeIpfsCIDValue(cid: string): string {
    const IPFS_PREFIX = "ipfs://";
    const IPFS_POSTFIX = ".ipfs.localhost:42426";
    let normalizedCid = cid.trim();
    if (normalizedCid.startsWith(IPFS_PREFIX)) {
        normalizedCid = normalizedCid.substring(IPFS_PREFIX.length);
        if (normalizedCid.endsWith(IPFS_POSTFIX)) {
            normalizedCid = normalizedCid.substring(0, normalizedCid.length - IPFS_POSTFIX.length);
        }
    }
    if (!isValidIpfsCidValue(normalizedCid)) {
        return "";
    }
    try {
        const parsedCid = CID.parse(normalizedCid);
        return parsedCid.version === 0 ? parsedCid.toV1().toString() : parsedCid.toString();
    } catch (error) {
        LogError("Invalid CID when trying to normalize IPFS value: " + error);
        return "";
    }
}
function loadImageWithTimeoutInternal(url: string, timeoutMs: number, logFailures: boolean): Promise<boolean> {
    return new Promise((resolve) => {
        const img = new Image();
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
                if (logFailures) {
                    LogError(`Failed to load image: ${url}`);
                }
                resolve(false);
            }
        };
        timeoutId = window.setTimeout(() => {
            if (!resolved) {
                cleanup();
                if (logFailures) {
                    LogError(`Image load timeout (${timeoutMs}ms): ${url}`);
                }
                resolve(false);
            }
        }, timeoutMs);
        img.src = url;
    });
}
function loadVideoMetadataWithTimeout(url: string, timeoutMs: number = 8000): Promise<boolean> {
    return new Promise((resolve) => {
        const video = document.createElement("video");
        let timeoutId: number;
        let resolved = false;
        const cleanup = () => {
            if (timeoutId) clearTimeout(timeoutId);
            resolved = true;
            video.onloadedmetadata = null;
            video.onerror = null;
            video.removeAttribute("src");
            video.load();
        };
        const finish = (result: boolean) => {
            if (resolved) {
                return;
            }
            cleanup();
            resolve(result);
        };
        video.muted = true;
        video.playsInline = true;
        video.preload = "metadata";
        video.onloadedmetadata = () => finish(true);
        video.onerror = () => finish(false);
        timeoutId = window.setTimeout(() => finish(false), timeoutMs);
        video.src = url;
        video.load();
    });
}

initializeIpfsGatewayCache();

export async function GetConfiguredIpfsGateway(): Promise<string> {
    initializeIpfsGatewayCache();
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
    return "";
}
export async function AddFileToIPFS(cid: string, csrfToken: string): Promise<CID | null> {
    let response = await HttpPostJson("/files/ipfs/add", {"cid": cid}, csrfToken);
    if (response[0] === 200) {
        return stringToCID(response[1].cid);
    }
    LogError("Failed to add file to IPFS: " + response[1].status);
    return null;
}
export async function GetAvatarUploadAuth(csrfToken: string): Promise<any> {
    const response = await HttpPostJson("/files/avatar/sign", {}, csrfToken);
    if (response[0] === 200) {
        return response[1];
    }
    LogDebug("Failed to get avatar upload auth: " + (response[1].status || response[0]));
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
export function ResolveIpfsContentUrl(cid: string): string {
    const normalizedCid = normalizeIpfsCIDValue(cid);
    if (normalizedCid === "") {
        return "";
    }
    const cachedUrl = ipfsResolvedUrlCache.get(normalizedCid);
    if (cachedUrl) {
        return cachedUrl;
    }
    const resolvedUrl = CIDToSubdomainURL(normalizedCid);
    if (resolvedUrl !== "") {
        ipfsResolvedUrlCache.set(normalizedCid, resolvedUrl);
    }
    return resolvedUrl;
}
export async function ProbeIpfsMediaType(cid: string, timeoutMs: number = 8000): Promise<"file" | "image" | "video"> {
    const normalizedCid = normalizeIpfsCIDValue(cid);
    if (normalizedCid === "") {
        return "file";
    }
    const cachedProbe = ipfsMediaProbeCache.get(normalizedCid);
    if (cachedProbe) {
        return cachedProbe;
    }
    const probeMediaType = async (): Promise<"file" | "image" | "video"> => {
        const resolvedUrl = ResolveIpfsContentUrl(normalizedCid);
        if (resolvedUrl === "") {
            return "file";
        }
        if (await loadImageWithTimeoutInternal(resolvedUrl, timeoutMs, false)) {
            return "image";
        }
        if (await loadVideoMetadataWithTimeout(resolvedUrl, timeoutMs)) {
            return "video";
        }
        return "file";
    };
    const probePromise = probeMediaType().catch((error) => {
        LogError("Failed to probe IPFS media type: " + error);
        return "file" as const;
    });
    ipfsMediaProbeCache.set(normalizedCid, probePromise);
    return probePromise;
}
export function CIDToSubdomainURL(cid: string): string {
    const IPFS_POSTFIX = ".ipfs.localhost:42426";
    const cidv1 = normalizeIpfsCIDValue(cid);
    if (cidv1 === "") {
        return "";
    }
    try {
        if (IsGatewayMode()) {
            const ipfsGateway = GetBootstrappedIpfsGateway();
            if (ipfsGateway === "") {
                return "";
            }
            return "https://" + ipfsGateway + "/ipfs/" + cidv1;
        }
        return "http://" + cidv1 + IPFS_POSTFIX;
    } catch (error) {
        LogError("Invalid CID when trying to convert to subdomain syntax: " + error)
    }
    return "";
}
export async function CIDToSubdomainURLAsync(cid: string): Promise<string> {
    const IPFS_POSTFIX = ".ipfs.localhost:42426";
    const cidv1 = normalizeIpfsCIDValue(cid);
    if (cidv1 === "") {
        return "";
    }
    try {
        if (IsGatewayMode()) {
            const ipfsGateway = await GetConfiguredIpfsGateway();
            if (ipfsGateway === "") {
                return "";
            }
            return "https://" + ipfsGateway + "/ipfs/" + cidv1;
        }
        return "http://" + cidv1 + IPFS_POSTFIX;
    } catch (error) {
        LogError("Invalid CID when trying to convert to subdomain syntax: " + error)
    }
    return "";
}
export async function GetIPFSFile(cid: string): Promise<Blob> {
    if (!isValidIpfsCidValue(cid)) {
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
        const localGatewayUrl = "http://" + cidv1 + ".ipfs.localhost:42426";
        const publicGatewayUrl = await CIDToSubdomainURLAsync(cidv1);
        const fetchFromGateway = async (gatewayName: string, gatewayUrl: string): Promise<Blob> => {
            LogInfo(`Fetching IPFS file from ${gatewayName} gateway: ${gatewayUrl}`);
            const response = await fetch(gatewayUrl);
            if (!response.ok) {
                throw new Error(`${gatewayName} gateway returned status: ${response.status}`);
            }
            return response.blob();
        };
        if (IsGatewayMode()) {
            if (publicGatewayUrl === "") {
                throw new Error("IPFS gateway is not configured");
            }
            return await fetchFromGateway("public", publicGatewayUrl);
        }
        try {
            return await fetchFromGateway("local", localGatewayUrl);
        } catch (localError) {
            LogError("Error fetching from local gateway: " + localError);
            if (publicGatewayUrl !== "") {
                return await fetchFromGateway("public", publicGatewayUrl);
            }
            throw localError;
        }
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
    return loadImageWithTimeoutInternal(url, timeoutMs, true);
}
export async function checkIPFSContentExists(cid: string, timeoutMs: number = 3000): Promise<boolean> {
    try {
        const url = ResolveIpfsContentUrl(cid);
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
                } else if (isValidNetworkUrl(avatarCid)) {
                    return avatarCid;
                }
                const avatarURL = CIDToSubdomainURL(avatarCid);
                if (isValidNetworkUrl(avatarURL)) {
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
    if (!isValidIpfsCidValue(cid)) throw new Error("Invalid CID");
    return CID.parse(cid);
}
export async function UploadAvatarToIPFSService(file: File, csrfToken: string): Promise<string | null> {
    const auth = await GetAvatarUploadAuth(csrfToken);
    if (!auth) return null;
    try {
        if (auth.type === "yourplace") {
            return await uploadToYourPlace(file, auth.uploadUrl, auth.key);
        }
        LogDebug("Unknown pinning service type: " + auth.type);
        return null;
    } catch (error) {
        LogDebug("UploadAvatarToIPFSService failed: " + error);
        return null;
    }
}
export async function UploadToIPFSService(file: File, csrfToken: string): Promise<string | null> {
    const auth = await GetNFTUploadAuth(csrfToken);
    if (!auth) return null;
    try {
        if (auth.type === "yourplace") {
            return await uploadToYourPlace(file, auth.uploadUrl, auth.key);
        }
        LogDebug("Unknown pinning service type: " + auth.type);
        return null;
    } catch (error) {
        LogDebug("UploadToIPFSService failed: " + error);
        return null;
    }
}
async function uploadToYourPlace(file: File, url: string, key: string): Promise<string | null> {
    const formData = new FormData();
    formData.append("file", file);
    const response = await fetch(url, {
        method: "POST",
        headers: {"Authorization": "Bearer " + key},
        body: formData,
    });
    if (!response.ok) {
        LogDebug("YourPlace pinning upload failed with status: " + response.status);
        return null;
    }
    const result = await response.json();
    if (result.Hash && isValidIpfsCidValue(result.Hash)) {
        return result.Hash;
    }
    LogDebug("YourPlace pinning upload returned invalid response");
    return null;
}
async function iterableToBlobArray(asyncIterable: AsyncIterable<Uint8Array>): Promise<BlobPart[]> {
    const blobParts: BlobPart[] = [];
    for await (const chunk of asyncIterable) {
        blobParts.push(new Uint8Array(chunk));
    }
    return blobParts;
}
