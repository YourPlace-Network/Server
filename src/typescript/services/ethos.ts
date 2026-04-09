import {IsValidBaseAddress} from "../util/security";

export interface EthosResult {
    label: string;
    score: number;
}

const ethosLabels: [number, string][] = [
    [799, "Untrusted"],
    [1199, "Questionable"],
    [1399, "Neutral"],
    [1599, "Known"],
    [1799, "Established"],
    [1999, "Reputable"],
    [2199, "Exemplary"],
    [2399, "Distinguished"],
    [2599, "Revered"],
    [2800, "Renowned"],
];

export function EthosGetRatingLabel(score: number): string {
    for (const [threshold, label] of ethosLabels) {
        if (score <= threshold) return label;
    }
    return "Renowned";
}
export async function EthosGetScore(address: string): Promise<EthosResult | null> {
    if (!IsValidBaseAddress(address)) return null;
    try {
        let response = await fetch("https://api.ethos.network/api/v2/user/by/address/" + encodeURIComponent(address), {
            headers: {"X-Ethos-Client": "YourPlace"},
        });
        if (!response.ok) return null;
        let data = await response.json();
        if (data && typeof data.score === "number") {
            return {label: EthosGetRatingLabel(data.score), score: data.score};
        }
        return null;
    } catch {
        return null;
    }
}
