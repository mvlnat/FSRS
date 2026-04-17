package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/ziyangli/fsrs/backend/internal/middleware"
	"github.com/ziyangli/fsrs/backend/internal/repository"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

const (
	minPasswordLength = 8
	tokenLifetime     = 7 * 24 * time.Hour
)

type AuthHandler struct {
	userRepo      *repository.UserRepository
	jwtSecret     []byte
	secureCookies bool
}

func NewAuthHandler(userRepo *repository.UserRepository, jwtSecret string, secureCookies bool) *AuthHandler {
	return &AuthHandler{
		userRepo:      userRepo,
		jwtSecret:     []byte(jwtSecret),
		secureCookies: secureCookies,
	}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerResponse struct {
	Message string `json:"message"`
}

type authResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

const maxPasswordBytes = 72

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeStrictJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}

	req.Email = normalizeEmail(req.Email)

	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	if !emailRegex.MatchString(req.Email) {
		http.Error(w, "Invalid email format", http.StatusBadRequest)
		return
	}

	if len(req.Password) < minPasswordLength {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if len([]byte(req.Password)) > maxPasswordBytes {
		http.Error(w, "Password must be 72 bytes or fewer", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = h.userRepo.Create(r.Context(), req.Email, string(hash))
	if err != nil && err != repository.ErrDuplicate {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(registerResponse{
		Message: "If the email is available, the account is ready to sign in.",
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeStrictJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}

	req.Email = normalizeEmail(req.Email)

	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err == repository.ErrNotFound {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := h.setTokenCookie(w, user.ID.String(), user.TokenVersion); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authResponse{
		ID:    user.ID.String(),
		Email: user.Email,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if userID, ok := middleware.GetUserID(r.Context()); ok && h.userRepo != nil {
		if err := h.userRepo.IncrementTokenVersion(r.Context(), userID); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	h.clearTokenCookie(w)

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err == repository.ErrNotFound {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authResponse{
		ID:    user.ID.String(),
		Email: user.Email,
	})
}

func (h *AuthHandler) setTokenCookie(w http.ResponseWriter, userID string, tokenVersion int) error {
	expiresAt := time.Now().Add(tokenLifetime)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, middleware.TokenClaims{
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(tokenLifetime.Seconds()),
		Expires:  expiresAt,
	})
	return nil
}

func (h *AuthHandler) clearTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}
