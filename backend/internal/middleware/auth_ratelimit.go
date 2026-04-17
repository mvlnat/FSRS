package middleware

import (
	"context"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AuthThrottle interface {
	Allow(
		ctx context.Context,
		scope string,
		key string,
		limit int,
		window time.Duration,
		blockDuration time.Duration,
	) (bool, time.Duration, error)
}

type AuthRateLimitMiddleware struct {
	throttle      AuthThrottle
	scope         string
	limit         int
	window        time.Duration
	blockDuration time.Duration
	trustProxy    bool
}

func NewAuthRateLimitMiddleware(
	throttle AuthThrottle,
	scope string,
	limit int,
	window time.Duration,
	blockDuration time.Duration,
) *AuthRateLimitMiddleware {
	return &AuthRateLimitMiddleware{
		throttle:      throttle,
		scope:         scope,
		limit:         limit,
		window:        window,
		blockDuration: blockDuration,
	}
}

func (m *AuthRateLimitMiddleware) SetTrustProxy(trust bool) {
	m.trustProxy = trust
}

func (m *AuthRateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.throttle == nil {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := authClientIP(r, m.trustProxy)
		allowed, retryAfter, err := m.throttle.Allow(
			r.Context(),
			m.scope,
			clientIP,
			m.limit,
			m.window,
			m.blockDuration,
		)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !allowed {
			writeRetryAfter(w, retryAfter)
			http.Error(w, "Too many authentication attempts", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func authClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			if parsedIP := net.ParseIP(realIP); parsedIP != nil {
				return parsedIP.String()
			}
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func writeRetryAfter(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}
