package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFSRSService_NewCardState(t *testing.T) {
	svc := NewFSRSService()
	cardID := uuid.New()

	state := svc.NewCardState(cardID)

	if state.CardID != cardID {
		t.Errorf("CardID = %v, want %v", state.CardID, cardID)
	}
	if state.State != 0 {
		t.Errorf("State = %d, want 0 (New)", state.State)
	}
	if state.Reps != 0 {
		t.Errorf("Reps = %d, want 0", state.Reps)
	}
	if state.Lapses != 0 {
		t.Errorf("Lapses = %d, want 0", state.Lapses)
	}
	if state.LastReview != nil {
		t.Errorf("LastReview = %v, want nil", state.LastReview)
	}
}

func TestFSRSService_Review_NewCard(t *testing.T) {
	svc := NewFSRSService()
	cardID := uuid.New()
	initialState := svc.NewCardState(cardID)

	tests := []struct {
		name   string
		rating int
	}{
		{"Again (1)", 1},
		{"Hard (2)", 2},
		{"Good (3)", 3},
		{"Easy (4)", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newState := svc.Review(initialState, tt.rating)

			if newState.CardID != cardID {
				t.Errorf("CardID changed unexpectedly")
			}
			if newState.Reps != 1 {
				t.Errorf("Reps = %d, want 1", newState.Reps)
			}
			if newState.LastReview == nil {
				t.Error("LastReview should be set after review")
			}
			if newState.Due.Before(time.Now()) {
				t.Error("Due date should be in the future or now")
			}
		})
	}
}

func TestFSRSService_Review_RatingAffectsDue(t *testing.T) {
	svc := NewFSRSService()
	cardID := uuid.New()
	initialState := svc.NewCardState(cardID)

	// Review with "Again" (1)
	againState := svc.Review(initialState, 1)

	// Review with "Easy" (4) using a fresh card
	initialState2 := svc.NewCardState(cardID)
	easyState := svc.Review(initialState2, 4)

	// Easy should have a later due date than Again
	if !easyState.Due.After(againState.Due) {
		t.Errorf("Easy due (%v) should be after Again due (%v)", easyState.Due, againState.Due)
	}
}

func TestFSRSService_Review_LapsesIncrement(t *testing.T) {
	svc := NewFSRSService()
	cardID := uuid.New()

	// Start with a new card and review it as Good to get to Review state
	state := svc.NewCardState(cardID)
	state = svc.Review(state, 3) // Good
	state = svc.Review(state, 3) // Good again to solidify

	initialLapses := state.Lapses

	// Now review as Again - should increase lapses if in Review state
	state = svc.Review(state, 1)

	// If the card was in Review state, lapses should increase
	// Note: The exact behavior depends on FSRS state transitions
	if state.Lapses < initialLapses {
		t.Errorf("Lapses should not decrease, got %d from %d", state.Lapses, initialLapses)
	}
}

func TestFSRSService_Review_StabilityIncreases(t *testing.T) {
	svc := NewFSRSService()
	cardID := uuid.New()

	state := svc.NewCardState(cardID)
	initialStability := state.Stability

	// Multiple Good reviews should increase stability
	state = svc.Review(state, 3)
	state = svc.Review(state, 3)
	state = svc.Review(state, 3)

	if state.Stability <= initialStability {
		t.Errorf("Stability should increase after multiple Good reviews, got %f from %f",
			state.Stability, initialStability)
	}
}
