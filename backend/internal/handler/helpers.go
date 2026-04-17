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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func requireUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return uuid.Nil, false
	}

	return userID, true
}

func parseUUIDParam(w http.ResponseWriter, r *http.Request, paramName, resourceName string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, paramName))
	if err != nil {
		http.Error(w, "Invalid "+resourceName+" ID", http.StatusBadRequest)
		return uuid.Nil, false
	}

	return id, true
}

func requireOwnedDeck(
	w http.ResponseWriter,
	r *http.Request,
	deckRepo *repository.DeckRepository,
	deckID uuid.UUID,
	userID uuid.UUID,
) (*model.Deck, bool) {
	deck, err := deckRepo.GetOwnedByID(r.Context(), deckID, userID)
	if err == repository.ErrNotFound {
		http.Error(w, "Deck not found", http.StatusNotFound)
		return nil, false
	}
	if err == repository.ErrForbidden {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil, false
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil, false
	}

	return deck, true
}

func requireOwnedCard(
	w http.ResponseWriter,
	r *http.Request,
	cardRepo *repository.CardRepository,
	cardID uuid.UUID,
	userID uuid.UUID,
) (*model.Card, bool) {
	card, err := cardRepo.GetOwnedByID(r.Context(), cardID, userID)
	if err == repository.ErrNotFound {
		http.Error(w, "Card not found", http.StatusNotFound)
		return nil, false
	}
	if err == repository.ErrForbidden {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil, false
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil, false
	}

	return card, true
}

func requireOwnedTag(
	w http.ResponseWriter,
	r *http.Request,
	tagRepo *repository.TagRepository,
	tagID uuid.UUID,
	userID uuid.UUID,
) (*model.Tag, bool) {
	tag, err := tagRepo.GetOwnedByID(r.Context(), tagID, userID)
	if err == repository.ErrNotFound {
		http.Error(w, "Tag not found", http.StatusNotFound)
		return nil, false
	}
	if err == repository.ErrForbidden {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil, false
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil, false
	}

	return tag, true
}

func parseUniqueTagIDs(w http.ResponseWriter, rawTagIDs []string) ([]uuid.UUID, bool) {
	tagIDs := make([]uuid.UUID, 0, len(rawTagIDs))
	seenTagIDs := make(map[uuid.UUID]struct{}, len(rawTagIDs))

	for _, rawTagID := range rawTagIDs {
		tagID, err := uuid.Parse(rawTagID)
		if err != nil {
			http.Error(w, "Invalid tag ID", http.StatusBadRequest)
			return nil, false
		}
		if _, seen := seenTagIDs[tagID]; seen {
			continue
		}
		seenTagIDs[tagID] = struct{}{}
		tagIDs = append(tagIDs, tagID)
	}

	return tagIDs, true
}

func validateTagIDsForDeck(
	w http.ResponseWriter,
	r *http.Request,
	tagRepo *repository.TagRepository,
	deckID uuid.UUID,
	tagIDs []uuid.UUID,
) bool {
	if len(tagIDs) == 0 {
		return true
	}

	tags, err := tagRepo.GetByIDs(r.Context(), tagIDs)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return false
	}
	if len(tags) != len(tagIDs) {
		http.Error(w, "Invalid tag ID", http.StatusBadRequest)
		return false
	}

	for _, tag := range tags {
		if tag.DeckID != deckID {
			http.Error(w, "Tags must belong to the same deck as the card", http.StatusBadRequest)
			return false
		}
	}

	return true
}
