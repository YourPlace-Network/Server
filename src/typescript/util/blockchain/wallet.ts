/* This file contains all the functions related to the wallet.
It acts as a sort of logical "firewall" between individual blockchain implementations and the core business requirements
for the application. This code is stateful using localstorage to keep a few values:
    "walletSelection" = text base name of the wallet selected by the user
    "accountAddress" = wallet address of the user
*/
import {Transaction} from "algosdk";
import {algoAuthLogin, algoConnectWallet, algoDisconnectWallet, algoReconnectSession, algoSetName, peraWallet, setAlgoAvatar, setAlgoPost} from "./algorand";
import {
    baseAuthLogin,
    baseConnectWallet,
    baseDisconnectWallet,
    baseFollowUser,
    baseUnfollowUser,
    baseGetAvatar,
    baseGetName,
    baseSetAvatar,
    baseSetBanner,
    baseSetDescription,
    baseSetLocation,
    baseSetName,
    baseSetVertical,
    baseSetWebsite,
    baseSubmitPost,
    baseSubmitPostAttach,
    baseTxn,
    mainnetBase,
    baseGetDescription
} from "./base";
import {IsValidAlgoAddress, IsValidBaseAddress, IsValidURL} from "../security";
import {LogError, LogInfo} from "../log";
import {phantomSolanaAuthLogin, phantomSolanaConnectWallet, solanaDisconnectWallet} from "./solana";
import {ShowDialogModal, ShowDialogModalHTMLUnsafe} from "../../components/modalDialog";

// ---------- Connection ---------- //
export async function WalletLogin() {
    let wallet = GetWallet();
    let address = GetAddress();
    switch (wallet) {
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
        case "pera":
            await algoDisconnectWallet();
            break;
        case "cbwalletbase":
            await baseDisconnectWallet();
            break;
        case "phantomsolana":
            await solanaDisconnectWallet();
            break;
    }
    SetWallet("");
    SetChain("");
    SetAddress("");
    localStorage.clear();
    window.DisconnectWalletCallback();
}
export async function ConnectWallet(wallet: string): Promise<string> {
    switch (wallet) {
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
        case "pera":
            algoReconnectSession();
            break;
        case "cbwalletbase":
            // Only reconnect if we have stored credentials - don't prompt for new connection
            const wagmiStore = localStorage.getItem("wagmi.store");
            if (wagmiStore) {
                await baseConnectWallet();
            }
            break;
    }
}

