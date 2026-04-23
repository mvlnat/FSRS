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

	"github.com/ziyangli/fsrs/backend/internal/middleware"
	"github.com/ziyangli/fsrs/backend/internal/model"
	"github.com/ziyangli/fsrs/backend/internal/repository"
)

type fakeAuthUserStore struct {
	user                *model.User
	createFn            func(ctx context.Context, email, passwordHash string) (*model.User, error)
	getByEmailFn        func(ctx context.Context, email string) (*model.User, error)
	getByIDFn           func(ctx context.Context, id uuid.UUID) (*model.User, error)
	incrementTokenFn    func(ctx context.Context, id uuid.UUID) error
	markEmailVerifiedFn func(ctx context.Context, id uuid.UUID) error
	resetPasswordFn     func(ctx context.Context, id uuid.UUID, passwordHash string) error
}

func (f fakeAuthUserStore) Create(ctx context.Context, email, passwordHash string) (*model.User, error) {
	if f.createFn != nil {
		return f.createFn(ctx, email, passwordHash)
	}
	return &model.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
	}, nil
}

func (f fakeAuthUserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if f.getByEmailFn != nil {
		return f.getByEmailFn(ctx, email)
	}
	if f.user != nil && f.user.Email == email {
		return f.user, nil
	}
	return nil, repository.ErrNotFound
}

func (f fakeAuthUserStore) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, id)
	}
	if f.user == nil {
		return nil, repository.ErrNotFound
	}
	return f.user, nil
}

func (f fakeAuthUserStore) IncrementTokenVersion(ctx context.Context, id uuid.UUID) error {
	if f.incrementTokenFn != nil {
		return f.incrementTokenFn(ctx, id)
	}
	return nil
}

func (f fakeAuthUserStore) MarkEmailVerified(ctx context.Context, id uuid.UUID) error {
	if f.markEmailVerifiedFn != nil {
		return f.markEmailVerifiedFn(ctx, id)
	}
	if f.user == nil || f.user.ID != id {
		return repository.ErrNotFound
	}
	now := time.Now()
	f.user.EmailVerifiedAt = &now
	return nil
}

func (f fakeAuthUserStore) ResetPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	if f.resetPasswordFn != nil {
		return f.resetPasswordFn(ctx, id, passwordHash)
	}
	if f.user == nil || f.user.ID != id {
		return repository.ErrNotFound
	}
	now := time.Now()
	f.user.PasswordHash = passwordHash
	f.user.TokenVersion++
	f.user.EmailVerifiedAt = &now
	return nil
}

type fakeAuthEmailTokenStore struct {
	createFn  func(ctx context.Context, userID uuid.UUID, purpose repository.AuthEmailTokenPurpose, tokenHash string, expiresAt time.Time) error
	consumeFn func(ctx context.Context, purpose repository.AuthEmailTokenPurpose, tokenHash string, now time.Time) (uuid.UUID, error)
}

func (f fakeAuthEmailTokenStore) Create(
	ctx context.Context,
	userID uuid.UUID,
	purpose repository.AuthEmailTokenPurpose,
	tokenHash string,
	expiresAt time.Time,
) error {
	if f.createFn == nil {
		return nil
	}
	return f.createFn(ctx, userID, purpose, tokenHash, expiresAt)
}

func (f fakeAuthEmailTokenStore) Consume(
	ctx context.Context,
	purpose repository.AuthEmailTokenPurpose,
	tokenHash string,
	now time.Time,
) (uuid.UUID, error) {
	if f.consumeFn == nil {
		return uuid.Nil, repository.ErrNotFound
	}
	return f.consumeFn(ctx, purpose, tokenHash, now)
}

type fakeAuthEmailSender struct {
	sendVerificationEmailFn  func(ctx context.Context, email, verificationURL string) error
	sendPasswordResetEmailFn func(ctx context.Context, email, resetURL string) error
	checkConfigFn            func() error
}

func (f fakeAuthEmailSender) SendVerificationEmail(ctx context.Context, email, verificationURL string) error {
	if f.sendVerificationEmailFn == nil {
		return nil
	}
	return f.sendVerificationEmailFn(ctx, email, verificationURL)
}

func (f fakeAuthEmailSender) SendPasswordResetEmail(ctx context.Context, email, resetURL string) error {
	if f.sendPasswordResetEmailFn == nil {
		return nil
	}
	return f.sendPasswordResetEmailFn(ctx, email, resetURL)
}

