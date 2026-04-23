package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/ziyangli/fsrs/backend/internal/middleware"
	"github.com/ziyangli/fsrs/backend/internal/model"
	"github.com/ziyangli/fsrs/backend/internal/repository"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

const (
	minPasswordLength                 = 8
	maxEmailLength                    = 255
	tokenLifetime                     = 7 * 24 * time.Hour
	emailVerificationTokenLifetime    = 24 * time.Hour
	passwordResetTokenLifetime        = time.Hour
	loginEmailScope                   = "auth_login_email"
	registerEmailScope                = "auth_register_email"
	verifyEmailResendScope            = "auth_verify_email_resend_email"
	passwordResetRequestScope         = "auth_password_reset_request_email"
	loginEmailLimit                   = 8
	registerEmailLimit                = 5
	verifyEmailResendLimit            = 5
	passwordResetRequestLimit         = 5
	loginEmailWindow                  = 15 * time.Minute
	registerEmailWindow               = time.Hour
	verifyEmailResendWindow           = time.Hour
	passwordResetRequestWindow        = time.Hour
	loginEmailBlockDuration           = 30 * time.Minute
	registerEmailBlockDuration        = time.Hour
	verifyEmailResendBlockDuration    = time.Hour
	passwordResetRequestBlockDuration = time.Hour
)

