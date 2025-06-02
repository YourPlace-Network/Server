import {IsValidBaseAddress} from "../security";
import {LogError} from "../log";

const mainnetEth = {
    chainId: 1,
    name: "Ethereum",
    currency: "ETH",
    explorerUrl: "https://etherscan.io",
    rpcUrl: "https://cloudflare-eth.com"
}
let ethInit: boolean = false;
let viemClient: any;

function initEthClients() {

}
function siwe() {
    /*let timestamp = new Date().toISOString();
    let domain = window.location.protocol + "//" + window.location.hostname;
    let siweObj = {
        domain: window.location.hostname,
        address: GetAddress(),
        statement: "Yeah man, I want to sign into YourPlace",
        uri: window.location.origin,
        version: "1",
        chainId: 1,

    }
    const siweMessage = `${domain} wants you to sign in with your Ethereum account:\n${GetAddress()}\n\nYeah man, I want to sign into YourPlace\n\nURI: ${domain}\nVersion: 1\nChain ID: 1\nNonce: ${nonce}\nIssued At: ${timestamp}`;*/
}
export async function ethGetName(_address: string): Promise<string> {
    if (!ethInit) {
        await initEthClients();
    }
    if (!IsValidBaseAddress(_address)) {
        LogError("Invalid Base address provided to baseGetName: " + _address);
        return "";
    }
    try {
        return await viemClient.getEnsName({address: _address as `0x${string}`});
    } catch (error) {
        LogError("Failed to get ENS name for Base address: " + _address + " - " + error);
        return "";
    }
}