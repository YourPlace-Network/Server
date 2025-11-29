import {DisconnectWallet, GetAddress} from "./wallet";
import {LogError, LogInfo} from "../log";
import {HttpGetJson, HttpPostJson} from "../network";
import {CIDToSubdomainURL} from "../ipfs";
import {IsGatewayMode} from "../miscellaneous";
import {ethers} from "ethers";
import {YP} from "../../services/yourplace";
import {createPublicClient, defineChain, getAddress, http as viemHttp, parseEther, UserRejectedRequestError} from "viem";
import {normalize} from "viem/ens";
import {base as viemBase} from "viem/chains";
import {SiweMessage} from "siwe";
import {
    connect as wagmiConnect,
    createConfig,
    createStorage,
    disconnect,
    getConnections,
    http as wagmiHttp,
    readContract,
    signMessage,
} from "@wagmi/core";
import {base as wagmiBase} from "@wagmi/core/chains";
import {baseAccount, coinbaseWallet} from "@wagmi/connectors";
import {getAvatar as cbGetAvatar, getName as cbGetName, getAddress as cbGetAddress} from "@coinbase/onchainkit/identity";
import {IsValidBaseAddress} from "../security";
import {Sleep} from "../time";
import {Web3} from "web3";
import PersistentCache from "../cache";

// ---------- Global Variables ---------- //
const baseURLCache = new PersistentCache({
    defaultTtl: 604800000, // 7 days
    keyPrefix: "baseURL_"
});

