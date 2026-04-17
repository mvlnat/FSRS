package handler

import (
	"net/http"
	"time"
	_ "time/tzdata"

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

const dueCalendarDateLayout = "2006-01-02"

func (h *StudyHandler) GetDueCards(w http.ResponseWriter, r *http.Request) {
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

	cards, err := h.cardRepo.GetDueCards(r.Context(), deckID, 50)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if cards == nil {
		cards = []model.CardWithState{}
	}

	writeJSON(w, http.StatusOK, cards)
}

func (h *StudyHandler) Review(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	cardID, ok := parseUUIDParam(w, r, "cardId", "card")
	if !ok {
		return
	}

	var req reviewRequest
	if !decodeJSONBody(w, r, &req, 0) {
		return
	}

	if req.Rating < 1 || req.Rating > 4 {
		http.Error(w, "Rating must be between 1 and 4", http.StatusBadRequest)
		return
	}

	if _, ok := requireOwnedCard(w, r, h.cardRepo, cardID, userID); !ok {
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

	writeJSON(w, http.StatusOK, newState)
}

func (h *StudyHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	location, timezone := resolveDueCalendarTimezone(r.URL.Query().Get("timezone"))

	now := time.Now().In(location)
	startDate, err := parseDueCalendarDate(r.URL.Query().Get("start"), now)
	if err != nil {
		http.Error(w, "Invalid start date", http.StatusBadRequest)
		return
	}

	endDate, err := parseDueCalendarDate(r.URL.Query().Get("end"), time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, location))
	if err != nil {
		http.Error(w, "Invalid end date", http.StatusBadRequest)
		return
	}

	if endDate.Before(startDate) {
		http.Error(w, "End date must be on or after start date", http.StatusBadRequest)
		return
	}

	if endDate.Sub(startDate) > 120*24*time.Hour {
		http.Error(w, "Date range must be 120 days or fewer", http.StatusBadRequest)
		return
	}

	calendar, err := h.cardRepo.GetDueCalendar(r.Context(), userID, timezone, startDate, endDate)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if calendar == nil {
		calendar = []model.DueCalendarDay{}
	}

	writeJSON(w, http.StatusOK, calendar)
}

func (h *StudyHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	stats, err := h.cardRepo.GetUserStudyStats(r.Context(), userID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func parseDueCalendarDate(value string, fallback time.Time) (time.Time, error) {
	if value == "" {
		return time.Date(fallback.Year(), fallback.Month(), fallback.Day(), 0, 0, 0, 0, fallback.Location()), nil
	}

	parsed, err := time.ParseInLocation(dueCalendarDateLayout, value, fallback.Location())
	if err != nil {
		return time.Time{}, err
	}

	return parsed, nil
}

func resolveDueCalendarTimezone(value string) (*time.Location, string) {
	if value == "" {
		return time.UTC, time.UTC.String()
	}

	location, err := time.LoadLocation(value)
	if err != nil {
		return time.UTC, time.UTC.String()
	}

	return location, location.String()
}
