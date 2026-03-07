import {ethers, getAddress} from "ethers";
import {SiweMessage} from "siwe";
import {HttpGetJson, HttpPostJson} from "../network";
import {IsValidBaseAddress} from "../security";
import {LogError, LogInfo} from "../log";
import {YP} from "../../services/yourplace";
import {mainnetBase} from "./base";
import {YP_NFT_CONTRACT_ADDRESS, YP_NFT_CONTRACT_ABI} from "./base";
import {ShowDialogModalHTMLUnsafe} from "../../components/modalDialog";
import {CIDToSubdomainURL} from "../ipfs";
import type {CollectibleData} from "./wallet";

const LOCAL_WALLET_KEY = "yp_local_wallet_ethereum";

interface LocalWalletData {
    address: string;
    blockchain: string;
    createdAt: string;
    mnemonic: string;
    privateKey: string;
    publicKey: string;
}

let ethersProvider: ethers.JsonRpcProvider | null = null;

async function getProvider(): Promise<ethers.JsonRpcProvider> {
    if (ethersProvider) {
        return ethersProvider;
    }
    const response = await HttpGetJson("/settings/base/url");
    if (response[0] === 200 && response[1] && response[1].baseURL !== "") {
        let url = response[1].baseURL;
        if (url.startsWith('/')) {
            url = window.location.origin + url;
        }
        ethersProvider = new ethers.JsonRpcProvider(url);
        return ethersProvider;
    }
    throw new Error("Failed to get Base RPC URL");
}

