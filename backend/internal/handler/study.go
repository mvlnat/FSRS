package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ziyangli/fsrs/backend/internal/middleware"
	"github.com/ziyangli/fsrs/backend/internal/model"
	"github.com/ziyangli/fsrs/backend/internal/repository"
	"github.com/ziyangli/fsrs/backend/internal/service"
)

type StudyHandler struct {
	cardRepo    *repository.CardRepository
	deckRepo    *repository.DeckRepository
	fsrsService *service.FSRSService
}

func NewStudyHandler(cardRepo *repository.CardRepository, deckRepo *repository.DeckRepository, fsrsService *service.FSRSService) *StudyHandler {
	return &StudyHandler{
		cardRepo:    cardRepo,
		deckRepo:    deckRepo,
		fsrsService: fsrsService,
	}
}

type reviewRequest struct {
	Rating int `json:"rating"` // 1=Again, 2=Hard, 3=Good, 4=Easy
}

func (h *StudyHandler) GetDueCards(w http.ResponseWriter, r *http.Request) {
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

	cards, err := h.cardRepo.GetDueCards(r.Context(), deckID, 50)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if cards == nil {
		cards = []model.CardWithState{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cards)
}

func (h *StudyHandler) Review(w http.ResponseWriter, r *http.Request) {
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

	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Rating < 1 || req.Rating > 4 {
		http.Error(w, "Rating must be between 1 and 4", http.StatusBadRequest)
		return
	}

	// Get card and check ownership
	card, err := h.cardRepo.GetByID(r.Context(), cardID)
	if err == repository.ErrNotFound {
		http.Error(w, "Card not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	deck, err := h.deckRepo.GetByID(r.Context(), card.DeckID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if deck.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	newState, err := h.cardRepo.ApplyReview(r.Context(), cardID, req.Rating, func(currentState *model.CardState) *model.CardState {
		if currentState == nil {
			currentState = h.fsrsService.NewCardState(cardID)
		}
		return h.fsrsService.Review(currentState, req.Rating)
	})
	if err == repository.ErrCardNotDue {
		http.Error(w, "Card is not due yet", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newState)
}

func (h *StudyHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	stats, err := h.cardRepo.GetUserStudyStats(r.Context(), userID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
