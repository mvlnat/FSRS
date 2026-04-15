package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ziyangli/fsrs/backend/internal/model"
)

type TagRepository struct {
	db *DB
}

func NewTagRepository(db *DB) *TagRepository {
	return &TagRepository{db: db}
}

func (r *TagRepository) Create(ctx context.Context, deckID uuid.UUID, name string) (*model.Tag, error) {
	tag := &model.Tag{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO tags (deck_id, name) VALUES ($1, $2)
		 RETURNING id, deck_id, name, created_at`,
		deckID, name,
	).Scan(&tag.ID, &tag.DeckID, &tag.Name, &tag.CreatedAt)

	if err != nil {
		if err.Error() == "ERROR: duplicate key value violates unique constraint \"tags_deck_id_name_key\" (SQLSTATE 23505)" {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return tag, nil
}

func (r *TagRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Tag, error) {
	tag := &model.Tag{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, deck_id, name, created_at FROM tags WHERE id = $1`,
		id,
	).Scan(&tag.ID, &tag.DeckID, &tag.Name, &tag.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return tag, nil
}

func (r *TagRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]model.Tag, error) {
	if len(ids) == 0 {
		return []model.Tag{}, nil
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, deck_id, name, created_at FROM tags WHERE id = ANY($1)`,
		ids,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.DeckID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *TagRepository) ListByDeck(ctx context.Context, deckID uuid.UUID) ([]model.Tag, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, deck_id, name, created_at FROM tags WHERE deck_id = $1 ORDER BY name ASC`,
		deckID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.DeckID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *TagRepository) Update(ctx context.Context, id uuid.UUID, name string) (*model.Tag, error) {
	tag := &model.Tag{}
	err := r.db.Pool.QueryRow(ctx,
		`UPDATE tags SET name = $2 WHERE id = $1
		 RETURNING id, deck_id, name, created_at`,
		id, name,
	).Scan(&tag.ID, &tag.DeckID, &tag.Name, &tag.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return tag, nil
}

func (r *TagRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, `DELETE FROM tags WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddTagToCard associates a tag with a card
func (r *TagRepository) AddTagToCard(ctx context.Context, cardID, tagID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO card_tags (card_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		cardID, tagID,
	)
	return err
}

// RemoveTagFromCard removes a tag association from a card
func (r *TagRepository) RemoveTagFromCard(ctx context.Context, cardID, tagID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM card_tags WHERE card_id = $1 AND tag_id = $2`,
		cardID, tagID,
	)
	return err
}

// GetTagsForCard returns all tags for a specific card
func (r *TagRepository) GetTagsForCard(ctx context.Context, cardID uuid.UUID) ([]model.Tag, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT t.id, t.deck_id, t.name, t.created_at
		 FROM tags t
		 JOIN card_tags ct ON t.id = ct.tag_id
		 WHERE ct.card_id = $1
		 ORDER BY t.name ASC`,
		cardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.DeckID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// SetCardTags replaces all tags for a card
func (r *TagRepository) SetCardTags(ctx context.Context, cardID uuid.UUID, tagIDs []uuid.UUID) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Remove existing tags
	_, err = tx.Exec(ctx, `DELETE FROM card_tags WHERE card_id = $1`, cardID)
	if err != nil {
		return err
	}

	// Add new tags
	for _, tagID := range tagIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO card_tags (card_id, tag_id) VALUES ($1, $2)`,
			cardID, tagID,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetTagsForCards returns tags for multiple cards (batch operation)
func (r *TagRepository) GetTagsForCards(ctx context.Context, cardIDs []uuid.UUID) (map[uuid.UUID][]model.Tag, error) {
	if len(cardIDs) == 0 {
		return make(map[uuid.UUID][]model.Tag), nil
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT ct.card_id, t.id, t.deck_id, t.name, t.created_at
		 FROM tags t
		 JOIN card_tags ct ON t.id = ct.tag_id
		 WHERE ct.card_id = ANY($1)
		 ORDER BY t.name ASC`,
		cardIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]model.Tag)
	for rows.Next() {
		var cardID uuid.UUID
		var t model.Tag
		if err := rows.Scan(&cardID, &t.ID, &t.DeckID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		result[cardID] = append(result[cardID], t)
	}
	return result, rows.Err()
}
