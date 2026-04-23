package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AuthEmailTokenPurpose string

const (
	AuthEmailTokenPurposeEmailVerification AuthEmailTokenPurpose = "email_verification"
	AuthEmailTokenPurposePasswordReset     AuthEmailTokenPurpose = "password_reset"
)

type AuthEmailTokenRepository struct {
	db *DB
}

func NewAuthEmailTokenRepository(db *DB) *AuthEmailTokenRepository {
	return &AuthEmailTokenRepository{db: db}
}

func (r *AuthEmailTokenRepository) Create(
	ctx context.Context,
	userID uuid.UUID,
	purpose AuthEmailTokenPurpose,
	tokenHash string,
	expiresAt time.Time,
) error {
	tokenHash = strings.TrimSpace(tokenHash)
	if userID == uuid.Nil || purpose == "" || tokenHash == "" || expiresAt.IsZero() {
		return ErrInvalidInput
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		DELETE FROM auth_email_tokens
		WHERE user_id = $1 AND purpose = $2 AND consumed_at IS NULL
	`, userID, purpose); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_email_tokens (user_id, purpose, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, userID, purpose, tokenHash, expiresAt.UTC()); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *AuthEmailTokenRepository) Consume(
	ctx context.Context,
	purpose AuthEmailTokenPurpose,
	tokenHash string,
	now time.Time,
) (uuid.UUID, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	if purpose == "" || tokenHash == "" {
		return uuid.Nil, ErrInvalidInput
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	var (
		userID     uuid.UUID
		expiresAt  time.Time
		consumedAt *time.Time
	)

	err = tx.QueryRow(ctx, `
		SELECT user_id, expires_at, consumed_at
		FROM auth_email_tokens
		WHERE purpose = $1 AND token_hash = $2
		FOR UPDATE
	`, purpose, tokenHash).Scan(&userID, &expiresAt, &consumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}

	if consumedAt != nil || !expiresAt.After(now.UTC()) {
		return uuid.Nil, ErrNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE auth_email_tokens
		SET consumed_at = $3
		WHERE purpose = $1 AND token_hash = $2
	`, purpose, tokenHash, now.UTC()); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}

	return userID, nil
}
