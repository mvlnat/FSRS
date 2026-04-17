package repository

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type AuthThrottleRepository struct {
	db *DB
}

type authThrottleState struct {
	AttemptCount    int
	WindowStartedAt time.Time
	BlockedUntil    *time.Time
}

func NewAuthThrottleRepository(db *DB) *AuthThrottleRepository {
	return &AuthThrottleRepository{db: db}
}

func (r *AuthThrottleRepository) Allow(
	ctx context.Context,
	scope string,
	key string,
	limit int,
	window time.Duration,
	blockDuration time.Duration,
) (bool, time.Duration, error) {
	scope = strings.TrimSpace(scope)
	key = strings.TrimSpace(key)
	if scope == "" || key == "" {
		return false, 0, fmt.Errorf("scope and key are required")
	}
	if limit <= 0 {
		return false, 0, fmt.Errorf("limit must be positive")
	}
	if window <= 0 || blockDuration <= 0 {
		return false, 0, fmt.Errorf("window and block duration must be positive")
	}

	now := time.Now().UTC()

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_throttles (scope, throttle_key, attempt_count, window_started_at, updated_at)
		VALUES ($1, $2, 0, $3, $3)
		ON CONFLICT (scope, throttle_key) DO NOTHING
	`, scope, key, now); err != nil {
		return false, 0, err
	}

	var state authThrottleState
	if err := tx.QueryRow(ctx, `
		SELECT attempt_count, window_started_at, blocked_until
		FROM auth_throttles
		WHERE scope = $1 AND throttle_key = $2
		FOR UPDATE
	`, scope, key).Scan(&state.AttemptCount, &state.WindowStartedAt, &state.BlockedUntil); err != nil {
		return false, 0, err
	}

	nextState, allowed, retryAfter := nextAuthThrottleState(now, state, limit, window, blockDuration)

	if _, err := tx.Exec(ctx, `
		UPDATE auth_throttles
		SET attempt_count = $3,
		    window_started_at = $4,
		    blocked_until = $5,
		    updated_at = $6
		WHERE scope = $1 AND throttle_key = $2
	`, scope, key, nextState.AttemptCount, nextState.WindowStartedAt, nextState.BlockedUntil, now); err != nil {
		return false, 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, 0, err
	}

	return allowed, retryAfter, nil
}

func (r *AuthThrottleRepository) Reset(ctx context.Context, scope string, key string) error {
	scope = strings.TrimSpace(scope)
	key = strings.TrimSpace(key)
	if scope == "" || key == "" {
		return nil
	}

	_, err := r.db.Pool.Exec(ctx, `
		DELETE FROM auth_throttles
		WHERE scope = $1 AND throttle_key = $2
	`, scope, key)
	return err
}

func nextAuthThrottleState(
	now time.Time,
	state authThrottleState,
	limit int,
	window time.Duration,
	blockDuration time.Duration,
) (authThrottleState, bool, time.Duration) {
	if state.BlockedUntil != nil && state.BlockedUntil.After(now) {
		return state, false, state.BlockedUntil.Sub(now)
	}

	if now.Sub(state.WindowStartedAt) >= window {
		state.AttemptCount = 0
		state.WindowStartedAt = now
		state.BlockedUntil = nil
	}

	state.AttemptCount++

	if state.AttemptCount > limit {
		blockedUntil := now.Add(blockDuration)
		state.BlockedUntil = &blockedUntil
		return state, false, blockDuration
	}

	return state, true, 0
}
