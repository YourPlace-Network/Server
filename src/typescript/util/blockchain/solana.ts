import {LogError, LogInfo} from "../log";
import {GetAddress, SetAddress} from "./wallet";

let solanaInit = false;
let solanaProvider: any = null;

async function initSolanaWallet() {
    if (solanaInit) { return; }
    if (!await solanaPhantomWalletExists()) {
        solanaInit = true;
        return;
    }
    solanaProvider = phantomSolanaGetProvider();
    solanaInit = true;
}
initSolanaWallet().then();

// ---------- Core Wallet Functions ---------- //
export async function phantomSolanaAuthLogin(): Promise<string> {
    LogInfo("Authenticating Solana wallet - TODO"); // todo
    return "";
}
export async function phantomSolanaConnectWallet(): Promise<string> {
    if (!await solanaPhantomWalletExists()) {
        LogError("No Solana wallet detected");
        window.open("https://phantom.app/", "_blank");
        return "";
    }
    if (!solanaProvider) {
        solanaProvider = phantomSolanaGetProvider();
    }
    if (!solanaProvider) {
        LogError("No Solana wallet detected");
        return "";
    }
    if (await solanaIsWalletConnected()) {
        LogInfo("Solana wallet already connected");
        return "";
    }
    try {
        const response = await solanaProvider.connect({onlyIfTrusted: true});
        return response as string;
    } catch (err) {
        return "";
    }
}
export async function solanaDisconnectWallet() {
    if (!solanaProvider) { return; }
    solanaProvider.disconnect();
}
export async function solanaIsWalletConnected(): Promise<boolean> {
    return Boolean(solanaProvider?.isConnected);
}
export async function phantomSolanaTxn(dest: string, payload: string) {
    let address = GetAddress();
    if (!address) {
        LogError("No Solana wallet address found");
        return;
    }
    const dataBuffer = Buffer.from(payload, "utf-8");
    //const transaction = new
}

// ---------- Phantom Wallet Functions ---------- //
export async function solanaPhantomWalletExists(): Promise<boolean> {
    if ("phantom" in window) {
        // @ts-ignore
        return Boolean(window.phantom?.solana?.isPhantom);
    }
    return false;
}
function phantomSolanaGetProvider(): any {
    if ("phantom" in window) {
        // @ts-ignore
        const provider = window.phantom?.solana;
        if (provider?.isPhantom) {
            return provider;
        }
    }
    return null;
}

// ---------- Network Functions ---------- //
function phantomSolanaChangeAddress(publicKey: string) {
    if (publicKey) {
        SetAddress(publicKey);
    } else {
        phantomSolanaConnectWallet().then();
    }
}

//solanaProvider?.on("accountChanged", phantomSolanaChangeAddress);
