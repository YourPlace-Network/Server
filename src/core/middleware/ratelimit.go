package middleware

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
	"net/http"
	"sync"
	"time"
)

type bucketEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var clientBuckets = struct {
	sync.RWMutex
	buckets map[string]*bucketEntry
}{
	buckets: make(map[string]*bucketEntry),
}

func RateLimitMiddleware() gin.HandlerFunc { // This filter enforces rate limits per IP address
	go cleanupOldBuckets()
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ip == "127.0.0.1" || ip == "::1" { // Don't rate limit localhost
			c.Next()
			return
		}
		now := time.Now()
		clientBuckets.RLock()
		entry, exists := clientBuckets.buckets[ip]
		clientBuckets.RUnlock()
		if !exists {
			clientBuckets.Lock()
			entry = &bucketEntry{
				limiter:  rate.NewLimiter(rate.Every(1*time.Second), 1000), // 1000 requests per second
				lastSeen: now,
			}
			clientBuckets.buckets[ip] = entry
			clientBuckets.Unlock()
		} else {
			clientBuckets.Lock()
			entry.lastSeen = now
			clientBuckets.Unlock()
		}
		if !entry.limiter.Allow() {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}
func cleanupOldBuckets() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cutoff := time.Now().Add(-30 * time.Minute)
			clientBuckets.Lock()
			for ip, entry := range clientBuckets.buckets {
				if entry.lastSeen.Before(cutoff) {
					delete(clientBuckets.buckets, ip)
				}
			}
			clientBuckets.Unlock()
		}
	}
}
