//go:build integration
// +build integration

package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ziyangli/fsrs/backend/internal/handler"
	"github.com/ziyangli/fsrs/backend/internal/middleware"
	"github.com/ziyangli/fsrs/backend/internal/repository"
	"github.com/ziyangli/fsrs/backend/internal/service"
	dbmigrations "github.com/ziyangli/fsrs/backend/migrations"
)

// Integration tests require a running PostgreSQL database
// Run with: go test -tags=integration ./internal/handler/...

var (
	testDB        *repository.DB
	testRouter    *chi.Mux
	testJWTSecret = "test-jwt-secret-for-integration"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Get database URL from environment or use default test DB
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://fsrs:fsrs@localhost:5432/fsrs_test?sslmode=disable"
	}

	var err error
	testDB, err = repository.NewDB(ctx, dbURL)
	if err != nil {
		panic("failed to connect to test database: " + err.Error())
	}
	defer testDB.Close()

	// Setup schema
	if err := setupTestSchema(ctx); err != nil {
		panic("failed to setup test schema: " + err.Error())
	}

	// Setup router
	testRouter = setupTestRouter()

	// Run tests
	code := m.Run()

	// Cleanup
	cleanupTestDB(ctx)

	os.Exit(code)
}

func setupTestSchema(ctx context.Context) error {
	if _, err := testDB.Pool.Exec(ctx, `
		DROP TABLE IF EXISTS reviews CASCADE;
		DROP TABLE IF EXISTS card_states CASCADE;
		DROP TABLE IF EXISTS card_tags CASCADE;
		DROP TABLE IF EXISTS tags CASCADE;
		DROP TABLE IF EXISTS cards CASCADE;
		DROP TABLE IF EXISTS decks CASCADE;
		DROP TABLE IF EXISTS users CASCADE;
	`); err != nil {
		return err
	}

	scripts, err := dbmigrations.OrderedScripts()
	if err != nil {
		return err
	}

	for _, script := range scripts {
		if _, err := testDB.Pool.Exec(ctx, script.SQL); err != nil {
			return err
		}
	}

	return nil
}

func cleanupTestDB(ctx context.Context) {
	testDB.Pool.Exec(ctx, `
		DROP TABLE IF EXISTS reviews CASCADE;
		DROP TABLE IF EXISTS card_states CASCADE;
		DROP TABLE IF EXISTS card_tags CASCADE;
		DROP TABLE IF EXISTS tags CASCADE;
		DROP TABLE IF EXISTS cards CASCADE;
		DROP TABLE IF EXISTS decks CASCADE;
		DROP TABLE IF EXISTS users CASCADE;
	`)
}

func setupTestRouter() *chi.Mux {
	userRepo := repository.NewUserRepository(testDB)
	deckRepo := repository.NewDeckRepository(testDB)
	cardRepo := repository.NewCardRepository(testDB)
	tagRepo := repository.NewTagRepository(testDB)
	fsrsService := service.NewFSRSService()

	authHandler := handler.NewAuthHandler(userRepo, testJWTSecret, false)
	deckHandler := handler.NewDeckHandler(deckRepo, cardRepo)
	cardHandler := handler.NewCardHandler(cardRepo, deckRepo, tagRepo)
	studyHandler := handler.NewStudyHandler(cardRepo, deckRepo, fsrsService)
	tagHandler := handler.NewTagHandler(tagRepo, deckRepo, cardRepo)

	authMiddleware := middleware.NewAuthMiddleware(testJWTSecret)

	r := chi.NewRouter()

	// Public routes
	r.Post("/api/auth/register", authHandler.Register)
	r.Post("/api/auth/login", authHandler.Login)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Handler)

		r.Post("/api/auth/logout", authHandler.Logout)
		r.Get("/api/auth/me", authHandler.Me)

		r.Get("/api/decks", deckHandler.List)
		r.Post("/api/decks", deckHandler.Create)
		r.Post("/api/decks/import", deckHandler.Import)
		r.Get("/api/decks/{id}", deckHandler.Get)
		r.Put("/api/decks/{id}", deckHandler.Update)
		r.Delete("/api/decks/{id}", deckHandler.Delete)
		r.Get("/api/decks/{id}/export", deckHandler.Export)

		r.Get("/api/decks/{id}/cards", cardHandler.ListByDeck)
		r.Post("/api/decks/{id}/cards", cardHandler.Create)
		r.Get("/api/cards/{id}", cardHandler.Get)
		r.Put("/api/cards/{id}", cardHandler.Update)
		r.Delete("/api/cards/{id}", cardHandler.Delete)

		r.Get("/api/decks/{deckId}/tags", tagHandler.ListByDeck)
		r.Post("/api/decks/{deckId}/tags", tagHandler.Create)
		r.Delete("/api/tags/{tagId}", tagHandler.Delete)
		r.Put("/api/cards/{cardId}/tags", tagHandler.SetCardTags)

		r.Get("/api/study/stats", studyHandler.GetStats)
		r.Get("/api/study/{deckId}", studyHandler.GetDueCards)
		r.Post("/api/study/{cardId}/review", studyHandler.Review)
	})

	return r
}

