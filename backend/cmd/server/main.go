package main

import (
	"context"
	"log"
	"net/http"
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
		jwtSecret = "dev-secret-change-in-production"
		log.Println("WARNING: Using default JWT secret. Set JWT_SECRET in production!")
	}
	port := getEnv("PORT", "8080")
	secureCookies := os.Getenv("SECURE_COOKIES") == "true"
	if environment == "production" && !secureCookies {
		log.Fatal("SECURE_COOKIES must be true in production; enable HTTPS at the proxy before starting the server")
	}
	corsOrigins := getCORSOrigins(environment)

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
	deckRepo := repository.NewDeckRepository(db)
	cardRepo := repository.NewCardRepository(db)
	tagRepo := repository.NewTagRepository(db)

	// Initialize services
	fsrsService := service.NewFSRSService()

	// Initialize handlers
	authHandler := handler.NewAuthHandler(userRepo, jwtSecret, secureCookies)
	deckHandler := handler.NewDeckHandler(deckRepo, cardRepo)
	cardHandler := handler.NewCardHandler(cardRepo, deckRepo, tagRepo)
	studyHandler := handler.NewStudyHandler(cardRepo, deckRepo, fsrsService)
	tagHandler := handler.NewTagHandler(tagRepo, deckRepo, cardRepo)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtSecret, userRepo)
	// 10 requests per minute for auth endpoints
	authRateLimiter := middleware.NewRateLimiter(10, time.Minute)
	authRateLimiter.SetTrustProxy(os.Getenv("TRUST_PROXY_HEADERS") == "true")

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   splitCSV(corsOrigins),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

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

	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getCORSOrigins(environment string) string {
	if value := os.Getenv("CORS_ORIGINS"); value != "" {
		return value
	}

	if environment == "production" {
		return "https://fsrs.ziyang.li"
	}

	return "http://localhost:5173,http://localhost:3000"
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
