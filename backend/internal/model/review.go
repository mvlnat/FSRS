package model

import (
	"time"

	"github.com/google/uuid"
)

type Review struct {
	ID         uuid.UUID `json:"id"`
	CardID     uuid.UUID `json:"card_id"`
	Rating     int       `json:"rating"` // 1=Again, 2=Hard, 3=Good, 4=Easy
	ReviewedAt time.Time `json:"reviewed_at"`
}