func TestIntegration_AuthFlow(t *testing.T) {
	// Register
	body := `{"email":"test@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("register: got status %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// Extract cookie
	cookies := rec.Result().Cookies()
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "token" {
			authCookie = c
			break
		}
	}
	if authCookie == nil {
		t.Fatal("expected auth cookie after register")
	}

	// Get current user
	req = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("me: got status %d, want %d", rec.Code, http.StatusOK)
	}

	var user struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&user); err != nil {
		t.Fatalf("failed to decode user: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("email = %s, want test@example.com", user.Email)
	}

	// Login
	body = `{"email":"test@example.com","password":"password123"}`
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login: got status %d, want %d", rec.Code, http.StatusOK)
	}

	// Logout
	req = httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: got status %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestIntegration_AuthEmailNormalization(t *testing.T) {
	body := `{"email":"Mixed.Case@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("register: got status %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	body = `{"email":"mixed.case@EXAMPLE.com","password":"password123"}`
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login: got status %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	body = `{"email":"MIXED.CASE@example.com","password":"password123"}`
	req = httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate register: got status %d, want %d, body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestIntegration_DeckCRUD(t *testing.T) {
	// Register and get auth cookie
	body := `{"email":"deck@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "token" {
			authCookie = c
			break
		}
	}

	// Create deck
	body = `{"name":"Test Deck","description":"A test deck"}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create deck: got status %d, want %d", rec.Code, http.StatusCreated)
	}

	var deck struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&deck); err != nil {
		t.Fatalf("failed to decode deck: %v", err)
	}
	if deck.Name != "Test Deck" {
		t.Errorf("deck name = %s, want Test Deck", deck.Name)
	}

	// List decks
	req = httptest.NewRequest(http.MethodGet, "/api/decks", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("list decks: got status %d, want %d", rec.Code, http.StatusOK)
	}

	// Add card to deck
	body = `{"front":"What is 2+2?","back":"4"}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deck.ID+"/cards", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create card: got status %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// Export deck
	req = httptest.NewRequest(http.MethodGet, "/api/decks/"+deck.ID+"/export", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("export deck: got status %d, want %d", rec.Code, http.StatusOK)
	}

	var export struct {
		Name  string `json:"name"`
		Cards []struct {
			Front string `json:"front"`
		} `json:"cards"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&export); err != nil {
		t.Fatalf("failed to decode export: %v", err)
	}
	if len(export.Cards) != 1 {
		t.Errorf("expected 1 card in export, got %d", len(export.Cards))
	}

	// Delete deck
	req = httptest.NewRequest(http.MethodDelete, "/api/decks/"+deck.ID, nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete deck: got status %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestIntegration_StudyFlow(t *testing.T) {
	// Register
	body := `{"email":"study@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "token" {
			authCookie = c
			break
		}
	}

	// Create deck
	body = `{"name":"Study Deck","description":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var deck struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&deck)

	// Create card
	body = `{"front":"Capital of France?","back":"Paris"}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deck.ID+"/cards", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var card struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&card)

	// Get due cards
	req = httptest.NewRequest(http.MethodGet, "/api/study/"+deck.ID, nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get due cards: got status %d, want %d", rec.Code, http.StatusOK)
	}

	var dueCards []struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&dueCards)
	if len(dueCards) != 1 {
		t.Fatalf("expected 1 due card, got %d", len(dueCards))
	}

	// Review card (rating 3 = Good)
	body = `{"rating":3}`
	req = httptest.NewRequest(http.MethodPost, "/api/study/"+card.ID+"/review", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("review card: got status %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Get study stats
	req = httptest.NewRequest(http.MethodGet, "/api/study/stats", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get stats: got status %d, want %d", rec.Code, http.StatusOK)
	}

	var stats struct {
		TotalReviews int `json:"totalReviews"`
	}
	json.NewDecoder(rec.Body).Decode(&stats)
	if stats.TotalReviews != 1 {
		t.Errorf("expected 1 total review, got %d", stats.TotalReviews)
	}
}

func TestIntegration_EditCardResetsStudyProgress(t *testing.T) {
	// Register
	body := `{"email":"edit-card@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "token" {
			authCookie = c
			break
		}
	}

	// Create deck
	body = `{"name":"Editable Deck","description":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var deck struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&deck)

	// Create card
	body = `{"front":"Original front","back":"Original back"}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deck.ID+"/cards", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var card struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&card)

	// Review once to create state and review history
	body = `{"rating":3}`
	req = httptest.NewRequest(http.MethodPost, "/api/study/"+card.ID+"/review", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("review card: got status %d, want %d", rec.Code, http.StatusOK)
	}

	// Edit the card, which should reset scheduling and review history
	body = `{"front":"Updated front","back":"Updated back","link":"https://example.com"}`
	req = httptest.NewRequest(http.MethodPut, "/api/cards/"+card.ID, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update card: got status %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Stats should be reset because prior reviews are no longer relevant
	req = httptest.NewRequest(http.MethodGet, "/api/study/stats", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get stats after edit: got status %d, want %d", rec.Code, http.StatusOK)
	}

	var stats struct {
		TotalReviews int `json:"totalReviews"`
	}
	json.NewDecoder(rec.Body).Decode(&stats)
	if stats.TotalReviews != 0 {
		t.Fatalf("expected reviews to reset after edit, got %d", stats.TotalReviews)
	}

	// The edited card should behave like a new card and be due immediately again
	req = httptest.NewRequest(http.MethodGet, "/api/study/"+deck.ID, nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get due cards after edit: got status %d, want %d", rec.Code, http.StatusOK)
	}

	var dueCards []struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&dueCards)
	if len(dueCards) != 1 || dueCards[0].ID != card.ID {
		t.Fatalf("expected edited card to be due again immediately, got %#v", dueCards)
	}
}

