package middleware

import (
	"net/http"
	"sync"
	"time"

	"arthik/config"
	"arthik/utils"

	"golang.org/x/time/rate"
)

var (
	rateLimiters   = make(map[string]*rateLimiterClient)
	rateLimiterMux sync.RWMutex
)

type rateLimiterClient struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit middleware limits requests per IP
func RateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := utils.GetClientIP(r)
		limiter := getRateLimiter(ip)

		if !limiter.Allow() {
			utils.LogAudit(ip, "unknown", "RATE_LIMIT_EXCEEDED", r.URL.Path, false)
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}

// getRateLimiter gets or creates a rate limiter for an IP
func getRateLimiter(ip string) *rate.Limiter {
	rateLimiterMux.Lock()
	defer rateLimiterMux.Unlock()

	client, exists := rateLimiters[ip]
	if !exists {
		limiter := rate.NewLimiter(config.RequestsPerSecond, config.BurstSize)
		rateLimiters[ip] = &rateLimiterClient{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		return limiter
	}

	client.lastSeen = time.Now()
	return client.limiter
}

// CleanupRateLimiters periodically cleans up old rate limiters
func CleanupRateLimiters() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rateLimiterMux.Lock()
		for ip, client := range rateLimiters {
			if time.Since(client.lastSeen) > 10*time.Minute {
				delete(rateLimiters, ip)
			}
		}
		rateLimiterMux.Unlock()
	}
}