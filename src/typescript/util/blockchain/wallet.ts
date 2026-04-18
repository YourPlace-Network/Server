/* This file contains all the functions related to the wallet.
It acts as a sort of logical "firewall" between individual blockchain implementations and the core business requirements
for the application. This code is stateful using localstorage to keep a few values:
    "walletSelection" = text base name of the wallet selected by the user
    "accountAddress" = wallet address of the user
*/
import {
    algoAuthLogin,
    algoBurnCollectible,
    algoConnectWallet,
    algoDisconnectWallet,
    algoFollowUser,
    algoGetAvatar,
    algoGetCollectibles,
    algoGetName,
    algoGetTransferFeeEstimate,
    algoMintCollectible,
    algoReconnectSession,
    algoSetBanner,
    algoSetBot,
    algoSetColors,
    algoSetDescription,
    algoSetLocation,
    algoSetMusicEmbed,
    algoSetName,
    algoSetNsfw,
    algoSetVertical,
    algoSetWebsite,
    algoSubmitComment,
    algoSubmitCommentAttach,
    algoSubmitDislike,
    algoSubmitEmojiReaction,
    algoSubmitLike,
    algoTransferCollectible,
    algoUnfollowUser,
    peraWallet,
    setAlgoAvatar,
    algoSubmitPost,
    setAlgoPostAttach,
} from "./algorand";
import {
    baseAuthLogin,
    baseBurnCollectible,
    baseConnectWallet,
    baseDisconnectWallet,
    baseFollowUser,
    baseGetAvatar,
    baseGetCollectibles,
    baseGetDescription,
    baseIsWalletConnected,
    baseGetName,
    baseReconnectWallet,
    baseGetTransferFeeEstimate,
    baseMintCollectible,
    baseSetAvatar,
    baseSetBanner,
    baseSetBot,
    baseSetColors,
    baseSetDescription,
    baseSetLocation,
    baseSetMusicEmbed,
    baseSetName,
    baseSetNsfw,
    baseSetVertical,
    baseSetWebsite,
    baseSubmitComment,
    baseSubmitCommentAttach,
    baseSubmitDislike,
    baseSubmitEmojiReaction,
    baseSubmitLike,
    baseSubmitPost,
    baseSubmitPostAttach,
    baseTransferCollectible,
    baseTxn,
    baseUnfollowUser,
    mainnetBase,
} from "./base";
import {
    ethereumAuthLogin,
    ethereumBurnCollectible,
    ethereumConnectWallet,
    ethereumDisconnectWallet,
    ethereumFollowUser,
    ethereumGetAvatar,
    ethereumGetCollectibles,
    ethereumGetDescription,
    ethereumIsWalletConnected,
    ethereumGetName,
    ethereumReconnectWallet,
    ethereumGetTransferFeeEstimate,
    ethereumMintCollectible,
    ethereumSetAvatar,
    ethereumSetBanner,
    ethereumSetBot,
    ethereumSetColors,
    ethereumSetDescription,
    ethereumSetLocation,
    ethereumSetMusicEmbed,
    ethereumSetName,
    ethereumSetNsfw,
    ethereumSetVertical,
    ethereumSetWebsite,
    ethereumSubmitComment,
    ethereumSubmitCommentAttach,
    ethereumSubmitDislike,
    ethereumSubmitEmojiReaction,
    ethereumSubmitLike,
    ethereumSubmitPost,
    ethereumSubmitPostAttach,
    ethereumTransferCollectible,
    ethereumTxn,
    ethereumUnfollowUser,
    mainnetEth,
} from "./ethereum";
import {
    hasLocalWalletEthereum,
    localWalletEthereumAuthLogin,
    localWalletEthereumBurnCollectible,
    localWalletEthereumConnect,
    localWalletEthereumDisconnect,
    localWalletEthereumFollowUser,
    localWalletEthereumGetCollectibles,
    localWalletEthereumMintCollectible,
    localWalletEthereumReconnect,
    localWalletEthereumSetAvatar,
    localWalletEthereumSetBanner,
    localWalletEthereumSetBot,
    localWalletEthereumSetColors,
    localWalletEthereumSetDescription,
    localWalletEthereumSetLocation,
    localWalletEthereumSetMusicEmbed,
    localWalletEthereumSetName,
    localWalletEthereumSetNsfw,
    localWalletEthereumSetVertical,
    localWalletEthereumSetWebsite,
    localWalletEthereumSubmitComment,
    localWalletEthereumSubmitCommentAttach,
    localWalletEthereumSubmitDislike,
    localWalletEthereumSubmitEmojiReaction,
    localWalletEthereumSubmitLike,
    localWalletEthereumSubmitPost,
    localWalletEthereumSubmitPostAttach,
    localWalletEthereumTransferCollectible,
    localWalletEthereumTxn,
    localWalletEthereumUnfollowUser,
} from "./localWallet";
import {PersistentCache} from "../cache";
import {CIDToSubdomainURL} from "../ipfs";
import {HttpPostJson} from "../network";
import {IsValidAlgoAddress, IsValidBaseAddress, IsValidURL} from "../security";
import {LogError, LogInfo} from "../log";
import {phantomSolanaAuthLogin, phantomSolanaConnectWallet, solanaDisconnectWallet} from "./solana";
import {ShowDialogModal, ShowDialogModalHTML} from "../../components/modalDialog";

