import {LogError, LogInfo} from "../log";

const PASSKEY_WALLET_KEY = "yp_passkey_wallet";
const PRF_SALT = new TextEncoder().encode("yourplace-wallet-encryption-v1");

interface PasskeyWalletStore {
    credentialId: string;
    encryptedWallets: string;
    iv: string;
    walletAddresses: { [blockchain: string]: string };
}
interface WalletDataMap {
    [blockchain: string]: string;
}

let cachedWalletData: WalletDataMap | null = null;

function arrayBufferToBase64(buffer: ArrayBuffer | Uint8Array): string {
    const bytes = buffer instanceof Uint8Array ? buffer : new Uint8Array(buffer);
    return btoa(String.fromCharCode(...bytes));
}
function base64ToArrayBuffer(base64: string): ArrayBuffer {
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
        bytes[i] = binary.charCodeAt(i);
    }
    return bytes.buffer;
}
async function deriveKeyFromPrf(prfOutput: ArrayBuffer): Promise<CryptoKey> {
    const masterKey = await crypto.subtle.importKey("raw", prfOutput, "HKDF", false, ["deriveKey"]);
    return crypto.subtle.deriveKey(
        {
            hash: "SHA-256",
            info: new TextEncoder().encode("YourPlace AES-GCM Wallet Key V1"),
            name: "HKDF",
            salt: new Uint8Array(16),
        },
        masterKey,
        {length: 256, name: "AES-GCM"},
        false,
        ["decrypt", "encrypt"]
    );
}
async function encryptWalletData(encryptionKey: CryptoKey, data: WalletDataMap): Promise<{encrypted: ArrayBuffer, iv: Uint8Array}> {
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const encrypted = await crypto.subtle.encrypt(
        {iv, name: "AES-GCM"},
        encryptionKey,
        new TextEncoder().encode(JSON.stringify(data))
    );
    return {encrypted, iv};
}
function getStoredData(): PasskeyWalletStore | null {
    const stored = localStorage.getItem(PASSKEY_WALLET_KEY);
    if (!stored) return null;
    try {
        return JSON.parse(stored) as PasskeyWalletStore;
    } catch (e) {
        return null;
    }
}
async function getPrfOutput(credentialId: ArrayBuffer): Promise<ArrayBuffer | null> {
    const challenge = crypto.getRandomValues(new Uint8Array(32));
    const getOptions: CredentialRequestOptions = {
        publicKey: {
            allowCredentials: [{id: credentialId, type: "public-key"}],
            challenge,
            extensions: {prf: {eval: {first: PRF_SALT}}} as AuthenticationExtensionsClientInputs,
            rpId: window.location.hostname,
            userVerification: "required",
        }
    };
    try {
        const assertion = await navigator.credentials.get(getOptions) as PublicKeyCredential;
        if (!assertion) return null;
        const results = (assertion.getClientExtensionResults() as any)?.prf?.results;
        if (!results?.first) {
            LogError("PRF output not available in assertion");
            return null;
        }
        return results.first;
    } catch (e) {
        LogError("Failed to get passkey assertion: " + e);
        return null;
    }
}

