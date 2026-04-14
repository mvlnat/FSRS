package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ziyangli/fsrs/backend/internal/middleware"
	"github.com/ziyangli/fsrs/backend/internal/model"
	"github.com/ziyangli/fsrs/backend/internal/repository"
)

// sanitizeFilename removes or replaces characters that could cause issues in Content-Disposition
var unsafeFilenameChars = regexp.MustCompile(`[^\w\s\-\.]`)

func sanitizeFilename(name string) string {
	// Replace unsafe characters with underscores
	safe := unsafeFilenameChars.ReplaceAllString(name, "_")
	// Collapse multiple underscores
	safe = strings.ReplaceAll(safe, "__", "_")
	// Trim leading/trailing underscores and spaces
	safe = strings.Trim(safe, "_ ")
	if safe == "" {
		safe = "deck"
	}
	return safe
}

type DeckHandler struct {
	deckRepo *repository.DeckRepository
	cardRepo *repository.CardRepository
}

func NewDeckHandler(deckRepo *repository.DeckRepository, cardRepo *repository.CardRepository) *DeckHandler {
	return &DeckHandler{deckRepo: deckRepo, cardRepo: cardRepo}
}

type createDeckRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *DeckHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	decksWithStats, err := h.deckRepo.ListByUserWithStats(r.Context(), userID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if decksWithStats == nil {
		decksWithStats = []model.DeckWithStats{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(decksWithStats)
}

func (h *DeckHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req createDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	deck, err := h.deckRepo.Create(r.Context(), userID, req.Name, req.Description)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(deck)
}

func (h *DeckHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	deckID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid deck ID", http.StatusBadRequest)
		return
	}

	deck, err := h.deckRepo.GetByID(r.Context(), deckID)
	if err == repository.ErrNotFound {
		http.Error(w, "Deck not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if deck.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deck)
}

func (h *DeckHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	deckID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid deck ID", http.StatusBadRequest)
		return
	}

	// Check ownership
	existing, err := h.deckRepo.GetByID(r.Context(), deckID)
	if err == repository.ErrNotFound {
		http.Error(w, "Deck not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if existing.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req createDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	deck, err := h.deckRepo.Update(r.Context(), deckID, req.Name, req.Description)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deck)
}

func (h *DeckHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	deckID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid deck ID", http.StatusBadRequest)
		return
	}

	// Check ownership
	existing, err := h.deckRepo.GetByID(r.Context(), deckID)
	if err == repository.ErrNotFound {
		http.Error(w, "Deck not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if existing.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.deckRepo.Delete(r.Context(), deckID); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *DeckHandler) Stats(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	deckID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid deck ID", http.StatusBadRequest)
		return
	}

	// Check ownership
	deck, err := h.deckRepo.GetByID(r.Context(), deckID)
	if err == repository.ErrNotFound {
		http.Error(w, "Deck not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if deck.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	stats, err := h.deckRepo.GetStats(r.Context(), deckID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// DeckExport is the JSON structure for importing/exporting decks
type DeckExport struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Cards       []CardExport `json:"cards"`
}

type CardExport struct {
	Front string `json:"front"`
	Back  string `json:"back"`
	Link  string `json:"link,omitempty"`
}

func (h *DeckHandler) Export(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	deckID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid deck ID", http.StatusBadRequest)
		return
	}

	// Check ownership
	deck, err := h.deckRepo.GetByID(r.Context(), deckID)
	if err == repository.ErrNotFound {
		http.Error(w, "Deck not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if deck.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get all cards
	cards, err := h.cardRepo.ListByDeck(r.Context(), deckID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build export
	export := DeckExport{
		Name:        deck.Name,
		Description: deck.Description,
		Cards:       make([]CardExport, len(cards)),
	}
	for i, card := range cards {
		export.Cards[i] = CardExport{
			Front: card.Front,
			Back:  card.Back,
			Link:  card.Link,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeFilename(deck.Name)+".json\"")
	json.NewEncoder(w).Encode(export)
}

func (h *DeckHandler) Import(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Limit request body size to 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)

	var export DeckExport
	if err := json.NewDecoder(r.Body).Decode(&export); err != nil {
		if err.Error() == "http: request body too large" {
			http.Error(w, "Request body too large (max 10MB)", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	if export.Name == "" {
		http.Error(w, "Deck name is required", http.StatusBadRequest)
		return
	}

	// Convert to repository format
	cards := make([]repository.CardImport, len(export.Cards))
	for i, c := range export.Cards {
		cards[i] = repository.CardImport{
			Front: c.Front,
			Back:  c.Back,
			Link:  c.Link,
		}
	}

	// Create deck and cards atomically
	deck, err := h.deckRepo.ImportDeckWithCards(r.Context(), userID, export.Name, export.Description, cards)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return deck with stats
	stats, err := h.deckRepo.GetStats(r.Context(), deck.ID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(model.DeckWithStats{
		Deck:  *deck,
		Stats: *stats,
	})
}
