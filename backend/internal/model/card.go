package model

import (
	"time"

	"github.com/google/uuid"
)

type Card struct {
	ID        uuid.UUID `json:"id"`
	DeckID    uuid.UUID `json:"deck_id"`
	Front     string    `json:"front"`
	Back      string    `json:"back"`
	Link      string    `json:"link"`
	CreatedAt time.Time `json:"created_at"`
}

type CardState struct {
	ID            uuid.UUID  `json:"id"`
	CardID        uuid.UUID  `json:"card_id"`
	Due           time.Time  `json:"due"`
	Stability     float64    `json:"stability"`
	Difficulty    float64    `json:"difficulty"`
	ElapsedDays   int        `json:"elapsed_days"`
	ScheduledDays int        `json:"scheduled_days"`
	Reps          int        `json:"reps"`
	Lapses        int        `json:"lapses"`
	State         int        `json:"state"` // 0=New, 1=Learning, 2=Review, 3=Relearning
	LastReview    *time.Time `json:"last_review"`
}

type CardWithState struct {
	Card
	State *CardState `json:"state,omitempty"`
}
