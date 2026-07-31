package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides per-IP rate limiting for upload endpoints.
type RateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*combinedLimiter
	config   RateLimitConfig
}

// RateLimitConfig defines rate limiting parameters.
type RateLimitConfig struct {
	UploadsPerMinute   int
	UploadsPerHour     int
	MaxBurstMultiplier int
}

// DefaultRateLimitConfig returns standard rate limits for upload endpoints.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		UploadsPerMinute:   10,
		UploadsPerHour:     100,
		MaxBurstMultiplier: 2,
	}
}

// NewRateLimiter creates a new rate limiter with default configuration.
func NewRateLimiter() *RateLimiter {
	return NewRateLimiterWithConfig(DefaultRateLimitConfig())
}

// NewRateLimiterWithConfig creates a rate limiter with custom configuration.
func NewRateLimiterWithConfig(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*combinedLimiter),
		config:   config,
	}
	go rl.cleanup()
	return rl
}

// Allow checks if the request from given IP should be allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	if ip == "" || ip == "unknown" {
		ip = "anonymous"
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[ip]
	if !exists {
		// Create combined limiter with both minute and hour limits
		rl.limiters[ip] = &combinedLimiter{
			minute:   rate.NewLimiter(rate.Limit(rl.config.UploadsPerMinute), rl.config.UploadsPerMinute*rl.config.MaxBurstMultiplier),
			hour:     rate.NewLimiter(rate.Limit(rl.config.UploadsPerHour/60), rl.config.UploadsPerHour*rl.config.MaxBurstMultiplier),
			lastUsed: time.Now(),
		}
		return true
	}

	return limiter.Allow()
}

// Cleanup removes expired entries to prevent memory leaks.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, limiter := range rl.limiters {
			limiter.mu.Lock()
			isExpired := time.Since(limiter.lastUsed) > 1*time.Hour
			limiter.mu.Unlock()
			
			if isExpired {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// combinedLimiter combines minute and hour rate limits using an AND approach.
type combinedLimiter struct {
	mu       sync.Mutex
	minute   *rate.Limiter
	hour     *rate.Limiter
	lastUsed time.Time
}

func (cl *combinedLimiter) Allow() bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	
	allowed := cl.minute.Allow() && cl.hour.Allow()
	if allowed {
		cl.lastUsed = time.Now()
	}
	return allowed
}

// GetWaitTime calculates how long to wait before next allowed request.
func (rl *RateLimiter) GetWaitTime(ip string) time.Duration {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	limiter, exists := rl.limiters[ip]
	if !exists {
		return 0
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	
	// For simplicity, return default wait time
	return 60 * time.Second
}

// RateLimitMiddleware wraps HTTP handlers with rate limiting.
func (rl *RateLimiter) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		
		if !rl.Allow(ip) {
			waitTime := rl.GetWaitTime(ip)
			
			w.Header().Set("Retry-After", formatRetryAfter(waitTime))
			w.Header().Set("X-RateLimit-Limit", "10/minute")
			w.Header().Set("X-RateLimit-Remaining", "0")
			
			if r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate_limit_exceeded","message":"Too many requests. Please try again later."}`))
				return
			}
			
			http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// UploadEndpointLimiter specifically handles upload endpoint rate limiting.
func (rl *RateLimiter) UploadEndpointLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to POST requests on template endpoints
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		
		path := r.URL.Path
		if !isTemplatePath(path) {
			next.ServeHTTP(w, r)
			return
		}
		
		ip := clientIP(r)
		if !rl.Allow(ip) {
			retryAfter := int(rl.GetWaitTime(ip).Seconds())
			if retryAfter == 0 {
				retryAfter = 60
			}
			
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("X-RateLimit-Done", "true")
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(fmt.Sprintf(`{"error":"upload_rate_limit_exceeded","message":"You have exceeded the upload limit. Please try again in %d seconds."}`, retryAfter)))
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func isTemplatePath(path string) bool {
	return path == "/template" || 
		   strings.HasPrefix(path, "/template/") ||
		   path == "/hotspot/upload" ||
		   strings.HasPrefix(path, "/hotspot/upload/") ||
		   path == "/assets/logo" ||
		   path == "/assets/background"
}

func formatRetryAfter(duration time.Duration) string {
	if duration <= 0 {
		return "60"
	}
	seconds := int(duration.Seconds())
	if seconds < 1 {
		seconds = 60
	}
	return strconv.Itoa(seconds)
}

// RateLimiterStats returns current rate limit statistics for monitoring.
func (rl *RateLimiter) RateLimiterStats(ip string) map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	stats := make(map[string]interface{})
	stats["ip"] = ip
	
	if limiter, exists := rl.limiters[ip]; exists {
		limiter.mu.Lock()
		defer limiter.mu.Unlock()
		
		minuteTokens := float64(limiter.minute.Tokens())
		hourTokens := float64(limiter.hour.Tokens())
		
		stats["minute_remaining"] = minuteTokens
		stats["hour_remaining"] = hourTokens
		stats["has_limiter"] = true
	} else {
		stats["has_limiter"] = false
	}
	
	return stats
}
