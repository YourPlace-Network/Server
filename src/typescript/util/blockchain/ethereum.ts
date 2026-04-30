import {DisconnectWallet, GetAddress, IsInsufficientFundsError, OnRampFiat, SetAddress} from "./wallet";
import type {CollectibleData} from "./wallet";
import {LogError, LogInfo} from "../log";
import {HttpGetJson, HttpPostJson} from "../network";
import {CIDToSubdomainURL} from "../ipfs";
import {ethers} from "ethers";
import {YP} from "../../services/yourplace";
import {createPublicClient, getAddress, http as viemHttp, UserRejectedRequestError} from "viem";
import {mainnet as viemMainnet} from "viem/chains";
import {SiweMessage} from "siwe";
import {
    connect as wagmiConnect,
    createConfig,
    createStorage,
    disconnect,
    getConnections,
    http as wagmiHttp,
    reconnect,
    signMessage,
} from "@wagmi/core";
import {mainnet as wagmiMainnet} from "@wagmi/core/chains";
import {injected} from "@wagmi/connectors";
import {IsValidBaseAddress, IsValidURL} from "../security";
import {Sleep} from "../time";
import {PersistentCache} from "../cache";

export const mainnetEth = {
    chainId: 1,
    name: "Ethereum",
    currency: "ETH",
    explorerUrl: "https://etherscan.io",
    rpcUrl: window.location.origin + "/rpc/ethereum",
    burnAddress: "0x000000000000000000000000000000000000dEaD",
}
let ethereumInit = false;
let ethereumViemClient: any;
let ethereumWagmiConfig: any;
const ethereumAvatarCache = new PersistentCache("ethereum_avatar");
const ethereumNameCache = new PersistentCache("ethereum_name");
const ethereumDescriptionCache = new PersistentCache("ethereum_description");
const ethereumEnsNameCache = new PersistentCache("ethereum_ens_name");
const ethereumEnsAvatarCache = new PersistentCache("ethereum_ens_avatar");
const ethereumEnsAddressCache = new PersistentCache("ethereum_ens_address");
let ethereumPrefetchedNonce: {nonce: string, issuedAt: string, fetchedAt: number} | null = null;
const ETHEREUM_NONCE_PREFETCH_VALIDITY_MS = 300000;
const ETHEREUM_WAGMI_STORAGE_KEY = "yourplace_ethereum.store";
const ETHEREUM_WAGMI_RECENT_CONNECTOR_KEY = "yourplace_ethereum.recentConnectorId";

async function initEthWallet() {
    if (ethereumInit) { return; }
    try {
        ethereumViemClient = createPublicClient({
            chain: viemMainnet,
            transport: viemHttp(mainnetEth.rpcUrl!),
        });
        ethereumWagmiConfig = createConfig({
            chains: [wagmiMainnet],
            multiInjectedProviderDiscovery: false,
            connectors: [
                injected({target: "metaMask"}),
            ],
            transports: {
                [wagmiMainnet.id]: wagmiHttp(mainnetEth.rpcUrl!),
            },
            storage: createStorage({
                key: "yourplace_ethereum",
                storage: window.localStorage,
            }),
            ssr: true,
        });
    } catch (e) {
        LogError("Failed to initialize Ethereum wallet: " + e);
        ethereumInit = false;
        return;
    }
    ethereumInit = true;
}
initEthWallet().then();

function clearEthereumWalletStorage() {
    localStorage.removeItem(ETHEREUM_WAGMI_STORAGE_KEY);
    localStorage.removeItem(ETHEREUM_WAGMI_RECENT_CONNECTOR_KEY);
    localStorage.removeItem("wagmi.store");
    localStorage.removeItem("wagmi.recentConnectorId");
}
function getEthereumConnectedAddress(): string {
    const address = getConnections(ethereumWagmiConfig)[0]?.accounts?.[0]?.toString() || "";
    if (address !== "" && IsValidBaseAddress(address)) {
        return address;
    }
    return "";
}
function syncEthereumStoredAddress(address: string): string {
    if (address !== "" && GetAddress() !== address) {
        SetAddress(address);
    }
    return address;
}
async function restoreEthereumConnection(): Promise<string> {
    const connector = ethereumWagmiConfig?.connectors?.[0];
    if (!connector) {
        return "";
    }
    try {
        await reconnect(ethereumWagmiConfig, {
            connectors: [connector],
        });
    } catch (_) {}
    return syncEthereumStoredAddress(getEthereumConnectedAddress());
}