function downloadWalletBackup(walletData: LocalWalletData): void {
    const jsonStr = JSON.stringify(walletData, null, 2);
    const blob = new Blob([jsonStr], {type: "application/json"});
    const url = URL.createObjectURL(blob);
    const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
    const filename = `yourplace-wallet-backup-${timestamp}.json`;
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

export function hasLocalWalletEthereum(): boolean {
    return localStorage.getItem(LOCAL_WALLET_KEY) !== null;
}

export function localWalletEthereumGetWallet(): ethers.Wallet | null {
    const stored = localStorage.getItem(LOCAL_WALLET_KEY);
    if (!stored) {
        return null;
    }
    try {
        const data: LocalWalletData = JSON.parse(stored);
        return new ethers.Wallet(data.privateKey);
    } catch (e) {
        LogError("Failed to parse local wallet data: " + e);
        return null;
    }
}

export async function localWalletEthereumCreate(): Promise<string> {
    const wallet = ethers.Wallet.createRandom();
    if (!wallet.mnemonic) {
        LogError("Failed to generate mnemonic for wallet");
        return "";
    }
    const walletData: LocalWalletData = {
        address: wallet.address,
        blockchain: "base",
        createdAt: new Date().toISOString(),
        mnemonic: wallet.mnemonic.phrase,
        privateKey: wallet.privateKey,
        publicKey: wallet.publicKey,
    };
    const walletDataJson = JSON.stringify(walletData);
    localStorage.setItem(LOCAL_WALLET_KEY, walletDataJson);
    downloadWalletBackup(walletData);
    return wallet.address;
}

export async function localWalletEthereumConnect(): Promise<string> {
    const stored = localStorage.getItem(LOCAL_WALLET_KEY);
    if (!stored) {
        return "";
    }
    try {
        const data: LocalWalletData = JSON.parse(stored);
        if (data.address && IsValidBaseAddress(data.address)) {
            return data.address;
        }
    } catch (e) {
        LogError("Failed to parse local wallet data: " + e);
    }
    return "";
}

export async function localWalletEthereumDisconnect(): Promise<void> {
    localStorage.removeItem(LOCAL_WALLET_KEY);
}
export function localWalletEthereumImport(walletJson: string): string {
    try {
        const data = JSON.parse(walletJson);
        if (!data.address || !data.blockchain || !data.createdAt || !data.mnemonic || !data.privateKey || !data.publicKey) {
            LogError("Invalid wallet backup file: missing required fields");
            return "";
        }
        if (typeof data.address !== "string" || typeof data.blockchain !== "string" ||
            typeof data.createdAt !== "string" || typeof data.mnemonic !== "string" ||
            typeof data.privateKey !== "string" || typeof data.publicKey !== "string") {
            LogError("Invalid wallet backup file: invalid field types");
            return "";
        }
        if (!IsValidBaseAddress(data.address)) {
            LogError("Invalid wallet backup file: invalid address");
            return "";
        }
        const wallet = new ethers.Wallet(data.privateKey);
        if (wallet.address.toLowerCase() !== data.address.toLowerCase()) {
            LogError("Invalid wallet backup file: private key does not match address");
            return "";
        }
        const cleanData: LocalWalletData = {
            address: data.address,
            blockchain: data.blockchain,
            createdAt: data.createdAt,
            mnemonic: data.mnemonic,
            privateKey: data.privateKey,
            publicKey: data.publicKey,
        };
        localStorage.setItem(LOCAL_WALLET_KEY, JSON.stringify(cleanData));
        return cleanData.address;
    } catch (e) {
        LogError("Failed to import wallet: " + e);
        return "";
    }
}

export async function localWalletEthereumReconnect(): Promise<void> {
    const address = await localWalletEthereumConnect();
    if (address) {
        LogInfo("Local wallet reconnected: " + address);
    }
}

export async function localWalletEthereumAuthLogin(): Promise<string> {
    const wallet = localWalletEthereumGetWallet();
    if (!wallet) {
        LogError("No local wallet found for login");
        return "no wallet found";
    }
    const csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
    if (!csrfToken || csrfToken === "") {
        LogError("CSRF token not found - localWalletEthereumAuthLogin()");
        return "csrf token not found";
    }
    const address = wallet.address.toLowerCase();
    if (!IsValidBaseAddress(address)) {
        LogError("Invalid address in local wallet");
        return "invalid address";
    }
    const response = await HttpGetJson("/login/nonce");
    if (response[0] != 200) {
        LogError("Failed to get login nonce from server: " + response[1]);
        return "nonce failed";
    }
    const nonce = response[1].nonce;
    const issuedAt = response[1].issuedAt;
    const checksumAddress = getAddress(address);
    LogInfo(`Creating SIWE with: domain=${window.location.host}, address=${checksumAddress}, uri=${window.location.origin}, chainId=${mainnetBase.ethChainID}, nonce=${nonce}, issuedAt=${issuedAt}`);
    const siweMsg = new SiweMessage({
        address: checksumAddress,
        chainId: mainnetBase.ethChainID,
        domain: window.location.host,
        issuedAt: issuedAt,
        nonce: nonce,
        statement: "Sign in to YourPlace",
        uri: window.location.origin,
        version: "1",
    });
    const siweMessage = siweMsg.prepareMessage();
    LogInfo("SIWE message: " + siweMessage);
    let signature: string;
    try {
        signature = await wallet.signMessage(siweMessage);
    } catch (error) {
        LogError("Failed to sign SIWE message: " + error);
        return "sign failed";
    }
    const loginPayload = {
        address: address,
        message: siweMessage,
        signature: signature,
    };
    const response2 = await HttpPostJson("/login/wallet/base/local", loginPayload, csrfToken);
    LogInfo(`Login response: status=${response2[0]}, body=${JSON.stringify(response2[1])}`);
    if (response2[0] != 200) {
        LogError("Failed to login with local wallet: " + JSON.stringify(response2[1]));
        return response2[1] ? response2[1].status : "Unknown error during local wallet login";
    }
    try {
        const status = response2[1].status as string;
        if (status === "Local wallet login success") {
            return "success";
        }
    } catch (e) {
        LogError("Failed to parse login response");
    }
    return "Failed Local Wallet Login: Unknown Error";
}

export async function localWalletEthereumTxn(dest: string, payload: string): Promise<string | undefined> {
    const wallet = localWalletEthereumGetWallet();
    if (!wallet) {
        LogError("localWalletEthereumTxn: No wallet found");
        return undefined;
    }
    try {
        const provider = await getProvider();
        const connectedWallet = wallet.connect(provider);
        const tx = await connectedWallet.sendTransaction({
            data: ethers.hexlify(Buffer.from(payload, "utf8")),
            to: dest,
            value: 0n,
        });
        LogInfo("localWalletEthereumTxn: Transaction sent, hash: " + tx.hash);
        return tx.hash;
    } catch (error: any) {
        LogError("localWalletEthereumTxn failed: " + error);
        if (error?.code === "INSUFFICIENT_FUNDS") {
            const address = wallet.address;
            const addressesParam = encodeURIComponent(JSON.stringify({[address]: ["base"]}));
            const fundUrl = `https://pay.coinbase.com/?appId=yourplace&addresses=${addressesParam}&defaultNetwork=base&defaultAsset=ETH`;
            ShowDialogModalHTMLUnsafe(
                `Your wallet doesn't have enough ETH to pay for transaction fees on the Base network.<br><br>` +
                `<a href="${fundUrl}" target="_blank" rel="noopener noreferrer">Click here to add funds via Coinbase</a>`
            );
        }
        return undefined;
    }
}

export async function localWalletEthereumBurnCollectible(tokenId: bigint): Promise<boolean> {
    const wallet = localWalletEthereumGetWallet();
    if (!wallet) return false;
    try {
        const provider = await getProvider();
        const connectedWallet = wallet.connect(provider);
        const contract = new ethers.Contract(YP_NFT_CONTRACT_ADDRESS, YP_NFT_CONTRACT_ABI, connectedWallet);
        await contract.burn(tokenId);
        return true;
    } catch (error) {
        LogError("localWalletEthereumBurnCollectible failed: " + error);
        return false;
    }
}
export async function localWalletEthereumGetCollectibles(ownerAddress: string): Promise<CollectibleData[]> {
    const results: CollectibleData[] = [];
    try {
        const provider = await getProvider();
        const contract = new ethers.Contract(YP_NFT_CONTRACT_ADDRESS, YP_NFT_CONTRACT_ABI, provider);
        const balance = await contract.balanceOf(ownerAddress);
        for (let i = 0n; i < balance; i++) {
            try {
                const tokenId = await contract.tokenOfOwnerByIndex(ownerAddress, i);
                const tokenUri = await contract.tokenURI(tokenId);
                let metadata: any = {};
                if (tokenUri) {
                    const metadataUrl = tokenUri.startsWith("ipfs://") ? CIDToSubdomainURL(tokenUri) : tokenUri;
                    if (metadataUrl) {
                        const resp = await fetch(metadataUrl);
                        if (resp.ok) metadata = await resp.json();
                    }
                }
                results.push({
                    blockchain: "base",
                    contractAddress: YP_NFT_CONTRACT_ADDRESS,
                    creator: "",
                    description: metadata.description || "",
                    imageUrl: metadata.image || "",
                    mimeType: metadata.image_mimetype || "image/png",
                    name: metadata.name || "Collectible #" + tokenId.toString(),
                    tokenId: tokenId.toString(),
                });
            } catch (innerError) {
                LogError("localWalletEthereumGetCollectibles: error fetching token " + i + ": " + innerError);
            }
        }
    } catch (error) {
        LogError("localWalletEthereumGetCollectibles failed: " + error);
    }
    return results;
}
export async function localWalletEthereumMintCollectible(metadataUri: string): Promise<string | undefined> {
    const wallet = localWalletEthereumGetWallet();
    if (!wallet) return undefined;
    try {
        const provider = await getProvider();
        const connectedWallet = wallet.connect(provider);
        const contract = new ethers.Contract(YP_NFT_CONTRACT_ADDRESS, YP_NFT_CONTRACT_ABI, connectedWallet);
        const tx = await contract.mint(metadataUri, {value: 100000000000000n});
        return tx.hash;
    } catch (error) {
        LogError("localWalletEthereumMintCollectible failed: " + error);
        return undefined;
    }
}
export async function localWalletEthereumTransferCollectible(tokenId: bigint, toAddress: string): Promise<boolean> {
    const wallet = localWalletEthereumGetWallet();
    if (!wallet) return false;
    try {
        const provider = await getProvider();
        const connectedWallet = wallet.connect(provider);
        const contract = new ethers.Contract(YP_NFT_CONTRACT_ADDRESS, YP_NFT_CONTRACT_ABI, connectedWallet);
        await contract.safeTransferFrom(wallet.address, toAddress, tokenId);
        return true;
    } catch (error) {
        LogError("localWalletEthereumTransferCollectible failed: " + error);
        return false;
    }
}
export async function localWalletEthereumFollowUser(toAddress: string, toBlockchain: string): Promise<string | undefined> {
    const jsonData = YP.follow(toAddress, toBlockchain);
    return await localWalletEthereumTxn(toAddress, jsonData);
}
export async function localWalletEthereumSetAvatar(avatarAddress: string): Promise<string | undefined> {
    const jsonData = YP.metadataAvatar(avatarAddress);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSetBanner(bannerAddress: string): Promise<string | undefined> {
    const jsonData = YP.metadataBanner(bannerAddress);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSetColors(colors: Record<string, string>): Promise<string | undefined> {
    const jsonData = YP.metadataColors(colors);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSetDescription(description: string): Promise<string | undefined> {
    const jsonData = YP.metadataDescription(description);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSetLocation(location: string): Promise<string | undefined> {
    const jsonData = YP.metadataLocation(location);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSetName(name: string): Promise<string | undefined> {
    const jsonData = YP.metadataName(name);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSetBot(bot: boolean): Promise<string | undefined> {
    const jsonData = YP.metadataBot(bot);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSetNsfw(nsfw: boolean): Promise<string | undefined> {
    const jsonData = YP.metadataNsfw(nsfw);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSetVertical(vertical: string): Promise<string | undefined> {
    const jsonData = YP.metadataVertical(vertical);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSetWebsite(website: string): Promise<string | undefined> {
    const jsonData = YP.metadataWebsite(website);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSubmitPost(payload: string): Promise<string | undefined> {
    const jsonData = YP.post(payload);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSubmitPostAttach(payload: string, attach: string[][]): Promise<string | undefined> {
    const jsonData = YP.postAttach(payload, attach);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSubmitComment(parentTxHash: string, payload: string): Promise<string | undefined> {
    const jsonData = YP.comment(parentTxHash, payload);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSubmitCommentAttach(parentTxHash: string, payload: string, attach: string[][]): Promise<string | undefined> {
    const jsonData = YP.commentAttach(parentTxHash, payload, attach);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSubmitDislike(targetTxHash: string, targetType: string): Promise<string | undefined> {
    const jsonData = YP.dislike(targetTxHash, targetType);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSubmitEmojiReaction(targetTxHash: string, targetType: string, emoji: string): Promise<string | undefined> {
    const jsonData = YP.emojiReact(targetTxHash, targetType, emoji);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumSubmitLike(targetTxHash: string, targetType: string): Promise<string | undefined> {
    const jsonData = YP.like(targetTxHash, targetType);
    return await localWalletEthereumTxn(mainnetBase.burnAddress, jsonData);
}
export async function localWalletEthereumUnfollowUser(toAddress: string, toBlockchain: string): Promise<string | undefined> {
    const jsonData = YP.unfollow(toAddress, toBlockchain);
    return await localWalletEthereumTxn(toAddress, jsonData);
}
