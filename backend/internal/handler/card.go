package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ziyangli/fsrs/backend/internal/middleware"
	"github.com/ziyangli/fsrs/backend/internal/model"
	"github.com/ziyangli/fsrs/backend/internal/repository"
)

type CardHandler struct {
	cardRepo *repository.CardRepository
	deckRepo *repository.DeckRepository
	tagRepo  *repository.TagRepository
}

func NewCardHandler(cardRepo *repository.CardRepository, deckRepo *repository.DeckRepository, tagRepo *repository.TagRepository) *CardHandler {
	return &CardHandler{
		cardRepo: cardRepo,
		deckRepo: deckRepo,
		tagRepo:  tagRepo,
	}
}

type createCardRequest struct {
	Front string `json:"front"`
	Back  string `json:"back"`
	Link  string `json:"link"`
}

type updateCardRequest struct {
	Front  string   `json:"front"`
	Back   string   `json:"back"`
	Link   string   `json:"link"`
	TagIDs []string `json:"tag_ids"`
}

func (h *CardHandler) ListByDeck(w http.ResponseWriter, r *http.Request) {
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

	cards, err := h.cardRepo.ListByDeck(r.Context(), deckID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if cards == nil {
		cards = []model.CardWithState{}
	}

	// Fetch tags for all cards
	if len(cards) > 0 {
		cardIDs := make([]uuid.UUID, len(cards))
		for i, c := range cards {
			cardIDs[i] = c.ID
		}

		tagMap, err := h.tagRepo.GetTagsForCards(r.Context(), cardIDs)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		for i := range cards {
			if tags, ok := tagMap[cards[i].ID]; ok {
				cards[i].Tags = tags
			} else {
				cards[i].Tags = []model.Tag{}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cards)
}

func (h *CardHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req createCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Front) == "" || strings.TrimSpace(req.Back) == "" {
		http.Error(w, "Front and back are required", http.StatusBadRequest)
		return
	}

	req.Link, err = normalizeOptionalExternalLink(req.Link)
	if err != nil {
		http.Error(w, "Link must be a valid http or https URL", http.StatusBadRequest)
		return
	}

	card, err := h.cardRepo.Create(r.Context(), deckID, req.Front, req.Back, req.Link)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(card)
}

func (h *CardHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	cardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid card ID", http.StatusBadRequest)
		return
	}

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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

func (h *CardHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	cardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid card ID", http.StatusBadRequest)
		return
	}

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

	var req updateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Front) == "" || strings.TrimSpace(req.Back) == "" {
		http.Error(w, "Front and back are required", http.StatusBadRequest)
		return
	}

	req.Link, err = normalizeOptionalExternalLink(req.Link)
	if err != nil {
		http.Error(w, "Link must be a valid http or https URL", http.StatusBadRequest)
		return
	}

	replaceTags := req.TagIDs != nil
	tagIDs := make([]uuid.UUID, 0, len(req.TagIDs))
	if replaceTags {
		seenTagIDs := make(map[uuid.UUID]struct{}, len(req.TagIDs))
		for _, idStr := range req.TagIDs {
			tagID, err := uuid.Parse(idStr)
			if err != nil {
				http.Error(w, "Invalid tag ID", http.StatusBadRequest)
				return
			}
			if _, seen := seenTagIDs[tagID]; seen {
				continue
			}
			seenTagIDs[tagID] = struct{}{}
			tagIDs = append(tagIDs, tagID)
		}
	}

	if len(tagIDs) > 0 {
		tags, err := h.tagRepo.GetByIDs(r.Context(), tagIDs)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if len(tags) != len(tagIDs) {
			http.Error(w, "Invalid tag ID", http.StatusBadRequest)
			return
		}
		for _, tag := range tags {
			if tag.DeckID != card.DeckID {
				http.Error(w, "Tags must belong to the same deck as the card", http.StatusBadRequest)
				return
			}
		}
	}

	updated, err := h.cardRepo.UpdateWithTags(r.Context(), cardID, req.Front, req.Back, req.Link, tagIDs, replaceTags)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (h *CardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	cardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid card ID", http.StatusBadRequest)
		return
	}

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

	if err := h.cardRepo.Delete(r.Context(), cardID); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
