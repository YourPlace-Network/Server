import {hasLocalWalletEthereum, localWalletEthereumConnect} from "./blockchain/localWallet";
import {GetAddress, GetChain, GetWallet, IsValidAddress, ReconnectWallet, SetAddress, SetChain, SetWallet, WalletIsConnected, WalletSubmitComment, WalletSubmitDislike, WalletSubmitEmojiReaction, WalletSubmitLike, WalletSubmitPost} from "./blockchain/wallet";
import {LogError} from "./log";
import {HttpGetJson} from "./network";

type RedirectAction = "comment" | "dislike" | "like" | "post" | "reaction" | "redirect";
type RedirectVariable = Record<string, unknown>;

interface AuthCheckResponse {
    address?: string;
    blockchain?: string;
    status?: string;
}
interface RedirectPayload {
    action: RedirectAction;
    createdAt: number;
    variable: RedirectVariable;
}

const REDIRECT_STORAGE_KEY = "yp_redirect";
const REDIRECT_TIMEOUT_MS = 10 * 60 * 1000;
const MAX_PATH_LENGTH = 2048;
const MAX_PAYLOAD_LENGTH = 100000;
const MAX_TX_HASH_LENGTH = 128;
const MAX_EMOJI_LENGTH = 32;
const WALLET_ACTIONS: RedirectAction[] = ["comment", "dislike", "like", "post", "reaction"];

export function setRedirect(action: string, variable: RedirectVariable): boolean {
    if (!isRedirectAction(action) || !isValidVariable(action, variable)) {
        return false;
    }
    const payload: RedirectPayload = {
        action,
        createdAt: Date.now(),
        variable,
    };
    localStorage.setItem(REDIRECT_STORAGE_KEY, JSON.stringify(payload));
    return true;
}
export async function useRedirect(): Promise<boolean> {
    const payload = getStoredRedirect();
    if (!payload) {
        return false;
    }
    if (WALLET_ACTIONS.includes(payload.action) && !await ensureWalletAvailable()) {
        return false;
    }
    localStorage.removeItem(REDIRECT_STORAGE_KEY);
    try {
        switch (payload.action) {
            case "comment":
                await WalletSubmitComment(payload.variable.parentTxHash as string, payload.variable.payload as string);
                redirectBack(payload.variable.path);
                return true;
            case "dislike":
                await WalletSubmitDislike(payload.variable.targetTxHash as string, payload.variable.targetType as string);
                redirectBack(payload.variable.path);
                return true;
            case "like":
                await WalletSubmitLike(payload.variable.targetTxHash as string, payload.variable.targetType as string);
                redirectBack(payload.variable.path);
                return true;
            case "post":
                await WalletSubmitPost(payload.variable.payload as string);
                redirectBack(payload.variable.path);
                return true;
            case "reaction":
                await WalletSubmitEmojiReaction(payload.variable.targetTxHash as string, payload.variable.targetType as string, payload.variable.emoji as string);
                redirectBack(payload.variable.path);
                return true;
            case "redirect":
                window.location.replace(payload.variable.path as string);
                return true;
        }
    } catch (e) {
        LogError("Failed to use redirect: " + e);
    }
    return false;
}