// ---------- Types ---------- //
export interface CollectibleData {
    blockchain: string;
    contractAddress: string;
    creator: string;
    description: string;
    imageUrl: string;
    mimeType: string;
    name: string;
    tokenId: string;
}

// ---------- Request Deduplication ---------- //
const inflight = new Map<string, Promise<any>>();
function Dedup<T>(key: string, fn: () => Promise<T>): Promise<T> {
    const existing = inflight.get(key);
    if (existing) {
        return existing as Promise<T>;
    }
    const promise = fn().finally(() => {
        inflight.delete(key);
    });
    inflight.set(key, promise);
    return promise;
}

// ---------- Cached Profile Lookups ---------- //
const cachedAvatars: Record<string, PersistentCache> = {
    "algorand": new PersistentCache("algo_avatar"),
    "base": new PersistentCache("base_avatar"),
    "ethereum": new PersistentCache("ethereum_avatar"),
};
const cachedNames: Record<string, PersistentCache> = {
    "algorand": new PersistentCache("algo_name"),
    "base": new PersistentCache("base_name"),
    "ethereum": new PersistentCache("ethereum_name"),
};
export function WalletGetCachedAvatar(chain: string, address: string): string | null {
    return cachedAvatars[chain]?.get<string>(address) ?? null;
}
export function WalletGetCachedName(chain: string, address: string): string | null {
    return cachedNames[chain]?.get<string>(address) ?? null;
}

// ---------- Connection ---------- //
export async function WalletLogin() {
    let wallet = GetWallet();
    let address = GetAddress();
    switch (wallet) {
        case "localwalletethereum":
            let localLoginStatus = await localWalletEthereumAuthLogin();
            if (localLoginStatus !== "success") {
                LogError("Failed to login with local wallet: " + localLoginStatus);
                return "";
            }
            return localLoginStatus;
        case "metamaskethereum":
            let ethLoginStatus = await ethereumAuthLogin();
            if (ethLoginStatus !== "success") {
                LogError("Failed to login to Ethereum wallet: " + ethLoginStatus);
                return "";
            }
            return ethLoginStatus;
        case "pera":
            if (!address) {
                LogError("No address found for Pera wallet login");
                return "";
            }
            return await algoAuthLogin(address);
        case "cbwalletbase":
            let loginStatus = await baseAuthLogin();
            if (loginStatus === "wallet_not_deployed") {
                return "wallet_not_deployed";
            }
            if (loginStatus === "popup_blocked") {
                ShowDialogModal("Your browser blocked the wallet popup. Please allow popups for this site and try again.");
                return "popup_blocked";
            }
            if (loginStatus !== "success") {
                LogError("Failed to login to Base wallet: " + loginStatus);
                return "";
            }
            return loginStatus;
        case "phantomsolana":
            return await phantomSolanaAuthLogin();
        case null:
            LogError("No wallet selected");
            break;
        default:
            LogError("Invalid wallet selection");
            break;
    }
    LogError("Failed to login to wallet");
    return "";
}
export async function DisconnectWallet() {
    let wallet = GetWallet()!;
    switch (wallet) {
        case "cbwalletbase":
            await baseDisconnectWallet();
            break;
        case "localwalletethereum":
            break;
        case "metamaskethereum":
            await ethereumDisconnectWallet();
            break;
        case "pera":
            await algoDisconnectWallet();
            break;
        case "phantomsolana":
            await solanaDisconnectWallet();
            break;
    }
    SetWallet("");
    SetChain("");
    SetAddress("");
    const localWallets: Record<string, string> = {};
    for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i);
        if (key?.startsWith("yp_local_wallet_")) {
            localWallets[key] = localStorage.getItem(key)!;
        }
    }
    localStorage.clear();
    for (const [key, value] of Object.entries(localWallets)) {
        localStorage.setItem(key, value);
    }
    window.DisconnectWalletCallback();
}
export async function ConnectWallet(wallet: string): Promise<string> {
    switch (wallet) {
        case "cbwalletbase":
            LogInfo("Connecting to Base wallet");
            let addressBase = await baseConnectWallet();
            if (!addressBase || addressBase === "") {
                LogError("Base wallet returned empty address");
                SetWallet("");
                SetChain("");
                SetAddress("");
                return "Failed to connect to Base wallet: Empty address";
            }
            let validAddress = IsValidBaseAddress(addressBase);
            if (!validAddress) {
                LogError("Failed to connect to Base wallet: Connected address invalid");
                SetWallet("");
                SetChain("");
                SetAddress("");
                return "Failed to connect to Base wallet 1: Connected address invalid";
            }
            SetWallet("cbwalletbase");
            SetChain("base");
            SetAddress(addressBase);
            return "success";
        case "localwalletethereum":
            LogInfo("Connecting to local Ethereum wallet");
            let addressLocal = await localWalletEthereumConnect();
            if (!addressLocal || addressLocal === "") {
                LogError("Local wallet returned empty address");
                SetWallet("");
                SetChain("");
                SetAddress("");
                return "Failed to connect to local wallet: Empty address";
            }
            if (!IsValidBaseAddress(addressLocal)) {
                LogError("Failed to connect to local wallet: Invalid address");
                SetWallet("");
                SetChain("");
                SetAddress("");
                return "Failed to connect to local wallet: Invalid address";
            }
            SetWallet("localwalletethereum");
            SetChain("base");
            SetAddress(addressLocal);
            return "success";
        case "metamaskethereum":
            LogInfo("Connecting to MetaMask Ethereum wallet");
            let addressEth = await ethereumConnectWallet();
            if (!addressEth || addressEth === "") {
                LogError("MetaMask returned empty address");
                SetWallet("");
                SetChain("");
                SetAddress("");
                return "Failed to connect to MetaMask wallet: Empty address";
            }
            if (!IsValidBaseAddress(addressEth)) {
                LogError("Failed to connect to MetaMask wallet: Invalid address");
                SetWallet("");
                SetChain("");
                SetAddress("");
                return "Failed to connect to MetaMask wallet: Invalid address";
            }
            SetWallet("metamaskethereum");
            SetChain("ethereum");
            SetAddress(addressEth);
            return "success";
        case "pera":
            LogInfo("Connecting to Pera wallet");
            let address = await algoConnectWallet("pera");
            if (!address || address === "") {
                LogError("Pera wallet returned empty address");
                SetWallet("");
                SetChain("");
                SetAddress("");
                return "Failed to connect to Pera wallet: Empty address";
            }
            if (IsValidAlgoAddress(address)) {
                SetWallet("pera");
                SetChain("algorand");
                SetAddress(address);
                return "success";
            }
            LogError("Failed to connect to Pera wallet: Invalid address");
            return "Failed to connect to Pera wallet: Invalid address";
        case "phantomsolana":
            let addressSolana = await phantomSolanaConnectWallet();
            if (!addressSolana || addressSolana === "") {
                SetWallet("phantomsolana");
                SetChain("solana");
                SetAddress(addressSolana);
            } else {
                LogError("Failed to connect to Phantom wallet");
                return "Failed to connect to Phantom wallet on Solana";
            }
            break;
        default:
            LogError("Invalid wallet selection");
            break;
    }
    return "Wallet connect failed";
}
export async function ReconnectWallet() {
    let wallet = GetWallet();
    let address = GetAddress();
    if (!wallet || !address) {
        // No wallet or address stored - nothing to reconnect (this is normal on first visit)
        return;
    }
    switch (wallet) {
        case "cbwalletbase":
            // Only reconnect if we have stored credentials - don't prompt for new connection
            const wagmiStore = localStorage.getItem("yourplace.store");
            if (wagmiStore) {
                await baseReconnectWallet();
            }
            break;
        case "localwalletethereum":
            await localWalletEthereumReconnect();
            break;
        case "metamaskethereum":
            const ethWagmiStore = localStorage.getItem("yourplace_ethereum.store");
            if (ethWagmiStore) {
                await ethereumReconnectWallet();
            }
            break;
        case "pera":
            await algoReconnectSession();
            break;
    }
}

