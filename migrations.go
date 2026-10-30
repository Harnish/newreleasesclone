package main

import (
	"database/sql"
	"fmt"
)

// migration is one schema step. Add new ones at the end with the next version;
// never edit or reorder released ones. Each runs in a single transaction
// together with its PRAGMA user_version bump, so a failure leaves no
// half-applied step.
type migration struct {
	version int
	// fkOff runs the step with foreign key enforcement off (table rebuilds).
	// PRAGMA foreign_keys is a no-op inside a transaction, so it is toggled
	// around the transaction instead.
	fkOff bool
	stmts []string
}

// Versions start at 2: anything older than v2 had a schema that v2 drops and
// recreates (projects/old releases tables).
var migrations = []migration{
	{version: 2, stmts: []string{
		`DROP TABLE IF EXISTS releases`,
		`DROP TABLE IF EXISTS projects`,
		`CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at    TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id),
			expires_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS repos (
			id            TEXT PRIMARY KEY,
			platform      TEXT NOT NULL,
			repo_url      TEXT NOT NULL,
			name          TEXT NOT NULL,
			last_refresh  TEXT NOT NULL DEFAULT '',
			refresh_count INTEGER NOT NULL DEFAULT 0,
			UNIQUE(platform, repo_url)
		)`,
		`CREATE TABLE IF NOT EXISTS user_repos (
			user_id    TEXT NOT NULL REFERENCES users(id),
			repo_id    TEXT NOT NULL REFERENCES repos(id),
			created_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (user_id, repo_id)
		)`,
		`CREATE TABLE IF NOT EXISTS releases (
			id            TEXT NOT NULL,
			repo_id       TEXT NOT NULL REFERENCES repos(id),
			name          TEXT NOT NULL,
			version       TEXT NOT NULL,
			platform      TEXT NOT NULL,
			url           TEXT NOT NULL,
			published_at  TEXT NOT NULL DEFAULT '',
			description   TEXT NOT NULL DEFAULT '',
			release_notes TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (id, repo_id)
		)`,
	}},
	{version: 3, stmts: []string{
		`CREATE TABLE IF NOT EXISTS config (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS push_subscriptions (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id),
			endpoint   TEXT NOT NULL,
			p256dh     TEXT NOT NULL,
			auth       TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT '',
			UNIQUE(user_id, endpoint)
		)`,
	}},
	{version: 4, stmts: []string{
		`CREATE TABLE IF NOT EXISTS webhooks (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id),
			repo_id    TEXT NOT NULL REFERENCES repos(id),
			url        TEXT NOT NULL,
			secret     TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT ''
		)`,
	}},
	{version: 5, stmts: []string{
		`ALTER TABLE user_repos ADD COLUMN push_enabled INTEGER NOT NULL DEFAULT 1`,
	}},
	{version: 6, stmts: []string{
		`ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN email_verified INTEGER NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS email_verification_tokens (
			token      TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL
		)`,
	}},
	{version: 7, stmts: []string{
		`ALTER TABLE users ADD COLUMN email_digest INTEGER NOT NULL DEFAULT 0`,
	}},
	{version: 8, stmts: []string{
		`ALTER TABLE users ADD COLUMN rss_token TEXT`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_rss_token ON users(rss_token) WHERE rss_token IS NOT NULL`,
	}},
	{version: 9, stmts: []string{
		`ALTER TABLE users ADD COLUMN page_size INTEGER NOT NULL DEFAULT 10`,
	}},
	{version: 10, fkOff: true, stmts: []string{
		// Rebuild users: email becomes the unique login, username goes away.
		`CREATE TABLE users_new (
			id             TEXT PRIMARY KEY,
			email          TEXT NOT NULL UNIQUE,
			password_hash  TEXT NOT NULL,
			created_at     TEXT NOT NULL DEFAULT '',
			email_verified INTEGER NOT NULL DEFAULT 0,
			email_digest   INTEGER NOT NULL DEFAULT 0,
			rss_token      TEXT,
			page_size      INTEGER NOT NULL DEFAULT 10
		)`,
		`INSERT INTO users_new (id, email, password_hash, created_at, email_verified, email_digest, rss_token, page_size)
			SELECT id, email, password_hash, created_at, email_verified, email_digest, rss_token, page_size
			FROM users`,
		`DROP TABLE users`,
		`ALTER TABLE users_new RENAME TO users`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_rss_token ON users(rss_token) WHERE rss_token IS NOT NULL`,
	}},
	{version: 11, stmts: []string{
		`CREATE TABLE IF NOT EXISTS gitlab_settings (
			user_id             TEXT PRIMARY KEY REFERENCES users(id),
			gitlab_url          TEXT NOT NULL DEFAULT '',
			gitlab_token        TEXT NOT NULL DEFAULT '',
			awesome_enabled     INTEGER NOT NULL DEFAULT 0,
			awesome_repo_name   TEXT NOT NULL DEFAULT '',
			awesome_gitlab_path TEXT NOT NULL DEFAULT '',
			updated_at          TEXT NOT NULL DEFAULT ''
		)`,
		`ALTER TABLE user_repos ADD COLUMN gitlab_sync_enabled INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE user_repos ADD COLUMN gitlab_sync_frequency TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE user_repos ADD COLUMN gitlab_project_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE user_repos ADD COLUMN last_gitlab_sync_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE user_repos ADD COLUMN last_gitlab_sync_error TEXT NOT NULL DEFAULT ''`,
	}},
	{version: 12, stmts: []string{
		`ALTER TABLE gitlab_settings ADD COLUMN gitlab_group TEXT NOT NULL DEFAULT ''`,
	}},
	{version: 13, stmts: []string{
		`ALTER TABLE user_repos ADD COLUMN email_immediate INTEGER NOT NULL DEFAULT 0`,
	}},
	{version: 14, stmts: []string{
		`ALTER TABLE releases ADD COLUMN is_prerelease INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE user_repos ADD COLUMN include_prereleases INTEGER NOT NULL DEFAULT 0`,
	}},
	{version: 15, stmts: []string{
		// One row per repo once an icon lookup has been attempted. data is NULL/empty
		// when the platform has no icon; checked_at throttles re-lookups.
		`CREATE TABLE repo_icons (
			repo_id      TEXT PRIMARY KEY REFERENCES repos(id),
			content_type TEXT NOT NULL DEFAULT '',
			data         BLOB,
			checked_at   INTEGER NOT NULL DEFAULT 0,
			fetched_at   INTEGER NOT NULL DEFAULT 0
		)`,
	}},
	{version: 16, stmts: []string{
		// Upstream one-line project description; description_checked_at (unix) throttles re-lookups.
		`ALTER TABLE repos ADD COLUMN description TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE repos ADD COLUMN description_checked_at INTEGER NOT NULL DEFAULT 0`,
	}},
}

func migrate(db *sql.DB) error { return runMigrations(db, migrations) }

func runMigrations(db *sql.DB, ms []migration) error {
	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("failed to read schema version: %w", err)
	}
	for _, m := range ms {
		if m.version <= current {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(db *sql.DB, m migration) (err error) {
	if m.fkOff {
		if _, err := db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
			return fmt.Errorf("failed to disable FK for v%d: %w", m.version, err)
		}
		defer func() {
			if _, ferr := db.Exec("PRAGMA foreign_keys=ON"); ferr != nil && err == nil {
				err = fmt.Errorf("failed to re-enable FK after v%d: %w", m.version, ferr)
			}
		}()
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin migration v%d: %w", m.version, err)
	}
	for _, stmt := range m.stmts {
		if _, err := tx.Exec(stmt); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to migrate to v%d: %w (statement: %.60q)", m.version, err, stmt)
		}
	}
	// user_version is part of the transaction, so it bumps only if every statement applied.
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to set schema version v%d: %w", m.version, err)
	}
	return tx.Commit()
}
