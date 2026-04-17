package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/ziyangli/fsrs/backend/internal/repository"
	dbmigrations "github.com/ziyangli/fsrs/backend/migrations"
)

const (
	migrationsTableName                = "schema_migrations"
	internalCanonicalEmailMigrationKey = "internal:canonicalize_user_emails"
)

func RunMigrations(ctx context.Context, db *repository.DB) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}

	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return err
	}

	scripts, err := dbmigrations.OrderedScripts()
	if err != nil {
		return err
	}

	for _, script := range scripts {
		if _, ok := applied[script.Name]; ok {
			continue
		}

		if err := applyNamedMigration(ctx, db, script.Name, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, script.SQL)
			return err
		}); err != nil {
			return fmt.Errorf("apply migration %s: %w", script.Name, err)
		}
	}

	if _, ok := applied[internalCanonicalEmailMigrationKey]; ok {
		return nil
	}

	if err := applyNamedMigration(ctx, db, internalCanonicalEmailMigrationKey, func(tx pgx.Tx) error {
		return ensureCanonicalUserEmails(ctx, tx)
	}); err != nil {
		return fmt.Errorf("apply migration %s: %w", internalCanonicalEmailMigrationKey, err)
	}

	return nil
}

func ensureMigrationTable(ctx context.Context, db *repository.DB) error {
	_, err := db.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func loadAppliedMigrations(ctx context.Context, db *repository.DB) (map[string]struct{}, error) {
	rows, err := db.Pool.Query(ctx, `SELECT name FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = struct{}{}
	}

	return applied, rows.Err()
}

func applyNamedMigration(ctx context.Context, db *repository.DB, name string, migrationFn func(tx pgx.Tx) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `LOCK TABLE schema_migrations IN ACCESS EXCLUSIVE MODE`); err != nil {
		return err
	}

	var alreadyApplied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&alreadyApplied); err != nil {
		return err
	}
	if alreadyApplied {
		return tx.Commit(ctx)
	}

	if err := migrationFn(tx); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func ensureCanonicalUserEmails(ctx context.Context, tx pgx.Tx) error {
	var canonicalEmail string
	var conflictingEmails string
	err := tx.QueryRow(ctx, `
		SELECT
			LOWER(BTRIM(email)) AS canonical_email,
			STRING_AGG(email, ', ' ORDER BY created_at) AS conflicting_emails
		FROM users
		GROUP BY LOWER(BTRIM(email))
		HAVING COUNT(*) > 1
		LIMIT 1
	`).Scan(&canonicalEmail, &conflictingEmails)
	if err == nil {
		return fmt.Errorf(
			"duplicate user emails must be resolved before startup: canonical email %q maps to multiple rows (%s)",
			canonicalEmail,
			conflictingEmails,
		)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET email = LOWER(BTRIM(email))
		WHERE email <> LOWER(BTRIM(email))
	`); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_canonical
		ON users ((LOWER(BTRIM(email))))
	`); err != nil {
		return err
	}

	return nil
}