// ---------- Getters ---------- //
export function GetAddress() {
    let address = localStorage.getItem("accountAddress");
    if (address !== null) {
        return address;
    }
    return null;
}
export function GetWallet() {
    const supportedWallets = ["cbwalletbase", "localwalletethereum", "metamaskethereum", "pera", "phantomsolana"];
    let wallet = localStorage.getItem("walletSelection");
    if (wallet !== null && supportedWallets.includes(wallet)) {
        return wallet;
    }
    return null
}
export function GetChain() {
    const supportedChains = ["algorand", "base", "ethereum", "solana"];
    let chain = localStorage.getItem("blockchain");
    if (chain !== null && supportedChains.includes(chain)) {
        return chain;
    }
    return null;
}
export async function WalletGetAvatar(chain?: string, address?: string): Promise<string> {
    if (!chain) chain = GetChain()!;
    if (!address) address = GetAddress()!;
    return Dedup(chain + ":avatar:" + address, async () => {
        let avatar;
        switch (chain) {
            case "algorand":
                avatar = await algoGetAvatar(address!);
                break;
            case "base":
                avatar = await baseGetAvatar(address!);
                break;
            case "ethereum":
                avatar = await ethereumGetAvatar(address!);
                break;
        }
        if (avatar) {
            if (avatar.startsWith("ipfs://")) {
                return CIDToSubdomainURL(avatar) || "";
            }
            if (IsValidURL(avatar)) {
                return avatar;
            }
        }
        return "";
    });
}
export async function WalletGetName(chain: string, address: string): Promise<string|null> {
    return Dedup(chain + ":name:" + address, async () => {
        let name;
        switch (chain) {
            case "algorand":
                name = await algoGetName(address);
                break;
            case "base":
                name = await baseGetName(address);
                break;
            case "ethereum":
                name = await ethereumGetName(address);
                break;
        }
        if (name) {
            return name;
        }
        return "";
    });
}
export async function WalletGetDescription(chain?: string, address?: string): Promise<string|null> {
    if (!chain) chain = GetChain()!;
    if (!address) address = GetAddress()!;
    return Dedup(chain + ":description:" + address, async () => {
        let description;
        switch (chain) {
            case "algorand":
                return null;
            case "base":
                description = await baseGetDescription(address!);
                break;
            case "ethereum":
                description = await ethereumGetDescription(address!);
                break;
        }
        if (description) {
            return description;
        }
        return null;
    });
}
export function WalletGetExplorerAddressLink(address: string, blockchain?: string) {
    let chain = blockchain || GetChain();
    if (chain == "algorand") {
        return `https://allo.info/account/${address}`;
    } else if (chain == "base") {
        return mainnetBase.explorerUrl + "/address/" + address;
    } else if (chain == "ethereum") {
        return mainnetEth.explorerUrl + "/address/" + address;
    }
    return "";
}
export function WalletGetExplorerTxLink(tx: string, blockchain?: string) {
    if (tx == "") {
        return "";
    }
    let chain = blockchain || GetChain();
    if (chain == "algorand") {
        return `https://allo.info/tx/${tx}`;
    } else if (chain == "base") {
        return mainnetBase.explorerUrl + "/tx/" + tx;
    } else if (chain == "ethereum") {
        return mainnetEth.explorerUrl + "/tx/" + tx;
    }
    return "";
}
export function WalletGetYourPlaceAddressLink(address: string) {
    let chain = GetChain();
    if (chain == "algorand") {
        return `/p/algorand/${address}`;
    } else if (chain == "base") {
        return `/p/base/${address}`;
    } else if (chain == "ethereum") {
        return `/p/ethereum/${address}`;
    } else if (chain == "solana") {
        return `/p/solana/${address}`;
    } else {
        return `/p/${address}`;
    }
}