export async function ethereumAuthLogin(): Promise<string> {
    if (!ethereumInit) {
        await initEthWallet();
    }
    let address = syncEthereumStoredAddress(getEthereumConnectedAddress());
    if (address === "") {
        address = await ethereumReconnectWallet();
    }
    if (address === "") {
        address = await ethereumConnectWallet();
    }
    let csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
    if (!csrfToken || csrfToken === "") {
        LogError("CSRF token not found - ethereumAuthLogin()");
        return "csrf token not found";
    }
    if (!address || address === "" || !IsValidBaseAddress(address)) {
        LogError("Invalid Ethereum address - ethereumAuthLogin()");
        return "invalid address";
    }
    let nonce: string;
    let issuedAt: string;
    if (ethereumPrefetchedNonce && (Date.now() - ethereumPrefetchedNonce.fetchedAt) < ETHEREUM_NONCE_PREFETCH_VALIDITY_MS) {
        nonce = ethereumPrefetchedNonce.nonce;
        issuedAt = ethereumPrefetchedNonce.issuedAt;
        ethereumPrefetchedNonce = null;
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
        chainId: mainnetEth.chainId,
        nonce: nonce,
        issuedAt: issuedAt,
    });
    const siweMessage = siweMsg.prepareMessage();
    let signature: any;
    try {
        signature = await signMessage(ethereumWagmiConfig, {
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
    const response2 = await HttpPostJson("/login/wallet/ethereum", loginPayload, csrfToken);
    if (response2[0] != 200) {
        LogError("Failed to login with Ethereum: " + JSON.stringify(response2[1]));
        await Sleep(3000);
        await DisconnectWallet();
        return response2[1] ? response2[1].status : "Unknown error during Ethereum login";
    }
    try {
        let status = response2[1].status as string;
        if (status === "Ethereum wallet login success") {
            return "success";
        }
    } catch (e) {
        LogError("Failed to parse login response");
    }
    return "Failed Ethereum Login: Unknown Error";
}
export async function ethereumConnectWallet(): Promise<string> {
    if (!ethereumInit) {
        await initEthWallet();
    }
    try {
        let address = syncEthereumStoredAddress(getEthereumConnectedAddress());
        if (address !== "") {
            return address;
        }
        address = await restoreEthereumConnection();
        if (address !== "") {
            return address;
        }
        if (getConnections(ethereumWagmiConfig).length > 0) {
            await disconnect(ethereumWagmiConfig);
        }
        clearEthereumWalletStorage();
        const {accounts} = await wagmiConnect(ethereumWagmiConfig, {
            chainId: wagmiMainnet.id,
            connector: ethereumWagmiConfig.connectors[0],
        });
        address = accounts[0].toString();
        if (!address || address === "" || !IsValidBaseAddress(address)) {
            LogError("Failed to connect to Ethereum Wallet: Invalid address returned");
            return "";
        }
        return syncEthereumStoredAddress(address);
    } catch (error: unknown) {
        if (error instanceof Error) {
            if (error instanceof UserRejectedRequestError) {
                LogInfo("User declined to connect wallet");
            } else {
                LogError("Failed to connect to Ethereum: " + error);
            }
        }
        return "";
    }
}
export async function ethereumReconnectWallet(): Promise<string> {
    if (!ethereumInit) {
        await initEthWallet();
    }
    return await restoreEthereumConnection();
}
export async function ethereumDisconnectWallet(): Promise<void> {
    if (!ethereumInit) return;
    await disconnect(ethereumWagmiConfig);
}
export async function ethereumIsWalletConnected(): Promise<boolean> {
    if (!ethereumInit) {
        await initEthWallet();
    }
    const address = getEthereumConnectedAddress();
    if (address !== "") {
        syncEthereumStoredAddress(address);
        return true;
    }
    try {
        const accounts = await ethereumWagmiConfig?.connectors?.[0]?.getAccounts();
        const accountAddress = accounts?.[0]?.toString() || "";
        if (accountAddress !== "" && IsValidBaseAddress(accountAddress)) {
            syncEthereumStoredAddress(accountAddress);
            return true;
        }
    } catch (_) {}
    return false;
}
export async function ethereumTxn(dest: string, payload: string) {
    if (!ethereumInit) {
        await initEthWallet();
    }
    let address = "";
    try {
        address = syncEthereumStoredAddress(getEthereumConnectedAddress());
        if (address === "") {
            address = await ethereumReconnectWallet();
        }
        if (address === "") {
            address = await ethereumConnectWallet();
        }
        if (!address) {
            LogError("ethereumTxn: No address found");
            return;
        }
        const connections = getConnections(ethereumWagmiConfig);
        if (!connections.length) {
            LogError("ethereumTxn: Failed to connect to Ethereum Wallet");
            return;
        }
        const connector = connections[0]?.connector;
        const provider = await connector?.getProvider() as { request: (args: { method: string; params: unknown[] }) => Promise<string> } | undefined;
        if (!provider) {
            LogError("ethereumTxn: Failed to get provider from connector");
            return;
        }
        const txHash = await provider.request({
            method: "eth_sendTransaction",
            params: [{
                from: address as `0x${string}`,
                to: dest as `0x${string}`,
                value: "0x0",
                data: ethers.hexlify(Buffer.from(payload, "utf8")) as `0x${string}`,
            }],
        });
        return txHash;
    } catch (error: unknown) {
        if (IsInsufficientFundsError(error)) { OnRampFiat(address, "ethereum"); return; }
        if (error instanceof Error) {
            LogError("ethereumTxn failed: " + error.message);
        } else {
            LogError("ethereumTxn failed with unknown error: " + String(error));
        }
    }
}

export async function ethereumSetAvatar(avatarAddress: string) {
    let jsonData = YP.metadataAvatar(avatarAddress);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSetBanner(bannerAddress: string) {
    let jsonData = YP.metadataBanner(bannerAddress);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSetColors(colors: Record<string, string>) {
    let jsonData = YP.metadataColors(colors);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSetDescription(description: string) {
    let jsonData = YP.metadataDescription(description);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSetLocation(location: string) {
    let jsonData = YP.metadataLocation(location);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSetMusicEmbed(music: string) {
    let jsonData = YP.metadataMusic(music);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSetName(name: string) {
    let jsonData = YP.metadataName(name);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSetBot(bot: boolean) {
    let jsonData = YP.metadataBot(bot);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSetNsfw(nsfw: boolean) {
    let jsonData = YP.metadataNsfw(nsfw);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSetVertical(vertical: string) {
    let jsonData = YP.metadataVertical(vertical);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSetWebsite(website: string) {
    let jsonData = YP.metadataWebsite(website);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSubmitPost(payload: string) {
    let jsonData = YP.post(payload);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSubmitPostAttach(payload: string, attach: string[][]) {
    let jsonData = YP.postAttach(payload, attach);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumPublishFiles(attach: string[][]) {
    let jsonData = YP.filePublish(attach);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumDeleteFiles(cids: string[]) {
    let jsonData = YP.fileDelete(cids);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumFollowUser(toAddress: string, toBlockchain: string) {
    let jsonData = YP.follow(toAddress, toBlockchain);
    return await ethereumTxn(toAddress, jsonData);
}
export async function ethereumUnfollowUser(toAddress: string, toBlockchain: string) {
    let jsonData = YP.unfollow(toAddress, toBlockchain);
    return await ethereumTxn(toAddress, jsonData);
}
export async function ethereumSubmitComment(parentTxHash: string, payload: string) {
    let jsonData = YP.comment(parentTxHash, payload);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSubmitCommentAttach(parentTxHash: string, payload: string, attach: string[][]) {
    let jsonData = YP.commentAttach(parentTxHash, payload, attach);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSubmitDislike(targetTxHash: string, targetType: string) {
    let jsonData = YP.dislike(targetTxHash, targetType);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSubmitEmojiReaction(targetTxHash: string, targetType: string, emoji: string) {
    let jsonData = YP.emojiReact(targetTxHash, targetType, emoji);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}
export async function ethereumSubmitLike(targetTxHash: string, targetType: string) {
    let jsonData = YP.like(targetTxHash, targetType);
    return await ethereumTxn(mainnetEth.burnAddress, jsonData);
}

export async function ethereumGetAvatar(address: string): Promise<string> {
    if (!ethereumInit) await initEthWallet();
    if (!IsValidBaseAddress(address)) {
        LogError("Invalid Ethereum address provided to ethereumGetAvatar: " + address);
        return "";
    }
    const cached = ethereumAvatarCache.get<string>(address);
    if (cached !== null) {
        return cached;
    }
    try {
        const response = await HttpGetJson("/profile/avatar/ethereum/" + address);
        if (response[0] === 200 && response[1] && response[1].avatarAddress) {
            const avatarAddress = response[1].avatarAddress.trim();
            if (avatarAddress.length > 0) {
                if (IsValidURL(avatarAddress)) {
                    ethereumAvatarCache.set(address, avatarAddress);
                    return avatarAddress;
                }
                const avatarUrl = CIDToSubdomainURL(avatarAddress);
                if (avatarUrl !== "") {
                    ethereumAvatarCache.set(address, avatarUrl);
                    return avatarUrl;
                }
            }
        }
    } catch (error) {
        LogError("Failed to get local avatar: " + error);
    }
    try {
        const ensAvatar = await ethereumGetEnsAvatar(address);
        if (ensAvatar && ensAvatar !== "") {
            ethereumAvatarCache.set(address, ensAvatar);
            return ensAvatar;
        }
    } catch (_) {}
    return "";
}
export async function ethereumGetName(_address: string): Promise<string> {
    if (!ethereumInit) await initEthWallet();
    if (!IsValidBaseAddress(_address)) {
        LogError("Invalid Ethereum address provided to ethereumGetName: " + _address);
        return "";
    }
    const cached = ethereumNameCache.get<string>(_address);
    if (cached !== null) {
        return cached;
    }
    try {
        const response = await HttpGetJson("/profile/name/ethereum/" + _address);
        if (response[0] === 200 && response[1] && response[1].name) {
            const name = response[1].name.trim();
            if (name.length > 0) {
                ethereumNameCache.set(_address, name);
                return name;
            }
        }
        const ensName = await ethereumGetEnsName(_address);
        if (ensName && ensName !== "") {
            ethereumNameCache.set(_address, ensName);
            return ensName;
        }
    } catch (error) {
        LogError("Failed to get Ethereum name: " + error);
    }
    return "";
}
export async function ethereumGetDescription(_address: string): Promise<string> {
    if (!ethereumInit) await initEthWallet();
    if (!IsValidBaseAddress(_address)) {
        LogError("Invalid Ethereum address provided to ethereumGetDescription: " + _address);
        return "";
    }
    const cached = ethereumDescriptionCache.get<string>(_address);
    if (cached !== null) {
        return cached;
    }
    try {
        const response = await HttpGetJson("/profile/description/ethereum/" + _address);
        if (response[0] === 200 && response[1] && response[1].description && response[1].description !== "") {
            ethereumDescriptionCache.set(_address, response[1].description.trim());
            return response[1].description.trim();
        }
    } catch (error) {
        LogError("Failed to get description: " + error);
    }
    return "";
}
export async function ethereumGetCollectibles(_address: string): Promise<CollectibleData[]> {
    return [];
}
export async function ethereumBurnCollectible(_tokenId: bigint): Promise<boolean> {
    return false;
}
export async function ethereumTransferCollectible(_tokenId: bigint, _toAddress: string): Promise<boolean> {
    return false;
}
export async function ethereumMintCollectible(_metadataUri: string): Promise<string | undefined> {
    return undefined;
}
export async function ethereumGetTransferFeeEstimate(_toAddress: string, _tokenId: bigint): Promise<string> {
    return "-- ETH";
}

export async function ethereumGetEnsName(address: string): Promise<string> {
    if (!ethereumInit) await initEthWallet();
    const cached = ethereumEnsNameCache.get<string>(address);
    if (cached !== null) {
        return cached;
    }
    try {
        const ensName = await ethereumViemClient.getEnsName({address: address as `0x${string}`});
        if (ensName) {
            ethereumEnsNameCache.set(address, ensName);
            return ensName;
        }
    } catch (_) {}
    ethereumEnsNameCache.set(address, "");
    return "";
}
export async function ethereumGetEnsAvatar(address: string): Promise<string> {
    if (!ethereumInit) await initEthWallet();
    const cached = ethereumEnsAvatarCache.get<string>(address);
    if (cached !== null) {
        return cached;
    }
    const ensName = await ethereumGetEnsName(address);
    if (!ensName || ensName === "") {
        ethereumEnsAvatarCache.set(address, "");
        return "";
    }
    try {
        const ensAvatar = await ethereumViemClient.getEnsAvatar({name: ensName});
        if (ensAvatar) {
            ethereumEnsAvatarCache.set(address, ensAvatar);
            return ensAvatar;
        }
    } catch (_) {}
    ethereumEnsAvatarCache.set(address, "");
    return "";
}
export async function ethereumGetEnsAddress(ensName: string): Promise<string> {
    const cached = ethereumEnsAddressCache.get<string>(ensName);
    if (cached !== null) {
        return cached;
    }
    try {
        const ensAddress = await ethereumViemClient.getEnsAddress({name: ensName});
        if (ensAddress) {
            ethereumEnsAddressCache.set(ensName, ensAddress);
            return ensAddress;
        }
    } catch (_) {}
    ethereumEnsAddressCache.set(ensName, "");
    return "";
}
