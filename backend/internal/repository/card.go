package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ziyangli/fsrs/backend/internal/model"
)

type CardRepository struct {
	db *DB
}

func NewCardRepository(db *DB) *CardRepository {
	return &CardRepository{db: db}
}

func (r *CardRepository) Create(ctx context.Context, deckID uuid.UUID, front, back, link string) (*model.Card, error) {
	card := &model.Card{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO cards (deck_id, front, back, link) VALUES ($1, $2, $3, $4)
		 RETURNING id, deck_id, front, back, link, created_at`,
		deckID, front, back, link,
	).Scan(&card.ID, &card.DeckID, &card.Front, &card.Back, &card.Link, &card.CreatedAt)

	if err != nil {
		return nil, err
	}
	return card, nil
}

func (r *CardRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Card, error) {
	card := &model.Card{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, deck_id, front, back, link, created_at FROM cards WHERE id = $1`,
		id,
	).Scan(&card.ID, &card.DeckID, &card.Front, &card.Back, &card.Link, &card.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return card, nil
}

func (r *CardRepository) ListByDeck(ctx context.Context, deckID uuid.UUID) ([]model.CardWithState, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT c.id, c.deck_id, c.front, c.back, c.link, c.created_at,
			   cs.id, cs.card_id, cs.due, cs.stability, cs.difficulty,
			   cs.elapsed_days, cs.scheduled_days, cs.reps, cs.lapses, cs.state, cs.last_review
		FROM cards c
		LEFT JOIN card_states cs ON c.id = cs.card_id
		WHERE c.deck_id = $1
		ORDER BY c.created_at DESC
	`, deckID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []model.CardWithState
	for rows.Next() {
		var c model.CardWithState
		var stateID, stateCardID *uuid.UUID
		var due *time.Time
		var stability, difficulty *float64
		var elapsedDays, scheduledDays, reps, lapses, state *int
		var lastReview *time.Time

		if err := rows.Scan(
			&c.ID, &c.DeckID, &c.Front, &c.Back, &c.Link, &c.CreatedAt,
			&stateID, &stateCardID, &due, &stability, &difficulty,
			&elapsedDays, &scheduledDays, &reps, &lapses, &state, &lastReview,
		); err != nil {
			return nil, err
		}

		if stateID != nil {
			c.State = &model.CardState{
				ID:            *stateID,
				CardID:        *stateCardID,
				Due:           *due,
				Stability:     *stability,
				Difficulty:    *difficulty,
				ElapsedDays:   *elapsedDays,
				ScheduledDays: *scheduledDays,
				Reps:          *reps,
				Lapses:        *lapses,
				State:         *state,
				LastReview:    lastReview,
			}
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func (r *CardRepository) Update(ctx context.Context, id uuid.UUID, front, back, link string) (*model.Card, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	card := &model.Card{}
	err = tx.QueryRow(ctx,
		`UPDATE cards SET front = $2, back = $3, link = $4 WHERE id = $1
		 RETURNING id, deck_id, front, back, link, created_at`,
		id, front, back, link,
	).Scan(&card.ID, &card.DeckID, &card.Front, &card.Back, &card.Link, &card.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	// Editing card content invalidates prior scheduling and review history.
	if _, err := tx.Exec(ctx, `DELETE FROM card_states WHERE card_id = $1`, id); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM reviews WHERE card_id = $1`, id); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return card, nil
}

func (r *CardRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, `DELETE FROM cards WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CardRepository) GetDueCards(ctx context.Context, deckID uuid.UUID, limit int) ([]model.CardWithState, error) {
	now := time.Now()

	rows, err := r.db.Pool.Query(ctx, `
		SELECT c.id, c.deck_id, c.front, c.back, c.link, c.created_at,
			   cs.id, cs.card_id, cs.due, cs.stability, cs.difficulty,
			   cs.elapsed_days, cs.scheduled_days, cs.reps, cs.lapses, cs.state, cs.last_review
		FROM cards c
		LEFT JOIN card_states cs ON c.id = cs.card_id
		WHERE c.deck_id = $1 AND (cs.id IS NULL OR cs.due <= $2)
		ORDER BY COALESCE(cs.due, c.created_at) ASC
		LIMIT $3
	`, deckID, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []model.CardWithState
	for rows.Next() {
		var c model.CardWithState
		var stateID, stateCardID *uuid.UUID
		var due *time.Time
		var stability, difficulty *float64
		var elapsedDays, scheduledDays, reps, lapses, state *int
		var lastReview *time.Time

		if err := rows.Scan(
			&c.ID, &c.DeckID, &c.Front, &c.Back, &c.Link, &c.CreatedAt,
			&stateID, &stateCardID, &due, &stability, &difficulty,
			&elapsedDays, &scheduledDays, &reps, &lapses, &state, &lastReview,
		); err != nil {
			return nil, err
		}

		if stateID != nil {
			c.State = &model.CardState{
				ID:            *stateID,
				CardID:        *stateCardID,
				Due:           *due,
				Stability:     *stability,
				Difficulty:    *difficulty,
				ElapsedDays:   *elapsedDays,
				ScheduledDays: *scheduledDays,
				Reps:          *reps,
				Lapses:        *lapses,
				State:         *state,
				LastReview:    lastReview,
			}
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func (r *CardRepository) GetStateByCardID(ctx context.Context, cardID uuid.UUID) (*model.CardState, error) {
	state, err := scanCardState(r.db.Pool.QueryRow(ctx, `
		SELECT id, card_id, due, stability, difficulty, elapsed_days, scheduled_days, reps, lapses, state, last_review
		FROM card_states WHERE card_id = $1
	`, cardID))

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return state, nil
}

func (r *CardRepository) UpsertState(ctx context.Context, state *model.CardState) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO card_states (id, card_id, due, stability, difficulty, elapsed_days, scheduled_days, reps, lapses, state, last_review)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (card_id) DO UPDATE SET
			due = EXCLUDED.due,
			stability = EXCLUDED.stability,
			difficulty = EXCLUDED.difficulty,
			elapsed_days = EXCLUDED.elapsed_days,
			scheduled_days = EXCLUDED.scheduled_days,
			reps = EXCLUDED.reps,
			lapses = EXCLUDED.lapses,
			state = EXCLUDED.state,
			last_review = EXCLUDED.last_review
	`, state.ID, state.CardID, state.Due, state.Stability, state.Difficulty,
		state.ElapsedDays, state.ScheduledDays, state.Reps, state.Lapses, state.State, state.LastReview)
	return err
}

