package middleware

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
	"net/http"
	"sync"
	"time"
)

var clientBuckets = struct {
	sync.RWMutex
	buckets map[string]*rate.Limiter
}{
	buckets: make(map[string]*rate.Limiter),
}

func RateLimitMiddleware() gin.HandlerFunc { // This filter enforces rate limits per IP address
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ip == "127.0.0.1" || ip == "::1" { // Don't rate limit localhost
			c.Next()
			return
		}
		clientBuckets.RLock()
		bucket, exists := clientBuckets.buckets[ip]
		clientBuckets.RUnlock()
		if !exists {
			clientBuckets.Lock()
			bucket = rate.NewLimiter(rate.Every(1*time.Second), 1000) // 1000 requests per second
			clientBuckets.buckets[ip] = bucket
			clientBuckets.Unlock()
		}
		if bucket.Allow() == false {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}