export const mainnetBase = {
    ethChainID: 8453,
    name: "Base",
    currency: "ETH",
    explorerUrl: "https://basescan.org",
    ensUniversalResolverAddress: "0xce01f8eee7E479C928F8919abD53E553a36CeF67",
    ensBasenameResolverAddress: "0xC6d566A56A1aFf6508b41f6c90ff131615583BCD",
    ensBaseResolverAddress: "0xC6d566A56A1aFf6508b41f6c90ff131615583BCD",
    burnAddress: "0x0000000000000000000000000000000000000000",
    rpcUrl: "", // Set dynamically in initBaseWallet
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
    defaultTtl: 21600000, // 6 hours
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
        // Get RPC URL - uses proxy in gateway mode
        const rpcUrl = await baseGetURL();
        if (!rpcUrl) {
            LogError("Failed to get Base RPC URL");
            return;
        }
        mainnetBase.rpcUrl = rpcUrl;
        viemClient = createPublicClient({
            transport: viemHttp(rpcUrl, {retryCount: 3, retryDelay: 1000}),
            chain: defineChain(viemBase),
        });
        wagmiConfig = createConfig({
            chains: [wagmiBase],
            multiInjectedProviderDiscovery: false,
            connectors: [
                baseAccount({
                    appName: metadataYourPlace.name,
                    appLogoUrl: metadataYourPlace.icons[1],
                })],
            transports: {
                [wagmiBase.id]: wagmiHttp(rpcUrl, {retryCount: 3, retryDelay: 1000}),
            },
            storage: createStorage({
                key: "yourplace",
                storage: window.localStorage,
            }),
            ssr: true,
        });
        web3Client = new Web3(rpcUrl);
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
    let issuedAt = response[1].issuedAt;
    const checksumAddress = getAddress(address);
    LogInfo(`Creating SIWE with: domain=${window.location.host}, address=${checksumAddress}, uri=${window.location.origin}, chainId=${mainnetBase.ethChainID}, nonce=${nonce}, issuedAt=${issuedAt}`);
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
    LogInfo("SIWE message: " + siweMessage);
    let signature: any;
    try {
        signature = await signMessage(wagmiConfig, {
            account: address as `0x${string}`,
            message: siweMessage,
        });
    } catch(error) {
        LogError("Failed to sign SIWE message");
        return "sign failed";
    }
    let loginPayload = {
        message: siweMessage,
        address: address,
        signature: signature,
    };
    const response2 = await HttpPostJson("/login/wallet/base", loginPayload, csrfToken);
    LogInfo(`Login response: status=${response2[0]}, body=${JSON.stringify(response2[1])}`);
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
    // LogInfo("finish implementing baseIsWalletConnected()");
    return false
}
export async function baseTxn(dest: string, payload: string) {
    let address = GetAddress();
    if (!address) {
        LogError("baseTxn: No address found");
        return;
    }
    if (!baseInit) {
        await initBaseWallet();
    }
    try {
        let connections = getConnections(wagmiConfig);
        LogInfo("baseTxn: Current connections: " + connections.length);
        if (!connections.length) {
            await baseConnectWallet();
            connections = getConnections(wagmiConfig);
            if (!connections.length) {
                LogError("baseTxn: Failed to connect to Base Wallet");
                return;
            }
        }
        const connector = connections[0]?.connector;
        LogInfo("baseTxn: Using connector: " + connector?.name + ", address: " + address + ", dest: " + dest);
        // Get the provider from the connector and use eth_sendTransaction directly
        const provider = await connector?.getProvider() as { request: (args: { method: string; params: unknown[] }) => Promise<string> } | undefined;
        if (!provider) {
            LogError("baseTxn: Failed to get provider from connector");
            return;
        }
        LogInfo("baseTxn: Got provider, sending transaction via eth_sendTransaction");
        const txHash = await provider.request({
            method: "eth_sendTransaction",
            params: [{
                from: address as `0x${string}`,
                to: dest as `0x${string}`,
                value: "0x0",
                data: ethers.hexlify(Buffer.from(payload, "utf8")) as `0x${string}`,
            }],
        });
        LogInfo("baseTxn: Transaction sent successfully, hash: " + txHash);
        return txHash;
    } catch (error: unknown) {
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
    baseTxn(mainnetBase.burnAddress, jsonData).then();
}
export async function baseSetBanner(bannerAddress: string) {
    let jsonData = YP.metadataBanner(bannerAddress);
    baseTxn(mainnetBase.burnAddress, jsonData).then();
}
export async function baseSetDescription(description: string) {
    let jsonData = YP.metadataDescription(description);
    baseTxn(mainnetBase.burnAddress, jsonData).then();
}
export async function baseSetLocation(location: string) {
    let jsonData = YP.metadataLocation(location);
    baseTxn(mainnetBase.burnAddress, jsonData).then();
}
export async function baseSetWebsite(website: string) {
    let jsonData = YP.metadataWebsite(website);
    baseTxn(mainnetBase.burnAddress, jsonData).then();
}
export async function baseSetBirthday(birthday: string) {
    let jsonData = YP.metadataBirthday(birthday);
    baseTxn(mainnetBase.burnAddress, jsonData).then();
}
export async function baseSetName(name: string) {
    let jsonData = YP.metadataName(name);
    baseTxn(mainnetBase.burnAddress, jsonData).then();
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

// ---------- Get Functions ---------- //
async function baseGetURL(): Promise<string|null> {
    // In gateway mode, use the server-side RPC proxy to avoid exposing the RPC URL
    if (IsGatewayMode()) {
        const proxyUrl = window.location.origin + "/rpc/base";
        baseURLCache.set("rpcUrl", proxyUrl);
        return proxyUrl;
    }
    const cached = baseURLCache.get("rpcUrl");
    if (cached !== null) return cached as string;
    let response = await HttpGetJson("/settings/base/url");
    if (response[0] === 200) {
        const url = response[1].baseURL || "/api/rpc/base";
        baseURLCache.set("rpcUrl", url);
        return url;
    } else {
        const defaultUrl = "/api/rpc/base";
        baseURLCache.set("rpcUrl", defaultUrl);
        return defaultUrl;
    }
}
export async function baseGetAvatar(address: string): Promise<string> {
    if (!baseInit) await initBaseWallet();
    if (!IsValidBaseAddress(address)) {
        LogError("Invalid Base address provided to baseGetAvatar: " + address);
        return "";
    }
    // Check cache first (invalidate stale ipfs:// URLs and empty results)
    const cached = baseAvatarCache.get(address);
    if (cached !== null && cached !== "" && !cached.toString().startsWith("ipfs://")) return cached as string;
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
export function baseSetCachedName(_address: string, basename: string): void {
    if (!IsValidBaseAddress(_address)) {
        LogError("Invalid Base address provided to baseSetCachedName: " + _address);
        return;
    }
    LogInfo("Manually caching basename for address: " + _address + " -> " + basename);
    baseEnsCache.set(_address, basename);
}
export async function baseGetAddressFromENS(ensName: string): Promise<string | null> {
    if (!baseInit) await initBaseWallet();
    if (!ensName || ensName.trim() === "") {
        LogError("Empty ENS name provided to baseGetAddressFromENS");
        return null;
    }
    const cacheKey = "addr_" + ensName;
    const cached = baseEnsCache.get(cacheKey);
    if (cached !== null) return cached as string;
    try {
        const address = await cbGetAddress({name: ensName});
        if (address) {
            baseEnsCache.set(cacheKey, address);
            return address.toLowerCase();
        }
        return null;
    } catch (error) {
        return null;
    }
}
export async function baseGetENSText(_address: string, key: string): Promise<string> {
    if (viemClient === null || !viemClient || !baseInit) {
        await initBaseWallet();
    }
    let baseName = "";
    try {
        baseName = await baseGetName(_address);
        LogInfo("baseGetENSText: Got basename '" + baseName + "' for address " + _address + ", fetching key '" + key + "'");
        if (!baseName || baseName === "") {
            LogInfo("baseGetENSText: No basename found, returning empty");
            return "";
        }
        let textRecord = await viemClient!.getEnsText({
            name: normalize(baseName),
            key: key,
            universalResolverAddress: mainnetBase.ensBaseResolverAddress,
        });
        if (textRecord) {
            LogInfo("baseGetENSText: Found text record '" + key + "' = '" + textRecord.substring(0, 100) + "'");
            return textRecord;
        }
        LogInfo("baseGetENSText: Text record '" + key + "' returned null/empty for " + baseName);
    } catch (error) {
        LogError("baseGetENSText: Error fetching '" + key + "' for " + (baseName || _address) + ": " + error);
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
    // Priority 1: Local IPFS avatar from YourPlace
    try {
        const response = await HttpGetJson("/profile/avatar/base/" + address);
        if (response[0] === 200 && response[1] && response[1].avatarAddress) {
            const avatarAddress = response[1].avatarAddress.trim();
            if (avatarAddress.length > 0) {
                // Convert ipfs:// URL to HTTP gateway URL
                const avatarUrl = CIDToSubdomainURL(avatarAddress);
                if (avatarUrl !== "") {
                    baseAvatarCache.set(address, avatarUrl);
                    return avatarUrl;
                }
            }
        }
    } catch (error) {
        LogError("Failed to get local avatar: " + error);
    }
    // Priority 2: ENS avatar from basename
    let baseName = await baseGetName(address);
    if (baseName && baseName !== "") {
        try {
            const ensAvatar = await cbGetAvatar({ensName: baseName, chain: wagmiBase});
            if (ensAvatar && ensAvatar !== "") {
                baseAvatarCache.set(address, ensAvatar);
                return ensAvatar as string;
            }
            LogInfo("No ENS Avatar found for basename: " + baseName);
        } catch (error) {
            LogError("Failed to get Base ENS avatar: " + error);
        }
    }
    baseAvatarCache.set(address, ""); // Cache empty result to avoid repeated failed lookups
    return "";
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