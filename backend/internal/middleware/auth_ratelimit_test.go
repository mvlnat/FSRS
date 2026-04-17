package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeAuthThrottle struct {
	allowFn func(
		ctx context.Context,
		scope string,
		key string,
		limit int,
		window time.Duration,
		blockDuration time.Duration,
	) (bool, time.Duration, error)
}

func (f fakeAuthThrottle) Allow(
	ctx context.Context,
	scope string,
	key string,
	limit int,
	window time.Duration,
	blockDuration time.Duration,
) (bool, time.Duration, error) {
	return f.allowFn(ctx, scope, key, limit, window, blockDuration)
}

func TestAuthRateLimitMiddleware_AllowsRequest(t *testing.T) {
	middleware := NewAuthRateLimitMiddleware(
		fakeAuthThrottle{
			allowFn: func(_ context.Context, scope, key string, limit int, window, blockDuration time.Duration) (bool, time.Duration, error) {
				if scope != "auth_ip" {
					t.Fatalf("scope = %q, want %q", scope, "auth_ip")
				}
				if key != "192.0.2.10" {
					t.Fatalf("key = %q, want %q", key, "192.0.2.10")
				}
				if limit != 20 {
					t.Fatalf("limit = %d, want %d", limit, 20)
				}
				if window != 5*time.Minute {
					t.Fatalf("window = %s, want %s", window, 5*time.Minute)
				}
				if blockDuration != 15*time.Minute {
					t.Fatalf("block duration = %s, want %s", blockDuration, 15*time.Minute)
				}
				return true, 0, nil
			},
		},
		"auth_ip",
		20,
		5*time.Minute,
		15*time.Minute,
	)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestAuthRateLimitMiddleware_RejectsLimitedRequest(t *testing.T) {
	middleware := NewAuthRateLimitMiddleware(
		fakeAuthThrottle{
			allowFn: func(_ context.Context, _ string, _ string, _ int, _ time.Duration, _ time.Duration) (bool, time.Duration, error) {
				return false, 90 * time.Second, nil
			},
		},
		"auth_ip",
		20,
		5*time.Minute,
		15*time.Minute,
	)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") != "90" {
		t.Fatalf("Retry-After = %q, want %q", rec.Header().Get("Retry-After"), "90")
	}
}

func TestAuthRateLimitMiddleware_UsesTrustedProxyHeader(t *testing.T) {
	middleware := NewAuthRateLimitMiddleware(
		fakeAuthThrottle{
			allowFn: func(_ context.Context, _ string, key string, _ int, _ time.Duration, _ time.Duration) (bool, time.Duration, error) {
				if key != "198.51.100.20" {
					t.Fatalf("key = %q, want %q", key, "198.51.100.20")
				}
				return true, 0, nil
			},
		},
		"auth_ip",
		20,
		5*time.Minute,
		15*time.Minute,
	)
	middleware.SetTrustProxy(true)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Real-IP", "198.51.100.20")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestAuthRateLimitMiddleware_HandlesThrottleError(t *testing.T) {
	middleware := NewAuthRateLimitMiddleware(
		fakeAuthThrottle{
			allowFn: func(_ context.Context, _ string, _ string, _ int, _ time.Duration, _ time.Duration) (bool, time.Duration, error) {
				return false, 0, errors.New("db unavailable")
			},
		},
		"auth_ip",
		20,
		5*time.Minute,
		15*time.Minute,
	)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
