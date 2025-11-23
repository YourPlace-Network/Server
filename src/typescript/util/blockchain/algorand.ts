import {PeraWalletConnect} from "@perawallet/connect";
import algosdk, {Algodv2, type CustomTokenHeader, Indexer} from "algosdk";
import {HttpGetJson, HttpPostJson} from "../network";
import {DisconnectWallet, GetAddress, GetWallet, ReconnectWallet} from "./wallet";
import {YP} from "../../services/yourplace";
import {LogError, LogInfo} from "../log";
import {bytesToBase64} from "byte-base64";

// ---------- Algorand Variables & Objects ---------- //
export let algod: Algodv2;
export let indexer: Indexer;
export let peraWallet = new PeraWalletConnect({shouldShowSignTxnToast: false, chainId: 416001});
//export let txnlabManager: WalletManager;
let algoInitialized = false;

let algodURL: string, algodToken: string; let indexerURL: string, indexerToken: string;
const TESTNET_GENESIS_ID = 'testnet-v1.0';
const TESTNET_GENESIS_HASH_STRING = 'SGO1GKSzyE7IEPItTxCByw9x8FmnrCDexi9/cOUJOiI=';
const TESTNET_GENESIS_HASH = new Uint8Array(Buffer.from(TESTNET_GENESIS_HASH_STRING));
const MAINNET_GENESIS_ID = 'mainnet-v1.0';
const MAINNET_GENESIS_HASH_STRING = 'wGHE2Pwdvd7S12BL5FaOP20EGYesN73ktiC1qzkkit8=';
const MAINNET_GENESIS_HASH = new Uint8Array(Buffer.from(MAINNET_GENESIS_HASH_STRING));

