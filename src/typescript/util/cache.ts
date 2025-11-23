import {LogError} from "./log";

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

export interface ProfileData {
    name: string | null;
    avatar: string | null;
    description: string | null;
    address: string;
    blockchain: string;
}

export const globalProfileCache = new PersistentCache({
    defaultTtl: 259200000,
    keyPrefix: "profile_"
});

export default PersistentCache;