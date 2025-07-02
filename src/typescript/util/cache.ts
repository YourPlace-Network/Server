import {LogError, LogInfo} from "./log";

interface CacheItem {
    value: any;
    timestamp: number;
    ttl: number;
}
interface CacheOptions {
    defaultTtl?: number; // milliseconds
    keyPrefix?: string;
}

class PersistentCache {
    private defaultTtl: number;
    private keyPrefix: string;

    constructor(options: CacheOptions = {}) {
        this.defaultTtl = options.defaultTtl || 300000; // 5 minute default
        this.keyPrefix = options.keyPrefix || "cache_";
    }
    private getStorageKey(key: string): string {
        return this.keyPrefix + key;
    }
    set(key: string, value: any, ttl?: number): void {
        const item: CacheItem = {
            value,
            timestamp: Date.now(),
            ttl: ttl || this.defaultTtl
        };
        try {
            localStorage.setItem(this.getStorageKey(key), JSON.stringify(item));
        } catch (error) {
            console.error("Failed to set cache item:", error);
        }
    }
    get(key: string): any {
        try {
            const stored = localStorage.getItem(this.getStorageKey(key));
            if (!stored) {
                return null;
            }
            const item: CacheItem = JSON.parse(stored);
            const now = Date.now();
            if (now - item.timestamp > item.ttl) {
                this.delete(key);
                return null;
            }
            return item.value;
        } catch (error) {
            LogError("Failed to get cache item: " + error);
            return null;
        }
    }
    delete(key: string): void {
        localStorage.removeItem(this.getStorageKey(key));
    }
    clear(): void {
        const keys = Object.keys(localStorage);
        keys.forEach(key => {
            if (key.startsWith(this.keyPrefix)) {
                localStorage.removeItem(key);
            }
        });
    }
    has(key: string): boolean {
        return this.get(key) !== null;
    }
    cleanup(): void {
        const keys = Object.keys(localStorage);
        keys.forEach(key => {
            if (key.startsWith(this.keyPrefix)) {
                const stored = localStorage.getItem(key);
                if (stored) {
                    try {
                        const item: CacheItem = JSON.parse(stored);
                        const now = Date.now();
                        if (now - item.timestamp > item.ttl) {
                            localStorage.removeItem(key);
                        }
                    } catch (error) {
                        localStorage.removeItem(key);
                    }
                }
            }
        });
    }
}

// Marketplace-specific caching to minimize blockchain RPC calls
const marketplaceCache = new PersistentCache({
    defaultTtl: 1800000, // 30 minutes for marketplace data
    keyPrefix: "marketplace_"
});

const transactionCache = new PersistentCache({
    defaultTtl: 86400000, // 24 hours for confirmed transactions
    keyPrefix: "tx_"
});

const listingCache = new PersistentCache({
    defaultTtl: 3600000, // 60 minutes for listings
    keyPrefix: "listing_"
});

const offerCache = new PersistentCache({
    defaultTtl: 900000, // 15 minutes for offers
    keyPrefix: "offer_"
});

// Cache marketplace listings
export function cacheMarketplaceListing(listingId: string, data: any): void {
    try {
        listingCache.set(listingId, data);
        LogInfo("Cached marketplace listing: " + listingId);
    } catch (error) {
        LogError("Failed to cache marketplace listing: " + error);
    }
}

export function getCachedMarketplaceListing(listingId: string): any {
    try {
        return listingCache.get(listingId);
    } catch (error) {
        LogError("Failed to get cached marketplace listing: " + error);
        return null;
    }
}

// Cache offers for listings
export function cacheListingOffers(listingId: string, offers: any[]): void {
    try {
        offerCache.set(listingId + "_offers", offers);
        LogInfo("Cached offers for listing: " + listingId);
    } catch (error) {
        LogError("Failed to cache listing offers: " + error);
    }
}

export function getCachedListingOffers(listingId: string): any[] | null {
    try {
        return offerCache.get(listingId + "_offers") as any[] || null;
    } catch (error) {
        LogError("Failed to get cached listing offers: " + error);
        return null;
    }
}

// Cache transaction confirmations
export function cacheTransactionConfirmation(txHash: string, confirmed: boolean): void {
    try {
        transactionCache.set(txHash + "_confirmed", confirmed);
        if (confirmed) {
            LogInfo("Cached transaction confirmation: " + txHash);
        }
    } catch (error) {
        LogError("Failed to cache transaction confirmation: " + error);
    }
}

export function getCachedTransactionConfirmation(txHash: string): boolean | null {
    try {
        return transactionCache.get(txHash + "_confirmed") as boolean || null;
    } catch (error) {
        LogError("Failed to get cached transaction confirmation: " + error);
        return null;
    }
}

// Cache user's marketplace activity
export function cacheUserMarketplaceData(address: string, type: "listings" | "offers" | "transactions", data: any[]): void {
    try {
        marketplaceCache.set(address + "_" + type, data);
        LogInfo("Cached user marketplace data: " + address + " (" + type + ")");
    } catch (error) {
        LogError("Failed to cache user marketplace data: " + error);
    }
}

export function getCachedUserMarketplaceData(address: string, type: "listings" | "offers" | "transactions"): any[] | null {
    try {
        return marketplaceCache.get(address + "_" + type) as any[] || null;
    } catch (error) {
        LogError("Failed to get cached user marketplace data: " + error);
        return null;
    }
}

// Invalidate cache when new transactions are detected
export function invalidateMarketplaceCache(listingId?: string, address?: string): void {
    try {
        if (listingId) {
            listingCache.delete(listingId);
            offerCache.delete(listingId + "_offers");
            LogInfo("Invalidated cache for listing: " + listingId);
        }
        
        if (address) {
            marketplaceCache.delete(address + "_listings");
            marketplaceCache.delete(address + "_offers");
            marketplaceCache.delete(address + "_transactions");
            LogInfo("Invalidated cache for address: " + address);
        }
    } catch (error) {
        LogError("Failed to invalidate marketplace cache: " + error);
    }
}

// Batch cache operations for efficiency
export function batchCacheMarketplaceData(operations: Array<{type: string, key: string, data: any}>): void {
    try {
        operations.forEach(op => {
            switch (op.type) {
                case "listing":
                    listingCache.set(op.key, op.data);
                    break;
                case "offer":
                    offerCache.set(op.key, op.data);
                    break;
                case "transaction":
                    transactionCache.set(op.key, op.data);
                    break;
                case "user":
                    marketplaceCache.set(op.key, op.data);
                    break;
            }
        });
        LogInfo("Batch cached " + operations.length + " marketplace operations");
    } catch (error) {
        LogError("Failed to batch cache marketplace data: " + error);
    }
}

// Cache warming for frequently accessed data
export async function warmMarketplaceCache(): Promise<void> {
    try {
        LogInfo("Warming marketplace cache...");
        // This would be called periodically to pre-load popular listings
        // Implementation would fetch active listings and cache them
    } catch (error) {
        LogError("Failed to warm marketplace cache: " + error);
    }
}

export default PersistentCache;