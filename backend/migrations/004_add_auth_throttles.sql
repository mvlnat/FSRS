CREATE TABLE IF NOT EXISTS auth_throttles (
    scope TEXT NOT NULL,
    throttle_key TEXT NOT NULL,
    attempt_count INT NOT NULL DEFAULT 0,
    window_started_at TIMESTAMPTZ NOT NULL,
    blocked_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (scope, throttle_key)
);

CREATE INDEX IF NOT EXISTS idx_auth_throttles_updated_at
ON auth_throttles(updated_at);

CREATE INDEX IF NOT EXISTS idx_auth_throttles_blocked_until
ON auth_throttles(blocked_until)
WHERE blocked_until IS NOT NULL;
