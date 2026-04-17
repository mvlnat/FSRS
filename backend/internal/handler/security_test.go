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

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ziyangli/fsrs/backend/internal/middleware"
	"github.com/ziyangli/fsrs/backend/internal/model"
)

func TestUserModelOmitsPasswordHashFromJSON(t *testing.T) {
	payload, err := json.Marshal(model.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: "super-secret-hash",
	})
	if err != nil {
		t.Fatalf("marshal user: %v", err)
	}

	if bytes.Contains(payload, []byte("password_hash")) {
		t.Fatalf("expected password_hash field to be omitted from JSON, got %s", payload)
	}
	if bytes.Contains(payload, []byte("super-secret-hash")) {
		t.Fatalf("expected password hash value to be omitted from JSON, got %s", payload)
	}
}

func TestMiddleware_RequiresAuth(t *testing.T) {
	authMiddleware := middleware.NewAuthMiddleware("test-secret", nil)

	handler := authMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		cookie     string
		authHeader string
		wantStatus int
	}{
		{
			name:       "no auth",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid cookie",
			cookie:     "invalid-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid auth header",
			authHeader: "Bearer invalid-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "malformed auth header",
			authHeader: "NotBearer token",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "token", Value: tt.cookie})
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestAuthMiddleware_UserIDExtraction(t *testing.T) {
	ctx := context.Background()

	userID, ok := middleware.GetUserID(ctx)
	if ok {
		t.Error("expected ok=false when no user ID in context")
	}
	if userID != uuid.Nil {
		t.Error("expected nil UUID when no user ID in context")
	}

	expectedID := uuid.New()
	ctx = context.WithValue(ctx, middleware.UserIDKey, expectedID)
	userID, ok = middleware.GetUserID(ctx)
	if !ok {
		t.Error("expected ok=true when user ID in context")
	}
	if userID != expectedID {
		t.Errorf("got user ID %s, want %s", userID, expectedID)
	}
}

func TestAuthHandler_SetTokenCookieUsesExpectedClaimsAndFlags(t *testing.T) {
	h := &AuthHandler{
		jwtSecret:     []byte("test-secret"),
		secureCookies: true,
	}
	rec := httptest.NewRecorder()
	userID := uuid.NewString()

	if err := h.setTokenCookie(rec, userID, 7); err != nil {
		t.Fatalf("setTokenCookie: %v", err)
	}

	tokenCookie := findCookie(rec.Result().Cookies(), "token")
	if tokenCookie == nil {
		t.Fatal("expected token cookie to be set")
	}
	if tokenCookie.Path != "/" {
		t.Fatalf("cookie path = %q, want /", tokenCookie.Path)
	}
	if !tokenCookie.HttpOnly {
		t.Fatal("expected token cookie to be HttpOnly")
	}
	if !tokenCookie.Secure {
		t.Fatal("expected token cookie to be Secure")
	}
	if tokenCookie.MaxAge <= 0 {
		t.Fatalf("cookie MaxAge = %d, want positive", tokenCookie.MaxAge)
	}

	setCookieHeader := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookieHeader, "SameSite=Strict") {
		t.Fatalf("expected SameSite=Strict in Set-Cookie header, got %q", setCookieHeader)
	}

	claims := &middleware.TokenClaims{}
	token, err := jwt.ParseWithClaims(tokenCookie.Value, claims, func(token *jwt.Token) (any, error) {
		return h.jwtSecret, nil
	})
	if err != nil {
		t.Fatalf("parse token cookie: %v", err)
	}
	if !token.Valid {
		t.Fatal("expected signed token cookie to be valid")
	}
	if claims.Subject != userID {
		t.Fatalf("claims subject = %q, want %q", claims.Subject, userID)
	}
	if claims.TokenVersion != 7 {
		t.Fatalf("claims token version = %d, want 7", claims.TokenVersion)
	}
	if claims.ExpiresAt == nil || time.Until(claims.ExpiresAt.Time) <= 0 {
		t.Fatalf("expected token expiration in the future, got %#v", claims.ExpiresAt)
	}
}

func TestAuthHandler_LogoutClearsCookie(t *testing.T) {
	h := &AuthHandler{secureCookies: true}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	tokenCookie := findCookie(rec.Result().Cookies(), "token")
	if tokenCookie == nil {
		t.Fatal("expected token cookie to be set")
	}
	if tokenCookie.MaxAge != -1 {
		t.Fatalf("cookie MaxAge = %d, want -1", tokenCookie.MaxAge)
	}
	if !tokenCookie.Expires.Equal(time.Unix(0, 0)) {
		t.Fatalf("cookie Expires = %s, want unix epoch", tokenCookie.Expires)
	}
	if tokenCookie.Path != "/" {
		t.Fatalf("cookie path = %q, want /", tokenCookie.Path)
	}
	if !tokenCookie.HttpOnly {
		t.Fatal("expected cleared cookie to remain HttpOnly")
	}
	if !tokenCookie.Secure {
		t.Fatal("expected cleared cookie to retain Secure flag")
	}

	setCookieHeader := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookieHeader, "SameSite=Strict") {
		t.Fatalf("expected SameSite=Strict in Set-Cookie header, got %q", setCookieHeader)
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}

	return nil
}
