package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	requests   map[string][]time.Time
	mu         sync.Mutex
	limit      int
	window     time.Duration
	trustProxy bool // Only trust proxy-provided client IP headers when explicitly enabled
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests:   make(map[string][]time.Time),
		limit:      limit,
		window:     window,
		trustProxy: false,
	}
	// Cleanup old entries periodically
	go rl.cleanup()
	return rl
}

// SetTrustProxy configures whether to trust proxy-provided client IP headers.
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

// getClientIP extracts the client IP address from the request.
// When proxy trust is enabled, prefer X-Real-IP from the reverse proxy.
func (rl *RateLimiter) getClientIP(r *http.Request) string {
	if rl.trustProxy {
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			if parsedIP := net.ParseIP(realIP); parsedIP != nil {
				return parsedIP.String()
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