func (r *CardRepository) CreateReview(ctx context.Context, cardID uuid.UUID, rating int) (*model.Review, error) {
	review := &model.Review{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO reviews (card_id, rating) VALUES ($1, $2)
		 RETURNING id, card_id, rating, reviewed_at`,
		cardID, rating,
	).Scan(&review.ID, &review.CardID, &review.Rating, &review.ReviewedAt)

	if err != nil {
		return nil, err
	}
	return review, nil
}

// StudyStats contains user-level study statistics
type StudyStats struct {
	TotalReviews       int     `json:"totalReviews"`
	ReviewsLast24Hours int     `json:"reviewsLast24Hours"`
	ReviewsLast7Days   int     `json:"reviewsLast7Days"`
	AvgRating          float64 `json:"avgRating"`
	RetentionRate      float64 `json:"retentionRate"` // % of reviews rated 3 or 4
}

func (r *CardRepository) GetUserStudyStats(ctx context.Context, userID uuid.UUID) (*StudyStats, error) {
	stats := &StudyStats{}
	var avgRating, retentionRate *float64

	err := r.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(r.id) as total_reviews,
			COUNT(*) FILTER (WHERE r.reviewed_at >= NOW() - INTERVAL '24 hours') as reviews_last_24_hours,
			COUNT(*) FILTER (WHERE r.reviewed_at >= NOW() - INTERVAL '7 days') as reviews_last_7_days,
			AVG(r.rating)::float as avg_rating,
			(COUNT(*) FILTER (WHERE r.rating >= 3)::float / NULLIF(COUNT(*)::float, 0)) * 100 as retention_rate
		FROM reviews r
		JOIN cards c ON r.card_id = c.id
		JOIN decks d ON c.deck_id = d.id
		WHERE d.user_id = $1
	`, userID).Scan(&stats.TotalReviews, &stats.ReviewsLast24Hours, &stats.ReviewsLast7Days, &avgRating, &retentionRate)
	if err != nil {
		return nil, err
	}

	if avgRating != nil {
		stats.AvgRating = *avgRating
	}
	if retentionRate != nil {
		stats.RetentionRate = *retentionRate
	}

	return stats, nil
}

func (r *CardRepository) ApplyReview(ctx context.Context, cardID uuid.UUID, rating int, reviewFn func(currentState *model.CardState) *model.CardState) (*model.CardState, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var lockedCardID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM cards WHERE id = $1 FOR UPDATE`, cardID).Scan(&lockedCardID); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	currentState, err := scanCardState(tx.QueryRow(ctx, `
		SELECT id, card_id, due, stability, difficulty, elapsed_days, scheduled_days, reps, lapses, state, last_review
		FROM card_states WHERE card_id = $1
		FOR UPDATE
	`, cardID))
	if errors.Is(err, pgx.ErrNoRows) {
		currentState = nil
	} else if err != nil {
		return nil, err
	}

	if currentState != nil && currentState.Due.After(time.Now()) {
		return nil, ErrCardNotDue
	}

	newState := reviewFn(currentState)
	if newState == nil {
		return nil, errors.New("review function returned nil state")
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO card_states (id, card_id, due, stability, difficulty, elapsed_days, scheduled_days, reps, lapses, state, last_review)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (card_id) DO UPDATE SET
			due = EXCLUDED.due,
			stability = EXCLUDED.stability,
			difficulty = EXCLUDED.difficulty,
			elapsed_days = EXCLUDED.elapsed_days,
			scheduled_days = EXCLUDED.scheduled_days,
			reps = EXCLUDED.reps,
			lapses = EXCLUDED.lapses,
			state = EXCLUDED.state,
			last_review = EXCLUDED.last_review
	`, newState.ID, newState.CardID, newState.Due, newState.Stability, newState.Difficulty,
		newState.ElapsedDays, newState.ScheduledDays, newState.Reps, newState.Lapses, newState.State, newState.LastReview); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO reviews (card_id, rating) VALUES ($1, $2)`,
		cardID, rating,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return newState, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCardState(scanner rowScanner) (*model.CardState, error) {
	state := &model.CardState{}
	var lastReview *time.Time

	if err := scanner.Scan(
		&state.ID, &state.CardID, &state.Due, &state.Stability, &state.Difficulty,
		&state.ElapsedDays, &state.ScheduledDays, &state.Reps, &state.Lapses, &state.State, &lastReview,
	); err != nil {
		return nil, err
	}

	state.LastReview = lastReview
	return state, nil
}
