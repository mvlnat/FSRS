package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ziyangli/fsrs/backend/internal/model"
)

type contextKey string

const (
	UserIDKey             contextKey = "userID"
	LegacyTokenCookieName            = "token"
	SecureTokenCookieName            = "__Host-token"
)

type UserReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

type TokenClaims struct {
	TokenVersion int `json:"ver"`
	jwt.RegisteredClaims
}

type AuthMiddleware struct {
	jwtSecret []byte
	userRepo  UserReader
}

func NewAuthMiddleware(jwtSecret string, userRepo UserReader) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret: []byte(jwtSecret),
		userRepo:  userRepo,
	}
}

func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := tokenFromRequest(r)

		// Fall back to Authorization header
		if tokenString == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if tokenString == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims := &TokenClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return m.jwtSecret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if m.userRepo != nil {
			user, err := m.userRepo.GetByID(r.Context(), userID)
			if err != nil || user == nil || user.TokenVersion != claims.TokenVersion {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func tokenFromRequest(r *http.Request) string {
	for _, cookieName := range []string{SecureTokenCookieName, LegacyTokenCookieName} {
		cookie, err := r.Cookie(cookieName)
		if err == nil {
			return cookie.Value
		}
	}

	return ""
}

func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}
