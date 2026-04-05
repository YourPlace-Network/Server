import {PeraWalletConnect} from "@perawallet/connect";
import algosdk, {Algodv2, type CustomTokenHeader} from "algosdk";
import {HideDialogModal, ShowDialogModalHTML} from "../../components/modalDialog";
import {HttpGetJson, HttpPostJson} from "../network";
import {DisconnectWallet, GetAddress, GetWallet, IsInsufficientFundsError, OnRampFiat, ReconnectWallet} from "./wallet";
import type {CollectibleData} from "./wallet";
import {YP} from "../../services/yourplace";
import {LogError, LogInfo} from "../log";
import {SiwaMessage} from "@avmkit/siwa";
import {PersistentCache} from "../cache";
import {IsValidAlgoAddress, IsValidURL} from "../security";
import {CIDToSubdomainURL} from "../ipfs";

// ---------- Algorand Variables & Objects ---------- //
export let algod: Algodv2;
export let peraWallet = new PeraWalletConnect({shouldShowSignTxnToast: true, chainId: 416001});
let algoInitialized = false;
let algodURL: string, algodToken: string;
const avatarCache = new PersistentCache("algo_avatar");
const nameCache = new PersistentCache("algo_name");
const nfdNameCache = new PersistentCache("algo_nfd_name");
const nfdAddressCache = new PersistentCache("algo_nfd_address");
const nfdAvatarCache = new PersistentCache("algo_nfd_avatar");
const TESTNET_GENESIS_ID = 'testnet-v1.0';
const TESTNET_GENESIS_HASH_STRING = 'SGO1GKSzyE7IEPItTxCByw9x8FmnrCDexi9/cOUJOiI=';
const MAINNET_GENESIS_ID = 'mainnet-v1.0';
const MAINNET_GENESIS_HASH_STRING = 'wGHE2Pwdvd7S12BL5FaOP20EGYesN73ktiC1qzkkit8=';

// ---------- Initialization Functions ---------- //
async function initAlgoWallet() {
    let response = await HttpGetJson("/settings/services/algorand");
    if (response[0] == 200) {
        algodURL = response[1].algodURL;
        algodToken = response[1].algodToken;
        SetAlgodClient();
        algoInitialized = true;
    } else {
        console.log("Error getting Algorand parameters: ", response[1]);
    }
}
function SetAlgodClient() {
    let url = new URL(algodURL);
    if (url.host.endsWith("purestake.io")) {
        let auth: CustomTokenHeader = {"X-API-Key": algodToken};
        algod = new Algodv2(auth, algodURL, 443);
    } else if (url.host.endsWith("algonode.cloud")) {
        let auth: CustomTokenHeader = {"X-Algo-API-Token": algodToken};
        algod = new Algodv2(auth, algodURL, 443);
    } else {
        algod = new Algodv2(algodToken, algodURL, 443);
    }
}

