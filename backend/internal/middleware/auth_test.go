package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ziyangli/fsrs/backend/internal/model"
)

type fakeUserReader struct {
	user *model.User
}

func (f fakeUserReader) GetByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	if f.user != nil && f.user.ID == id {
		return f.user, nil
	}
	return nil, nil
}

func TestAuthMiddleware_AllowsCurrentToken(t *testing.T) {
	userID := uuid.New()
	token := signedToken(t, "test-secret", userID.String(), 2)
	authMiddleware := NewAuthMiddleware("test-secret", fakeUserReader{
		user: &model.User{
			ID:           userID,
			TokenVersion: 2,
		},
	})

	handler := authMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, ok := GetUserID(r.Context())
		if !ok {
			t.Fatal("expected user ID in context")
		}
		if gotUserID != userID {
			t.Fatalf("user ID = %s, want %s", gotUserID, userID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: LegacyTokenCookieName, Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_RejectsRevokedToken(t *testing.T) {
	userID := uuid.New()
	token := signedToken(t, "test-secret", userID.String(), 1)
	authMiddleware := NewAuthMiddleware("test-secret", fakeUserReader{
		user: &model.User{
			ID:           userID,
			TokenVersion: 2,
		},
	})

	handler := authMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: SecureTokenCookieName, Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_PrefersSecureCookieOverLegacy(t *testing.T) {
	userID := uuid.New()
	validSecureToken := signedToken(t, "test-secret", userID.String(), 2)
	invalidLegacyToken := "invalid-token"
	authMiddleware := NewAuthMiddleware("test-secret", fakeUserReader{
		user: &model.User{
			ID:           userID,
			TokenVersion: 2,
		},
	})

	handler := authMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: LegacyTokenCookieName, Value: invalidLegacyToken})
	req.AddCookie(&http.Cookie{Name: SecureTokenCookieName, Value: validSecureToken})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func signedToken(t *testing.T, secret, subject string, tokenVersion int) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, TokenClaims{
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return tokenString
}
