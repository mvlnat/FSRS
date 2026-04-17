package handler

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

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

const maxTagNameLength = 100

func validateTagName(name string) error {
	if name == "" {
		return fmt.Errorf("Tag name is required")
	}
	if utf8.RuneCountInString(name) > maxTagNameLength {
		return fmt.Errorf("Tag name must be %d characters or fewer", maxTagNameLength)
	}
	return nil
}

type setCardTagsRequest struct {
	TagIDs []string `json:"tag_ids"`
}

// ListByDeck returns all tags for a deck
func (h *TagHandler) ListByDeck(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	deckID, ok := parseUUIDParam(w, r, "deckId", "deck")
	if !ok {
		return
	}

	if _, ok := requireOwnedDeck(w, r, h.deckRepo, deckID, userID); !ok {
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

	writeJSON(w, http.StatusOK, tags)
}

// Create creates a new tag for a deck
func (h *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	deckID, ok := parseUUIDParam(w, r, "deckId", "deck")
	if !ok {
		return
	}

	if _, ok := requireOwnedDeck(w, r, h.deckRepo, deckID, userID); !ok {
		return
	}

	var req createTagRequest
	if !decodeJSONBody(w, r, &req, 0) {
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if err := validateTagName(req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tag, err := h.tagRepo.Create(r.Context(), deckID, req.Name)
	if err == repository.ErrInvalidInput {
		http.Error(w, "Tag name is required", http.StatusBadRequest)
		return
	}
	if err == repository.ErrDuplicate {
		http.Error(w, "Tag already exists", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, tag)
}

// Delete deletes a tag
func (h *TagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	tagID, ok := parseUUIDParam(w, r, "tagId", "tag")
	if !ok {
		return
	}

	if _, ok := requireOwnedTag(w, r, h.tagRepo, tagID, userID); !ok {
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
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	cardID, ok := parseUUIDParam(w, r, "cardId", "card")
	if !ok {
		return
	}

	card, ok := requireOwnedCard(w, r, h.cardRepo, cardID, userID)
	if !ok {
		return
	}

	var req setCardTagsRequest
	if !decodeJSONBody(w, r, &req, 0) {
		return
	}

	tagIDs, ok := parseUniqueTagIDs(w, req.TagIDs)
	if !ok {
		return
	}
	if !validateTagIDsForDeck(w, r, h.tagRepo, card.DeckID, tagIDs) {
		return
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

	writeJSON(w, http.StatusOK, tags)
}