// ---------- Setters ---------- //
export function SetWallet(wallet: string) {
    if (wallet === "") {
        localStorage.removeItem("walletSelection");
    } else {
        localStorage.setItem("walletSelection", wallet);
    }
}
export function SetAddress(address: string) {
    if (address === "") {
        localStorage.removeItem("accountAddress");
    } else {
        localStorage.setItem("accountAddress", address);
    }
}
export function SetChain(chain: string) {
    if (chain === "") {
        localStorage.removeItem("blockchain");
    } else {
        localStorage.setItem("blockchain", chain);
    }
}
export async function WalletSetAvatar(avatarURL: string): Promise<boolean> {
    if (!IsValidURL(avatarURL)) {
        return false;
    }
    let walletSelection = GetWallet()!;
    switch (walletSelection) {
        case "cbwalletbase":
            return !!await baseSetAvatar(avatarURL);
        case "localwalletethereum":
            return !!await localWalletEthereumSetAvatar(avatarURL);
        case "metamaskethereum":
            return !!await ethereumSetAvatar(avatarURL);
        case "pera":
            await setAlgoAvatar(avatarURL);
            return true;
    }
    return false;
}
export async function WalletSetBanner(bannerURL: string): Promise<boolean> {
    if (!IsValidURL(bannerURL)) {
        return false;
    }
    let walletSelection = GetWallet()!;
    switch (walletSelection) {
        case "cbwalletbase":
            return !!await baseSetBanner(bannerURL);
        case "localwalletethereum":
            return !!await localWalletEthereumSetBanner(bannerURL);
        case "metamaskethereum":
            return !!await ethereumSetBanner(bannerURL);
        case "pera":
            return await algoSetBanner(bannerURL);
    }
    return false;
}
export async function WalletSetColors(colors: Record<string, string>): Promise<boolean> {
    if (!colors || Object.keys(colors).length === 0) {
        return false;
    }
    let walletSelection = GetWallet()!;
    switch (walletSelection) {
        case "cbwalletbase":
            return !!await baseSetColors(colors);
        case "localwalletethereum":
            return !!await localWalletEthereumSetColors(colors);
        case "metamaskethereum":
            return !!await ethereumSetColors(colors);
        case "pera":
            return await algoSetColors(colors);
    }
    return false;
}
export async function WalletSetDescription(description: string): Promise<boolean> {
    let walletSelection = GetWallet()!;
    switch (walletSelection) {
        case "cbwalletbase":
            return !!await baseSetDescription(description);
        case "localwalletethereum":
            return !!await localWalletEthereumSetDescription(description);
        case "metamaskethereum":
            return !!await ethereumSetDescription(description);
        case "pera":
            return await algoSetDescription(description);
    }
    return false;
}
export async function WalletSetLocation(location: string): Promise<boolean> {
    let walletSelection = GetWallet()!;
    switch (walletSelection) {
        case "cbwalletbase":
            return !!await baseSetLocation(location);
        case "localwalletethereum":
            return !!await localWalletEthereumSetLocation(location);
        case "metamaskethereum":
            return !!await ethereumSetLocation(location);
        case "pera":
            return await algoSetLocation(location);
    }
    return false;
}
export async function WalletSetBot(bot: boolean): Promise<boolean> {
    let walletSelection = GetWallet()!;
    switch (walletSelection) {
        case "cbwalletbase":
            return !!await baseSetBot(bot);
        case "localwalletethereum":
            return !!await localWalletEthereumSetBot(bot);
        case "metamaskethereum":
            return !!await ethereumSetBot(bot);
        case "pera":
            return await algoSetBot(bot);
    }
    return false;
}
export async function WalletSetNsfw(nsfw: boolean): Promise<boolean> {
    let walletSelection = GetWallet()!;
    switch (walletSelection) {
        case "cbwalletbase":
            return !!await baseSetNsfw(nsfw);
        case "localwalletethereum":
            return !!await localWalletEthereumSetNsfw(nsfw);
        case "metamaskethereum":
            return !!await ethereumSetNsfw(nsfw);
        case "pera":
            return await algoSetNsfw(nsfw);
    }
    return false;
}
export async function WalletSetVertical(vertical: string): Promise<boolean> {
    let walletSelection = GetWallet()!;
    switch (walletSelection) {
        case "cbwalletbase":
            return !!await baseSetVertical(vertical);
        case "localwalletethereum":
            return !!await localWalletEthereumSetVertical(vertical);
        case "metamaskethereum":
            return !!await ethereumSetVertical(vertical);
        case "pera":
            return await algoSetVertical(vertical);
    }
    return false;
}
export async function WalletSetMusicEmbed(music: string): Promise<boolean> {
    let walletSelection = GetWallet()!;
    switch (walletSelection) {
        case "cbwalletbase":
            return !!await baseSetMusicEmbed(music);
        case "localwalletethereum":
            return !!await localWalletEthereumSetMusicEmbed(music);
        case "metamaskethereum":
            return !!await ethereumSetMusicEmbed(music);
        case "pera":
            return await algoSetMusicEmbed(music);
    }
    return false;
}
export async function WalletSetWebsite(website: string): Promise<boolean> {
    let walletSelection = GetWallet()!;
    switch (walletSelection) {
        case "cbwalletbase":
            return !!await baseSetWebsite(website);
        case "localwalletethereum":
            return !!await localWalletEthereumSetWebsite(website);
        case "metamaskethereum":
            return !!await ethereumSetWebsite(website);
        case "pera":
            return await algoSetWebsite(website);
    }
    return false;
}
export async function WalletSetName(name: string): Promise<boolean> {
    let wallet = GetWallet()!;
    switch (wallet) {
        case "cbwalletbase":
            return !!await baseSetName(name);
        case "localwalletethereum":
            return !!await localWalletEthereumSetName(name);
        case "metamaskethereum":
            return !!await ethereumSetName(name);
        case "pera":
            return await algoSetName(name);
        default:
            return false;
    }
}
export async function WalletSubmitPost(payload: string): Promise<boolean> {
    let wallet = GetWallet();
    if (!wallet) {
        LogError("No wallet connected - cannot submit post");
        return false;
    }
    const isConnected = await WalletIsConnected();
    if (!isConnected) {
        LogInfo("Wallet session expired - attempting to reconnect");
        await ReconnectWallet();
    }
    switch (wallet) {
        case "cbwalletbase":
            await baseSubmitPost(payload);
            return true;
        case "localwalletethereum":
            await localWalletEthereumSubmitPost(payload);
            return true;
        case "metamaskethereum":
            await ethereumSubmitPost(payload);
            return true;
        case "pera":
            await algoSubmitPost(payload);
            return true;
        default:
            LogError("Invalid wallet selection: " + wallet);
            return false;
    }
}
export async function WalletSubmitPostAttach(payload: string, attach: string[][]): Promise<boolean> {
    let wallet = GetWallet();
    if (!wallet) {
        LogError("No wallet connected - cannot submit post with attachments");
        return false;
    }
    const isConnected = await WalletIsConnected();
    if (!isConnected) {
        LogInfo("Wallet session expired - attempting to reconnect");
        await ReconnectWallet();
    }
    switch (wallet) {
        case "cbwalletbase":
            await baseSubmitPostAttach(payload, attach);
            return true;
        case "localwalletethereum":
            await localWalletEthereumSubmitPostAttach(payload, attach);
            return true;
        case "metamaskethereum":
            await ethereumSubmitPostAttach(payload, attach);
            return true;
        case "pera":
            return await setAlgoPostAttach(payload, attach);
        default:
            LogError("Invalid wallet selection: " + wallet);
            return false;
    }
}
export async function WalletSubmitComment(parentTxHash: string, payload: string): Promise<boolean> {
    let wallet = GetWallet();
    if (!wallet) {
        LogError("No wallet connected - cannot submit comment");
        return false;
    }
    const isConnected = await WalletIsConnected();
    if (!isConnected) {
        LogInfo("Wallet session expired - attempting to reconnect");
        await ReconnectWallet();
    }
    switch (wallet) {
        case "cbwalletbase":
            await baseSubmitComment(parentTxHash, payload);
            return true;
        case "localwalletethereum":
            await localWalletEthereumSubmitComment(parentTxHash, payload);
            return true;
        case "metamaskethereum":
            await ethereumSubmitComment(parentTxHash, payload);
            return true;
        case "pera":
            return await algoSubmitComment(parentTxHash, payload);
        default:
            LogError("Invalid wallet selection: " + wallet);
            return false;
    }
}
export async function WalletSubmitCommentAttach(parentTxHash: string, payload: string, attach: string[][]): Promise<boolean> {
    let wallet = GetWallet();
    if (!wallet) {
        LogError("No wallet connected - cannot submit comment with attachments");
        return false;
    }
    const isConnected = await WalletIsConnected();
    if (!isConnected) {
        LogInfo("Wallet session expired - attempting to reconnect");
        await ReconnectWallet();
    }
    switch (wallet) {
        case "cbwalletbase":
            await baseSubmitCommentAttach(parentTxHash, payload, attach);
            return true;
        case "localwalletethereum":
            await localWalletEthereumSubmitCommentAttach(parentTxHash, payload, attach);
            return true;
        case "metamaskethereum":
            await ethereumSubmitCommentAttach(parentTxHash, payload, attach);
            return true;
        case "pera":
            return await algoSubmitCommentAttach(parentTxHash, payload, attach);
        default:
            LogError("Invalid wallet selection: " + wallet);
            return false;
    }
}
export async function WalletSubmitLike(targetTxHash: string, targetType: string): Promise<boolean> {
    let wallet = GetWallet();
    if (!wallet) {
        LogError("No wallet connected - cannot submit like");
        return false;
    }
    const isConnected = await WalletIsConnected();
    if (!isConnected) {
        LogInfo("Wallet session expired - attempting to reconnect");
        await ReconnectWallet();
    }
    switch (wallet) {
        case "cbwalletbase":
            await baseSubmitLike(targetTxHash, targetType);
            return true;
        case "localwalletethereum":
            await localWalletEthereumSubmitLike(targetTxHash, targetType);
            return true;
        case "metamaskethereum":
            await ethereumSubmitLike(targetTxHash, targetType);
            return true;
        case "pera":
            return await algoSubmitLike(targetTxHash, targetType);
        default:
            LogError("Invalid wallet selection: " + wallet);
            return false;
    }
}
export async function WalletSubmitDislike(targetTxHash: string, targetType: string): Promise<boolean> {
    let wallet = GetWallet();
    if (!wallet) {
        LogError("No wallet connected - cannot submit dislike");
        return false;
    }
    const isConnected = await WalletIsConnected();
    if (!isConnected) {
        LogInfo("Wallet session expired - attempting to reconnect");
        await ReconnectWallet();
    }
    switch (wallet) {
        case "cbwalletbase":
            await baseSubmitDislike(targetTxHash, targetType);
            return true;
        case "localwalletethereum":
            await localWalletEthereumSubmitDislike(targetTxHash, targetType);
            return true;
        case "metamaskethereum":
            await ethereumSubmitDislike(targetTxHash, targetType);
            return true;
        case "pera":
            return await algoSubmitDislike(targetTxHash, targetType);
        default:
            LogError("Invalid wallet selection: " + wallet);
            return false;
    }
}
export async function WalletSubmitEmojiReaction(targetTxHash: string, targetType: string, emoji: string): Promise<boolean> {
    let wallet = GetWallet();
    if (!wallet) {
        LogError("No wallet connected - cannot submit emoji reaction");
        return false;
    }
    const isConnected = await WalletIsConnected();
    if (!isConnected) {
        LogInfo("Wallet session expired - attempting to reconnect");
        await ReconnectWallet();
    }
    switch (wallet) {
        case "cbwalletbase":
            await baseSubmitEmojiReaction(targetTxHash, targetType, emoji);
            return true;
        case "localwalletethereum":
            await localWalletEthereumSubmitEmojiReaction(targetTxHash, targetType, emoji);
            return true;
        case "metamaskethereum":
            await ethereumSubmitEmojiReaction(targetTxHash, targetType, emoji);
            return true;
        case "pera":
            return await algoSubmitEmojiReaction(targetTxHash, targetType, emoji);
        default:
            LogError("Invalid wallet selection: " + wallet);
            return false;
    }
}
export async function WalletSendPostNudge(address: string) {
    let wallet = GetWallet()!;
    let nudge = "👋 Your friends sent you this invitation to join https://yourplace.network - Your profile is awaiting!";
    switch (wallet) {
        case "cbwalletbase":
            const txnIdBase = await baseTxn(address, nudge);
            if (!txnIdBase) {
                ShowDialogModal("Failed to send nudge - try again later");
                break;
            }
            ShowDialogModalHTML("We'll send them a note! Thanks<br><br><a href=\"" + WalletGetExplorerTxLink(txnIdBase) + "\" rel=\"noopener noreferrer\" target=\"_blank\">View Transaction</a>");
            break;
        case "localwalletethereum":
            const txnIdLocal = await localWalletEthereumTxn(address, nudge);
            if (!txnIdLocal) {
                ShowDialogModal("Failed to send nudge - try again later");
                break;
            }
            ShowDialogModalHTML("We'll send them a note! Thanks<br><br><a href=\"" + WalletGetExplorerTxLink(txnIdLocal) + "\" rel=\"noopener noreferrer\" target=\"_blank\">View Transaction</a>");
            break;
        case "metamaskethereum":
            const txnIdEth = await ethereumTxn(address, nudge);
            if (!txnIdEth) {
                ShowDialogModal("Failed to send nudge - try again later");
                break;
            }
            ShowDialogModalHTML("We'll send them a note! Thanks<br><br><a href=\"" + WalletGetExplorerTxLink(txnIdEth) + "\" rel=\"noopener noreferrer\" target=\"_blank\">View Transaction</a>");
            break;
        case "pera":
            break;
        default:
            LogError("Invalid wallet selection");
            break;
    }
}
export async function WalletFollowUser(toAddress: string, toBlockchain: string): Promise<string> {
    let wallet = GetWallet()!;
    let loggedInAddress = GetAddress()!;
    if (toAddress === loggedInAddress) {
        ShowDialogModal("You can't follow yourself, silly :D How did you even do that??");
        return "";
    }
    switch (wallet) {
        case "cbwalletbase":
            let txIDBase = await baseFollowUser(toAddress, toBlockchain);
            if (txIDBase) {
                return txIDBase.toString();
            }
            break;
        case "localwalletethereum":
            let txIDLocal = await localWalletEthereumFollowUser(toAddress, toBlockchain);
            if (txIDLocal) {
                return txIDLocal.toString();
            }
            break;
        case "metamaskethereum":
            let txIDEth = await ethereumFollowUser(toAddress, toBlockchain);
            if (txIDEth) {
                return txIDEth.toString();
            }
            break;
        case "pera":
            return await algoFollowUser(toAddress, toBlockchain);
    }
    return "";
}
export async function WalletUnfollowUser(toAddress: string, toBlockchain: string): Promise<string> {
    let wallet = GetWallet()!;
    let loggedInAddress = GetAddress()!;
    if (toAddress === loggedInAddress) {
        ShowDialogModal("You can't unfollow yourself, silly :D How did you even do that??");
        return "";
    }
    switch (wallet) {
        case "cbwalletbase":
            let txIDBase = await baseUnfollowUser(toAddress, toBlockchain);
            if (txIDBase) {
                return txIDBase.toString();
            }
            break;
        case "localwalletethereum":
            let txIDLocal = await localWalletEthereumUnfollowUser(toAddress, toBlockchain);
            if (txIDLocal) {
                return txIDLocal.toString();
            }
            break;
        case "metamaskethereum":
            let txIDEth = await ethereumUnfollowUser(toAddress, toBlockchain);
            if (txIDEth) {
                return txIDEth.toString();
            }
            break;
        case "pera":
            return await algoUnfollowUser(toAddress, toBlockchain);
    }
    return "";
}

