package handler

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/ziyangli/fsrs/backend/internal/model"
	"github.com/ziyangli/fsrs/backend/internal/repository"
)

// sanitizeFilename removes or replaces characters that could cause issues in Content-Disposition
var unsafeFilenameChars = regexp.MustCompile(`[^\w\s\-\.]`)
var repeatedUnderscores = regexp.MustCompile(`_+`)
var repeatedWhitespace = regexp.MustCompile(`\s+`)

func sanitizeFilename(name string) string {
	safe := repeatedWhitespace.ReplaceAllString(name, " ")
	safe = unsafeFilenameChars.ReplaceAllString(safe, "_")
	safe = repeatedUnderscores.ReplaceAllString(safe, "_")
	safe = strings.TrimSpace(safe)
	safe = strings.Trim(safe, " ._-")
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

const maxDeckNameLength = 255

func normalizeDeckName(name string) string {
	return strings.TrimSpace(name)
}

func validateDeckName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if utf8.RuneCountInString(name) > maxDeckNameLength {
		return fmt.Errorf("name must be %d characters or fewer", maxDeckNameLength)
	}
	return nil
}

func (h *DeckHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
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

	writeJSON(w, http.StatusOK, decksWithStats)
}

func (h *DeckHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req createDeckRequest
	if !decodeJSONBody(w, r, &req, 0) {
		return
	}

	req.Name = normalizeDeckName(req.Name)
	if err := validateDeckName(req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	deck, err := h.deckRepo.Create(r.Context(), userID, req.Name, req.Description)
	if err == repository.ErrInvalidInput {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, deck)
}

func (h *DeckHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	deckID, ok := parseUUIDParam(w, r, "id", "deck")
	if !ok {
		return
	}

	deck, ok := requireOwnedDeck(w, r, h.deckRepo, deckID, userID)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, deck)
}

func (h *DeckHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	deckID, ok := parseUUIDParam(w, r, "id", "deck")
	if !ok {
		return
	}

	if _, ok := requireOwnedDeck(w, r, h.deckRepo, deckID, userID); !ok {
		return
	}

	var req createDeckRequest
	if !decodeJSONBody(w, r, &req, 0) {
		return
	}

	req.Name = normalizeDeckName(req.Name)
	if err := validateDeckName(req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	deck, err := h.deckRepo.Update(r.Context(), deckID, req.Name, req.Description)
	if err == repository.ErrInvalidInput {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, deck)
}

func (h *DeckHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	deckID, ok := parseUUIDParam(w, r, "id", "deck")
	if !ok {
		return
	}

	if _, ok := requireOwnedDeck(w, r, h.deckRepo, deckID, userID); !ok {
		return
	}

	if err := h.deckRepo.Delete(r.Context(), deckID); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *DeckHandler) Stats(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	deckID, ok := parseUUIDParam(w, r, "id", "deck")
	if !ok {
		return
	}

	if _, ok := requireOwnedDeck(w, r, h.deckRepo, deckID, userID); !ok {
		return
	}

	stats, err := h.deckRepo.GetStats(r.Context(), deckID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, stats)
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
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	deckID, ok := parseUUIDParam(w, r, "id", "deck")
	if !ok {
		return
	}

	deck, ok := requireOwnedDeck(w, r, h.deckRepo, deckID, userID)
	if !ok {
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

	w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeFilename(deck.Name)+".json\"")
	writeJSON(w, http.StatusOK, export)
}

func (h *DeckHandler) Import(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var export DeckExport
	if !decodeJSONBody(w, r, &export, 10*1024*1024) {
		return
	}

	export.Name = normalizeDeckName(export.Name)
	if err := validateDeckName(export.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for i, card := range export.Cards {
		if strings.TrimSpace(card.Front) == "" || strings.TrimSpace(card.Back) == "" {
			http.Error(w, fmt.Sprintf("Card %d must include both front and back content", i+1), http.StatusBadRequest)
			return
		}

		normalizedLink, err := normalizeOptionalExternalLink(card.Link)
		if err != nil {
			http.Error(w, fmt.Sprintf("Card %d link must be a valid http or https URL", i+1), http.StatusBadRequest)
			return
		}

		export.Cards[i].Link = normalizedLink
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
	if err == repository.ErrInvalidInput {
		http.Error(w, "Deck import contains invalid content", http.StatusBadRequest)
		return
	}
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

	writeJSON(w, http.StatusCreated, model.DeckWithStats{
		Deck:  *deck,
		Stats: *stats,
	})
}
