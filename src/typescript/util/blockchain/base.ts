import {DisconnectWallet, GetAddress} from "./wallet";
import {LogError, LogInfo} from "../log";
import {HttpGetJson, HttpPostJson} from "../network";
import {ethers} from "ethers";
import {YP} from "../../services/yourplace";
import {createPublicClient, defineChain, http as viemHttp, parseEther, UserRejectedRequestError} from "viem";
import {normalize as viemNormalize} from "viem/ens";
import {base as viemBase} from "viem/chains";
import {
    connect as wagmiConnect,
    createConfig,
    createStorage,
    disconnect,
    getConnections,
    type GetEnsAvatarReturnType,
    http as wagmiHttp,
    readContract,
    sendTransaction,
    signMessage,
} from "@wagmi/core";
import {base as wagmiBase} from "@wagmi/core/chains";
import {coinbaseWallet} from "@wagmi/connectors";
import {getName as cbGetName, getAvatar as cbGetAvatar} from "@coinbase/onchainkit/identity";
import {IsValidBaseAddress} from "../security";
import {Sleep} from "../time";
import {Web3} from "web3";
import PersistentCache from "../cache";

// ---------- Global Variables ---------- //
export const mainnetBase = {
    ethChainID: 8453,
    name: "Base",
    currency: "ETH",
    explorerUrl: "https://basescan.org",
    mainnetURL: "https://mainnet.base.org",
    rpcUrl: await baseGetURL()!,
    ensUniversalResolverAddress: "0xce01f8eee7E479C928F8919abD53E553a36CeF67",
    ensBasenameResolverAddress: "0xC6d566A56A1aFf6508b41f6c90ff131615583BCD",
    ensBaseResolverAddress: "0xC6d566A56A1aFf6508b41f6c90ff131615583BCD",
}
const metadataYourPlace = {
    name: "YourPlace",
    description: "Distributed Social Media",
    url: "https://yourplace.network",
    icons: [
        "https://yourplace.network/static/image/yourplace-logo.svg",
        "https://yourplace.network/static/image/yourplace-logo.png"
    ],
    throttle: 500, // milliseconds
}
const baseEnsCache = new PersistentCache({
    defaultTtl: 3600000, // 1 hour
    keyPrefix: "baseEnsCache_"
});
const baseAvatarCache = new PersistentCache({
    defaultTtl: 3600000, // 1 hour
    keyPrefix: "baseAvatarCache_"
});
const pendingOnchainkitRequests = new Map<string, Promise<string>>();
const pendingAvatarRequests = new Map<string, Promise<string>>();
let baseInit = false;
let viemClient: any;
let wagmiConfig: any;
let web3Client: Web3;

// ---------- Initialization Functions ---------- //
async function initBaseWallet() {
    if (baseInit) { return; }
    try {
        viemClient = createPublicClient({
            transport: viemHttp(mainnetBase.rpcUrl!, {retryCount: 10, retryDelay: 1000}),
            chain: defineChain(viemBase),
        });
        wagmiConfig = createConfig({
            chains: [wagmiBase],
            connectors: [coinbaseWallet({
                appName: metadataYourPlace.name,
                appLogoUrl: metadataYourPlace.icons[1],
                preference: {
                    options: "eoaOnly"
                },
            })],
            transports: {
                [wagmiBase.id]: wagmiHttp(mainnetBase.rpcUrl!, {retryCount: 10, retryDelay: 1000}),
            },
            storage: createStorage({
                key: "yourplace",
                storage: window.localStorage,
            }),
            ssr: true,
        });
        web3Client = new Web3(mainnetBase.rpcUrl!);
    } catch (e) {
        LogError("Failed to initialize Base wallet: " + e);
        baseInit = false;
        return;
    }
    baseInit = true;
}
initBaseWallet().then();

