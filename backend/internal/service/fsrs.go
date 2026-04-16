package service

import (
	"time"

	"github.com/google/uuid"
	"github.com/open-spaced-repetition/go-fsrs/v3"
	"github.com/ziyangli/fsrs/backend/internal/model"
)

type FSRSService struct {
	fsrs *fsrs.FSRS
}

func NewFSRSService() *FSRSService {
	params := fsrs.DefaultParam()
	return &FSRSService{
		fsrs: fsrs.NewFSRS(params),
	}
}

func (s *FSRSService) Review(cardState *model.CardState, rating int) *model.CardState {
	now := time.Now()
	lastReview := time.Time{}
	if cardState.LastReview != nil {
		lastReview = *cardState.LastReview
	}

	// Convert model state to FSRS card
	card := fsrs.Card{
		Due:           cardState.Due,
		Stability:     cardState.Stability,
		Difficulty:    cardState.Difficulty,
		ElapsedDays:   uint64(cardState.ElapsedDays),
		ScheduledDays: uint64(cardState.ScheduledDays),
		Reps:          uint64(cardState.Reps),
		Lapses:        uint64(cardState.Lapses),
		State:         fsrs.State(cardState.State),
		LastReview:    lastReview,
	}

	// Convert rating (1-4) to FSRS rating
	fsrsRating := fsrs.Rating(rating)

	// Schedule the card
	schedulingInfo := s.fsrs.Repeat(card, now)
	newCard := schedulingInfo[fsrsRating].Card
	var reviewedAt *time.Time
	if !newCard.LastReview.IsZero() {
		lastReview := newCard.LastReview
		reviewedAt = &lastReview
	}

	return &model.CardState{
		ID:            cardState.ID,
		CardID:        cardState.CardID,
		Due:           newCard.Due,
		Stability:     newCard.Stability,
		Difficulty:    newCard.Difficulty,
		ElapsedDays:   int(newCard.ElapsedDays),
		ScheduledDays: int(newCard.ScheduledDays),
		Reps:          int(newCard.Reps),
		Lapses:        int(newCard.Lapses),
		State:         int(newCard.State),
		LastReview:    reviewedAt,
	}
}

func (s *FSRSService) NewCardState(cardID uuid.UUID) *model.CardState {
	now := time.Now()
	return &model.CardState{
		ID:            uuid.New(),
		CardID:        cardID,
		Due:           now,
		Stability:     0,
		Difficulty:    0,
		ElapsedDays:   0,
		ScheduledDays: 0,
		Reps:          0,
		Lapses:        0,
		State:         0, // New
		LastReview:    nil,
	}
}
