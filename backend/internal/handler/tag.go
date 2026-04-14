package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ziyangli/fsrs/backend/internal/middleware"
	"github.com/ziyangli/fsrs/backend/internal/model"
	"github.com/ziyangli/fsrs/backend/internal/repository"
)

type TagHandler struct {
	tagRepo  *repository.TagRepository
	deckRepo *repository.DeckRepository
	cardRepo *repository.CardRepository
}

func NewTagHandler(tagRepo *repository.TagRepository, deckRepo *repository.DeckRepository, cardRepo *repository.CardRepository) *TagHandler {
	return &TagHandler{
		tagRepo:  tagRepo,
		deckRepo: deckRepo,
		cardRepo: cardRepo,
	}
}

type createTagRequest struct {
	Name string `json:"name"`
}

type setCardTagsRequest struct {
	TagIDs []string `json:"tag_ids"`
}

// ListByDeck returns all tags for a deck
func (h *TagHandler) ListByDeck(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	deckID, err := uuid.Parse(chi.URLParam(r, "deckId"))
	if err != nil {
		http.Error(w, "Invalid deck ID", http.StatusBadRequest)
		return
	}

	// Check deck ownership
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

	tags, err := h.tagRepo.ListByDeck(r.Context(), deckID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if tags == nil {
		tags = []model.Tag{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}

// Create creates a new tag for a deck
func (h *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	deckID, err := uuid.Parse(chi.URLParam(r, "deckId"))
	if err != nil {
		http.Error(w, "Invalid deck ID", http.StatusBadRequest)
		return
	}

	// Check deck ownership
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

	var req createTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Tag name is required", http.StatusBadRequest)
		return
	}

	tag, err := h.tagRepo.Create(r.Context(), deckID, req.Name)
	if err == repository.ErrDuplicate {
		http.Error(w, "Tag already exists", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tag)
}

// Delete deletes a tag
func (h *TagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	tagID, err := uuid.Parse(chi.URLParam(r, "tagId"))
	if err != nil {
		http.Error(w, "Invalid tag ID", http.StatusBadRequest)
		return
	}

	// Get tag to check ownership
	tag, err := h.tagRepo.GetByID(r.Context(), tagID)
	if err == repository.ErrNotFound {
		http.Error(w, "Tag not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check deck ownership
	deck, err := h.deckRepo.GetByID(r.Context(), tag.DeckID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if deck.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.tagRepo.Delete(r.Context(), tagID); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SetCardTags sets the tags for a card
func (h *TagHandler) SetCardTags(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	cardID, err := uuid.Parse(chi.URLParam(r, "cardId"))
	if err != nil {
		http.Error(w, "Invalid card ID", http.StatusBadRequest)
		return
	}

	// Get card to check ownership
	card, err := h.cardRepo.GetByID(r.Context(), cardID)
	if err == repository.ErrNotFound {
		http.Error(w, "Card not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check deck ownership
	deck, err := h.deckRepo.GetByID(r.Context(), card.DeckID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if deck.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req setCardTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Parse tag IDs
	tagIDs := make([]uuid.UUID, 0, len(req.TagIDs))
	for _, idStr := range req.TagIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "Invalid tag ID", http.StatusBadRequest)
			return
		}
		tagIDs = append(tagIDs, id)
	}

	if err := h.tagRepo.SetCardTags(r.Context(), cardID, tagIDs); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return updated tags
	tags, err := h.tagRepo.GetTagsForCard(r.Context(), cardID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if tags == nil {
		tags = []model.Tag{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}
