import {DisconnectWallet, GetAddress} from "./wallet";
import {LogError, LogInfo} from "../log";
import {HttpGetJson, HttpPostJson} from "../network";
import {CIDToSubdomainURL} from "../ipfs";
import {ensNormalize, ethers} from "ethers";
import {YP} from "../../services/yourplace";
import {createPublicClient, defineChain, getAddress, http as viemHttp, UserRejectedRequestError, toCoinType} from "viem";
import {base as viemBase} from "viem/chains";
import {SiweMessage} from "siwe";
import {
    connect as wagmiConnect,
    createConfig,
    createStorage,
    disconnect,
    getConnections, getEnsAvatar, getEnsName,
    http as wagmiHttp,
    readContract,
    signMessage,
} from "@wagmi/core";
import {getName as ockGetName, getAvatar as ockGetAvatar, getAddress as ockGetAddress} from "@coinbase/onchainkit/identity";
import {base as wagmiBase} from "@wagmi/core/chains";
import {baseAccount} from "@wagmi/connectors";
import {IsValidBaseAddress} from "../security";
import {Sleep} from "../time";
import {PersistentCache} from "../cache";
import {setOnchainKitConfig} from "@coinbase/onchainkit";

// ---------- Global Variables ---------- //
export const mainnetBase = {
    ethChainID: 8453,
    name: "Base",
    currency: "ETH",
    explorerUrl: "https://basescan.org",
    rpcUrl: await baseGetURL()!,
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
        viemClient = createPublicClient({
            transport: viemHttp(mainnetBase.rpcUrl!),
            chain: viemBase,
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
        });
        setOnchainKitConfig({
            chain: viemBase,
            rpcUrl: mainnetBase.rpcUrl!,
            defaultPublicClients: {
                [viemBase.id]: viemClient,
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
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetBanner(bannerAddress: string) {
    let jsonData = YP.metadataBanner(bannerAddress);
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
export async function baseSetWebsite(website: string) {
    let jsonData = YP.metadataWebsite(website);
    return await baseTxn(mainnetBase.burnAddress, jsonData);
}
export async function baseSetName(name: string) {
    let jsonData = YP.metadataName(name);
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

// ---------- Get Functions ---------- //
async function baseGetURL(): Promise<string|null> {
    let response = await HttpGetJson("/settings/base/url");
    if (response[0] === 200 && response[1] && response[1].baseURL !== "") {
        let url = response[1].baseURL;
        if (url.startsWith('/')) {
            url = window.location.origin + url;
        }
        return url;
    }
    LogError("Failed to get Base RPC URL from server: " + response[1]);
    return null;
}
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
                const avatarUrl = CIDToSubdomainURL(avatarAddress);
                if (avatarUrl !== "") {
                    avatarCache.set(address, avatarUrl);
                    return avatarUrl;
                }
            }
        } else {
            const ensAvatar = await baseGetEnsAvatar(address);
            if (ensAvatar && ensAvatar !== "") {
                avatarCache.set(address, ensAvatar);
                return ensAvatar;
            }
        }
    } catch (error) {
        LogError("Failed to get local avatar: " + error);
    }
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
        LogInfo("baseGetDescription(): Cache hit for " + _address);
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
    } catch (e) {
        LogError("baseGetEnsName(): Error fetching ENS name: " + e);
    }
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
        return "";
    }
    try {
        const ensAvatar = await ockGetAvatar({ensName, chain: viemBase});
        if (ensAvatar) {
            ensAvatarCache.set(address, ensAvatar);
            return ensAvatar;
        }
    } catch (e) {
        LogError("baseGetEnsAvatar(): Error fetching ENS avatar: " + e);
    }
    return "";
}
export async function baseGetEnsAddress(ensName: string): Promise<string> {
    const cached = ensAddressCache.get<string>(ensName);
    if (cached !== null) {
        return cached;
    }
    const ensAddress = await ockGetAddress({name: ensName});
    if (ensAddress) {
        LogInfo("baseGetEnsAddress(): Fetched ENS address: " + ensAddress);
        ensAddressCache.set(ensName, ensAddress);
        return ensAddress;
    }
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