package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/ziyangli/fsrs/backend/internal/model"
	"github.com/ziyangli/fsrs/backend/internal/repository"
)

type fakeAuthUserStore struct {
	user *model.User
}

func (f fakeAuthUserStore) Create(_ context.Context, email, passwordHash string) (*model.User, error) {
	return &model.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
	}, nil
}

func (f fakeAuthUserStore) GetByEmail(_ context.Context, email string) (*model.User, error) {
	if f.user != nil && f.user.Email == email {
		return f.user, nil
	}
	return nil, repository.ErrNotFound
}

func (f fakeAuthUserStore) GetByID(_ context.Context, _ uuid.UUID) (*model.User, error) {
	return f.user, nil
}

func (f fakeAuthUserStore) IncrementTokenVersion(_ context.Context, _ uuid.UUID) error {
	return nil
}

type fakeHandlerAuthThrottle struct {
	allowFn func(
		ctx context.Context,
		scope string,
		key string,
		limit int,
		window time.Duration,
		blockDuration time.Duration,
	) (bool, time.Duration, error)
	resetFn func(ctx context.Context, scope string, key string) error
}

func (f fakeHandlerAuthThrottle) Allow(
	ctx context.Context,
	scope string,
	key string,
	limit int,
	window time.Duration,
	blockDuration time.Duration,
) (bool, time.Duration, error) {
	return f.allowFn(ctx, scope, key, limit, window, blockDuration)
}

func (f fakeHandlerAuthThrottle) Reset(ctx context.Context, scope string, key string) error {
	if f.resetFn == nil {
		return nil
	}
	return f.resetFn(ctx, scope, key)
}

func TestNormalizeEmail(t *testing.T) {
	got := normalizeEmail("  Test.User+alias@Example.COM ")
	want := "test.user+alias@example.com"

	if got != want {
		t.Fatalf("normalizeEmail() = %q, want %q", got, want)
	}
}

