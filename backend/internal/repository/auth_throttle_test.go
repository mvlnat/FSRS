package repository

import (
	"testing"
	"time"
)

func TestNextAuthThrottleState_AllowsWithinLimit(t *testing.T) {
	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	state := authThrottleState{
		AttemptCount:    1,
		WindowStartedAt: now.Add(-time.Minute),
	}

	nextState, allowed, retryAfter := nextAuthThrottleState(now, state, 3, 5*time.Minute, 15*time.Minute)

	if !allowed {
		t.Fatal("expected request to be allowed")
	}
	if retryAfter != 0 {
		t.Fatalf("retryAfter = %s, want 0", retryAfter)
	}
	if nextState.AttemptCount != 2 {
		t.Fatalf("AttemptCount = %d, want 2", nextState.AttemptCount)
	}
	if nextState.BlockedUntil != nil {
		t.Fatalf("BlockedUntil = %v, want nil", nextState.BlockedUntil)
	}
}

func TestNextAuthThrottleState_BlocksAfterLimitExceeded(t *testing.T) {
	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	state := authThrottleState{
		AttemptCount:    3,
		WindowStartedAt: now.Add(-time.Minute),
	}

	nextState, allowed, retryAfter := nextAuthThrottleState(now, state, 3, 5*time.Minute, 15*time.Minute)

	if allowed {
		t.Fatal("expected request to be blocked")
	}
	if retryAfter != 15*time.Minute {
		t.Fatalf("retryAfter = %s, want %s", retryAfter, 15*time.Minute)
	}
	if nextState.AttemptCount != 4 {
		t.Fatalf("AttemptCount = %d, want 4", nextState.AttemptCount)
	}
	if nextState.BlockedUntil == nil {
		t.Fatal("expected BlockedUntil to be set")
	}
	if got := nextState.BlockedUntil.Sub(now); got != 15*time.Minute {
		t.Fatalf("blocked duration = %s, want %s", got, 15*time.Minute)
	}
}

func TestNextAuthThrottleState_ReturnsExistingBlock(t *testing.T) {
	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	blockedUntil := now.Add(2 * time.Minute)
	state := authThrottleState{
		AttemptCount:    4,
		WindowStartedAt: now.Add(-time.Minute),
		BlockedUntil:    &blockedUntil,
	}

	nextState, allowed, retryAfter := nextAuthThrottleState(now, state, 3, 5*time.Minute, 15*time.Minute)

	if allowed {
		t.Fatal("expected request to remain blocked")
	}
	if retryAfter != 2*time.Minute {
		t.Fatalf("retryAfter = %s, want %s", retryAfter, 2*time.Minute)
	}
	if nextState.AttemptCount != state.AttemptCount {
		t.Fatalf("AttemptCount = %d, want %d", nextState.AttemptCount, state.AttemptCount)
	}
	if nextState.BlockedUntil == nil || !nextState.BlockedUntil.Equal(blockedUntil) {
		t.Fatalf("BlockedUntil = %v, want %v", nextState.BlockedUntil, blockedUntil)
	}
}

func TestNextAuthThrottleState_ResetsAfterWindowExpires(t *testing.T) {
	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	oldBlock := now.Add(-time.Minute)
	state := authThrottleState{
		AttemptCount:    99,
		WindowStartedAt: now.Add(-10 * time.Minute),
		BlockedUntil:    &oldBlock,
	}

	nextState, allowed, retryAfter := nextAuthThrottleState(now, state, 3, 5*time.Minute, 15*time.Minute)

	if !allowed {
		t.Fatal("expected request to be allowed after window reset")
	}
	if retryAfter != 0 {
		t.Fatalf("retryAfter = %s, want 0", retryAfter)
	}
	if nextState.AttemptCount != 1 {
		t.Fatalf("AttemptCount = %d, want 1", nextState.AttemptCount)
	}
	if !nextState.WindowStartedAt.Equal(now) {
		t.Fatalf("WindowStartedAt = %s, want %s", nextState.WindowStartedAt, now)
	}
	if nextState.BlockedUntil != nil {
		t.Fatalf("BlockedUntil = %v, want nil", nextState.BlockedUntil)
	}
}