// ---------- Collectible Functions ---------- //
export async function WalletBurnCollectible(tokenId: string, blockchain: string): Promise<boolean> {
    let wallet = GetWallet();
    if (!wallet) return false;
    switch (wallet) {
        case "cbwalletbase":
            return await baseBurnCollectible(BigInt(tokenId));
        case "localwalletethereum":
            return await localWalletEthereumBurnCollectible(BigInt(tokenId));
        case "metamaskethereum":
            return await ethereumBurnCollectible(BigInt(tokenId));
        case "pera":
            return await algoBurnCollectible(Number(tokenId));
    }
    return false;
}
export async function WalletGetCollectibles(address: string, blockchain: string): Promise<CollectibleData[]> {
    switch (blockchain) {
        case "base":
            let wallet = GetWallet();
            if (wallet === "localwalletethereum") {
                return await localWalletEthereumGetCollectibles(address);
            }
            return await baseGetCollectibles(address);
        case "algorand":
            return await algoGetCollectibles(address);
        case "ethereum":
            return await ethereumGetCollectibles(address);
    }
    return [];
}
export async function WalletGetTransferFeeEstimate(toAddress: string, tokenId: string, blockchain: string): Promise<string> {
    switch (blockchain) {
        case "base":
            return await baseGetTransferFeeEstimate(toAddress, BigInt(tokenId));
        case "algorand":
            return await algoGetTransferFeeEstimate();
        case "ethereum":
            return await ethereumGetTransferFeeEstimate(toAddress, BigInt(tokenId));
    }
    return "--";
}
export async function WalletMintCollectible(metadataUri: string, name?: string, unitName?: string): Promise<boolean> {
    let wallet = GetWallet();
    if (!wallet) return false;
    switch (wallet) {
        case "cbwalletbase":
            return !!await baseMintCollectible(metadataUri);
        case "localwalletethereum":
            return !!await localWalletEthereumMintCollectible(metadataUri);
        case "metamaskethereum":
            return !!await ethereumMintCollectible(metadataUri);
        case "pera":
            return await algoMintCollectible(name || "", unitName || "", metadataUri.replace("ipfs://", ""));
    }
    return false;
}
export async function WalletTransferCollectible(tokenId: string, toAddress: string, blockchain: string): Promise<boolean> {
    let wallet = GetWallet();
    if (!wallet) return false;
    switch (wallet) {
        case "cbwalletbase":
            return await baseTransferCollectible(BigInt(tokenId), toAddress);
        case "localwalletethereum":
            return await localWalletEthereumTransferCollectible(BigInt(tokenId), toAddress);
        case "metamaskethereum":
            return await ethereumTransferCollectible(BigInt(tokenId), toAddress);
        case "pera":
            return await algoTransferCollectible(Number(tokenId), toAddress);
    }
    return false;
}

