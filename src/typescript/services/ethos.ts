import {IsValidBaseAddress} from "../util/security";

export async function EthosGetScore(address: string): Promise<number | null> {
    if (!IsValidBaseAddress(address)) return null;
    try {
        let response = await fetch("https://api.ethos.network/api/v2/user/by/address/" + encodeURIComponent(address), {
            headers: {"X-Ethos-Client": "YourPlace"},
        });
        if (!response.ok) return null;
        let data = await response.json();
        if (data && typeof data.score === "number") {
            return data.score;
        }
        return null;
    } catch {
        return null;
    }
}