func TestAuthHandler_Register_Validation(t *testing.T) {
	// These tests only check validation that happens BEFORE repo access
	tests := []struct {
		name       string
		body       map[string]string
		wantStatus int
	}{
		{
			name:       "missing email",
			body:       map[string]string{"password": "password123"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing password",
			body:       map[string]string{"email": "test@example.com"},
			wantStatus: http.StatusBadRequest,
		},
	}

	// Create handler with nil repo - only tests validation before repo access
	h := &AuthHandler{jwtSecret: []byte("test-secret")}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.Register(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestAuthHandler_Login_Validation(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]string
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing email",
			body:        map[string]string{"password": "password123"},
			wantStatus:  http.StatusBadRequest,
			wantMessage: "Email and password are required",
		},
		{
			name:        "missing password",
			body:        map[string]string{"email": "test@example.com"},
			wantStatus:  http.StatusBadRequest,
			wantMessage: "Email and password are required",
		},
		{
			name:        "invalid email format",
			body:        map[string]string{"email": "not-an-email", "password": "password123"},
			wantStatus:  http.StatusBadRequest,
			wantMessage: "Invalid email format",
		},
		{
			name:        "overlong email",
			body:        map[string]string{"email": strings.Repeat("a", 246) + "@example.com", "password": "password123"},
			wantStatus:  http.StatusBadRequest,
			wantMessage: "255 characters or fewer",
		},
	}

	h := &AuthHandler{
		userRepo:   fakeAuthUserStore{},
		jwtSecret:  []byte("test-secret"),
		authThrottle: fakeHandlerAuthThrottle{
			allowFn: func(_ context.Context, _ string, _ string, _ int, _ time.Duration, _ time.Duration) (bool, time.Duration, error) {
				t.Fatal("expected login validation to reject before throttle lookup")
				return false, 0, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.Login(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantMessage) {
				t.Fatalf("unexpected response body: %s", rec.Body.String())
			}
		})
	}
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	h := &AuthHandler{jwtSecret: []byte("test-secret")}

	// Test invalid JSON body
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAuthHandler_Login_RejectsEmailThrottleLimit(t *testing.T) {
	h := &AuthHandler{
		jwtSecret: []byte("test-secret"),
		authThrottle: fakeHandlerAuthThrottle{
			allowFn: func(_ context.Context, scope, key string, limit int, window, blockDuration time.Duration) (bool, time.Duration, error) {
				if scope != loginEmailScope {
					t.Fatalf("scope = %q, want %q", scope, loginEmailScope)
				}
				if key != "test@example.com" {
					t.Fatalf("key = %q, want %q", key, "test@example.com")
				}
				if limit != loginEmailLimit {
					t.Fatalf("limit = %d, want %d", limit, loginEmailLimit)
				}
				if window != loginEmailWindow {
					t.Fatalf("window = %s, want %s", window, loginEmailWindow)
				}
				if blockDuration != loginEmailBlockDuration {
					t.Fatalf("block duration = %s, want %s", blockDuration, loginEmailBlockDuration)
				}
				return false, 2 * time.Minute, nil
			},
		},
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/login",
		bytes.NewReader([]byte(`{"email":"test@example.com","password":"password123"}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") != "120" {
		t.Fatalf("Retry-After = %q, want %q", rec.Header().Get("Retry-After"), "120")
	}
}

func TestAuthHandler_Register_RejectsEmailThrottleLimit(t *testing.T) {
	h := &AuthHandler{
		jwtSecret: []byte("test-secret"),
		authThrottle: fakeHandlerAuthThrottle{
			allowFn: func(_ context.Context, scope, key string, limit int, window, blockDuration time.Duration) (bool, time.Duration, error) {
				if scope != registerEmailScope {
					t.Fatalf("scope = %q, want %q", scope, registerEmailScope)
				}
				if key != "test@example.com" {
					t.Fatalf("key = %q, want %q", key, "test@example.com")
				}
				if limit != registerEmailLimit {
					t.Fatalf("limit = %d, want %d", limit, registerEmailLimit)
				}
				if window != registerEmailWindow {
					t.Fatalf("window = %s, want %s", window, registerEmailWindow)
				}
				if blockDuration != registerEmailBlockDuration {
					t.Fatalf("block duration = %s, want %s", blockDuration, registerEmailBlockDuration)
				}
				return false, 15 * time.Minute, nil
			},
		},
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/register",
		bytes.NewReader([]byte(`{"email":"Test@example.com","password":"password123"}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") != "900" {
		t.Fatalf("Retry-After = %q, want %q", rec.Header().Get("Retry-After"), "900")
	}
}

func TestAuthHandler_Login_ResetsEmailThrottleOnSuccess(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}

	var resetScope string
	var resetKey string

	h := &AuthHandler{
		userRepo: fakeAuthUserStore{
			user: &model.User{
				ID:           uuid.New(),
				Email:        "test@example.com",
				PasswordHash: string(passwordHash),
				TokenVersion: 3,
			},
		},
		jwtSecret: []byte("test-secret"),
		authThrottle: fakeHandlerAuthThrottle{
			allowFn: func(_ context.Context, _ string, _ string, _ int, _ time.Duration, _ time.Duration) (bool, time.Duration, error) {
				return true, 0, nil
			},
			resetFn: func(_ context.Context, scope string, key string) error {
				resetScope = scope
				resetKey = key
				return nil
			},
		},
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/login",
		bytes.NewReader([]byte(`{"email":"Test@example.com","password":"password123"}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if resetScope != loginEmailScope {
		t.Fatalf("reset scope = %q, want %q", resetScope, loginEmailScope)
	}
	if resetKey != "test@example.com" {
		t.Fatalf("reset key = %q, want %q", resetKey, "test@example.com")
	}
}

func TestAuthHandler_Register_RejectsOverlongPassword(t *testing.T) {
	h := &AuthHandler{jwtSecret: []byte("test-secret")}

	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": strings.Repeat("a", maxPasswordBytes+1),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "72 bytes or fewer") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestAuthHandler_Register_RejectsShortUnicodePassword(t *testing.T) {
	h := &AuthHandler{jwtSecret: []byte("test-secret")}

	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": strings.Repeat("🙂", 4),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "at least 8 characters") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestAuthHandler_Register_RejectsOverlongEmail(t *testing.T) {
	h := &AuthHandler{jwtSecret: []byte("test-secret")}

	body, _ := json.Marshal(map[string]string{
		"email":    strings.Repeat("a", 246) + "@example.com",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "255 characters or fewer") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestAuthHandler_Register_RejectsUnknownFields(t *testing.T) {
	h := &AuthHandler{jwtSecret: []byte("test-secret")}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/register",
		bytes.NewReader([]byte(`{"email":"test@example.com","password":"password123","extra":"value"}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "unknown fields") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	h := &AuthHandler{}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusNoContent)
	}

	// Check cookie is cleared
	cookies := rec.Result().Cookies()
	var tokenCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "token" {
			tokenCookie = c
			break
		}
	}

	if tokenCookie == nil {
		t.Error("expected token cookie to be set")
	} else if tokenCookie.MaxAge != -1 {
		t.Errorf("expected cookie MaxAge to be -1, got %d", tokenCookie.MaxAge)
	}
}