// ---------- Pera WalletConnect ---------- //
export async function algoConnectWallet(name: string): Promise<string> {
    if (!algoInitialized) {
        await initAlgoWallet();
    }
    LogInfo("Connecting Algorand Wallet via Pera");
    try {
        const accounts = await peraWallet.connect();
        if (accounts && accounts.length > 0) {
            const account = accounts[0];
            peraWallet.connector?.on("disconnect", DisconnectWallet);
            localStorage.setItem("accountAddress", account);
            localStorage.setItem("walletSelection", "pera");
            localStorage.setItem("blockchain", "algorand");
            LogInfo("Connected to Pera Wallet: " + account);
            return account;
        }
    } catch (error) {
        LogError("Failed to connect to Pera Wallet: " + error);
    }
    return "";
}
export async function algoReconnectSession() {
    if (!algoInitialized) {
        await initAlgoWallet();
    }
    try {
        const accounts = await peraWallet.reconnectSession();
        if (accounts && accounts.length > 0) {
            peraWallet.connector?.on("disconnect", DisconnectWallet);
            localStorage.setItem("accountAddress", accounts[0]);
            localStorage.setItem("walletSelection", "pera");
        }
    } catch {
    }
}
export function algoHandleDisconnectWallet(event: any) {
    event.preventDefault();
    algoDisconnectWallet();
}
export async function algoDisconnectWallet() {
    console.log("algoDisconnectWallet()");
    if (GetWallet() == "pera") {
        try {
            if (peraWallet.isConnected) {
                await peraWallet.disconnect();
            }
        } catch (error) {
            console.log("algoDisconnectWallet() - error: " + error);
        }
    }
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
}
export async function algoConnectSession(): Promise<string> {
    console.log("algoConnectSession()");
    let account = "";
    await peraWallet.connect().then((accounts) => {
        account = accounts[0];
        peraWallet.connector!.on("disconnect", DisconnectWallet);
        console.log("algoConnectSession(): connected - " + account)
        localStorage.setItem("accountAddress", account);
        localStorage.setItem("walletSelection", "pera");
    });
    return account;
}
export async function algoAuthLogin(address: string): Promise<string> {
    if (GetWallet() != "pera") {
        LogError("algoAuthLogin called with non-pera wallet");
        return "";
    }
    if (!algoInitialized) {
        await initAlgoWallet();
    }
    const response = await HttpGetJson("/login/nonce");
    if (response[0] != 200) {
        LogError("Failed to get login nonce from server: " + response[1]);
        return "";
    }
    const nonce = response[1].nonce;
    const domain = response[1].domain;
    const issuedAt = response[1].issuedAt;
    const siwaMessage = new SiwaMessage({
        domain: domain,
        address: address,
        statement: "Sign in with Algorand to YourPlace",
        uri: window.location.origin,
        version: "1",
        chainId: 416001,
        nonce: nonce,
        issuedAt: issuedAt,
    });
    const messageToSign = siwaMessage.prepareMessage();
    let signedTxn: Uint8Array[];
    try {
        const suggestedParams = await algod.getTransactionParams().do();
        const txn = algosdk.makePaymentTxnWithSuggestedParamsFromObject({
            suggestedParams: suggestedParams,
            sender: address,
            receiver: address,
            amount: 0,
            note: new Uint8Array(Buffer.from(messageToSign)),
        });
        const txnGroup = [{txn: txn, signers: [address]}];
        signedTxn = await peraWallet.signTransaction([txnGroup]);
    } catch (error) {
        LogError("Failed to sign SIWA transaction: " + error);
        return "";
    }
    if (!signedTxn || signedTxn.length === 0) {
        LogError("No signed transaction returned from Pera Wallet");
        return "";
    }
    const encodedTransaction = Buffer.from(signedTxn[0]).toString("base64");
    const decodedTxn = algosdk.decodeSignedTransaction(signedTxn[0]);
    const signature = Buffer.from(decodedTxn.sig!).toString("base64");
    const csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
    const payload = {
        message: messageToSign,
        signature: signature,
        encodedTransaction: encodedTransaction,
        address: address,
    };
    const loginResponse = await HttpPostJson("/login/wallet/pera", payload, csrfToken);
    if (loginResponse[0] == 200) {
        LogInfo("SIWA Login Success");
        return "success";
    } else {
        LogError("SIWA Login Error: " + JSON.stringify(loginResponse[1]));
        return "";
    }
}

// ---------- Getters ---------- //
export async function getAlgoTxn(destination: string, payload: string, amount: number): Promise<algosdk.Transaction> {
    const suggestedParams = await algod.getTransactionParams().do();
    const ptxn = algosdk.makePaymentTxnWithSuggestedParamsFromObject({
        suggestedParams,
        sender: GetAddress()!,
        receiver: destination,
        amount: amount,
        note: new Uint8Array(Buffer.from(payload)),
    });
    return ptxn;
}

