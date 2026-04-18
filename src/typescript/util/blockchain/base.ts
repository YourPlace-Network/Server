import {DisconnectWallet, GetAddress, IsInsufficientFundsError, OnRampFiat, SetAddress} from "./wallet";
import type {CollectibleData} from "./wallet";
import {LogError, LogInfo} from "../log";
import {HttpGetJson, HttpPostJson} from "../network";
import {CIDToSubdomainURL} from "../ipfs";
import {ensNormalize, ethers} from "ethers";
import {YP} from "../../services/yourplace";
import {createPublicClient, defineChain, getAddress, http as viemHttp, UserRejectedRequestError, toCoinType} from "viem";
import {base as viemBase, mainnet as viemMainnet} from "viem/chains";
import {SiweMessage} from "siwe";
import {
    connect as wagmiConnect,
    createConfig,
    createStorage,
    disconnect,
    getConnections, getEnsAvatar, getEnsName,
    http as wagmiHttp,
    reconnect,
    readContract,
    sendTransaction,
    signMessage,
} from "@wagmi/core";
import {getName as ockGetName, getAvatar as ockGetAvatar, getAddress as ockGetAddress} from "@coinbase/onchainkit/identity";
import {base as wagmiBase} from "@wagmi/core/chains";
import {baseAccount} from "@wagmi/connectors";
import {IsValidBaseAddress, IsValidURL} from "../security";
import {Sleep} from "../time";
import {PersistentCache} from "../cache";
import {setOnchainKitConfig} from "@coinbase/onchainkit";
import { Attribution } from "ox/erc8021"

// ---------- Global Variables ---------- //
export const mainnetBase = {
    ethChainID: 8453,
    name: "Base",
    currency: "ETH",
    explorerUrl: "https://basescan.org",
    rpcUrl: window.location.origin + "/rpc/base",
    // ENS Addresses: https://docs.ens.domains/learn/deployments/
    // https://github.com/base/basenames
    //ensUniversalResolverAddress: "0xce01f8eee7E479C928F8919abD53E553a36CeF67",
    //ensBasenameResolverAddress: "0xC6d566A56A1aFf6508b41f6c90ff131615583BCD",
    //universalResolver: "0xeEeEEEeE14D718C2B47D9923Deab1335E144EeEe",
    //universalResolver: "0x0000000000D8e504002cC26E3Ec46D81971C1664",
    //universalResolver: "0xF29100983E058B709F3D539b0c765937B804AC15",
    //universalResolver: "0xED73a03F19e8D849E44a39252d222c6ad5217E1e",
    //universalResolver: await baseGetUniversalResolverAddress(),
    //universalResolver: "0x91d1777781884d03a6757a803996e38de2a42967fb37eeaca72729271025a9e2",
    //universalResolver: "0xC014B9c02b0EDeA17255Ce019e6ab6c24E4AD073",
    //universalResolver: "0x6533C94869D28fAA8dF77cc63f9e2b2D6Cf77eBA",
    universalResolver: "0xf74b949f2105178eEEd4Ef35a131715E967337ab",
    burnAddress: "0x0000000000000000000000000000000000000000",
}
export const YP_NFT_CONTRACT_ADDRESS = "0xe1F3Af40dfdfcF0aA175225be4Feb970a8864A31" as `0x${string}`;
export const YP_NFT_CONTRACT_ABI = [
    {
        inputs: [{name: "tokenURI", type: "string"}],
        name: "mint",
        outputs: [{name: "", type: "uint256"}],
        stateMutability: "payable",
        type: "function",
    },
    {
        inputs: [],
        name: "mintFee",
        outputs: [{name: "", type: "uint256"}],
        stateMutability: "view",
        type: "function",
    },
    {
        inputs: [{name: "tokenId", type: "uint256"}],
        name: "burn",
        outputs: [],
        stateMutability: "nonpayable",
        type: "function",
    },
    {
        inputs: [{name: "owner", type: "address"}],
        name: "balanceOf",
        outputs: [{name: "", type: "uint256"}],
        stateMutability: "view",
        type: "function",
    },
    {
        inputs: [],
        name: "contractURI",
        outputs: [{name: "", type: "string"}],
        stateMutability: "view",
        type: "function",
    },
    {
        inputs: [{name: "tokenId", type: "uint256"}],
        name: "ownerOf",
        outputs: [{name: "", type: "address"}],
        stateMutability: "view",
        type: "function",
    },
    {
        inputs: [
            {name: "from", type: "address"},
            {name: "to", type: "address"},
            {name: "tokenId", type: "uint256"},
        ],
        name: "safeTransferFrom",
        outputs: [],
        stateMutability: "nonpayable",
        type: "function",
    },
    {
        inputs: [
            {name: "owner", type: "address"},
            {name: "index", type: "uint256"},
        ],
        name: "tokenOfOwnerByIndex",
        outputs: [{name: "", type: "uint256"}],
        stateMutability: "view",
        type: "function",
    },
    {
        inputs: [{name: "tokenId", type: "uint256"}],
        name: "tokenURI",
        outputs: [{name: "", type: "string"}],
        stateMutability: "view",
        type: "function",
    },
] as const;
let prefetchedNonce: {nonce: string, issuedAt: string, fetchedAt: number} | null = null;
const NONCE_PREFETCH_VALIDITY_MS = 300000;
const metadataYourPlace = {
    name: "YourPlace",
    description: "Distributed Social Media",
    url: "https://yourplace.network",
    icons: [
        "https://yourplace.network/static/image/yourplace-logo.svg",
        "https://yourplace.network/static/image/yourplace-logo.png"
    ],
    throttle: 500, // milliseconds
    baseBuilderCode: "bc_w72oslhy",
}
const BASE_WAGMI_STORAGE_KEY = "yourplace.store";
const BASE_WAGMI_RECENT_CONNECTOR_KEY = "yourplace.recentConnectorId";
let baseInit = false;
let viemClient: any;
let wagmiConfig: any;
let ockProvider: any;
const avatarCache = new PersistentCache("base_avatar");
const nameCache = new PersistentCache("base_name");
const descriptionCache = new PersistentCache("base_description");
const ensNameCache = new PersistentCache("base_ens_name");
const ensAvatarCache = new PersistentCache("base_ens_avatar");
const ensAddressCache = new PersistentCache("base_ens_address");
// ensDescriptionCache removed - ENS description fetching not supported

