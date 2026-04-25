package model

import (
	"time"

	"github.com/google/uuid"
)

type Deck struct {
	ID                   uuid.UUID `json:"id"`
	UserID               uuid.UUID `json:"user_id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	FuzzEnabled          bool      `json:"fuzz_enabled"`
	NewCardFrontTemplate string    `json:"new_card_front_template"`
	NewCardBackTemplate  string    `json:"new_card_back_template"`
	CreatedAt            time.Time `json:"created_at"`
}

type DeckStats struct {
	Total    int `json:"total"`
	New      int `json:"new"`
	Due      int `json:"due"`
	Learning int `json:"learning"`
}

type DeckWithStats struct {
	Deck
	Stats DeckStats `json:"stats"`
}