func (f fakeAuthEmailSender) CheckConfig() error {
	if f.checkConfigFn == nil {
		return nil
	}
	return f.checkConfigFn()
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
		userRepo:  fakeAuthUserStore{},
		jwtSecret: []byte("test-secret"),
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
	verifiedAt := time.Now()

	var resetScope string
	var resetKey string

	h := &AuthHandler{
		userRepo: fakeAuthUserStore{
			user: &model.User{
				ID:              uuid.New(),
				Email:           "test@example.com",
				PasswordHash:    string(passwordHash),
				TokenVersion:    3,
				EmailVerifiedAt: &verifiedAt,
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

func TestAuthHandler_Login_RejectsUnverifiedEmailAfterValidPassword(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}

	var resetCalled bool

	h := &AuthHandler{
		userRepo: fakeAuthUserStore{
			user: &model.User{
				ID:           uuid.New(),
				Email:        "test@example.com",
				PasswordHash: string(passwordHash),
				TokenVersion: 2,
			},
		},
		jwtSecret: []byte("test-secret"),
		authThrottle: fakeHandlerAuthThrottle{
			allowFn: func(_ context.Context, _ string, _ string, _ int, _ time.Duration, _ time.Duration) (bool, time.Duration, error) {
				return true, 0, nil
			},
			resetFn: func(_ context.Context, scope string, key string) error {
				if scope != loginEmailScope || key != "test@example.com" {
					t.Fatalf("unexpected reset scope/key: %s %s", scope, key)
				}
				resetCalled = true
				return nil
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

	if rec.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "Email not verified") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
	if !resetCalled {
		t.Fatal("expected login throttle reset after correct password")
	}
}

func TestAuthHandler_Register_SendsVerificationEmail(t *testing.T) {
	userID := uuid.New()
	var sentEmail string
	var verificationURL string
	var storedPurpose repository.AuthEmailTokenPurpose
	var storedHash string

	h := &AuthHandler{
		userRepo: fakeAuthUserStore{
			createFn: func(_ context.Context, email, passwordHash string) (*model.User, error) {
				if email != "test@example.com" {
					t.Fatalf("email = %q, want test@example.com", email)
				}
				if passwordHash == "" || passwordHash == "password123" {
					t.Fatal("expected password hash to be generated")
				}
				return &model.User{ID: userID, Email: email, PasswordHash: passwordHash}, nil
			},
		},
		emailTokenRepo: fakeAuthEmailTokenStore{
			createFn: func(_ context.Context, gotUserID uuid.UUID, purpose repository.AuthEmailTokenPurpose, tokenHash string, expiresAt time.Time) error {
				if gotUserID != userID {
					t.Fatalf("userID = %s, want %s", gotUserID, userID)
				}
				storedPurpose = purpose
				storedHash = tokenHash
				if !expiresAt.After(time.Now()) {
					t.Fatal("expected token expiry in the future")
				}
				return nil
			},
		},
		emailSender: fakeAuthEmailSender{
			sendVerificationEmailFn: func(_ context.Context, email, url string) error {
				sentEmail = email
				verificationURL = url
				return nil
			},
		},
		appBaseURL: "http://localhost:5173",
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/register",
		bytes.NewReader([]byte(`{"email":"Test@example.com","password":"password123"}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("got status %d, want %d, body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if sentEmail != "test@example.com" {
		t.Fatalf("sent email = %q, want %q", sentEmail, "test@example.com")
	}
	if storedPurpose != repository.AuthEmailTokenPurposeEmailVerification {
		t.Fatalf("stored purpose = %q", storedPurpose)
	}
	if len(storedHash) != 64 {
		t.Fatalf("expected sha256 hex hash, got %q", storedHash)
	}
	if !strings.HasPrefix(verificationURL, "http://localhost:5173/verify-email?token=") {
		t.Fatalf("unexpected verification URL: %s", verificationURL)
	}
}

func TestAuthHandler_ConfirmEmailVerification_ConsumesTokenAndMarksUserVerified(t *testing.T) {
	userID := uuid.New()
	var markedUserID uuid.UUID

	h := &AuthHandler{
		userRepo: fakeAuthUserStore{
			markEmailVerifiedFn: func(_ context.Context, id uuid.UUID) error {
				markedUserID = id
				return nil
			},
		},
		emailTokenRepo: fakeAuthEmailTokenStore{
			consumeFn: func(_ context.Context, purpose repository.AuthEmailTokenPurpose, tokenHash string, now time.Time) (uuid.UUID, error) {
				if purpose != repository.AuthEmailTokenPurposeEmailVerification {
					t.Fatalf("purpose = %q", purpose)
				}
				if len(tokenHash) != 64 {
					t.Fatalf("expected sha256 hex hash, got %q", tokenHash)
				}
				if now.IsZero() {
					t.Fatal("expected current time")
				}
				return userID, nil
			},
		},
		emailSender: fakeAuthEmailSender{},
		appBaseURL:  "http://localhost:5173",
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/verify-email/confirm",
		bytes.NewReader([]byte(`{"token":"abc123"}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ConfirmEmailVerification(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if markedUserID != userID {
		t.Fatalf("marked userID = %s, want %s", markedUserID, userID)
	}
}

func TestAuthHandler_ConfirmPasswordReset_UpdatesPasswordAndClearsCookie(t *testing.T) {
	userID := uuid.New()
	var resetUserID uuid.UUID
	var resetPasswordHash string

	h := &AuthHandler{
		userRepo: fakeAuthUserStore{
			resetPasswordFn: func(_ context.Context, id uuid.UUID, passwordHash string) error {
				resetUserID = id
				resetPasswordHash = passwordHash
				return nil
			},
		},
		emailTokenRepo: fakeAuthEmailTokenStore{
			consumeFn: func(_ context.Context, purpose repository.AuthEmailTokenPurpose, tokenHash string, now time.Time) (uuid.UUID, error) {
				if purpose != repository.AuthEmailTokenPurposePasswordReset {
					t.Fatalf("purpose = %q", purpose)
				}
				if len(tokenHash) != 64 {
					t.Fatalf("expected sha256 hex hash, got %q", tokenHash)
				}
				return userID, nil
			},
		},
		emailSender: fakeAuthEmailSender{},
		appBaseURL:  "http://localhost:5173",
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/password-reset/confirm",
		bytes.NewReader([]byte(`{"token":"reset-token","password":"password123"}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ConfirmPasswordReset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if resetUserID != userID {
		t.Fatalf("reset userID = %s, want %s", resetUserID, userID)
	}
	if resetPasswordHash == "" || resetPasswordHash == "password123" {
		t.Fatal("expected password to be hashed")
	}

	var tokenCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == middleware.LegacyTokenCookieName {
			tokenCookie = cookie
			break
		}
	}
	if tokenCookie == nil || tokenCookie.MaxAge != -1 {
		t.Fatalf("expected auth cookie to be cleared, got %#v", tokenCookie)
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
		if c.Name == middleware.LegacyTokenCookieName {
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