// ---------- Initialization Functions ---------- //
async function initBaseWallet() {
    if (baseInit) { return; }
    try {
        const BASE_DATA_SUFFIX = Attribution.toDataSuffix({
            codes: [metadataYourPlace.baseBuilderCode],
        });
        viemClient = createPublicClient({
            chain: viemBase,
            transport: viemHttp(mainnetBase.rpcUrl!),
        });
        const mainnetClient = createPublicClient({
            chain: viemMainnet,
            transport: viemHttp("/rpc/ethereum"),
        });
        wagmiConfig = createConfig({
            chains: [wagmiBase],
            multiInjectedProviderDiscovery: false,
            connectors: [
                baseAccount({
                    appName: metadataYourPlace.name,
                    appLogoUrl: metadataYourPlace.icons[0],
                })],
            transports: {
                [wagmiBase.id]: wagmiHttp(mainnetBase.rpcUrl!),
            },
            storage: createStorage({
                key: "yourplace",
                storage: window.localStorage,
            }),
            ssr: true,
            dataSuffix: BASE_DATA_SUFFIX,
        });
        setOnchainKitConfig({
            chain: viemBase,
            rpcUrl: mainnetBase.rpcUrl!,
            defaultPublicClients: {
                [viemBase.id]: viemClient,
                [viemMainnet.id]: mainnetClient,
            },
        });
    } catch (e) {
        LogError("Failed to initialize Base wallet: " + e);
        baseInit = false;
        return;
    }
    baseInit = true;
}
initBaseWallet().then();

