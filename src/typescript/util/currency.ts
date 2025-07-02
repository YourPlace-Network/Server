import {LogError} from "./log";

// Currency information for supported currencies
interface CurrencyInfo {
    symbol: string;
    name: string;
    smallUnitName: string;
    smallUnitFactor: number;
    supportedChains: string[];
}

// Supported currencies configuration
const SUPPORTED_CURRENCIES: Record<string, CurrencyInfo> = {
    ETH: {
        symbol: "ETH",
        name: "Ethereum", 
        smallUnitName: "wei",
        smallUnitFactor: 1e18,
        supportedChains: ["base", "ethereum"]
    },
    // Stubbed currencies for forward compatibility
    BTC: {
        symbol: "BTC",
        name: "Bitcoin",
        smallUnitName: "sats", 
        smallUnitFactor: 1e8,
        supportedChains: ["bitcoin"]
    },
    ALGO: {
        symbol: "ALGO",
        name: "Algorand",
        smallUnitName: "microalgos",
        smallUnitFactor: 1e6,
        supportedChains: ["algorand"]
    },
    SOL: {
        symbol: "SOL", 
        name: "Solana",
        smallUnitName: "lamports",
        smallUnitFactor: 1e9,
        supportedChains: ["solana"]
    }
};

// Currency class for handling multi-currency amounts
export class Currency {
    public symbol: string;
    public amount: string;      // Major unit (e.g., "1.5" ETH)
    public smallUnit: string;   // Minor unit (e.g., "1500000000000000000" wei)
    public blockchain: string;

    constructor(symbol: string, amount: string, smallUnit: string, blockchain: string) {
        this.symbol = symbol.toUpperCase();
        this.amount = amount;
        this.smallUnit = smallUnit;
        this.blockchain = blockchain.toLowerCase();
    }

    // Convert major unit to small unit
    public convertToSmallUnit(): string {
        if (this.smallUnit) {
            return this.smallUnit;
        }

        const info = SUPPORTED_CURRENCIES[this.symbol];
        if (!info) {
            LogError("Unsupported currency: " + this.symbol);
            return "";
        }

        const amount = parseFloat(this.amount);
        if (isNaN(amount)) {
            LogError("Invalid amount: " + this.amount);
            return "";
        }

        const smallUnitAmount = Math.floor(amount * info.smallUnitFactor);
        return smallUnitAmount.toString();
    }

    // Convert small unit to major unit
    public convertToMajorUnit(): string {
        if (this.amount) {
            return this.amount;
        }

        const info = SUPPORTED_CURRENCIES[this.symbol];
        if (!info) {
            LogError("Unsupported currency: " + this.symbol);
            return "";
        }

        const smallUnitAmount = parseInt(this.smallUnit, 10);
        if (isNaN(smallUnitAmount)) {
            LogError("Invalid small unit amount: " + this.smallUnit);
            return "";
        }

        const majorAmount = smallUnitAmount / info.smallUnitFactor;
        return majorAmount.toString();
    }

    // Get the name of the small unit
    public getSmallUnitName(): string {
        const info = SUPPORTED_CURRENCIES[this.symbol];
        return info ? info.smallUnitName : "units";
    }

    // Format for display
    public formatDisplay(): string {
        if (this.amount) {
            return `${this.amount} ${this.symbol}`;
        }

        const majorUnit = this.convertToMajorUnit();
        if (majorUnit) {
            return `${majorUnit} ${this.symbol}`;
        }

        return `${this.smallUnit} ${this.getSmallUnitName()}`;
    }
}

// Utility functions
export function isSupported(symbol: string): boolean {
    return SUPPORTED_CURRENCIES[symbol.toUpperCase()] !== undefined;
}

export function isImplemented(symbol: string): boolean {
    return symbol.toUpperCase() === "ETH";
}

export function validateForBlockchain(symbol: string, blockchain: string): boolean {
    const info = SUPPORTED_CURRENCIES[symbol.toUpperCase()];
    if (!info) {
        return false;
    }

    // Only validate for implemented currencies
    if (isImplemented(symbol)) {
        return info.supportedChains.includes(blockchain.toLowerCase());
    }

    return true; // Allow stubbed currencies for forward compatibility
}

export function getSupportedCurrencies(): string[] {
    return Object.keys(SUPPORTED_CURRENCIES);
}

export function getImplementedCurrencies(): string[] {
    return ["ETH"];
}

export function getCurrencyInfo(symbol: string): CurrencyInfo | null {
    return SUPPORTED_CURRENCIES[symbol.toUpperCase()] || null;
}

// Convert wei to ETH (for backward compatibility)
export function weiToEth(wei: string): string {
    const currency = new Currency("ETH", "", wei, "base");
    return currency.convertToMajorUnit();
}

// Convert ETH to wei (for backward compatibility)
export function ethToWei(eth: string): string {
    const currency = new Currency("ETH", eth, "", "base");
    return currency.convertToSmallUnit();
}