// ---------- Getters ---------- //
export function GetAddress() {
    let address = localStorage.getItem("accountAddress");
    if (address !== null) {
        return address.toLowerCase();
    }
    return null;
}
export function GetWallet() {
    const supportedWallets = ["pera", "cbwalletbase", "phantomsolana"];
    let wallet = localStorage.getItem("walletSelection");
    if (wallet !== null && supportedWallets.includes(wallet)) {
        return wallet;
    }
    return null
}
export function GetChain() {
    const supportedChains = ["algorand", "base", "solana"];
    let chain = localStorage.getItem("blockchain");
    if (chain !== null && supportedChains.includes(chain)) {
        return chain;
    }
    return null;
}
export async function WalletGetAvatar(chain?: string, address?: string): Promise<string> {
    let avatar;
    if (!chain) chain = GetChain()!;
    if (!address) address = GetAddress()!;
    switch (chain) {
        case "algorand":
            return "";
        case "base":
            avatar = await baseGetAvatar(address);
    }
    if (avatar && IsValidURL(avatar)) {
        return avatar;
    }
    return "";
}
export async function WalletGetName(chain: string, address: string): Promise<string|null> {
    let name;
    switch (chain) {
        case "algorand":
            return null;
        case "base":
            name = await baseGetName(address);
    }
    if (name) {
        return name;
    }
    return "";
}
export async function WalletGetDescription(chain?: string, address?: string): Promise<string|null> {
    let description;
    if (!chain) {
        chain = GetChain()!;
    }
    if (!address) {
        address = GetAddress()!;
    }
    switch (chain) {
        case "algorand":
            return null;
        case "base":
            description = await baseGetDescription(address);
    }
    if (description) {
        return description;
    }
    return null;
}
export function WalletGetExplorerAddressLink(address: string) {
    let chain = GetChain();
    if (chain == "algorand") {
        return `https://explorer.perawallet.app/address/${address}`;
    } else if (chain == "base") {
        return mainnetBase.explorerUrl + "/address/" + address;
    }
    return "";
}
export function WalletGetExplorerTxLink(tx: string) {
    if (tx == "") {
        return "";
    }
    let chain = GetChain();
    if (chain == "algorand") {
        return `https://explorer.perawallet.app/tx/${tx}`;
    } else if (chain == "base") {
        return mainnetBase.explorerUrl + "/tx/" + tx;
    }
    return "";
}
export function WalletGetYourPlaceAddressLink(address: string) {
    let chain = GetChain();
    let host = window.location.host;
    if (chain == "algorand") {
        return `${host}/p/algorand/${address}`;
    } else if (chain == "base") {
        return `${host}/p/base/${address}`;
    } else if (chain == "solana") {
        return `${host}/p/solana/${address}`;
    } else {
        return `${host}/p/${address}`;
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
    let lowerAddress = address.toLowerCase();
    if (address === "") {
        localStorage.removeItem("accountAddress");
    } else {
        localStorage.setItem("accountAddress", lowerAddress);
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
    let walletSelection = GetWallet()!;
    if (walletSelection == "pera") {
        await setAlgoAvatar(avatarURL);
    } else if (walletSelection == "cbwalletbase") {
        return !!await baseSetAvatar(avatarURL);
    }
    return false;
}
export async function WalletSetBanner(bannerURL: string): Promise<boolean> {
    let walletSelection = GetWallet()!;
    if (walletSelection == "pera") {
        //algo stuff
    } else if (walletSelection == "cbwalletbase") {
        return !!await baseSetBanner(bannerURL);
    }
    return false;
}
export async function WalletSetDescription(description: string): Promise<boolean> {
    let walletSelection = GetWallet()!;
    if (walletSelection == "pera") {
        //algo stuff
    } else if (walletSelection == "cbwalletbase") {
        return !!await baseSetDescription(description);
    }
    return false;
}
export async function WalletSetLocation(location: string): Promise<boolean> {
    let walletSelection = GetWallet()!;
    if (walletSelection == "pera") {
        //algo stuff
    } else if (walletSelection == "cbwalletbase") {
        return !!await baseSetLocation(location);
    }
    return false;
}
export async function WalletSetVertical(vertical: string): Promise<boolean> {
    let walletSelection = GetWallet()!;
    if (walletSelection == "pera") {
        //algo stuff
    } else if (walletSelection == "cbwalletbase") {
        return !!await baseSetVertical(vertical);
    }
    return false;
}
export async function WalletSetWebsite(website: string): Promise<boolean> {
    let walletSelection = GetWallet()!;
    if (walletSelection == "pera") {
        //algo stuff
    } else if (walletSelection == "cbwalletbase") {
        return !!await baseSetWebsite(website);
    }
    return false;
}
export async function WalletSetName(name: string): Promise<boolean> {
    let wallet = GetWallet()!;
    switch (wallet) {
        case "pera":
            return await algoSetName(name);
        case "cbwalletbase":
            return !!await baseSetName(name);
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
        case "pera":
            await setAlgoPost(payload);
            return true;
        case "cbwalletbase":
            await baseSubmitPost(payload);
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
        case "pera":
            //await setAlgoPostAttach(payload, attach);
            return true;
        case "cbwalletbase":
            await baseSubmitPostAttach(payload, attach);
            return true;
        default:
            LogError("Invalid wallet selection: " + wallet);
            return false;
    }
}
export async function WalletSendPostNudge(address: string) {
    let wallet = GetWallet()!;
    let nudge = "👋 Your friends sent you this invitation to join https://yourplace.network - Your profile is awaiting!";
    switch (wallet) {
        case "pera":
            break;
        case "cbwalletbase":
            const txnId = await baseTxn(address, nudge);
            if (!txnId) {
                ShowDialogModal("Failed to send nudge - try again later");
                break;
            }
            ShowDialogModalHTMLUnsafe("We'll send them a note! Thanks<br><br><a href=\"" + WalletGetExplorerTxLink(txnId) + "\" rel=\"noopener noreferrer\" target=\"_blank\">View Transaction</a>");
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
        case "pera":
            break;
        case "cbwalletbase":
            let txID = await baseFollowUser(toAddress, toBlockchain);
            if (txID) {
                return txID.toString();
            }
            break;
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
        case "pera":
            break;
        case "cbwalletbase":
            let txID = await baseUnfollowUser(toAddress, toBlockchain);
            if (txID) {
                return txID.toString();
            }
            break;
    }
    return "";
}

// ---------- Utility ---------- //
export async function WalletIsConnected(): Promise<boolean> {
    let wallet = GetWallet();
    switch (wallet) {
        case "pera":
            return !!peraWallet.connector?.connected;
        case "cbwalletbase":
            return false;
    }
    return false;
}
export function IsValidAddress(address: string, chain?: string): boolean {
    let wallet: string | null = null;
    if (chain) {
        switch (chain) {
            case "base":
                wallet = "cbwalletbase";
        }
    } else {
        wallet = GetWallet();
        if (wallet === null) {
            console.log("Wallet not selected - IsValidAddress()");
            return false;
        }
    }
    switch (wallet) {
        case "pera":
            return IsValidAddress(address);
        case "cbwalletbase":
            return IsValidBaseAddress(address);
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
    } else if (wallet == "cbwalletbase" || wallet == "eth") {
        let first = address.slice(0, 6);
        let middle = "...";
        let end = address.slice(35, 41);
        return first + middle + end;
    }
}
