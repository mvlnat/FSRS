package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ziyangli/fsrs/backend/internal/model"
)

var ErrNotFound = errors.New("not found")
var ErrDuplicate = errors.New("duplicate entry")
var ErrCardNotDue = errors.New("card not due")
var ErrInvalidInput = errors.New("invalid input")
var ErrForbidden = errors.New("forbidden")

type UserRepository struct {
	db *DB
}

func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, email, passwordHash string) (*model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(BTRIM(email)) = LOWER(BTRIM($1)))`,
		email,
	).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicate
	}

	user := &model.User{}
	err = r.db.Pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2)
		 RETURNING id, email, password_hash, token_version, email_verified_at, created_at`,
		email, passwordHash,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.TokenVersion, &user.EmailVerifiedAt, &user.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user := &model.User{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, token_version, email_verified_at, created_at
		 FROM users
		 WHERE LOWER(BTRIM(email)) = LOWER(BTRIM($1))`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.TokenVersion, &user.EmailVerifiedAt, &user.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	user := &model.User{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, email, password_hash, token_version, email_verified_at, created_at
		 FROM users
		 WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.TokenVersion, &user.EmailVerifiedAt, &user.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) IncrementTokenVersion(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, `
		UPDATE users
		SET token_version = token_version + 1
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) MarkEmailVerified(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Pool.Exec(ctx, `
		UPDATE users
		SET email_verified_at = COALESCE(email_verified_at, NOW())
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) ResetPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	result, err := r.db.Pool.Exec(ctx, `
		UPDATE users
		SET password_hash = $2,
		    token_version = token_version + 1,
		    email_verified_at = COALESCE(email_verified_at, NOW())
		WHERE id = $1
	`, id, passwordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
