package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	requests       map[string][]time.Time
	mu             sync.Mutex
	limit          int
	window         time.Duration
	trustProxy     bool // Only trust X-Forwarded-For when behind a known proxy
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests:   make(map[string][]time.Time),
		limit:      limit,
		window:     window,
		trustProxy: true, // Enable by default for reverse proxy setups (nginx)
	}
	// Cleanup old entries periodically
	go rl.cleanup()
	return rl
}

// SetTrustProxy configures whether to trust X-Forwarded-For header
func (rl *RateLimiter) SetTrustProxy(trust bool) {
	rl.trustProxy = trust
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, times := range rl.requests {
			var valid []time.Time
			for _, t := range times {
				if now.Sub(t) < rl.window {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(rl.requests, ip)
			} else {
				rl.requests[ip] = valid
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.getClientIP(r)

		rl.mu.Lock()
		now := time.Now()

		// Filter out old requests
		var valid []time.Time
		for _, t := range rl.requests[ip] {
			if now.Sub(t) < rl.window {
				valid = append(valid, t)
			}
		}

		if len(valid) >= rl.limit {
			rl.mu.Unlock()
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		valid = append(valid, now)
		rl.requests[ip] = valid
		rl.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the client IP address from the request
func (rl *RateLimiter) getClientIP(r *http.Request) string {
	// Only trust proxy headers if configured to do so
	if rl.trustProxy {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// The first one is the original client
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			// Take only the first IP (the original client)
			if idx := strings.Index(forwarded, ","); idx != -1 {
				forwarded = strings.TrimSpace(forwarded[:idx])
			}
			// Validate it looks like an IP address
			if parsedIP := net.ParseIP(forwarded); parsedIP != nil {
				return forwarded
			}
		}

		// Also check X-Real-IP (set by nginx)
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			if parsedIP := net.ParseIP(realIP); parsedIP != nil {
				return realIP
			}
		}
	}

	// Fall back to RemoteAddr, stripping the port
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