// ---------- On-Ramp ---------- //
export function IsInsufficientFundsError(error: any): boolean {
    if (error?.code === "INSUFFICIENT_FUNDS") return true;
    const msg = String(error).toLowerCase();
    return msg.includes("insufficient funds") || msg.includes("overspend");
}
const coinbaseOnrampChains: Record<string, {network: string; asset: string}> = {
    "base": {network: "base", asset: "ETH"},
    "ethereum": {network: "ethereum", asset: "ETH"},
};
export function OnRampFiat(address: string, blockchain: string) {
    document.querySelectorAll(".modal.show").forEach(el => {
        const instance = window.bootstrap.Modal.getInstance(el);
        if (instance) instance.hide();
    });
    document.querySelectorAll(".modal-backdrop").forEach(el => el.remove());
    const chainConfig = coinbaseOnrampChains[blockchain];
    if (!chainConfig) {
        showOnRampFallback(address);
        return;
    }
    ShowDialogModalHTML(
        "<div>" +
            "<p>Your wallet has insufficient funds to complete this transaction</p>" +
            "<div class='onramp-buy-btn-wrap'>" +
                "<div class='onramp-arrow onramp-arrow-right'></div>" +
                "<button class='onramp-buy-btn' id='onRampBuyBtn'>Buy Crypto</button>" +
                "<div class='onramp-arrow onramp-arrow-left'></div>" +
            "</div>" +
            "<div>Address:</div>" +
            "<div class='onRampAddressRow'><span id='onRampAddress' class='onRampAddress'>" + address + "</span><i class='bi bi-copy clickable onRampAddressCopy' id='onRampAddressCopy'></i></div>" +
        "</div>"
    );
    bindOnRampAddressCopy(address);
    const buyBtn = document.getElementById("onRampBuyBtn");
    if (buyBtn) {
        buyBtn.addEventListener("click", async () => {
            buyBtn.textContent = "Loading...";
            (buyBtn as HTMLButtonElement).disabled = true;
            const csrfEl = document.getElementById("csrfToken") as HTMLInputElement | null;
            const csrfToken = csrfEl?.value || "";
            const [status, data] = await HttpPostJson("/services/coinbase/onramp/token", {blockchain}, csrfToken);
            if (status === 200 && data?.token) {
                const url = "https://pay.coinbase.com/buy/select-asset?sessionToken=" + encodeURIComponent(data.token) +
                    "&defaultNetwork=" + encodeURIComponent(chainConfig.network) +
                    "&defaultAsset=" + encodeURIComponent(chainConfig.asset);
                window.open(url, "_blank", "noopener,noreferrer");
                buyBtn.textContent = "Buy Crypto";
                (buyBtn as HTMLButtonElement).disabled = false;
            } else if (status === 401) {
                window.location.href = "/login";
            } else {
                showOnRampFallback(address);
            }
        });
    }
}
function bindOnRampAddressCopy(address: string) {
    const addrEl = document.getElementById("onRampAddress");
    if (addrEl) {
        addrEl.addEventListener("click", () => {
            navigator.clipboard.writeText(address);
            addrEl.textContent = "Copied!";
            setTimeout(() => { addrEl.textContent = address; }, 1500);
        });
    }
    const copyEl = document.getElementById("onRampAddressCopy");
    if (copyEl) {
        const copyTooltip = new window.bootstrap.Tooltip(copyEl, {title: "Copy", trigger: "hover", placement: "right"});
        copyEl.addEventListener("click", () => {
            navigator.clipboard.writeText(address).then();
            copyTooltip.setContent({".tooltip-inner": "Copied"});
            copyTooltip.show();
            setTimeout(() => {
                copyTooltip.hide();
                copyTooltip.setContent({".tooltip-inner": "Copy"});
            }, 1500);
        });
    }
}
function showOnRampFallback(address: string) {
    ShowDialogModalHTML(
        "<div>" +
            "<p>Your wallet has insufficient funds to complete this transaction.</p>" +
            "<p>Please visit <a href='https://coinbase.com' target='_blank' rel='noopener noreferrer'>Coinbase.com</a> and fund your wallet address:</p>" +
            "<div class='onRampAddressRow'><span id='onRampAddress' class='onRampAddress'>" + address + "</span><i class='bi bi-copy clickable onRampAddressCopy' id='onRampAddressCopy'></i></div>" +
        "</div>"
    );
    bindOnRampAddressCopy(address);
}