export function getPasskeyWalletAddress(blockchain: string): string | null {
    const data = getStoredData();
    if (!data) return null;
    return data.walletAddresses[blockchain] || null;
}
export function getPasskeyWalletCachedData(blockchain: string): string | null {
    if (!cachedWalletData) return null;
    return cachedWalletData[blockchain] || null;
}
export function hasPasskeyWallet(blockchain?: string): boolean {
    const data = getStoredData();
    if (!data) return false;
    if (blockchain) {
        return !!data.walletAddresses[blockchain];
    }
    return true;
}
export async function isPasskeyPrfSupported(): Promise<boolean> {
    if (!window.PublicKeyCredential) return false;
    try {
        const available = await PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable();
        if (!available) return false;
        if (typeof (PublicKeyCredential as any).getClientCapabilities === "function") {
            const caps = await (PublicKeyCredential as any).getClientCapabilities();
            if (caps?.extensions && !caps.extensions.includes("prf")) {
                return false;
            }
        }
        return true;
    } catch (e) {
        LogInfo("Passkey PRF support check failed: " + e);
        return false;
    }
}
export async function passkeyWalletAdd(blockchain: string, walletDataJson: string, address: string): Promise<boolean> {
    const existingData = getStoredData();
    if (!existingData) {
        return passkeyWalletCreate(blockchain, walletDataJson, address);
    }
    const credentialId = base64ToArrayBuffer(existingData.credentialId);
    const prfOutput = await getPrfOutput(credentialId);
    if (!prfOutput) {
        LogError("Failed to get PRF output for adding wallet");
        return false;
    }
    try {
        const encryptionKey = await deriveKeyFromPrf(prfOutput);
        const iv = new Uint8Array(base64ToArrayBuffer(existingData.iv));
        const encrypted = new Uint8Array(base64ToArrayBuffer(existingData.encryptedWallets));
        const decrypted = await crypto.subtle.decrypt({iv, name: "AES-GCM"}, encryptionKey, encrypted);
        const wallets: WalletDataMap = JSON.parse(new TextDecoder().decode(decrypted));
        wallets[blockchain] = walletDataJson;
        const newEncrypted = await encryptWalletData(encryptionKey, wallets);
        const newAddresses = {...existingData.walletAddresses, [blockchain]: address};
        const newStoreData: PasskeyWalletStore = {
            credentialId: existingData.credentialId,
            encryptedWallets: arrayBufferToBase64(newEncrypted.encrypted),
            iv: arrayBufferToBase64(newEncrypted.iv),
            walletAddresses: newAddresses,
        };
        localStorage.setItem(PASSKEY_WALLET_KEY, JSON.stringify(newStoreData));
        cachedWalletData = wallets;
        LogInfo("Wallet added to passkey store for blockchain: " + blockchain);
        return true;
    } catch (e) {
        LogError("Failed to add wallet to passkey store: " + e);
        return false;
    }
}
export function passkeyWalletClearCache(): void {
    cachedWalletData = null;
}
export async function passkeyWalletCreate(blockchain: string, walletDataJson: string, address: string): Promise<boolean> {
    const challenge = crypto.getRandomValues(new Uint8Array(32));
    const userId = crypto.getRandomValues(new Uint8Array(16));
    const createOptions: CredentialCreationOptions = {
        publicKey: {
            authenticatorSelection: {
                authenticatorAttachment: "platform",
                residentKey: "required",
                userVerification: "required",
            },
            challenge,
            extensions: {prf: {}} as AuthenticationExtensionsClientInputs,
            pubKeyCredParams: [
                {alg: -7, type: "public-key"},
                {alg: -257, type: "public-key"},
            ],
            rp: {id: window.location.hostname, name: "YourPlace"},
            user: {
                displayName: "YourPlace Wallet",
                id: userId,
                name: `wallet-${address.slice(0, 8)}`,
            },
        }
    };
    try {
        const credential = await navigator.credentials.create(createOptions) as PublicKeyCredential;
        if (!credential) {
            LogError("Failed to create passkey credential");
            return false;
        }
        const prfEnabled = (credential.getClientExtensionResults() as any)?.prf?.enabled;
        if (!prfEnabled) {
            LogError("PRF extension not supported by authenticator");
            return false;
        }
        const prfOutput = await getPrfOutput(credential.rawId);
        if (!prfOutput) {
            LogError("Failed to get PRF output after credential creation");
            return false;
        }
        const encryptionKey = await deriveKeyFromPrf(prfOutput);
        const wallets: WalletDataMap = {[blockchain]: walletDataJson};
        const {encrypted, iv} = await encryptWalletData(encryptionKey, wallets);
        const passkeyData: PasskeyWalletStore = {
            credentialId: arrayBufferToBase64(credential.rawId),
            encryptedWallets: arrayBufferToBase64(encrypted),
            iv: arrayBufferToBase64(iv),
            walletAddresses: {[blockchain]: address},
        };
        localStorage.setItem(PASSKEY_WALLET_KEY, JSON.stringify(passkeyData));
        cachedWalletData = wallets;
        LogInfo("Passkey wallet created successfully for blockchain: " + blockchain);
        return true;
    } catch (e) {
        LogError("Failed to create passkey wallet: " + e);
        return false;
    }
}
export function passkeyWalletDelete(blockchain?: string): void {
    if (!blockchain) {
        localStorage.removeItem(PASSKEY_WALLET_KEY);
        cachedWalletData = null;
        return;
    }
    const data = getStoredData();
    if (!data) return;
    delete data.walletAddresses[blockchain];
    if (cachedWalletData) {
        delete cachedWalletData[blockchain];
    }
    if (Object.keys(data.walletAddresses).length === 0) {
        localStorage.removeItem(PASSKEY_WALLET_KEY);
        cachedWalletData = null;
    } else {
        localStorage.setItem(PASSKEY_WALLET_KEY, JSON.stringify(data));
    }
}
export async function passkeyWalletUnlock(blockchain?: string): Promise<string | null> {
    if (cachedWalletData) {
        if (blockchain) {
            return cachedWalletData[blockchain] || null;
        }
        const keys = Object.keys(cachedWalletData);
        return keys.length > 0 ? cachedWalletData[keys[0]] : null;
    }
    const stored = getStoredData();
    if (!stored) {
        LogError("No passkey wallet found");
        return null;
    }
    try {
        const credentialId = base64ToArrayBuffer(stored.credentialId);
        const prfOutput = await getPrfOutput(credentialId);
        if (!prfOutput) {
            return null;
        }
        const encryptionKey = await deriveKeyFromPrf(prfOutput);
        const iv = new Uint8Array(base64ToArrayBuffer(stored.iv));
        const encrypted = new Uint8Array(base64ToArrayBuffer(stored.encryptedWallets));
        const decrypted = await crypto.subtle.decrypt(
            {iv, name: "AES-GCM"},
            encryptionKey,
            encrypted
        );
        cachedWalletData = JSON.parse(new TextDecoder().decode(decrypted));
        LogInfo("Passkey wallet unlocked successfully");
        if (blockchain) {
            return cachedWalletData![blockchain] || null;
        }
        const keys = Object.keys(cachedWalletData!);
        return keys.length > 0 ? cachedWalletData![keys[0]] : null;
    } catch (e) {
        LogError("Failed to unlock passkey wallet: " + e);
        return null;
    }
}