func TestIntegration_ReviewRejectsCardThatIsNotDue(t *testing.T) {
	body := `{"email":"repeat-review@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	authCookie := rec.Result().Cookies()[0]

	body = `{"name":"Review Guard Deck","description":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var deck struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&deck)

	body = `{"front":"Guard question","back":"Guard answer"}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deck.ID+"/cards", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var card struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&card)

	body = `{"rating":4}`
	req = httptest.NewRequest(http.MethodPost, "/api/study/"+card.ID+"/review", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("first review: got status %d, want %d", rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/study/"+card.ID+"/review", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("second review: got status %d, want %d, body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestIntegration_SetCardTagsRejectsTagsFromAnotherDeck(t *testing.T) {
	body := `{"email":"tag-scope@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	authCookie := rec.Result().Cookies()[0]

	body = `{"name":"Deck One","description":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var deckOne struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&deckOne)

	body = `{"name":"Deck Two","description":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var deckTwo struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&deckTwo)

	body = `{"front":"Tagged question","back":"Tagged answer"}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deckOne.ID+"/cards", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var card struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&card)

	body = `{"name":"Wrong Deck Tag"}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deckTwo.ID+"/tags", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var tag struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&tag)

	body = `{"tag_ids":["` + tag.ID + `"]}`
	req = httptest.NewRequest(http.MethodPut, "/api/cards/"+card.ID+"/tags", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("set card tags: got status %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestIntegration_UpdateCardRejectsBlankContent(t *testing.T) {
	body := `{"email":"blank-card@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	authCookie := rec.Result().Cookies()[0]

	body = `{"name":"Validation Deck","description":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var deck struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&deck)

	body = `{"front":"Original","back":"Answer"}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deck.ID+"/cards", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var card struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&card)

	body = `{"front":"   ","back":"Still there","link":""}`
	req = httptest.NewRequest(http.MethodPut, "/api/cards/"+card.ID, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("update card: got status %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestIntegration_DeckValidationRejectsBlankTrimmedNames(t *testing.T) {
	body := `{"email":"blank-deck@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	authCookie := rec.Result().Cookies()[0]

	body = `{"name":"   ","description":"ignored"}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create blank deck: got status %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	body = `{"name":"Valid Deck","description":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var deck struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&deck); err != nil {
		t.Fatalf("decode deck: %v", err)
	}

	body = `{"name":"   ","description":"still invalid"}`
	req = httptest.NewRequest(http.MethodPut, "/api/decks/"+deck.ID, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("update blank deck: got status %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestIntegration_TagValidationAndDuplicates(t *testing.T) {
	body := `{"email":"blank-tag@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	authCookie := rec.Result().Cookies()[0]

	body = `{"name":"Tag Deck","description":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var deck struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&deck); err != nil {
		t.Fatalf("decode deck: %v", err)
	}

	body = `{"name":"   "}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deck.ID+"/tags", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create blank tag: got status %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	body = `{"name":"Biology"}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deck.ID+"/tags", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create tag: got status %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deck.ID+"/tags", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate tag: got status %d, want %d, body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestIntegration_StudyStatsUseRollingWindows(t *testing.T) {
	body := `{"email":"rolling-stats@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	authCookie := rec.Result().Cookies()[0]

	body = `{"name":"Stats Deck","description":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var deck struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&deck)

	body = `{"front":"Stats question","back":"Stats answer"}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deck.ID+"/cards", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var card struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&card)

	if _, err := testDB.Pool.Exec(context.Background(), `
		INSERT INTO reviews (card_id, rating, reviewed_at) VALUES
			($1, 4, NOW() - INTERVAL '23 hours'),
			($1, 3, NOW() - INTERVAL '6 days 23 hours'),
			($1, 2, NOW() - INTERVAL '8 days')
	`, card.ID); err != nil {
		t.Fatalf("seed reviews: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/study/stats", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get rolling stats: got status %d, want %d", rec.Code, http.StatusOK)
	}

	var stats struct {
		TotalReviews       int `json:"totalReviews"`
		ReviewsLast24Hours int `json:"reviewsLast24Hours"`
		ReviewsLast7Days   int `json:"reviewsLast7Days"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}

	if stats.TotalReviews != 3 {
		t.Fatalf("total reviews = %d, want 3", stats.TotalReviews)
	}
	if stats.ReviewsLast24Hours != 1 {
		t.Fatalf("reviewsLast24Hours = %d, want 1", stats.ReviewsLast24Hours)
	}
	if stats.ReviewsLast7Days != 2 {
		t.Fatalf("reviewsLast7Days = %d, want 2", stats.ReviewsLast7Days)
	}
}

func TestIntegration_Authorization(t *testing.T) {
	// Register two users
	body1 := `{"email":"user1@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body1)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)
	cookie1 := rec.Result().Cookies()[0]

	body2 := `{"email":"user2@example.com","password":"password123"}`
	req = httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body2)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)
	cookie2 := rec.Result().Cookies()[0]

	// User 1 creates a deck
	body := `{"name":"User1 Deck","description":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie1)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var deck struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rec.Body).Decode(&deck)

	// User 2 tries to access User 1's deck
	req = httptest.NewRequest(http.MethodGet, "/api/decks/"+deck.ID, nil)
	req.AddCookie(cookie2)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden when accessing another user's deck, got %d", rec.Code)
	}

	// User 2 tries to delete User 1's deck
	req = httptest.NewRequest(http.MethodDelete, "/api/decks/"+deck.ID, nil)
	req.AddCookie(cookie2)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden when deleting another user's deck, got %d", rec.Code)
	}
}