// ---------- Core Wallet Functions ---------- //
export async function baseAuthLogin(): Promise<string> {
    // RET: string - "success" or error message or ""
    if (!baseInit) {
        LogError("Base wallet not initialized - baseAuthLogin()");
        await initBaseWallet();
        await baseConnectWallet();
    }
    let csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
    if (!csrfToken || csrfToken === "") {
        LogError("CSRF token not found - baseAuthLogin()");
        return "csrf token not found";
    }
    let address = GetAddress()!;
    if (!address || address === "" || !IsValidBaseAddress(address)) {
        LogError("Invalid Base address - baseAuthLogin()");
        return "invalid address";
    }
    const response = await HttpGetJson("/login/nonce");
    if (response[0] != 200) {
        LogError("Failed to get login nonce from server: " + response[1]);
        return "nonce failed";
    }
    let nonce = response[1].nonce;
    let payload = `0x${Buffer.from(nonce, "utf8").toString("hex")}`;
    let signature: any;
    try {
        signature = await signMessage(wagmiConfig, {
            account: address as `0x${string}`,
            message: nonce,
        });
    } catch(error) {
        LogError("Failed to sign Base login message");
        return "sign failed";
    }
    let loginPayload = {
        payload: payload,
        address: address,
        signature: signature,
    };
    const response2 = await HttpPostJson("/login/wallet/base", loginPayload, csrfToken);
    if (response2[0] != 200) {
        LogError("Failed to login with Base");
        await Sleep(3000);
        await DisconnectWallet();
        return response2[1] ? response[1].status : "Unknown error during Base login";
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
        // Check if already connected and get existing connection
        const connections = getConnections(wagmiConfig);
        if (connections.length > 0) {
            // Get the address from the existing connection
            const accounts = connections[0].accounts;
            if (accounts && accounts.length > 0) {
                const _address = accounts[0].toString();
                if (_address && _address !== "" && IsValidBaseAddress(_address)) {
                    return _address;
                }
            }
            // If we can't get a valid address, disconnect first
            await disconnect(wagmiConfig);
            localStorage.removeItem("wagmi.store"); // https://github.com/wevm/wagmi/issues/3425
        } else {
            localStorage.removeItem("wagmi.store"); // https://github.com/wevm/wagmi/issues/3425
        }
        // Now connect fresh
        const {accounts} = await wagmiConnect(wagmiConfig, {
            chainId: wagmiBase.id,
            connector: wagmiConfig.connectors[0],
        });
        let _address = accounts[0].toString();
        if (!_address || _address === "" || !IsValidBaseAddress(_address)) {
            LogError("Failed to connect to Base Wallet: Invalid address returned");
            return "";
        }
        return _address;
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
export async function baseDisconnectWallet(): Promise<void> {
    await disconnect(wagmiConfig);
    localStorage.clear();
}
export async function baseIsWalletConnected(): Promise<boolean> {
    // todo: check if wallet is connected
    LogInfo("finish implementing baseIsWalletConnected()");
    return false
}
export async function baseTxn(dest: string, payload: string) {
    let address = GetAddress();
    if (!address) { return; }
    if (!baseInit) {
        await initBaseWallet();
    }
    try {
        const connections = getConnections(wagmiConfig);
        if (!connections.length) {
            await baseConnectWallet();
            const newConnections = getConnections(wagmiConfig);
            if (!newConnections.length) {
                LogError("Failed to connect to Base Wallet to perform a baseTxn");
                return;
            }
        }
        const txHash = await sendTransaction(wagmiConfig, {
            account: address as `0x${string}`,
            to: dest as `0x${string}`,
            value: parseEther("0"),
            data: ethers.hexlify(Buffer.from(payload, "utf8")) as `0x${string}`,
            connector: connections[0]?.connector,
            chainId: wagmiBase.id,
        });
        return txHash;
    } catch (error) {
        LogError("Failed to send Base transaction: " + error);
    }
}

// ---------- Set Functions ---------- //
export async function baseSetAvatar(avatarAddress: string) {
    let jsonData = YP.metadataAvatar(avatarAddress);
    let address = GetAddress()!;
    baseTxn(address, jsonData).then();
}
export async function baseSetBanner(bannerAddress: string) {
    let jsonData = YP.metadataBanner(bannerAddress);
    let address = GetAddress()!;
    baseTxn(address, jsonData).then();
}
export async function baseSetDescription(description: string) {
    let jsonData = YP.metadataDescription(description);
    let address = GetAddress()!;
    baseTxn(address, jsonData).then();
}
export async function baseSetLocation(location: string) {
    let jsonData = YP.metadataLocation(location);
    let address = GetAddress()!;
    baseTxn(address, jsonData).then();
}
export async function baseSetWebsite(website: string) {
    let jsonData = YP.metadataWebsite(website);
    let address = GetAddress()!;
    baseTxn(address, jsonData).then();
}
export async function baseSetBirthday(birthday: string) {
    let jsonData = YP.metadataBirthday(birthday);
    let address = GetAddress()!;
    baseTxn(address, jsonData).then();
}
export async function baseSetName(name: string) {
    let jsonData = YP.metadataName(name);
    let address = GetAddress()!;
    baseTxn(address, jsonData).then();
}
export async function baseSubmitPost(payload: string) {
    let address = GetAddress()!;
    let jsonData = YP.post(payload);
    const txnID = await baseTxn(address, jsonData);
    return txnID;
}
export async function baseSubmitPostAttach(payload: string, attach: string[][]) {
    let address = GetAddress()!;
    let jsonData = YP.postAttach(payload, attach);
    return await baseTxn(address, jsonData);
}
export async function baseFollowUser(toAddress: string, toBlockchain: string) {
    let address = GetAddress()!;
    let jsonData = YP.follow(toAddress, toBlockchain);
    const txnID = await baseTxn(address, jsonData);
    return txnID;
}

// ---------- Get Functions ---------- //
async function baseGetURL(): Promise<string|null> {
    let response = await HttpGetJson("/settings/base/url");
    if (response[0] === 200) {
        return response[1].baseURL;
    }
    return null;
}
export async function baseGetAvatar(address: string): Promise<string> {
    if (!baseInit) await initBaseWallet();
    if (!IsValidBaseAddress(address)) {
        LogError("Invalid Base address provided to baseGetAvatar: " + address);
        return "";
    }
    // Check cache first
    const cached = baseAvatarCache.get(address);
    if (cached !== null) return cached as string;
    // Check if request is already in progress
    if (pendingAvatarRequests.has(address)) return await pendingAvatarRequests.get(address)!;
    // Create new request
    const requestPromise = performBaseAvatarRequest(address);
    pendingAvatarRequests.set(address, requestPromise);
    try {
        return await requestPromise;
    } finally {
        pendingAvatarRequests.delete(address);
    }
}
export async function baseGetName(_address: string): Promise<string> {
    // https://gist.github.com/hughescoin/95b680619d602782396fa954e981adae
    if (!baseInit) await initBaseWallet();
    if (!IsValidBaseAddress(_address)) {
        LogError("Invalid Base address provided to baseGetName: " + _address);
        return "";
    }
    // Check cache first
    const cached = baseEnsCache.get(_address);
    if (cached !== null) return cached as string;
    // Check if request is already in progress
    if (pendingOnchainkitRequests.has(_address)) return await pendingOnchainkitRequests.get(_address)!;
    // Create new request
    const requestPromise = performBasenameRequest(_address);
    pendingOnchainkitRequests.set(_address, requestPromise);
    try {
        return await requestPromise;
    } finally {
        pendingOnchainkitRequests.delete(_address);
    }
}
export async function baseGetENSText(_address: string, key: string): Promise<string> {
    if (viemClient === null || !viemClient || !baseInit) {
        await initBaseWallet();
    }
    let baseName = await baseGetName(_address);
    let textRecord = await viemClient!.getEnsText({
        name: baseName,
        key: key,
        universalResolverAddress: mainnetBase.ensBaseResolverAddress,
    });
    if (textRecord) {
        return textRecord;
    }
    return "";
}
export async function baseGetNFTs(_address: string) {
    const minimalERC721ABI = [
        {
            inputs: [{ name: 'owner', type: 'address' }],
            name: 'balanceOf',
            outputs: [{ name: '', type: 'uint256' }],
            stateMutability: 'view',
            type: 'function'
        },
        {
            inputs: [
                { name: 'owner', type: 'address' },
                { name: 'index', type: 'uint256' }
            ],
            name: 'tokenOfOwnerByIndex',
            outputs: [{ name: '', type: 'uint256' }],
            stateMutability: 'view',
            type: 'function'
        }
    ] as const;
    try {
        const balance = await readContract(wagmiConfig, { // get balance of NFTs
            address: _address as `0x${string}`,
            abi: minimalERC721ABI,
            functionName: 'balanceOf',
            args: [_address as `0x${string}`],
        }) as bigint;
        const tokenIds = []; // Get all token IDs
        for (let i =0; i < balance; i++) {
            const tokenId = await readContract(wagmiConfig, {
                address: _address as `0x${string}`,
                abi: minimalERC721ABI,
                functionName: 'tokenOfOwnerByIndex',
                args: [_address as `0x${string}`, BigInt(i)],
            });
            tokenIds.push(tokenId);
        }
        let response =  {balance, tokenIds};
        console.log(response);
        return response;
    } catch (error) {
        LogError("Failed to get NFTs: " + error);
    }
}

// ---------- ENS Functions ---------- //
async function performBaseAvatarRequest(address: string): Promise<string> {
    let baseName = await baseGetName(address);
    if (!baseName || baseName === "") {
        LogInfo("No Base name found for address, skipping avatar lookup: " + address);
        return "";
    }
    try {
        const ensAvatar = await cbGetAvatar({ensName: baseName, chain: wagmiBase});
        if (!ensAvatar || ensAvatar === "") {
            LogInfo("No ENS Avatar found for Base address: " + address);
            baseAvatarCache.set(address, ""); // Cache empty result to avoid repeated failed lookups
            return "";
        }
        baseAvatarCache.set(address, ensAvatar);
        return ensAvatar as string;
    } catch (error) {
        LogError("Failed to get Base ENS avatar: " + error);
        baseAvatarCache.set(address, ""); // Cache empty result to avoid repeated failed lookups
        return "";
    }
}
async function performBasenameRequest(_address: string): Promise<string> {
    try {
        const name = await cbGetName({address: _address as `0x${string}`, chain: wagmiBase});
        if (name && name !== "") {
            const basename = name.toString();
            baseEnsCache.set(_address, basename);
            LogInfo("Cached new basename for address: " + _address + " -> " + basename);
            return basename;
        }
    } catch (error) {
        LogError("Failed to get Base ENS name: " + error);
    }
    LogInfo("No Base name found for address: " + _address);
    // Cache empty result to avoid repeated failed lookups
    baseEnsCache.set(_address, "");
    return "";

    // Viem
    /*try {
        const addressReverseNode = convertReverseNodeToBytes(_address as `0x${string}`, viemBase.id);
        const basename = await viemClient.readContract({
            abi: L2ResolverAbi,
            address: mainnetBase.ensBasenameResolverAddress as `0x${string}`,
            functionName: 'name',
            args: [addressReverseNode],
        });
        if (basename && basename !== "") {
            return basename as string;
        }
    } catch (error) {
        LogError("Failed to get Base ENS name 2: " + error);
    }*/
    // Wagmi
    /*try {
        const ensName = await fetchEnsName(wagmiConfig, {
            address: _address as `0x${string}`,
            chainId: wagmiBase.id,
            universalResolverAddress: mainnetBase.ensUniversalResolverAddress as `0x${string}`,
        });
        if (ensName && ensName !== "") {
            return ensName;
        }
    } catch (error) {
        LogError("Failed to get Base ENS name 3: " + error);
    }*/
}