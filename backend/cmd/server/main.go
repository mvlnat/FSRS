package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/ziyangli/fsrs/backend/internal/bootstrap"
	"github.com/ziyangli/fsrs/backend/internal/handler"
	"github.com/ziyangli/fsrs/backend/internal/middleware"
	"github.com/ziyangli/fsrs/backend/internal/repository"
	"github.com/ziyangli/fsrs/backend/internal/service"
)

const (
	defaultDevelopmentJWTSecret = "dev-secret-change-in-production"
	minJWTSecretBytes           = 32
	authIPScope                 = "auth_ip"
	authIPLimit                 = 20
	authIPWindow                = 5 * time.Minute
	authIPBlockDuration         = 15 * time.Minute
	readHeaderTimeout           = 5 * time.Second
	readTimeout                 = 15 * time.Second
	writeTimeout                = 30 * time.Second
	idleTimeout                 = 60 * time.Second
)

func main() {
	ctx := context.Background()
	environment := getEnv("ENV", "development")

	// Load config from environment
	databaseURL := getEnv("DATABASE_URL", "postgres://fsrs:fsrs@localhost:5432/fsrs?sslmode=disable")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// Only allow empty JWT_SECRET in development
		if environment == "production" {
			log.Fatal("JWT_SECRET environment variable is required in production")
		}
		jwtSecret = defaultDevelopmentJWTSecret
		log.Println("WARNING: Using default JWT secret. Set JWT_SECRET in production!")
	}
	if err := validateJWTSecret(environment, jwtSecret); err != nil {
		log.Fatalf("Invalid JWT_SECRET: %v", err)
	}
	port := getEnv("PORT", "8080")
	secureCookies := os.Getenv("SECURE_COOKIES") == "true"
	if environment == "production" && !secureCookies {
		log.Fatal("SECURE_COOKIES must be true in production; enable HTTPS at the proxy before starting the server")
	}
	allowedOrigins, err := getAllowedOrigins(environment)
	if err != nil {
		log.Fatalf("Invalid CORS_ORIGINS: %v", err)
	}

	// Connect to database
	db, err := repository.NewDB(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := bootstrap.RunMigrations(ctx, db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	authThrottleRepo := repository.NewAuthThrottleRepository(db)
	deckRepo := repository.NewDeckRepository(db)
	cardRepo := repository.NewCardRepository(db)
	tagRepo := repository.NewTagRepository(db)

	// Initialize services
	fsrsService := service.NewFSRSService()

	// Initialize handlers
	authHandler := handler.NewAuthHandler(userRepo, jwtSecret, secureCookies)
	authHandler.SetAuthThrottle(authThrottleRepo)
	deckHandler := handler.NewDeckHandler(deckRepo, cardRepo)
	cardHandler := handler.NewCardHandler(cardRepo, deckRepo, tagRepo)
	studyHandler := handler.NewStudyHandler(cardRepo, deckRepo, fsrsService)
	tagHandler := handler.NewTagHandler(tagRepo, deckRepo, cardRepo)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtSecret, userRepo)
	browserTrustMiddleware := middleware.NewBrowserTrustMiddleware(allowedOrigins)
	authRateLimiter := middleware.NewAuthRateLimitMiddleware(
		authThrottleRepo,
		authIPScope,
		authIPLimit,
		authIPWindow,
		authIPBlockDuration,
	)
	authRateLimiter.SetTrustProxy(os.Getenv("TRUST_PROXY_HEADERS") == "true")

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(browserTrustMiddleware.Handler)

	// Public routes with rate limiting
	r.Group(func(r chi.Router) {
		r.Use(authRateLimiter.Handler)
		r.Post("/api/auth/register", authHandler.Register)
		r.Post("/api/auth/login", authHandler.Login)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Handler)

		// Auth
		r.Post("/api/auth/logout", authHandler.Logout)
		r.Get("/api/auth/me", authHandler.Me)

		// Decks
		r.Get("/api/decks", deckHandler.List)
		r.Post("/api/decks", deckHandler.Create)
		r.Post("/api/decks/import", deckHandler.Import)
		r.Get("/api/decks/{id}", deckHandler.Get)
		r.Put("/api/decks/{id}", deckHandler.Update)
		r.Delete("/api/decks/{id}", deckHandler.Delete)
		r.Get("/api/decks/{id}/stats", deckHandler.Stats)
		r.Get("/api/decks/{id}/export", deckHandler.Export)

		// Cards
		r.Get("/api/decks/{id}/cards", cardHandler.ListByDeck)
		r.Post("/api/decks/{id}/cards", cardHandler.Create)
		r.Get("/api/cards/{id}", cardHandler.Get)
		r.Put("/api/cards/{id}", cardHandler.Update)
		r.Delete("/api/cards/{id}", cardHandler.Delete)

		// Tags
		r.Get("/api/decks/{deckId}/tags", tagHandler.ListByDeck)
		r.Post("/api/decks/{deckId}/tags", tagHandler.Create)
		r.Delete("/api/tags/{tagId}", tagHandler.Delete)
		r.Put("/api/cards/{cardId}/tags", tagHandler.SetCardTags)

		// Study
		r.Get("/api/study/stats", studyHandler.GetStats)
		r.Get("/api/study/schedule", studyHandler.GetSchedule)
		r.Get("/api/study/{deckId}", studyHandler.GetDueCards)
		r.Post("/api/study/{cardId}/review", studyHandler.Review)
	})

	server := newHTTPServer(":"+port, r)

	log.Printf("Server starting on port %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getAllowedOrigins(environment string) ([]string, error) {
	origins := os.Getenv("CORS_ORIGINS")
	if origins == "" {
		if environment == "production" {
			origins = "https://fsrs.ziyang.li"
		} else {
			origins = "http://localhost:5173,http://localhost:3000"
		}
	}

	return parseAllowedOrigins(origins)
}

func parseAllowedOrigins(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "*") {
			return nil, fmt.Errorf("wildcard origins are not allowed when credentials are enabled")
		}

		normalized, err := normalizeOrigin(trimmed)
		if err != nil {
			return nil, err
		}

		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("at least one allowed origin is required")
	}

	return result, nil
}

func normalizeOrigin(origin string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("origin %q must be an absolute http or https URL", origin)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("origin %q must not include a path", origin)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin %q must not include a query string or fragment", origin)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("origin %q must use http or https", origin)
	}

	return scheme + "://" + strings.ToLower(parsed.Host), nil
}

func validateJWTSecret(environment, jwtSecret string) error {
	if environment != "production" {
		return nil
	}
	if jwtSecret == defaultDevelopmentJWTSecret {
		return fmt.Errorf("development default value is not allowed in production")
	}
	if len(jwtSecret) < minJWTSecretBytes {
		return fmt.Errorf("must be at least %d bytes in production", minJWTSecretBytes)
	}
	return nil
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}