function clearBaseWalletStorage() {
    localStorage.removeItem(BASE_WAGMI_STORAGE_KEY);
    localStorage.removeItem(BASE_WAGMI_RECENT_CONNECTOR_KEY);
    localStorage.removeItem("wagmi.store");
    localStorage.removeItem("wagmi.recentConnectorId");
}
function getBaseConnectedAddress(): string {
    const address = getConnections(wagmiConfig)[0]?.accounts?.[0]?.toString() || "";
    if (address !== "" && IsValidBaseAddress(address)) {
        return address;
    }
    return "";
}
function syncBaseStoredAddress(address: string): string {
    if (address !== "" && GetAddress() !== address) {
        SetAddress(address);
    }
    return address;
}
async function restoreBaseConnection(): Promise<string> {
    const connector = wagmiConfig?.connectors?.[0];
    if (!connector) {
        return "";
    }
    try {
        await reconnect(wagmiConfig, {
            connectors: [connector],
        });
    } catch (_) {}
    return syncBaseStoredAddress(getBaseConnectedAddress());
}

// ---------- Core Wallet Functions ---------- //
export async function basePrefetchLoginNonce(): Promise<void> {
    try {
        const response = await HttpGetJson("/login/nonce");
        if (response[0] === 200) {
            prefetchedNonce = {
                nonce: response[1].nonce,
                issuedAt: response[1].issuedAt,
                fetchedAt: Date.now(),
            };
        }
    } catch (e) {
        LogError("Failed to pre-fetch login nonce: " + e);
    }
}
export async function baseAuthLogin(): Promise<string> {
    // RET: string - "success" or error message or ""
    if (!baseInit) {
        await initBaseWallet();
    }
    let address = syncBaseStoredAddress(getBaseConnectedAddress());
    if (address === "") {
        address = await baseReconnectWallet();
    }
    if (address === "") {
        address = await baseConnectWallet();
    }
    let csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
    if (!csrfToken || csrfToken === "") {
        LogError("CSRF token not found - baseAuthLogin()");
        return "csrf token not found";
    }
    if (!address || address === "" || !IsValidBaseAddress(address)) {
        LogError("Invalid Base address - baseAuthLogin()");
        return "invalid address";
    }
    let nonce: string;
    let issuedAt: string;
    if (prefetchedNonce && (Date.now() - prefetchedNonce.fetchedAt) < NONCE_PREFETCH_VALIDITY_MS) {
        nonce = prefetchedNonce.nonce;
        issuedAt = prefetchedNonce.issuedAt;
        prefetchedNonce = null;
    } else {
        const response = await HttpGetJson("/login/nonce");
        if (response[0] != 200) {
            LogError("Failed to get login nonce from server: " + response[1]);
            return "nonce failed";
        }
        nonce = response[1].nonce;
        issuedAt = response[1].issuedAt;
    }
    const checksumAddress = getAddress(address);
    const siweMsg = new SiweMessage({
        domain: window.location.host,
        address: checksumAddress,
        statement: "Sign in to YourPlace",
        uri: window.location.origin,
        version: "1",
        chainId: mainnetBase.ethChainID,
        nonce: nonce,
        issuedAt: issuedAt,
    });
    const siweMessage = siweMsg.prepareMessage();
    let signature: any;
    try {
        signature = await signMessage(wagmiConfig, {
            account: address as `0x${string}`,
            message: siweMessage,
        });
    } catch(error: any) {
        if (error?.message?.toLowerCase().includes("popup") || error?.message?.toLowerCase().includes("blocked")) {
            LogError("Popup was blocked by browser - please allow popups for this site");
            return "popup_blocked";
        }
        LogError("Failed to sign SIWE message: " + error?.message);
        return "sign failed";
    }
    let loginPayload = {
        message: siweMessage,
        address: address,
        signature: signature,
    };
    const response2 = await HttpPostJson("/login/wallet/base", loginPayload, csrfToken);
    // Handle undeployed smart wallet - prompt user to deploy
    if (response2[0] === 428 && response2[1]?.status === "wallet_not_deployed") {
        LogInfo("Smart wallet not deployed, prompting user to deploy...");
        const {ShowDialogModalWithCallback} = await import("../../components/modalDialog");
        ShowDialogModalWithCallback(
            "Your Coinbase Smart Wallet needs to be deployed before you can sign in. Click OK to open the Coinbase wallet deployment page.",
            () => {
                window.open("https://keys.coinbase.com/settings/deploy-wallet", "_blank");
                window.location.href = "/login";
            }
        );
        return "wallet_not_deployed";
    }
    if (response2[0] != 200) {
        LogError("Failed to login with Base: " + JSON.stringify(response2[1]));
        await Sleep(3000);
        await DisconnectWallet();
        return response2[1] ? response2[1].status : "Unknown error during Base login";
    }
    try {
        let status = response2[1].status as string;
        if (status === "Base wallet login success") {
            return "success";
        }
    } catch (e) {
        LogError("Failed to parse login response");
    }
    return "Failed Base Login: Unknown Error";
}
export async function baseConnectWallet(): Promise<string> {
    if (!baseInit) {
        await initBaseWallet();
    }
    try {
        let address = syncBaseStoredAddress(getBaseConnectedAddress());
        if (address !== "") {
            return address;
        }
        address = await restoreBaseConnection();
        if (address !== "") {
            return address;
        }
        if (getConnections(wagmiConfig).length > 0) {
            await disconnect(wagmiConfig);
        }
        clearBaseWalletStorage(); // https://github.com/wevm/wagmi/issues/3425
        const {accounts} = await wagmiConnect(wagmiConfig, {
            chainId: wagmiBase.id,
            connector: wagmiConfig.connectors[0],
        });
        address = accounts[0].toString();
        if (!address || address === "" || !IsValidBaseAddress(address)) {
            LogError("Failed to connect to Base Wallet: Invalid address returned");
            return "";
        }
        return syncBaseStoredAddress(address);
    } catch (error: unknown) {
        if (error instanceof Error) {
            if (error instanceof UserRejectedRequestError) {
                LogInfo("User declined to connect wallet");
            } else {
                LogError("Failed to connect to Base: " + error);
            }
            LogError("Unknown error occurred when attempting to connect wallet");
        }
        return "";
    }
}
export async function baseReconnectWallet(): Promise<string> {
    if (!baseInit) {
        await initBaseWallet();
    }
    return await restoreBaseConnection();
}
export async function baseDisconnectWallet(): Promise<void> {
    await disconnect(wagmiConfig);
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
export async function baseIsWalletConnected(): Promise<boolean> {
    if (!baseInit) {
        await initBaseWallet();
    }
    const address = getBaseConnectedAddress();
    if (address !== "") {
        syncBaseStoredAddress(address);
        return true;
    }
    try {
        const accounts = await wagmiConfig?.connectors?.[0]?.getAccounts();
        const accountAddress = accounts?.[0]?.toString() || "";
        if (accountAddress !== "" && IsValidBaseAddress(accountAddress)) {
            syncBaseStoredAddress(accountAddress);
            return true;
        }
    } catch (_) {}
    return false;
}
export async function baseTxn(dest: string, payload: string) {
    if (!baseInit) {
        await initBaseWallet();
    }
    let address = "";
    try {
        address = syncBaseStoredAddress(getBaseConnectedAddress());
        if (address === "") {
            address = await baseReconnectWallet();
        }
        if (address === "") {
            address = await baseConnectWallet();
        }
        if (!address) {
            LogError("baseTxn: No address found");
            return;
        }
        const connections = getConnections(wagmiConfig);
        LogInfo("baseTxn: Current connections: " + connections.length);
        if (!connections.length) {
            LogError("baseTxn: Failed to connect to Base Wallet");
            return;
        }
        const connector = connections[0]?.connector;
        LogInfo("baseTxn: Using connector: " + connector?.name + ", address: " + address + ", dest: " + dest);
        const txHash = await sendTransaction(wagmiConfig, {
            account: address as `0x${string}`,
            to: dest as `0x${string}`,
            value: BigInt(0),
            data: ethers.hexlify(Buffer.from(payload, "utf8")) as `0x${string}`,
            connector: connector,
        });
        LogInfo("baseTxn: Transaction sent successfully, hash: " + txHash);
        return txHash;
    } catch (error: unknown) {
        if (IsInsufficientFundsError(error)) { OnRampFiat(address, "base"); return; }
        if (error instanceof Error) {
            LogError("baseTxn failed: " + error.message);
            if (error.stack) {
                LogError("baseTxn stack: " + error.stack);
            }
        } else {
            LogError("baseTxn failed with unknown error: " + String(error));
        }
    }
}

// ---------- Set Functions ---------- //
export async function baseSetAvatar(avatarAddress: string) {
    let jsonData = YP.metadataAvatar(avatarAddress);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetBanner(bannerAddress: string) {
    let jsonData = YP.metadataBanner(bannerAddress);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetColors(colors: Record<string, string>) {
    let jsonData = YP.metadataColors(colors);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetDescription(description: string) {
    let jsonData = YP.metadataDescription(description);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetLocation(location: string) {
    let jsonData = YP.metadataLocation(location);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetMusicEmbed(music: string) {
    let jsonData = YP.metadataMusic(music);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetWebsite(website: string) {
    let jsonData = YP.metadataWebsite(website);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetName(name: string) {
    let jsonData = YP.metadataName(name);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetBot(bot: boolean) {
    let jsonData = YP.metadataBot(bot);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetNsfw(nsfw: boolean) {
    let jsonData = YP.metadataNsfw(nsfw);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetVertical(vertical: string) {
    let jsonData = YP.metadataVertical(vertical);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSubmitPost(payload: string) {
    let jsonData = YP.post(payload);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSubmitPostAttach(payload: string, attach: string[][]) {
    let jsonData = YP.postAttach(payload, attach);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseFollowUser(toAddress: string, toBlockchain: string) {
    let jsonData = YP.follow(toAddress, toBlockchain);
    return await baseTxn(toAddress, jsonData);
}
export async function baseUnfollowUser(toAddress: string, toBlockchain: string) {
    let jsonData = YP.unfollow(toAddress, toBlockchain);
    return await baseTxn(toAddress, jsonData);
}
export async function baseSubmitComment(parentTxHash: string, payload: string) {
    let jsonData = YP.comment(parentTxHash, payload);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSubmitCommentAttach(parentTxHash: string, payload: string, attach: string[][]) {
    let jsonData = YP.commentAttach(parentTxHash, payload, attach);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSubmitDislike(targetTxHash: string, targetType: string) {
    let jsonData = YP.dislike(targetTxHash, targetType);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSubmitEmojiReaction(targetTxHash: string, targetType: string, emoji: string) {
    let jsonData = YP.emojiReact(targetTxHash, targetType, emoji);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSubmitLike(targetTxHash: string, targetType: string) {
    let jsonData = YP.like(targetTxHash, targetType);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}

// ---------- Get Functions ---------- //
export async function baseGetAvatar(address: string): Promise<string> {
    if (!baseInit) await initBaseWallet();
    if (!IsValidBaseAddress(address)) {
        LogError("Invalid Base address provided to baseGetAvatar: " + address);
        return "";
    }
    const cached = avatarCache.get<string>(address);
    if (cached !== null) {
        return cached;
    }
    try {
        const response = await HttpGetJson("/profile/avatar/base/" + address);
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
    } catch (error) {
        LogError("Failed to get local avatar: " + error);
    }
    try {
        const ensAvatar = await baseGetEnsAvatar(address);
        if (ensAvatar && ensAvatar !== "") {
            avatarCache.set(address, ensAvatar);
            return ensAvatar;
        }
    } catch (_) {}
    return "";
}
export async function baseGetName(_address: string): Promise<string> {
    if (!baseInit) await initBaseWallet();
    if (!IsValidBaseAddress(_address)) {
        LogError("Invalid Base address provided to baseGetName: " + _address);
        return "";
    }
    const cached = nameCache.get<string>(_address);
    if (cached !== null) {
        return cached;
    }
    try {
        const response = await HttpGetJson("/profile/name/base/" + _address);
        if (response[0] === 200 && response[1] && response[1].name) {
            const name = response[1].name.trim();
            if (name.length > 0) {
                nameCache.set(_address, name);
                return name;
            }
        }
        const ensName = await baseGetEnsName(_address);
        if (ensName && ensName !== "") {
            nameCache.set(_address, ensName);
            return ensName;
        }
    } catch (error) {
        LogError("Failed to get Base name: " + error);
    }
    return "";
}
export async function baseGetDescription(_address: string): Promise<string> {
    if (!baseInit) await initBaseWallet();
    if (!IsValidBaseAddress(_address)) {
        LogError("Invalid Base address provided to baseGetDescription: " + _address);
        return "";
    }
    const cached = descriptionCache.get<string>(_address);
    if (cached !== null) {
        return cached;
    }
    try {
        const response = await HttpGetJson("/profile/description/base/" + _address);
        if (response[0] === 200 && response[1] && response[1].description && response[1].description !== "") {
            descriptionCache.set(_address, response[1].description.trim());
            return response[1].description.trim();
        }
        // ENS description fetching disabled - not supported right now
    } catch (error) {
        LogError("Failed to get description: " + error);
    }
    return "";
}
export async function baseBurnCollectible(tokenId: bigint): Promise<boolean> {
    if (!baseInit) await initBaseWallet();
    try {
        let connections = getConnections(wagmiConfig);
        if (!connections.length) {
            await baseConnectWallet();
            connections = getConnections(wagmiConfig);
            if (!connections.length) return false;
        }
        const connector = connections[0]?.connector;
        const provider = await connector?.getProvider() as { request: (args: { method: string; params: unknown[] }) => Promise<string> } | undefined;
        if (!provider) return false;
        const iface = new ethers.Interface(YP_NFT_CONTRACT_ABI);
        const data = iface.encodeFunctionData("burn", [tokenId]);
        await provider.request({
            method: "eth_sendTransaction",
            params: [{
                from: GetAddress() as `0x${string}`,
                to: YP_NFT_CONTRACT_ADDRESS,
                value: "0x0",
                data: data as `0x${string}`,
            }],
        });
        return true;
    } catch (error) {
        if (IsInsufficientFundsError(error)) { OnRampFiat(GetAddress()!, "base"); return false; }
        LogError("baseBurnCollectible failed: " + error);
        return false;
    }
}
export async function baseGetCollectibles(ownerAddress: string): Promise<CollectibleData[]> {
    if (!baseInit) await initBaseWallet();
    const results: CollectibleData[] = [];
    try {
        const balance = await readContract(wagmiConfig, {
            address: YP_NFT_CONTRACT_ADDRESS,
            abi: YP_NFT_CONTRACT_ABI,
            functionName: "balanceOf",
            args: [ownerAddress as `0x${string}`],
        }) as bigint;
        for (let i = 0n; i < balance; i++) {
            try {
                const tokenId = await readContract(wagmiConfig, {
                    address: YP_NFT_CONTRACT_ADDRESS,
                    abi: YP_NFT_CONTRACT_ABI,
                    functionName: "tokenOfOwnerByIndex",
                    args: [ownerAddress as `0x${string}`, i],
                }) as bigint;
                const tokenUri = await readContract(wagmiConfig, {
                    address: YP_NFT_CONTRACT_ADDRESS,
                    abi: YP_NFT_CONTRACT_ABI,
                    functionName: "tokenURI",
                    args: [tokenId],
                }) as string;
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
                LogError("baseGetCollectibles: error fetching token " + i + ": " + innerError);
            }
        }
    } catch (error) {
        LogError("baseGetCollectibles failed: " + error);
    }
    return results;
}
export async function baseGetTransferFeeEstimate(toAddress: string, tokenId: bigint): Promise<string> {
    if (!baseInit) await initBaseWallet();
    try {
        const gasEstimate = await viemClient.estimateContractGas({
            address: YP_NFT_CONTRACT_ADDRESS,
            abi: YP_NFT_CONTRACT_ABI,
            functionName: "safeTransferFrom",
            args: [GetAddress() as `0x${string}`, toAddress as `0x${string}`, tokenId],
            account: GetAddress() as `0x${string}`,
        });
        const gasPrice = await viemClient.getGasPrice();
        const feeWei = gasEstimate * gasPrice;
        const feeEth = Number(feeWei) / 1e18;
        return feeEth.toFixed(6) + " ETH";
    } catch (error) {
        LogError("baseGetTransferFeeEstimate failed: " + error);
        return "-- ETH";
    }
}
export async function baseMintCollectible(metadataUri: string): Promise<string | undefined> {
    if (!baseInit) await initBaseWallet();
    try {
        let connections = getConnections(wagmiConfig);
        if (!connections.length) {
            await baseConnectWallet();
            connections = getConnections(wagmiConfig);
            if (!connections.length) return undefined;
        }
        const connector = connections[0]?.connector;
        const provider = await connector?.getProvider() as { request: (args: { method: string; params: unknown[] }) => Promise<string> } | undefined;
        if (!provider) return undefined;
        const iface = new ethers.Interface(YP_NFT_CONTRACT_ABI);
        const data = iface.encodeFunctionData("mint", [metadataUri]);
        const txHash = await provider.request({
            method: "eth_sendTransaction",
            params: [{
                from: GetAddress() as `0x${string}`,
                to: YP_NFT_CONTRACT_ADDRESS,
                value: "0x5AF3107A4000",
                data: data as `0x${string}`,
            }],
        });
        return txHash;
    } catch (error) {
        if (IsInsufficientFundsError(error)) { OnRampFiat(GetAddress()!, "base"); return undefined; }
        LogError("baseMintCollectible failed: " + error);
        return undefined;
    }
}
export async function baseTransferCollectible(tokenId: bigint, toAddress: string): Promise<boolean> {
    if (!baseInit) await initBaseWallet();
    try {
        let connections = getConnections(wagmiConfig);
        if (!connections.length) {
            await baseConnectWallet();
            connections = getConnections(wagmiConfig);
            if (!connections.length) return false;
        }
        const connector = connections[0]?.connector;
        const provider = await connector?.getProvider() as { request: (args: { method: string; params: unknown[] }) => Promise<string> } | undefined;
        if (!provider) return false;
        const iface = new ethers.Interface(YP_NFT_CONTRACT_ABI);
        const data = iface.encodeFunctionData("safeTransferFrom", [GetAddress(), toAddress, tokenId]);
        await provider.request({
            method: "eth_sendTransaction",
            params: [{
                from: GetAddress() as `0x${string}`,
                to: YP_NFT_CONTRACT_ADDRESS,
                value: "0x0",
                data: data as `0x${string}`,
            }],
        });
        return true;
    } catch (error) {
        if (IsInsufficientFundsError(error)) { OnRampFiat(GetAddress()!, "base"); return false; }
        LogError("baseTransferCollectible failed: " + error);
        return false;
    }
}

// ---------- ENS Functions ---------- //
export async function baseGetEnsName(address: string): Promise<string> {
    if (!baseInit) await initBaseWallet();
    const cached = ensNameCache.get<string>(address);
    if (cached !== null) {
        return cached;
    }
    try {
        const ensName = await ockGetName({address: address as `0x${string}`, chain: viemBase});
        if (ensName) {
            ensNameCache.set(address, ensName);
            return ensName;
        }
    } catch (_) {}
    ensNameCache.set(address, "");
    return "";
}
export async function baseGetEnsAvatar(address: string): Promise<string> {
    if (!baseInit) await initBaseWallet();
    const cached = ensAvatarCache.get<string>(address);
    if (cached !== null) {
        return cached;
    }
    const ensName = await baseGetEnsName(address);
    if (!ensName || ensName === "") {
        ensAvatarCache.set(address, "");
        return "";
    }
    try {
        const ensAvatar = await ockGetAvatar({ensName, chain: viemBase});
        if (ensAvatar) {
            ensAvatarCache.set(address, ensAvatar);
            return ensAvatar;
        }
    } catch (_) {}
    ensAvatarCache.set(address, "");
    return "";
}
export async function baseGetEnsAddress(ensName: string): Promise<string> {
    const cached = ensAddressCache.get<string>(ensName);
    if (cached !== null) {
        return cached;
    }
    try {
        const ensAddress = await ockGetAddress({name: ensName});
        if (ensAddress) {
            ensAddressCache.set(ensName, ensAddress);
            return ensAddress;
        }
    } catch (_) {}
    ensAddressCache.set(ensName, "");
    return "";
}
// baseGetEnsDescription removed - ENS description/text fetching not supported right now
async function baseGetUniversalResolverAddress(): Promise<string> {
    try {
        const response = await fetch("https://raw.githubusercontent.com/ensdomains/ens-contracts/refs/heads/staging/deployments/mainnet/UniversalResolver.json");
        if (!response.ok) {
            LogError("Failed to fetch ENS Universal Resolver address: " + response.status);
            return "";
        }
        const data = await response.json();
        if (data && data.address) {
            return data.address;
        }
    } catch (error) {
        LogError("Failed to fetch ENS Universal Resolver address: " + error);
    }
    return "";
}
