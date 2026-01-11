import {IsValidBaseAddress} from "../../typescript/util/security.ts";
import {LogError, LogInfo} from "../../typescript/util/log.ts";
import {HttpGetJson} from "../../typescript/util/network.ts";
import {CIDToSubdomainURL} from "../../typescript/util/ipfs.ts";
import {baseGetEnsAvatar, baseGetEnsName} from "../../typescript/util/blockchain/base.ts";
import {getName as ockGetName} from "@coinbase/onchainkit/dist/identity/utils/getName.d.ts";
import {base as viemBase} from "viem/_types/chains/definitions/base.d.ts";
import {getAvatar as ockGetAvatar} from "@coinbase/onchainkit/dist/identity/utils/getAvatar.d.ts";
import {getAddress as ockGetAddress} from "@coinbase/onchainkit/dist/identity/utils/getAddress.d.ts";

const { getAvatar, getName, getAddress } = require("@coinbase/onchainkit/identity");
const { base } = require("viem/chains");


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
