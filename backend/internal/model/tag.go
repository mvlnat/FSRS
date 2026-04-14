package model

import (
	"time"

	"github.com/google/uuid"
)

type Tag struct {
	ID        uuid.UUID `json:"id"`
	DeckID    uuid.UUID `json:"deck_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
