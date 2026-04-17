package handler

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

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

	writeJSON(w, http.StatusOK, cards)
}

func (h *CardHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req createCardRequest
	if !decodeJSONBody(w, r, &req, 0) {
		return
	}

	if strings.TrimSpace(req.Front) == "" || strings.TrimSpace(req.Back) == "" {
		http.Error(w, "Front and back are required", http.StatusBadRequest)
		return
	}

	normalizedLink, err := normalizeOptionalExternalLink(req.Link)
	if err != nil {
		http.Error(w, "Link must be a valid http or https URL", http.StatusBadRequest)
		return
	}
	req.Link = normalizedLink

	card, err := h.cardRepo.Create(r.Context(), deckID, req.Front, req.Back, req.Link)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, card)
}

func (h *CardHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	cardID, ok := parseUUIDParam(w, r, "id", "card")
	if !ok {
		return
	}

	card, ok := requireOwnedCard(w, r, h.cardRepo, cardID, userID)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, card)
}

func (h *CardHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	cardID, ok := parseUUIDParam(w, r, "id", "card")
	if !ok {
		return
	}

	card, ok := requireOwnedCard(w, r, h.cardRepo, cardID, userID)
	if !ok {
		return
	}

	var req updateCardRequest
	if !decodeJSONBody(w, r, &req, 0) {
		return
	}

	if strings.TrimSpace(req.Front) == "" || strings.TrimSpace(req.Back) == "" {
		http.Error(w, "Front and back are required", http.StatusBadRequest)
		return
	}

	normalizedLink, err := normalizeOptionalExternalLink(req.Link)
	if err != nil {
		http.Error(w, "Link must be a valid http or https URL", http.StatusBadRequest)
		return
	}
	req.Link = normalizedLink

	replaceTags := req.TagIDs != nil
	tagIDs := make([]uuid.UUID, 0, len(req.TagIDs))
	if replaceTags {
		tagIDs, ok = parseUniqueTagIDs(w, req.TagIDs)
		if !ok {
			return
		}
		if !validateTagIDsForDeck(w, r, h.tagRepo, card.DeckID, tagIDs) {
			return
		}
	}

	updated, err := h.cardRepo.UpdateWithTags(r.Context(), cardID, req.Front, req.Back, req.Link, tagIDs, replaceTags)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (h *CardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	cardID, ok := parseUUIDParam(w, r, "id", "card")
	if !ok {
		return
	}

	if _, ok := requireOwnedCard(w, r, h.cardRepo, cardID, userID); !ok {
		return
	}

	if err := h.cardRepo.Delete(r.Context(), cardID); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
