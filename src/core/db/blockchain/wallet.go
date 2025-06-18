package blockchain

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"context"
	"sync"
	"time"
)

type Post struct {
	To         string
	From       string
	Blockchain string
	Status     string // Caching Status - backfilling, complete
}
type WalletCacheEntry struct {
	Value     interface{}
	Timestamp time.Time
}

var (
	walletCache  = make(map[string]WalletCacheEntry)
	cacheMux     sync.RWMutex
	cacheTTL     = 1 * time.Hour
	cacheTimeout = 60 * time.Second
)

// Wallet Data Caching Functions
func getCacheKey(operation, blockchain, param string) string {
	// Generates cache keys as "operation:blockchain:param"
	return operation + ":" + blockchain + ":" + param
}
func getCachedValue(key string) (interface{}, bool) {
	// Thread-safe cache lookup with expiration check
	cacheMux.RLock()
	defer cacheMux.RUnlock()
	if entry, exists := walletCache[key]; exists {
		if time.Since(entry.Timestamp) < cacheTTL {
			core.LogDebug("Wallet cache hit for: " + key)
			return entry.Value, true
		}
		// Remove expired entry
		delete(walletCache, key)
	}
	return nil, false
}
func setCachedValue(key string, value interface{}) {
	// Thread-safe cache storage
	cacheMux.Lock()
	defer cacheMux.Unlock()

	walletCache[key] = WalletCacheEntry{
		Value:     value,
		Timestamp: time.Now(),
	}
	core.LogDebug("Wallet cache set for: " + key)
}
func executeWithCache[T any](cacheKey string, operation func() (T, error)) (T, error) {
	// Execute with timeout and caching
	// Check cache first
	if cached, found := getCachedValue(cacheKey); found {
		if result, ok := cached.(T); ok {
			return result, nil
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cacheTimeout)
	defer cancel()

	// Execute operation with timeout
	resultChan := make(chan T, 1)
	errorChan := make(chan error, 1)

	go func() {
		result, err := operation()
		if err != nil {
			errorChan <- err
		} else {
			resultChan <- result
		}
	}()

	select {
	case result := <-resultChan:
		// Cache successful result
		setCachedValue(cacheKey, result)
		return result, nil
	case err := <-errorChan:
		var zero T
		return zero, err
	case <-ctx.Done():
		var zero T
		return zero, core.LogErrorReturn("Operation timeout for: " + cacheKey)
	}
}
func CleanWalletCache() {
	// Cache cleanup function (removes expired entries)
	cacheMux.Lock()
	defer cacheMux.Unlock()
	now := time.Now()
	cleaned := 0
	for key, entry := range walletCache {
		if now.Sub(entry.Timestamp) > cacheTTL {
			delete(walletCache, key)
			cleaned++
		}
	}
	if cleaned > 0 {
		core.LogDebug("Cleaned expired wallet cache entries")
	}
}

// Wallet Interaction Functions
func GetBalance(blockchain string, address string, database *db.Database) (float64, error) {
	cacheKey := getCacheKey("getBalance", blockchain, address)

	return executeWithCache(cacheKey, func() (float64, error) {
		if blockchain == "base" {
			balance, err := BaseGetBalance(address, database)
			if err != nil {
				return 0, err
			}
			return float64(balance.Uint64()), nil
		}
		return 0, core.LogErrorReturn("Could not get balance of address")
	})
}
func WalletGetName(blockchain string, address string, _blockchain *Blockchain) (string, error) {
	cacheKey := getCacheKey("getName", blockchain, address)

	return executeWithCache(cacheKey, func() (string, error) {
		if blockchain == "base" {
			names, err := _blockchain.Base.GetENSNames(address)
			if err != nil {
				return "", err
			}
			if len(names) > 0 {
				return names[0], nil
			}
		}
		return "", nil
	})
}
func WalletGetAddress(blockchain string, name string, _blockchain *Blockchain) (string, error) {
	cacheKey := getCacheKey("getAddress", blockchain, name)

	return executeWithCache(cacheKey, func() (string, error) {
		if blockchain == "base" {
			addresses, err := _blockchain.Base.GetENSAddresses(name)
			if err != nil {
				return "", core.LogErrorReturn("Could not get address from ENS name: " + err.Error())
			}
			if len(addresses) > 0 {
				return addresses[0], nil
			}
		}
		return "", nil
	})
}