type authUserStore interface {
	Create(ctx context.Context, email, passwordHash string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	IncrementTokenVersion(ctx context.Context, id uuid.UUID) error
	MarkEmailVerified(ctx context.Context, id uuid.UUID) error
	ResetPassword(ctx context.Context, id uuid.UUID, passwordHash string) error
}

type authEmailTokenStore interface {
	Create(ctx context.Context, userID uuid.UUID, purpose repository.AuthEmailTokenPurpose, tokenHash string, expiresAt time.Time) error
	Consume(ctx context.Context, purpose repository.AuthEmailTokenPurpose, tokenHash string, now time.Time) (uuid.UUID, error)
}

type authEmailSender interface {
	SendVerificationEmail(ctx context.Context, email, verificationURL string) error
	SendPasswordResetEmail(ctx context.Context, email, resetURL string) error
}

type authThrottle interface {
	Allow(
		ctx context.Context,
		scope string,
		key string,
		limit int,
		window time.Duration,
		blockDuration time.Duration,
	) (bool, time.Duration, error)
	Reset(ctx context.Context, scope string, key string) error
}

type AuthHandler struct {
	userRepo       authUserStore
	emailTokenRepo authEmailTokenStore
	emailSender    authEmailSender
	jwtSecret      []byte
	secureCookies  bool
	appBaseURL     string
	authThrottle   authThrottle
}

func NewAuthHandler(
	userRepo authUserStore,
	emailTokenRepo authEmailTokenStore,
	emailSender authEmailSender,
	jwtSecret string,
	secureCookies bool,
	appBaseURL string,
) *AuthHandler {
	return &AuthHandler{
		userRepo:       userRepo,
		emailTokenRepo: emailTokenRepo,
		emailSender:    emailSender,
		jwtSecret:      []byte(jwtSecret),
		secureCookies:  secureCookies,
		appBaseURL:     strings.TrimRight(strings.TrimSpace(appBaseURL), "/"),
	}
}

func (h *AuthHandler) SetAuthThrottle(throttle authThrottle) {
	h.authThrottle = throttle
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type emailRequest struct {
	Email string `json:"email"`
}

type verifyEmailConfirmRequest struct {
	Token string `json:"token"`
}

type passwordResetConfirmRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type registerResponse struct {
	Message string `json:"message"`
}

type authResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

type messageResponse struct {
	Message string `json:"message"`
}

const maxPasswordBytes = 72

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateEmail(email string) error {
	if len(email) > maxEmailLength {
		return authValidationError("Email must be 255 characters or fewer")
	}
	if !emailRegex.MatchString(email) {
		return authValidationError("Invalid email format")
	}
	return nil
}

func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < minPasswordLength {
		return authValidationError("Password must be at least 8 characters")
	}
	if len([]byte(password)) > maxPasswordBytes {
		return authValidationError("Password must be 72 bytes or fewer")
	}
	return nil
}

type authValidationError string

func (e authValidationError) Error() string {
	return string(e)
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
	if err := validateEmail(req.Email); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validatePassword(req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !h.allowEmailThrottle(w, r, registerEmailScope, req.Email, registerEmailLimit, registerEmailWindow, registerEmailBlockDuration) {
		return
	}
	if err := h.requireEmailAuthDependencies(); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	user, err := h.userRepo.Create(r.Context(), req.Email, string(hash))
	if err != nil && err != repository.ErrDuplicate {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err == repository.ErrDuplicate {
		user, err = h.userRepo.GetByEmail(r.Context(), req.Email)
		if err != nil && err != repository.ErrNotFound {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	if user != nil && user.EmailVerifiedAt == nil {
		if err := h.sendVerificationEmail(r.Context(), user); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(registerResponse{
		Message: "If the email is available, a verification email has been sent.",
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeStrictJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}

	req.Email = normalizeEmail(req.Email)
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}
	if err := validateEmail(req.Email); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !h.allowEmailThrottle(w, r, loginEmailScope, req.Email, loginEmailLimit, loginEmailWindow, loginEmailBlockDuration) {
		return
	}

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
	if err := h.resetEmailThrottle(r.Context(), loginEmailScope, req.Email); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if user.EmailVerifiedAt == nil {
		http.Error(w, "Email not verified", http.StatusForbidden)
		return
	}

	if err := h.setTokenCookie(w, user.ID.String(), user.TokenVersion); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authResponse{
		ID:            user.ID.String(),
		Email:         user.Email,
		EmailVerified: user.EmailVerifiedAt != nil,
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
		ID:            user.ID.String(),
		Email:         user.Email,
		EmailVerified: user.EmailVerifiedAt != nil,
	})
}

func (h *AuthHandler) ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	var req emailRequest
	if !decodeStrictJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}

	req.Email = normalizeEmail(req.Email)
	if req.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}
	if err := validateEmail(req.Email); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !h.allowEmailThrottle(w, r, verifyEmailResendScope, req.Email, verifyEmailResendLimit, verifyEmailResendWindow, verifyEmailResendBlockDuration) {
		return
	}
	if err := h.requireEmailAuthDependencies(); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil && err != repository.ErrNotFound {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err == nil && user.EmailVerifiedAt == nil {
		if err := h.sendVerificationEmail(r.Context(), user); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	writeJSONMessage(w, http.StatusAccepted, "If the account exists, a verification email has been sent.")
}

func (h *AuthHandler) ConfirmEmailVerification(w http.ResponseWriter, r *http.Request) {
	var req verifyEmailConfirmRequest
	if !decodeStrictJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}

	if strings.TrimSpace(req.Token) == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}
	if err := h.requireEmailAuthDependencies(); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userID, err := h.consumeEmailToken(r.Context(), repository.AuthEmailTokenPurposeEmailVerification, req.Token)
	if err == repository.ErrNotFound {
		http.Error(w, "Invalid or expired verification token", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.userRepo.MarkEmailVerified(r.Context(), userID); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	writeJSONMessage(w, http.StatusOK, "Email verified. You can now sign in.")
}

func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req emailRequest
	if !decodeStrictJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}

	req.Email = normalizeEmail(req.Email)
	if req.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}
	if err := validateEmail(req.Email); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !h.allowEmailThrottle(
		w,
		r,
		passwordResetRequestScope,
		req.Email,
		passwordResetRequestLimit,
		passwordResetRequestWindow,
		passwordResetRequestBlockDuration,
	) {
		return
	}
	if err := h.requireEmailAuthDependencies(); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil && err != repository.ErrNotFound {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err == nil {
		if err := h.sendPasswordResetEmail(r.Context(), user); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	writeJSONMessage(w, http.StatusAccepted, "If the account exists, a password reset email has been sent.")
}

func (h *AuthHandler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req passwordResetConfirmRequest
	if !decodeStrictJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}

	if strings.TrimSpace(req.Token) == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}
	if err := validatePassword(req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.requireEmailAuthDependencies(); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userID, err := h.consumeEmailToken(r.Context(), repository.AuthEmailTokenPurposePasswordReset, req.Token)
	if err == repository.ErrNotFound {
		http.Error(w, "Invalid or expired password reset token", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.userRepo.ResetPassword(r.Context(), userID, string(hash)); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.clearTokenCookie(w)
	writeJSONMessage(w, http.StatusOK, "Password has been reset. You can now sign in.")
}

func (h *AuthHandler) tokenCookieName() string {
	if h.secureCookies {
		return middleware.SecureTokenCookieName
	}

	return middleware.LegacyTokenCookieName
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
		Name:     h.tokenCookieName(),
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
	names := []string{h.tokenCookieName()}
	if h.secureCookies {
		names = append(names, middleware.LegacyTokenCookieName)
	}

	for _, name := range names {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   h.secureCookies,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
		})
	}
}

func (h *AuthHandler) allowEmailThrottle(
	w http.ResponseWriter,
	r *http.Request,
	scope string,
	email string,
	limit int,
	window time.Duration,
	blockDuration time.Duration,
) bool {
	if h.authThrottle == nil || email == "" {
		return true
	}

	allowed, retryAfter, err := h.authThrottle.Allow(r.Context(), scope, email, limit, window, blockDuration)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return false
	}
	if !allowed {
		writeAuthRetryAfter(w, retryAfter)
		http.Error(w, "Too many authentication attempts", http.StatusTooManyRequests)
		return false
	}

	return true
}

func writeAuthRetryAfter(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}

func (h *AuthHandler) requireEmailAuthDependencies() error {
	if h.userRepo == nil || h.emailTokenRepo == nil || h.emailSender == nil || h.appBaseURL == "" {
		return authValidationError("email auth is not configured")
	}
	return nil
}

func (h *AuthHandler) sendVerificationEmail(ctx context.Context, user *model.User) error {
	verificationURL, err := h.issueEmailTokenURL(
		ctx,
		user.ID,
		repository.AuthEmailTokenPurposeEmailVerification,
		emailVerificationTokenLifetime,
		"/verify-email",
	)
	if err != nil {
		return err
	}

	return h.emailSender.SendVerificationEmail(ctx, user.Email, verificationURL)
}

func (h *AuthHandler) sendPasswordResetEmail(ctx context.Context, user *model.User) error {
	resetURL, err := h.issueEmailTokenURL(
		ctx,
		user.ID,
		repository.AuthEmailTokenPurposePasswordReset,
		passwordResetTokenLifetime,
		"/reset-password",
	)
	if err != nil {
		return err
	}

	return h.emailSender.SendPasswordResetEmail(ctx, user.Email, resetURL)
}

func (h *AuthHandler) issueEmailTokenURL(
	ctx context.Context,
	userID uuid.UUID,
	purpose repository.AuthEmailTokenPurpose,
	lifetime time.Duration,
	path string,
) (string, error) {
	token, err := generateOpaqueToken()
	if err != nil {
		return "", err
	}

	if err := h.emailTokenRepo.Create(ctx, userID, purpose, hashOpaqueToken(token), time.Now().Add(lifetime)); err != nil {
		return "", err
	}

	return h.appBaseURL + path + "?token=" + url.QueryEscape(token), nil
}

func (h *AuthHandler) consumeEmailToken(
	ctx context.Context,
	purpose repository.AuthEmailTokenPurpose,
	token string,
) (uuid.UUID, error) {
	return h.emailTokenRepo.Consume(ctx, purpose, hashOpaqueToken(token), time.Now())
}

func (h *AuthHandler) resetEmailThrottle(ctx context.Context, scope string, email string) error {
	if h.authThrottle == nil {
		return nil
	}
	return h.authThrottle.Reset(ctx, scope, email)
}

func writeJSONMessage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(messageResponse{Message: message})
}

func generateOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashOpaqueToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