async function ensureWalletAvailable(): Promise<boolean> {
    if (GetWallet() && GetAddress()) {
        return true;
    }
    const auth = await getAuthContext();
    if (!auth || !auth.address || !auth.blockchain || !IsValidAddress(auth.address, auth.blockchain)) {
        return false;
    }
    const wallet = await inferWallet(auth.blockchain, auth.address);
    if (!wallet) {
        return false;
    }
    SetWallet(wallet);
    SetChain(auth.blockchain);
    SetAddress(auth.address);
    await ReconnectWallet();
    if (!await WalletIsConnected()) {
        return false;
    }
    return GetChain() === auth.blockchain && GetAddress() === auth.address;
}
async function getAuthContext(): Promise<AuthCheckResponse | null> {
    const response = await HttpGetJson("/login/check");
    if (response[0] !== 200 || !response[1]) {
        return null;
    }
    return response[1] as AuthCheckResponse;
}
function getCurrentRelativePath(): string {
    return window.location.pathname + window.location.search + window.location.hash;
}
function getStoredRedirect(): RedirectPayload | null {
    const stored = localStorage.getItem(REDIRECT_STORAGE_KEY);
    if (!stored) {
        return null;
    }
    try {
        const payload = JSON.parse(stored) as RedirectPayload;
        if (!isRedirectAction(payload.action) || !isValidVariable(payload.action, payload.variable) || !Number.isFinite(payload.createdAt)) {
            localStorage.removeItem(REDIRECT_STORAGE_KEY);
            return null;
        }
        if ((Date.now() - payload.createdAt) > REDIRECT_TIMEOUT_MS) {
            localStorage.removeItem(REDIRECT_STORAGE_KEY);
            return null;
        }
        return payload;
    } catch (_) {
        localStorage.removeItem(REDIRECT_STORAGE_KEY);
        return null;
    }
}
async function inferWallet(blockchain: string, address: string): Promise<string | null> {
    switch (blockchain) {
        case "algorand":
            return "pera";
        case "base":
            if (hasLocalWalletEthereum()) {
                const localAddress = await localWalletEthereumConnect();
                if (localAddress === address) {
                    return "localwalletethereum";
                }
            }
            return "cbwalletbase";
        case "ethereum":
            return "metamaskethereum";
        default:
            return null;
    }
}
function isRedirectAction(action: string): action is RedirectAction {
    return ["comment", "dislike", "like", "post", "reaction", "redirect"].includes(action);
}
function isSafePath(value: unknown): value is string {
    if (typeof value !== "string" || value === "" || value.length > MAX_PATH_LENGTH) {
        return false;
    }
    if (!value.startsWith("/") || value.startsWith("//") || value.includes("\\") || /[\u0000-\u001f\u007f]/.test(value)) {
        return false;
    }
    try {
        const url = new URL(value, window.location.origin);
        return url.origin === window.location.origin;
    } catch (_) {
        return false;
    }
}
function isTargetType(value: unknown): value is string {
    return value === "comment" || value === "post";
}
function isTextPayload(value: unknown): value is string {
    return typeof value === "string" && value.trim() !== "" && value.length <= MAX_PAYLOAD_LENGTH;
}
function isTxHash(value: unknown): value is string {
    return typeof value === "string" && value.trim() !== "" && value.length <= MAX_TX_HASH_LENGTH && !/[\s/\\\u0000-\u001f\u007f]/.test(value);
}
function isValidOptionalPath(value: unknown): boolean {
    return value === undefined || isSafePath(value);
}
function isValidVariable(action: RedirectAction, variable: RedirectVariable): boolean {
    if (!variable || typeof variable !== "object" || Array.isArray(variable)) {
        return false;
    }
    switch (action) {
        case "comment":
            return isTxHash(variable.parentTxHash) && isTextPayload(variable.payload) && isValidOptionalPath(variable.path);
        case "dislike":
        case "like":
            return isTxHash(variable.targetTxHash) && isTargetType(variable.targetType) && isValidOptionalPath(variable.path);
        case "post":
            return isTextPayload(variable.payload) && isValidOptionalPath(variable.path);
        case "reaction":
            return isTxHash(variable.targetTxHash) && isTargetType(variable.targetType) && typeof variable.emoji === "string" && variable.emoji !== "" && variable.emoji.length <= MAX_EMOJI_LENGTH && isValidOptionalPath(variable.path);
        case "redirect":
            return isSafePath(variable.path);
    }
}
function redirectBack(path: unknown): void {
    if (!isSafePath(path) || path === getCurrentRelativePath()) {
        return;
    }
    window.location.replace(path);
}
