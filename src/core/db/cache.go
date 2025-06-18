package db

import (
	"YourPlace/src/core"
	"context"
	"sync"
	"time"
)

type CacheEntry[T any] struct {
	Value     T
	Timestamp time.Time
}
type Cache[T any] struct {
	data    map[string]CacheEntry[T]
	mutex   sync.RWMutex
	ttl     time.Duration
	timeout time.Duration
	name    string
}
type CacheManager struct {
	caches map[string]interface{}
	mutex  sync.RWMutex
}
type CacheCleaner interface {
	Clean() int
}

var globalCacheManager = &CacheManager{
	caches: make(map[string]interface{}),
}

func NewCache[T any](name string, ttl, timeout time.Duration) *Cache[T] {
	globalCacheManager.mutex.Lock()
	defer globalCacheManager.mutex.Unlock()
	cache := &Cache[T]{
		data:    make(map[string]CacheEntry[T]),
		ttl:     ttl,
		timeout: timeout,
		name:    name,
	}
	globalCacheManager.caches[name] = cache
	return cache
}
func GetCache[T any](name string) *Cache[T] {
	globalCacheManager.mutex.RLock()
	defer globalCacheManager.mutex.RUnlock()
	if cache, exists := globalCacheManager.caches[name]; exists {
		if typedCache, ok := cache.(*Cache[T]); ok {
			return typedCache
		}
	}
	return nil
}
func CleanAllCaches() {
	globalCacheManager.mutex.RLock()
	defer globalCacheManager.mutex.RUnlock()
	totalCleaned := 0
	for name, cache := range globalCacheManager.caches {
		if cleaner, ok := cache.(CacheCleaner); ok {
			cleaned := cleaner.Clean()
			totalCleaned += cleaned
		} else {
			core.LogDebug("Cache " + name + " does not implement CacheCleaner interface")
		}
	}
	if totalCleaned > 0 {
		core.LogDebug("Total cleaned cache entries across all caches")
	}
}

func (c *Cache[T]) Get(key string) (T, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	var zero T
	if entry, exists := c.data[key]; exists {
		if time.Since(entry.Timestamp) < c.ttl {
			core.LogDebug("Cache hit for: " + key + " in " + c.name)
			return entry.Value, true
		}
		// Remove expired entry
		delete(c.data, key)
	}
	return zero, false
}
func (c *Cache[T]) Set(key string, value T) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data[key] = CacheEntry[T]{
		Value:     value,
		Timestamp: time.Now(),
	}
	core.LogDebug("Cache set for: " + key + " in " + c.name)
}
func (c *Cache[T]) ExecuteWithCache(key string, operation func() (T, error)) (T, error) {
	// Check cache first
	if cached, found := c.Get(key); found {
		return cached, nil
	}
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
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
		c.Set(key, result)
		return result, nil
	case err := <-errorChan:
		var zero T
		return zero, err
	case <-ctx.Done():
		var zero T
		return zero, core.LogErrorReturn("Operation timeout for: " + key)
	}
}
func (c *Cache[T]) Clean() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	now := time.Now()
	cleaned := 0
	for key, entry := range c.data {
		if now.Sub(entry.Timestamp) > c.ttl {
			delete(c.data, key)
			cleaned++
		}
	}
	if cleaned > 0 {
		core.LogDebug("Cleaned expired entries from " + c.name + " cache")
	}
	return cleaned
}
