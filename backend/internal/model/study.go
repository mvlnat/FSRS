package model

import "github.com/google/uuid"

type DueCalendarDeck struct {
	DeckID   uuid.UUID `json:"deck_id"`
	DeckName string    `json:"deck_name"`
	Count    int       `json:"count"`
}

type DueCalendarDay struct {
	Date  string            `json:"date"`
	Total int               `json:"total"`
	Decks []DueCalendarDeck `json:"decks"`
}
