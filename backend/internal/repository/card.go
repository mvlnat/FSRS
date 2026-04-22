package repository

import (
	"context"
	"errors"
	"sort"
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

func (r *CardRepository) GetOwnedByID(ctx context.Context, id, userID uuid.UUID) (*model.Card, error) {
	card := &model.Card{}
	var ownerID uuid.UUID
	err := r.db.Pool.QueryRow(ctx, `
		SELECT c.id, c.deck_id, c.front, c.back, c.link, c.created_at, d.user_id
		FROM cards c
		JOIN decks d ON c.deck_id = d.id
		WHERE c.id = $1
	`, id).Scan(&card.ID, &card.DeckID, &card.Front, &card.Back, &card.Link, &card.CreatedAt, &ownerID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if ownerID != userID {
		return nil, ErrForbidden
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

	return scanCardsWithState(rows)
}

func (r *CardRepository) Update(ctx context.Context, id uuid.UUID, front, back, link string) (*model.Card, error) {
	return r.UpdateWithTags(ctx, id, front, back, link, nil, false)
}

func (r *CardRepository) UpdateWithTags(ctx context.Context, id uuid.UUID, front, back, link string, tagIDs []uuid.UUID, replaceTags bool) (*model.Card, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	existing := &model.Card{}
	err = tx.QueryRow(ctx,
		`SELECT id, deck_id, front, back, link, created_at FROM cards WHERE id = $1 FOR UPDATE`,
		id,
	).Scan(&existing.ID, &existing.DeckID, &existing.Front, &existing.Back, &existing.Link, &existing.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	contentChanged := existing.Front != front || existing.Back != back || existing.Link != link

	card := existing
	if contentChanged {
		card = &model.Card{}
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
	}

	if replaceTags {
		if _, err := tx.Exec(ctx, `DELETE FROM card_tags WHERE card_id = $1`, id); err != nil {
			return nil, err
		}

		for _, tagID := range tagIDs {
			if _, err := tx.Exec(ctx,
				`INSERT INTO card_tags (card_id, tag_id) VALUES ($1, $2)`,
				id, tagID,
			); err != nil {
				return nil, err
			}
		}
	}

	if contentChanged {
		// Editing card content invalidates prior scheduling and review history.
		if _, err := tx.Exec(ctx, `DELETE FROM card_states WHERE card_id = $1`, id); err != nil {
			return nil, err
		}

		if _, err := tx.Exec(ctx, `DELETE FROM reviews WHERE card_id = $1`, id); err != nil {
			return nil, err
		}
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

	return scanCardsWithState(rows)
}

func (r *CardRepository) GetPendingLearningCards(ctx context.Context, deckID uuid.UUID, limit int) ([]model.CardWithState, error) {
	now := time.Now()

	rows, err := r.db.Pool.Query(ctx, `
		SELECT c.id, c.deck_id, c.front, c.back, c.link, c.created_at,
			   cs.id, cs.card_id, cs.due, cs.stability, cs.difficulty,
			   cs.elapsed_days, cs.scheduled_days, cs.reps, cs.lapses, cs.state, cs.last_review
		FROM cards c
		JOIN card_states cs ON c.id = cs.card_id
		WHERE c.deck_id = $1
			AND cs.state IN (1, 3)
			AND cs.scheduled_days = 0
			AND cs.due > $2
		ORDER BY cs.due ASC
		LIMIT $3
	`, deckID, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanCardsWithState(rows)
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

type dueCalendarRow struct {
	Date     time.Time
	DeckID   uuid.UUID
	DeckName string
	Count    int
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

func (r *CardRepository) GetDueCalendar(ctx context.Context, userID uuid.UUID, timezone string, startDate, endDate time.Time) ([]model.DueCalendarDay, error) {
	byDate := make(map[string]*model.DueCalendarDay)
	orderedDates := make([]string, 0)

	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			(cs.due AT TIME ZONE $2)::date AS due_date,
			d.id,
			d.name,
			COUNT(*) AS due_count
		FROM card_states cs
		JOIN cards c ON cs.card_id = c.id
		JOIN decks d ON c.deck_id = d.id
		WHERE d.user_id = $1
			AND (cs.due AT TIME ZONE $2)::date BETWEEN $3 AND $4
		GROUP BY due_date, d.id, d.name
		ORDER BY due_date ASC, due_count DESC, d.name ASC
	`, userID, timezone, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var row dueCalendarRow
		if err := rows.Scan(&row.Date, &row.DeckID, &row.DeckName, &row.Count); err != nil {
			return nil, err
		}

		addDueCalendarRow(byDate, &orderedDates, row.Date.Format("2006-01-02"), row.DeckID, row.DeckName, row.Count)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	nowLocalDate := time.Now().In(startDate.Location())
	nowLocalDate = time.Date(nowLocalDate.Year(), nowLocalDate.Month(), nowLocalDate.Day(), 0, 0, 0, 0, nowLocalDate.Location())
	if !nowLocalDate.Before(startDate) && !nowLocalDate.After(endDate) {

		newCardRows, err := r.db.Pool.Query(ctx, `
			SELECT
				d.id,
				d.name,
				COUNT(*) AS due_count
			FROM cards c
			JOIN decks d ON c.deck_id = d.id
			LEFT JOIN card_states cs ON c.id = cs.card_id
			WHERE d.user_id = $1
				AND cs.id IS NULL
			GROUP BY d.id, d.name
			ORDER BY due_count DESC, d.name ASC
		`, userID)
		if err != nil {
			return nil, err
		}
		defer newCardRows.Close()

		dateKey := nowLocalDate.Format("2006-01-02")
		for newCardRows.Next() {
			var deckID uuid.UUID
			var deckName string
			var count int
			if err := newCardRows.Scan(&deckID, &deckName, &count); err != nil {
				return nil, err
			}

			addDueCalendarRow(byDate, &orderedDates, dateKey, deckID, deckName, count)
		}
		if err := newCardRows.Err(); err != nil {
			return nil, err
		}
	}

	sort.Strings(orderedDates)

	calendar := make([]model.DueCalendarDay, 0, len(orderedDates))
	for _, dateKey := range orderedDates {
		calendar = append(calendar, *byDate[dateKey])
	}

	return calendar, nil
}

func addDueCalendarRow(byDate map[string]*model.DueCalendarDay, orderedDates *[]string, dateKey string, deckID uuid.UUID, deckName string, count int) {
	entry, exists := byDate[dateKey]
	if !exists {
		entry = &model.DueCalendarDay{
			Date:  dateKey,
			Decks: []model.DueCalendarDeck{},
		}
		byDate[dateKey] = entry
		*orderedDates = append(*orderedDates, dateKey)
	}

	entry.Total += count
	entry.Decks = append(entry.Decks, model.DueCalendarDeck{
		DeckID:   deckID,
		DeckName: deckName,
		Count:    count,
	})
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

func scanCardsWithState(rows pgx.Rows) ([]model.CardWithState, error) {
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