// ---------- Setters ---------- //
export async function setAlgoAvatar(avatarURL: string): Promise<any> {
    let address = GetAddress()!;
    let payload = YP.metadataAvatar(avatarURL);
    let txn = await algoCreatePostTxn(address, payload);
    await algoSubmitTxn(txn);
}
export async function algoSetName(name: string): Promise<boolean> {
    if (name == "" || name == null) return false;
    let address = GetAddress()!;
    let payload = YP.metadataName(name);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSetBanner(bannerURL: string): Promise<boolean> {
    if (bannerURL == "" || bannerURL == null) return false;
    let address = GetAddress()!;
    let payload = YP.metadataBanner(bannerURL);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSetColors(colors: Record<string, string>): Promise<boolean> {
    if (!colors || Object.keys(colors).length === 0) return false;
    let address = GetAddress()!;
    let payload = YP.metadataColors(colors);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSetDescription(description: string): Promise<boolean> {
    if (description == "" || description == null) return false;
    let address = GetAddress()!;
    let payload = YP.metadataDescription(description);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSetLocation(location: string): Promise<boolean> {
    if (location == "" || location == null) return false;
    let address = GetAddress()!;
    let payload = YP.metadataLocation(location);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSetBot(bot: boolean): Promise<boolean> {
    let address = GetAddress()!;
    let payload = YP.metadataBot(bot);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSetNsfw(nsfw: boolean): Promise<boolean> {
    let address = GetAddress()!;
    let payload = YP.metadataNsfw(nsfw);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSetVertical(vertical: string): Promise<boolean> {
    if (vertical == "" || vertical == null) return false;
    let address = GetAddress()!;
    let payload = YP.metadataVertical(vertical);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSetWebsite(website: string): Promise<boolean> {
    if (website == "" || website == null) return false;
    let address = GetAddress()!;
    let payload = YP.metadataWebsite(website);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSubmitPost(text: string): Promise<boolean> {
    if (text == "" || text == null) return false;
    let address = GetAddress()!;
    let payload = YP.post(text);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function setAlgoPostAttach(text: string, attachments: string[][]): Promise<boolean> {
    if (text == "" || text == null) return false;
    let address = GetAddress()!;
    let payload = YP.postAttach(text, attachments);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSubmitComment(parentTxHash: string, text: string): Promise<boolean> {
    if (text == "" || text == null) return false;
    if (parentTxHash == "" || parentTxHash == null) return false;
    let address = GetAddress()!;
    let payload = YP.comment(parentTxHash, text);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSubmitCommentAttach(parentTxHash: string, text: string, attachments: string[][]): Promise<boolean> {
    if (text == "" || text == null) return false;
    if (parentTxHash == "" || parentTxHash == null) return false;
    let address = GetAddress()!;
    let payload = YP.commentAttach(parentTxHash, text, attachments);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSubmitLike(targetTxHash: string, targetType: string): Promise<boolean> {
    if (targetTxHash == "" || targetTxHash == null) return false;
    let address = GetAddress()!;
    let payload = YP.like(targetTxHash, targetType);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSubmitDislike(targetTxHash: string, targetType: string): Promise<boolean> {
    if (targetTxHash == "" || targetTxHash == null) return false;
    let address = GetAddress()!;
    let payload = YP.dislike(targetTxHash, targetType);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoSubmitEmojiReaction(targetTxHash: string, targetType: string, emoji: string): Promise<boolean> {
    if (targetTxHash == "" || targetTxHash == null) return false;
    if (emoji == "" || emoji == null) return false;
    let address = GetAddress()!;
    let payload = YP.emojiReact(targetTxHash, targetType, emoji);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return false;
    await algoSubmitTxn(txn);
    return true;
}
export async function algoFollowUser(toAddress: string, toBlockchain: string): Promise<string> {
    if (toAddress == "" || toAddress == null) return "";
    if (toBlockchain == "" || toBlockchain == null) return "";
    let address = GetAddress()!;
    let payload = YP.follow(toAddress, toBlockchain);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return "";
    await algoSubmitTxn(txn);
    return "success";
}
export async function algoUnfollowUser(toAddress: string, toBlockchain: string): Promise<string> {
    if (toAddress == "" || toAddress == null) return "";
    if (toBlockchain == "" || toBlockchain == null) return "";
    let address = GetAddress()!;
    let payload = YP.unfollow(toAddress, toBlockchain);
    let txn = await algoCreatePostTxn(address, payload);
    if (!txn) return "";
    await algoSubmitTxn(txn);
    return "success";
}

// ---------- Collectible Functions ---------- //
export async function algoBurnCollectible(assetId: number): Promise<boolean> {
    if (!algoInitialized) await initAlgoWallet();
    try {
        let suggestedParams = await algod.getTransactionParams().do();
        const txn = algosdk.makeAssetDestroyTxnWithSuggestedParamsFromObject({
            suggestedParams: suggestedParams,
            sender: GetAddress()!,
            assetIndex: assetId,
        });
        const singleTxnGroups = [{txn: txn, signers: [GetAddress()!]}];
        if (!peraWallet.isConnected) {
            try { await peraWallet.reconnectSession(); } catch (_) { await peraWallet.connect(); }
        }
        ShowDialogModalHTML(
            '<div style="text-align: center;">' +
            '<img src="/static/image/pera-small.png" alt="Pera Wallet" style="width: 64px; height: 64px; margin-bottom: 16px;">' +
            '<p>Open your Pera Wallet to sign the transaction</p>' +
            '</div>'
        );
        const signedTxn = await peraWallet.signTransaction([singleTxnGroups]);
        HideDialogModal();
        await algod.sendRawTransaction(signedTxn).do();
        return true;
    } catch (error) {
        HideDialogModal();
        if (IsInsufficientFundsError(error)) { OnRampFiat(GetAddress()!, "algorand"); return false; }
        LogError("algoBurnCollectible failed: " + error);
        return false;
    }
}
export async function algoGetCollectibles(address: string): Promise<CollectibleData[]> {
    if (!algoInitialized) await initAlgoWallet();
    const results: CollectibleData[] = [];
    try {
        const accountInfo = await algod.accountInformation(address).do();
        const assets = accountInfo.assets || [];
        for (const asset of assets) {
            if (asset.amount <= 0n) continue;
            try {
                const assetInfo = await algod.getAssetByID(asset.assetId).do();
                const params = assetInfo.params;
                if (BigInt(params.total) !== 1n || BigInt(params.decimals) !== 0n) continue;
                let metadata: any = {};
                let imageUrl = "";
                let mimeType = "image/png";
                if (params.url && params.url.includes("#arc3")) {
                    const metadataUrl = params.url.startsWith("ipfs://") ? CIDToSubdomainURL(params.url.split("#")[0]) : params.url.split("#")[0];
                    if (metadataUrl) {
                        try {
                            const resp = await fetch(metadataUrl);
                            if (resp.ok) {
                                metadata = await resp.json();
                                imageUrl = metadata.image || "";
                                mimeType = metadata.image_mimetype || "image/png";
                            }
                        } catch (_) {}
                    }
                } else if (params.url) {
                    imageUrl = params.url;
                }
                results.push({
                    blockchain: "algorand",
                    contractAddress: asset.assetId.toString(),
                    creator: params.creator || "",
                    description: metadata.description || "",
                    imageUrl: imageUrl,
                    mimeType: mimeType,
                    name: params.name || "ASA #" + asset.assetId,
                    tokenId: asset.assetId.toString(),
                });
            } catch (innerError) {
                LogError("algoGetCollectibles: error fetching asset " + asset.assetId + ": " + innerError);
            }
        }
    } catch (error) {
        LogError("algoGetCollectibles failed: " + error);
    }
    return results;
}
export async function algoGetTransferFeeEstimate(): Promise<string> {
    if (!algoInitialized) await initAlgoWallet();
    try {
        const params = await algod.getTransactionParams().do();
        const fee = Number(params.minFee || 1000);
        return (fee / 1e6).toFixed(6) + " ALGO";
    } catch (error) {
        LogError("algoGetTransferFeeEstimate failed: " + error);
        return "-- ALGO";
    }
}
export async function algoMintCollectible(name: string, unitName: string, metadataCid: string): Promise<boolean> {
    if (!algoInitialized) await initAlgoWallet();
    const PLATFORM_FEE_RECEIVER = "QSDMOUR7FQTP7F2TL6GXTI2ZTXFNQIHCYLPXXUDILG542WU5N636BKZQUY";
    const PLATFORM_FEE_AMOUNT = 100;
    try {
        let suggestedParams = await algod.getTransactionParams().do();
        const feeTxn = algosdk.makePaymentTxnWithSuggestedParamsFromObject({
            suggestedParams: suggestedParams,
            sender: GetAddress()!,
            receiver: PLATFORM_FEE_RECEIVER,
            amount: PLATFORM_FEE_AMOUNT,
        });
        const asaTxn = algosdk.makeAssetCreateTxnWithSuggestedParamsFromObject({
            suggestedParams: suggestedParams,
            sender: GetAddress()!,
            total: 1,
            decimals: 0,
            assetName: name,
            unitName: unitName.substring(0, 8),
            assetURL: "ipfs://" + metadataCid + "#arc3",
            manager: GetAddress()!,
            reserve: GetAddress()!,
            freeze: GetAddress()!,
            clawback: GetAddress()!,
            defaultFrozen: false,
        });
        algosdk.assignGroupID([feeTxn, asaTxn]);
        const txnGroups = [
            {txn: feeTxn, signers: [GetAddress()!]},
            {txn: asaTxn, signers: [GetAddress()!]},
        ];
        if (!peraWallet.isConnected) {
            try { await peraWallet.reconnectSession(); } catch (_) { await peraWallet.connect(); }
        }
        ShowDialogModalHTML(
            '<div style="text-align: center;">' +
            '<img src="/static/image/pera-small.png" alt="Pera Wallet" style="width: 64px; height: 64px; margin-bottom: 16px;">' +
            '<p>Open your Pera Wallet to sign the transaction</p>' +
            '</div>'
        );
        const signedTxn = await peraWallet.signTransaction([txnGroups]);
        HideDialogModal();
        await algod.sendRawTransaction(signedTxn).do();
        return true;
    } catch (error) {
        HideDialogModal();
        if (IsInsufficientFundsError(error)) { OnRampFiat(GetAddress()!, "algorand"); return false; }
        LogError("algoMintCollectible failed: " + error);
        return false;
    }
}
export async function algoTransferCollectible(assetId: number, toAddress: string): Promise<boolean> {
    if (!algoInitialized) await initAlgoWallet();
    try {
        let suggestedParams = await algod.getTransactionParams().do();
        const txn = algosdk.makeAssetTransferTxnWithSuggestedParamsFromObject({
            suggestedParams: suggestedParams,
            sender: GetAddress()!,
            receiver: toAddress,
            assetIndex: assetId,
            amount: 1,
        });
        const singleTxnGroups = [{txn: txn, signers: [GetAddress()!]}];
        if (!peraWallet.isConnected) {
            try { await peraWallet.reconnectSession(); } catch (_) { await peraWallet.connect(); }
        }
        ShowDialogModalHTML(
            '<div style="text-align: center;">' +
            '<img src="/static/image/pera-small.png" alt="Pera Wallet" style="width: 64px; height: 64px; margin-bottom: 16px;">' +
            '<p>Open your Pera Wallet to sign the transaction</p>' +
            '</div>'
        );
        const signedTxn = await peraWallet.signTransaction([singleTxnGroups]);
        HideDialogModal();
        await algod.sendRawTransaction(signedTxn).do();
        return true;
    } catch (error) {
        HideDialogModal();
        if (IsInsufficientFundsError(error)) { OnRampFiat(GetAddress()!, "algorand"); return false; }
        LogError("algoTransferCollectible failed: " + error);
        return false;
    }
}

// ---------- Helper Functions ------------ //
const algoCreatePostTxn = async function(destination: string, payload: string): Promise<any> {
    if (!algoInitialized) {
        await initAlgoWallet();
    }
    await ReconnectWallet();
    try {
        let encoder = new TextEncoder();
        let note = encoder.encode(payload);
        let suggestedParams = await algod.getTransactionParams().do();
        const txn = algosdk.makePaymentTxnWithSuggestedParamsFromObject({
            suggestedParams: suggestedParams,
            sender: GetAddress()!,
            receiver: destination!,
            amount: 0,
            note: note,
        });
        const singleTxnGroups = [{txn: txn, signers: [GetAddress()!]}];
        if (!peraWallet.isConnected) {
            try {
                await peraWallet.reconnectSession();
            } catch (e) {
                await peraWallet.connect();
            }
        }
        let signedTxn = null;
        try {
            ShowDialogModalHTML(
                '<div style="text-align: center;">' +
                '<img src="/static/image/pera-small.png" alt="Pera Wallet" style="width: 64px; height: 64px; margin-bottom: 16px;">' +
                '<p>Open your Pera Wallet to sign the transaction</p>' +
                '</div>'
            );
            signedTxn = await peraWallet.signTransaction([singleTxnGroups]);
            HideDialogModal();
            return signedTxn;
        } catch (error) {
            HideDialogModal();
            LogError("algoCreatePostTxn: Couldn't sign transaction: " + error);
            return;
        }
    } catch (error) {
        LogError("algoCreatePostTxn() error: " + error);
        return;
    }
}
const algoSubmitTxn = async function (txn: any): Promise<string> {
    if (!algoInitialized) {
        await initAlgoWallet();
    }
    try {
        let response = await algod.sendRawTransaction(txn).do();
        LogInfo("algoSubmitTxn: Transaction submitted, txID: " + response.txid);
        return response.txid;
    } catch (error) {
        if (IsInsufficientFundsError(error)) { OnRampFiat(GetAddress()!, "algorand"); return ""; }
        LogError("algoSubmitTxn() error: " + error);
        return "";
    }
}

// ---------- NFD (NFDomains) Functions ---------- //
const NFD_API_URL = "https://api.nf.domains";
interface NfdRecord {
    name: string;
    owner: string;
    depositAccount?: string;
    caAlgo?: string[];
    avatar?: string;
    properties?: {
        internal?: Record<string, string>;
        userDefined?: Record<string, string>;
        verified?: Record<string, string>;
    };
}
async function fetchNfdByAddress(address: string): Promise<NfdRecord | null> {
    try {
        const response = await fetch(`${NFD_API_URL}/nfd/lookup?address=${address}&view=brief`);
        if (response.ok) {
            const data = await response.json();
            if (data && Object.keys(data).length > 0) {
                const firstKey = Object.keys(data)[0];
                return data[firstKey] as NfdRecord;
            }
        }
    } catch (_) {}
    try {
        const response = await HttpGetJson(`/services/algorand/nfd/lookup?address=${address}`);
        if (response[0] === 200 && response[1]) {
            return response[1] as NfdRecord;
        }
    } catch (error) {
        LogError("fetchNfdByAddress() error: " + error);
    }
    return null;
}
async function fetchNfdByName(name: string): Promise<NfdRecord | null> {
    try {
        const response = await fetch(`${NFD_API_URL}/nfd/${name}?view=brief`);
        if (response.ok) {
            return await response.json() as NfdRecord;
        }
    } catch (_) {}
    try {
        const response = await HttpGetJson(`/services/algorand/nfd/name?name=${name}`);
        if (response[0] === 200 && response[1]) {
            return response[1] as NfdRecord;
        }
    } catch (error) {
        LogError("fetchNfdByName() error: " + error);
    }
    return null;
}
export async function algoGetNfdName(address: string): Promise<string> {
    if (!IsValidAlgoAddress(address)) {
        return "";
    }
    const cached = nfdNameCache.get<string>(address);
    if (cached !== null) {
        return cached;
    }
    try {
        const nfd = await fetchNfdByAddress(address);
        if (nfd && nfd.name) {
            nfdNameCache.set(address, nfd.name);
            return nfd.name;
        }
    } catch (error) {
        LogError("algoGetNfdName() error: " + error);
    }
    nfdNameCache.set(address, "");
    return "";
}
export async function algoGetNfdAvatar(address: string): Promise<string> {
    if (!IsValidAlgoAddress(address)) {
        return "";
    }
    const cached = nfdAvatarCache.get<string>(address);
    if (cached !== null) {
        return cached;
    }
    try {
        const nfd = await fetchNfdByAddress(address);
        if (nfd) {
            let avatar = nfd.avatar ||
                nfd.properties?.verified?.avatar ||
                nfd.properties?.userDefined?.avatar || "";
            if (avatar) {
                if (avatar.startsWith("ipfs://")) {
                    avatar = CIDToSubdomainURL(avatar) || avatar;
                }
                nfdAvatarCache.set(address, avatar);
                return avatar;
            }
        }
    } catch (error) {
        LogError("algoGetNfdAvatar() error: " + error);
    }
    nfdAvatarCache.set(address, "");
    return "";
}
export async function algoGetNfdAddress(nfdName: string): Promise<string> {
    let name = nfdName.toLowerCase().trim();
    if (!name.endsWith(".algo")) {
        name = name + ".algo";
    }
    const cached = nfdAddressCache.get<string>(name);
    if (cached !== null) {
        return cached;
    }
    try {
        const nfd = await fetchNfdByName(name);
        if (nfd) {
            const address = nfd.caAlgo?.[0] || nfd.depositAccount || nfd.owner || "";
            if (address) {
                nfdAddressCache.set(name, address);
                return address;
            }
        }
    } catch (error) {
        LogError("algoGetNfdAddress() error: " + error);
    }
    nfdAddressCache.set(name, "");
    return "";
}
export async function algoGetAvatar(address: string): Promise<string> {
    if (!IsValidAlgoAddress(address)) {
        return "";
    }
    const cached = avatarCache.get<string>(address);
    if (cached !== null) {
        return cached;
    }
    try {
        const response = await HttpGetJson("/profile/avatar/algorand/" + address);
        if (response[0] === 200 && response[1] && response[1].avatarAddress) {
            const avatarAddress = response[1].avatarAddress.trim();
            if (avatarAddress.length > 0) {
                if (IsValidURL(avatarAddress)) {
                    avatarCache.set(address, avatarAddress);
                    return avatarAddress;
                }
                const avatarUrl = CIDToSubdomainURL(avatarAddress);
                if (avatarUrl !== "") {
                    avatarCache.set(address, avatarUrl);
                    return avatarUrl;
                }
            }
        }
        const nfdAvatar = await algoGetNfdAvatar(address);
        if (nfdAvatar && nfdAvatar !== "") {
            avatarCache.set(address, nfdAvatar);
            return nfdAvatar;
        }
    } catch (error) {
        LogError("algoGetAvatar() error: " + error);
    }
    return "";
}
export async function algoGetName(address: string): Promise<string> {
    if (!IsValidAlgoAddress(address)) {
        return "";
    }
    const cached = nameCache.get<string>(address);
    if (cached !== null) {
        return cached;
    }
    try {
        const response = await HttpGetJson("/profile/name/algorand/" + address);
        if (response[0] === 200 && response[1] && response[1].name) {
            const name = response[1].name.trim();
            if (name.length > 0) {
                nameCache.set(address, name);
                return name;
            }
        }
        const nfdName = await algoGetNfdName(address);
        if (nfdName && nfdName !== "") {
            nameCache.set(address, nfdName);
            return nfdName;
        }
    } catch (error) {
        LogError("algoGetName() error: " + error);
    }
    return "";
}
