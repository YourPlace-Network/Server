interface CacheEntry<T> {
    data: T;
    expiresAt: number;
}
const DEFAULT_LIFETIME_MS = 24 * 60 * 60 * 1000; // 24 hours
export class PersistentCache {
    private readonly prefix: string;
    private readonly lifetimeMs: number;
    constructor(prefix: string, lifetimeMs: number = DEFAULT_LIFETIME_MS) {
        this.prefix = prefix;
        this.lifetimeMs = lifetimeMs;
    }
    private buildKey(key: string): string {
        return `yp_cache_${this.prefix}_${key}`;
    }
    get<T>(key: string): T | null {
        const storageKey = this.buildKey(key);
        const raw = localStorage.getItem(storageKey);
        if (!raw) {
            return null;
        }
        try {
            const entry: CacheEntry<T> = JSON.parse(raw);
            if (Date.now() > entry.expiresAt) {
                localStorage.removeItem(storageKey);
                return null;
            }
            return entry.data;
        } catch {
            localStorage.removeItem(storageKey);
            return null;
        }
    }
    set<T>(key: string, data: T): void {
        const storageKey = this.buildKey(key);
        const entry: CacheEntry<T> = {
            data,
            expiresAt: Date.now() + this.lifetimeMs,
        };
        try {
            localStorage.setItem(storageKey, JSON.stringify(entry));
        } catch {
            // localStorage quota exceeded or unavailable
        }
    }
    remove(key: string): void {
        localStorage.removeItem(this.buildKey(key));
    }
    clear(): void {
        const keysToRemove: string[] = [];
        for (let i = 0; i < localStorage.length; i++) {
            const key = localStorage.key(i);
            if (key && key.startsWith(`yp_cache_${this.prefix}_`)) {
                keysToRemove.push(key);
            }
        }
        keysToRemove.forEach(key => localStorage.removeItem(key));
    }
}