// ---------- Utility ---------- //
export async function WalletIsConnected(): Promise<boolean> {
    let wallet = GetWallet();
    switch (wallet) {
        case "cbwalletbase":
            return await baseIsWalletConnected();
        case "localwalletethereum":
            return hasLocalWalletEthereum();
        case "metamaskethereum":
            return await ethereumIsWalletConnected();
        case "pera":
            return !!peraWallet.connector?.connected;
    }
    return false;
}
export function IsValidAddress(address: string, chain?: string): boolean {
    let wallet: string | null = null;
    if (chain) {
        switch (chain) {
            case "algorand":
                wallet = "pera";
                break;
            case "base":
                wallet = "cbwalletbase";
                break;
            case "ethereum":
                wallet = "metamaskethereum";
                break;
        }
    } else {
        wallet = GetWallet();
        if (wallet === null) {
            console.log("Wallet not selected - IsValidAddress()");
            return false;
        }
    }
    switch (wallet) {
        case "cbwalletbase":
        case "localwalletethereum":
        case "metamaskethereum":
            return IsValidBaseAddress(address);
        case "pera":
            return IsValidAlgoAddress(address);
    }
    return false;
}
export function TruncateAddress(address: string) {
    let wallet = GetWallet()!;
    if (wallet == "pera") {
        let first = address.slice(0, 6);
        let middle = "...";
        let end = address.slice(52, 58);
        return first + middle + end;
    } else if (wallet == "cbwalletbase" || wallet == "localwalletethereum" || wallet == "metamaskethereum") {
        let first = address.slice(0, 6);
        let middle = "...";
        let end = address.slice(35, 41);
        return first + middle + end;
    }
}
