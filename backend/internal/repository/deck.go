package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ziyangli/fsrs/backend/internal/model"
)

// CardImport represents a card to be imported
type CardImport struct {
	Front string
	Back  string
	Link  string
}

type DeckRepository struct {
	db *DB
}

func NewDeckRepository(db *DB) *DeckRepository {
	return &DeckRepository{db: db}
}

func (r *DeckRepository) Create(ctx context.Context, userID uuid.UUID, name, description string, fuzzEnabled bool) (*model.Deck, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidInput
	}

	deck := &model.Deck{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO decks (user_id, name, description, fuzz_enabled) VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, name, description, fuzz_enabled, created_at`,
		userID, name, description, fuzzEnabled,
	).Scan(&deck.ID, &deck.UserID, &deck.Name, &deck.Description, &deck.FuzzEnabled, &deck.CreatedAt)

	if err != nil {
		return nil, err
	}
	return deck, nil
}

func (r *DeckRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Deck, error) {
	deck := &model.Deck{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, description, fuzz_enabled, created_at FROM decks WHERE id = $1`,
		id,
	).Scan(&deck.ID, &deck.UserID, &deck.Name, &deck.Description, &deck.FuzzEnabled, &deck.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return deck, nil
}

func (r *DeckRepository) GetOwnedByID(ctx context.Context, id, userID uuid.UUID) (*model.Deck, error) {
	deck := &model.Deck{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, description, fuzz_enabled, created_at FROM decks WHERE id = $1`,
		id,
	).Scan(&deck.ID, &deck.UserID, &deck.Name, &deck.Description, &deck.FuzzEnabled, &deck.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if deck.UserID != userID {
		return nil, ErrForbidden
	}

	return deck, nil
}

func (r *DeckRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Deck, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, name, description, fuzz_enabled, created_at FROM decks WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decks []model.Deck
	for rows.Next() {
		var d model.Deck
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.Description, &d.FuzzEnabled, &d.CreatedAt); err != nil {
			return nil, err
		}
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

func (r *DeckRepository) Update(ctx context.Context, id uuid.UUID, name, description string, fuzzEnabled bool) (*model.Deck, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidInput
	}

	deck := &model.Deck{}
	err := r.db.Pool.QueryRow(ctx,
		`UPDATE decks SET name = $2, description = $3, fuzz_enabled = $4 WHERE id = $1
		 RETURNING id, user_id, name, description, fuzz_enabled, created_at`,
		id, name, description, fuzzEnabled,
	).Scan(&deck.ID, &deck.UserID, &deck.Name, &deck.Description, &deck.FuzzEnabled, &deck.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return deck, nil
}

func (r *DeckRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, `DELETE FROM decks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *DeckRepository) GetStats(ctx context.Context, deckID uuid.UUID) (*model.DeckStats, error) {
	stats := &model.DeckStats{}
	now := time.Now()

	err := r.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(c.id) as total,
			COUNT(CASE WHEN cs.id IS NULL THEN 1 END) as new,
			COUNT(CASE WHEN cs.due <= $2 AND cs.state IN (1, 3) THEN 1 END) as learning,
			COUNT(CASE WHEN cs.due <= $2 AND cs.state = 2 THEN 1 END) as due
		FROM cards c
		LEFT JOIN card_states cs ON c.id = cs.card_id
		WHERE c.deck_id = $1
	`, deckID, now).Scan(&stats.Total, &stats.New, &stats.Learning, &stats.Due)

	if err != nil {
		return nil, err
	}
	return stats, nil
}

// ListByUserWithStats returns all decks for a user with their stats in a single query
func (r *DeckRepository) ListByUserWithStats(ctx context.Context, userID uuid.UUID) ([]model.DeckWithStats, error) {
	now := time.Now()

	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			d.id, d.user_id, d.name, d.description, d.fuzz_enabled, d.created_at,
			COUNT(c.id) as total,
			COUNT(CASE WHEN cs.id IS NULL THEN 1 END) as new,
			COUNT(CASE WHEN cs.due <= $2 AND cs.state IN (1, 3) THEN 1 END) as learning,
			COUNT(CASE WHEN cs.due <= $2 AND cs.state = 2 THEN 1 END) as due
		FROM decks d
		LEFT JOIN cards c ON d.id = c.deck_id
		LEFT JOIN card_states cs ON c.id = cs.card_id
		WHERE d.user_id = $1
		GROUP BY d.id, d.user_id, d.name, d.description, d.fuzz_enabled, d.created_at
		ORDER BY d.created_at DESC
	`, userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decks []model.DeckWithStats
	for rows.Next() {
		var d model.DeckWithStats
		if err := rows.Scan(
			&d.Deck.ID, &d.Deck.UserID, &d.Deck.Name, &d.Deck.Description, &d.Deck.FuzzEnabled, &d.Deck.CreatedAt,
			&d.Stats.Total, &d.Stats.New, &d.Stats.Learning, &d.Stats.Due,
		); err != nil {
			return nil, err
		}
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

// ImportDeckWithCards creates a deck and all its cards atomically in a transaction
func (r *DeckRepository) ImportDeckWithCards(ctx context.Context, userID uuid.UUID, name, description string, fuzzEnabled bool, cards []CardImport) (*model.Deck, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidInput
	}

	for _, card := range cards {
		if strings.TrimSpace(card.Front) == "" || strings.TrimSpace(card.Back) == "" {
			return nil, ErrInvalidInput
		}
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Create deck
	deck := &model.Deck{}
	err = tx.QueryRow(ctx,
		`INSERT INTO decks (user_id, name, description, fuzz_enabled) VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, name, description, fuzz_enabled, created_at`,
		userID, name, description, fuzzEnabled,
	).Scan(&deck.ID, &deck.UserID, &deck.Name, &deck.Description, &deck.FuzzEnabled, &deck.CreatedAt)
	if err != nil {
		return nil, err
	}

	// Create all cards
	for _, card := range cards {
		_, err := tx.Exec(ctx,
			`INSERT INTO cards (deck_id, front, back, link) VALUES ($1, $2, $3, $4)`,
			deck.ID, card.Front, card.Back, card.Link,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return deck, nil
}
