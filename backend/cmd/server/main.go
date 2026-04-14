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

	"github.com/ziyangli/fsrs/backend/internal/handler"
	"github.com/ziyangli/fsrs/backend/internal/middleware"
	"github.com/ziyangli/fsrs/backend/internal/repository"
	"github.com/ziyangli/fsrs/backend/internal/service"
)

func main() {
	ctx := context.Background()

	// Load config from environment
	databaseURL := getEnv("DATABASE_URL", "postgres://fsrs:fsrs@localhost:5432/fsrs?sslmode=disable")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// Only allow empty JWT_SECRET in development
		if os.Getenv("ENV") == "production" {
			log.Fatal("JWT_SECRET environment variable is required in production")
		}
		jwtSecret = "dev-secret-change-in-production"
		log.Println("WARNING: Using default JWT secret. Set JWT_SECRET in production!")
	}
	port := getEnv("PORT", "8080")
	secureCookies := os.Getenv("SECURE_COOKIES") == "true"
	corsOrigins := getEnv("CORS_ORIGINS", "http://localhost:5173,http://localhost:3000,https://fsrs.ziyang.li,http://161.35.3.230")

	// Connect to database
	db, err := repository.NewDB(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := runMigrations(ctx, db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	deckRepo := repository.NewDeckRepository(db)
	cardRepo := repository.NewCardRepository(db)

	// Initialize services
	fsrsService := service.NewFSRSService()

	// Initialize handlers
	authHandler := handler.NewAuthHandler(userRepo, jwtSecret, secureCookies)
	deckHandler := handler.NewDeckHandler(deckRepo, cardRepo)
	cardHandler := handler.NewCardHandler(cardRepo, deckRepo)
	studyHandler := handler.NewStudyHandler(cardRepo, deckRepo, fsrsService)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtSecret)
	// 10 requests per minute for auth endpoints
	authRateLimiter := middleware.NewRateLimiter(10, time.Minute)

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   strings.Split(corsOrigins, ","),
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

		// Study
		r.Get("/api/study/stats", studyHandler.GetStats)
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

func runMigrations(ctx context.Context, db *repository.DB) error {
	migration := `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS decks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_decks_user_id ON decks(user_id);

CREATE TABLE IF NOT EXISTS cards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deck_id UUID NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    front TEXT NOT NULL,
    back TEXT NOT NULL,
    link TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cards_deck_id ON cards(deck_id);

CREATE TABLE IF NOT EXISTS card_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id UUID NOT NULL UNIQUE REFERENCES cards(id) ON DELETE CASCADE,
    due TIMESTAMPTZ NOT NULL,
    stability FLOAT DEFAULT 0,
    difficulty FLOAT DEFAULT 0,
    elapsed_days INT DEFAULT 0,
    scheduled_days INT DEFAULT 0,
    reps INT DEFAULT 0,
    lapses INT DEFAULT 0,
    state INT DEFAULT 0,
    last_review TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_card_states_due ON card_states(due);

CREATE TABLE IF NOT EXISTS reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id UUID NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 4),
    reviewed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reviews_card_id ON reviews(card_id);

-- Add link column if it doesn't exist (for existing installations)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'cards' AND column_name = 'link') THEN
        ALTER TABLE cards ADD COLUMN link TEXT DEFAULT '';
    END IF;
END $$;
`
	_, err := db.Pool.Exec(ctx, migration)
	return err
}