// ---------- Initialization Functions ---------- //
async function initAlgoWallet() {
    let response = await HttpGetJson("/settings/services/algorand");
    if (response[0] == 200) {
        let indexerURLClean = response[1].indexerURL;
        if (indexerURLClean === "https://indexer.yourplace.network") { // resolve the CNAME to allow for fast flux DNS
            let indexerDomain = indexerURLClean.replace(/^(https?:\/\/)/, "");
            //indexerURLClean = "https://" + await CnameResolve(indexerDomain);
            indexerURLClean = indexerURLClean.replace(/\.$/, "");
        }
        let algodURLClean = response[1].algodURL;
        if (algodURLClean === "https://algod.yourplace.network") { // resolve the CNAME to allow for fast flux DNS
            let algodDomain = algodURLClean.replace(/^(https?:\/\/)/, "");
            //algodURLClean = "https://" + await CnameResolve(algodDomain);
            algodURLClean = algodURLClean.replace(/\.$/, "");
        }
        algodURL = algodURLClean!;
        algodToken = response[1].algodToken;
        indexerURL = indexerURLClean!;
        indexerToken = response[1].indexerToken;

        /*txnlabManager = new WalletManager({ // txnlab wallet object
            wallets: [
                WalletId.PERA,
                WalletId.EXODUS,
                WalletId.KIBISIS,
                WalletId.DEFLY,
                {
                    id: WalletId.WALLETCONNECT,
                    options: {
                        projectId: "8f7393c672b4c75ce233c094330be3f9",
                        metadata: {
                            name: "YourPlace",
                            description: "Distributed Social Media",
                            url: "https://yourplace.network",
                            icons: ["https://yourplace.network/image/yourplace.logo.svg"]
                        }
                    }
                },
                {
                    id: WalletId.LUTE,
                    options: {
                        siteName: "YourPlace"
                    }
                },
            ],
            network: NetworkId.MAINNET,
            algod: {
                token: algodToken,
                baseServer: algodURL,
                port: 443,
            }
        });*/

        SetAlgodClient();
        SetIndexerClient();
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
function SetIndexerClient() {
    let url = new URL(algodURL);
    if (url.host.endsWith("purestake.io")) {
        let auth: CustomTokenHeader = {"X-API-Key": indexerToken};
        indexer = new Indexer(auth, indexerURL, 443);
    } else if (url.host.endsWith("algonode.cloud")) {
        let auth: CustomTokenHeader = {"X-Algo-API-Token": indexerToken};
        indexer = new Indexer(auth, indexerURL, 443);
    } else {
        indexer = new Indexer(indexerToken, indexerURL, 443);
    }
}

// ---------- Pera WalletConnect ---------- //
export async function algoConnectWallet(name: string): Promise<string> {
    if (!algoInitialized) {
        await initAlgoWallet();
    }
    LogInfo("Connecting Algorand Wallet");
    //await txnlabManager.getWallet(WalletId.PERA)!.connect();
    /*await txnlabManager.wallets.at(0)!.connect();
    const activeAccount = txnlabManager.activeAccount;
    if (activeAccount) {
        return activeAccount.address;
    }*/
    return "";

    //txnlabManager.activeWallet!.connect();
    //return txnlabManager.activeWalletAccounts![0].address.toString();
}
export function algoReconnectSession() {
    peraWallet.reconnectSession().then((accounts) => {
        let account = accounts[0];
        peraWallet.connector?.on("disconnect", DisconnectWallet);
        localStorage.setItem("accountAddress", accounts[0]);
        localStorage.setItem("walletSelection", "pera");
    });
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
    localStorage.clear();
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
export async function algoAuthLogin(address: string) {
    if (GetWallet() == "pera") {
        const response = await HttpGetJson("/login/nonce");
        if (response[0] != 200) {
            LogError("Failed to get login nonce from server: " + response[1]);
            return;
        }
        let nonce = response[1].nonce
        LogInfo("Login Nonce: " + nonce);
        const encoder = new TextEncoder();
        const nonceArray = encoder.encode(nonce);
        let suggestedParams = await algod.getTransactionParams().do();
        suggestedParams.fee = BigInt(0);
        suggestedParams.flatFee = true;
        suggestedParams.genesisHash = MAINNET_GENESIS_HASH;
        suggestedParams.genesisID = MAINNET_GENESIS_ID;
        const txn = algosdk.makePaymentTxnWithSuggestedParamsFromObject({
            sender: address,
            receiver: address,
            amount: 0o00000,
            note: nonceArray,
            suggestedParams: suggestedParams,
        });
        const singleTxnGroups = [{txn: txn, signers: [address]}];
        let signedTxn: Uint8Array[];
        try {
            signedTxn = await peraWallet.signTransaction([singleTxnGroups]);
        } catch (error) {
            console.log(error);
            return;
        }
        let encodedTxn = bytesToBase64(signedTxn[0]);
        let csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
        let payload = {
            txn: encodedTxn,
            nonce: nonce
        };
        const loginResponse = await HttpPostJson("/login/wallet/pera/", payload, csrfToken);
        if (loginResponse[0] == 200) {
            LogInfo("Login Response Success");
            window.location.replace("/");
            return;
        } else {
            LogError("Login Response Error");
            console.log(loginResponse);
            return;
        }
    }
}
export async function algoEnrollRequest() {
    let address = GetAddress()!;
    let payload = YP.enroll(address);
    let txn = await algoCreatePostTxn(address, payload);
    await algoSubmitTxn(txn);
}

// ---------- Getters ---------- //
export async function getAlgoName(address: string): Promise<string> {
    let prefix = "yp/1/mn";
    try {
        let noteObj = await algoGetPostTxn(address, address, prefix);
        return noteObj.n;
    } catch (error) {
        return "None";
    }
}
export async function getAlgoAvatar(address: string): Promise<string> {
    let prefix = "yp/1/ma:";
    try {
        let noteObj = await algoGetPostTxn(address, address, prefix);
        return noteObj.a;
    } catch (error) {
        return "None";
    }
}
export async function getAlgoBanner(address: string): Promise<string> {
    let prefix = "yp/1/mb:";
    try {
        let noteObj = await algoGetPostTxn(address, address, prefix);
        return noteObj.b;
    } catch (error) {
        return "None";
    }
}
export async function getPosts(address: string, limit: number) {  // loads algo transactions into IndexedDB
    if (limit > 1000 || limit < 0 || !limit) limit = 100;
    let prefix = "yp/1/p:";
    try {
        let txnInfo = await indexer.searchForTransactions()
            .txType("pay")
            .addressRole("sender").address(address)
            .addressRole("receiver").address(address)
            .notePrefix(encode(prefix)).limit(limit).do();
        if (txnInfo.transactions.length == 0) return null;
        let note = decode(txnInfo.transactions[0].note);
        note = note.slice(prefix.length);
        let noteObj = JSON.parse(note);
        return noteObj.p;
    } catch (error) {
        return "None";
    }
}
export async function getPostCount(address: string): Promise<number> {
    let prefix = "yp/1/p:";
    let postCount = 0;
    let nextToken = "";
    let numTxn = 1;
    let posts = [];
    while (numTxn > 0) {
        let nextPage = nextToken;
        let response = await indexer.lookupAccountTransactions(address)
            .notePrefix(encode(prefix))
            .nextToken(nextPage).do();
        let transactions = response["transactions"];
        numTxn = transactions.length;
        posts.push(response);
        postCount++
        if (numTxn > 0) {
            let temp = response["nextToken"];
            if (temp) {
                nextToken = temp
            }
        }
    }
    console.log(posts);
    return postCount;
}
async function getLoginNonce(address: string) {

}
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

    return false;
}
export async function setAlgoPost(text: string) {
    if (text == "" || text == null) return;
    let address = GetAddress()!;
}

// ---------- Helper Functions ------------ //
const encode = (str: string): Uint8Array => new Uint8Array(Buffer.from(str, 'binary'));
const decode = (bytes: Uint8Array | undefined): string => {
    if (!bytes) return "";
    return Buffer.from(bytes).toString('binary');
};

const algoGetPostTxn = async function (source: string, destination: string, prefix: string): Promise<any> {
    try {
        let txnInfo = await indexer!.searchForTransactions()
            .txType("pay")
            .addressRole("sender").address(source)
            .addressRole("receiver").address(destination)
            .notePrefix(encode(prefix)).limit(1).do();
        if (txnInfo.transactions.length == 0) return null;
        return txnInfo;
    } catch (e) {
        console.log("Could not get post txn: " + e);
        return null;
    }
}
const algoCreatePostTxn = async function(destination: string, payload: string): Promise<any> {
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
        let signedTxn = null;
        try {
            signedTxn = await peraWallet.signTransaction([singleTxnGroups]);
            return signedTxn;
        } catch (error) {
            console.log("Couldn't sign or submit enrollment transaction: ", error);
            return;
        }
    } catch (error) {
        console.log("algoCreatePostTxn() error: ", error);
        return;
    }
}
const algoSubmitTxn = async function (txn: any): Promise<any> {
    try {
        await algod.sendRawTransaction(txn).do();
    } catch (error) {
        console.log("algoSubmitTxn() error: ", error);
    }
}
