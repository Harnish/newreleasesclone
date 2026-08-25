package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var store *Store

const releaseLimit = 50

type Store struct {
	db        *sql.DB
	vapidPub  string
	vapidPriv string
}

type pushSub struct {
	UserID   string
	Endpoint string
	P256dh   string
	Auth     string
}

func NewStore(dbPath string) (*Store, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("failed to apply %s: %w", pragma, err)
		}
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	pub, priv, err := getOrCreateVAPIDKeys(db)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize VAPID keys: %w", err)
	}

	return &Store{db: db, vapidPub: pub, vapidPriv: priv}, nil
}

func migrate(db *sql.DB) error {
	var version int
	db.QueryRow("PRAGMA user_version").Scan(&version)

	if version < 2 {
		// Drop v1 schema (projects + old releases) so we can recreate cleanly.
		db.Exec("DROP TABLE IF EXISTS releases")
		db.Exec("DROP TABLE IF EXISTS projects")

		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS users (
				id            TEXT PRIMARY KEY,
				username      TEXT NOT NULL UNIQUE,
				password_hash TEXT NOT NULL,
				created_at    TEXT NOT NULL DEFAULT ''
			);
			CREATE TABLE IF NOT EXISTS sessions (
				id         TEXT PRIMARY KEY,
				user_id    TEXT NOT NULL REFERENCES users(id),
				expires_at TEXT NOT NULL
			);
			CREATE TABLE IF NOT EXISTS repos (
				id            TEXT PRIMARY KEY,
				platform      TEXT NOT NULL,
				repo_url      TEXT NOT NULL,
				name          TEXT NOT NULL,
				last_refresh  TEXT NOT NULL DEFAULT '',
				refresh_count INTEGER NOT NULL DEFAULT 0,
				UNIQUE(platform, repo_url)
			);
			CREATE TABLE IF NOT EXISTS user_repos (
				user_id    TEXT NOT NULL REFERENCES users(id),
				repo_id    TEXT NOT NULL REFERENCES repos(id),
				created_at TEXT NOT NULL DEFAULT '',
				PRIMARY KEY (user_id, repo_id)
			);
			CREATE TABLE IF NOT EXISTS releases (
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
			);
		`); err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}

		if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	if version < 3 {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS config (
				key   TEXT PRIMARY KEY,
				value TEXT NOT NULL
			);
			CREATE TABLE IF NOT EXISTS push_subscriptions (
				id         TEXT PRIMARY KEY,
				user_id    TEXT NOT NULL REFERENCES users(id),
				endpoint   TEXT NOT NULL,
				p256dh     TEXT NOT NULL,
				auth       TEXT NOT NULL,
				created_at TEXT NOT NULL DEFAULT '',
				UNIQUE(user_id, endpoint)
			);
		`); err != nil {
			return fmt.Errorf("failed to migrate to v3: %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 3"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	if version < 4 {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS webhooks (
				id         TEXT PRIMARY KEY,
				user_id    TEXT NOT NULL REFERENCES users(id),
				repo_id    TEXT NOT NULL REFERENCES repos(id),
				url        TEXT NOT NULL,
				secret     TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT ''
			);
		`); err != nil {
			return fmt.Errorf("failed to migrate to v4: %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 4"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	if version < 5 {
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN push_enabled INTEGER NOT NULL DEFAULT 1`); err != nil {
			return fmt.Errorf("failed to migrate to v5: %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 5"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	if version < 6 {
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to migrate to v6 (email): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN email_verified INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("failed to migrate to v6 (email_verified): %w", err)
		}
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS email_verification_tokens (
				token      TEXT PRIMARY KEY,
				user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				expires_at TEXT NOT NULL
			)
		`); err != nil {
			return fmt.Errorf("failed to migrate to v6 (tokens table): %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 6"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	if version < 7 {
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN email_digest INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("failed to migrate to v7 (email_digest): %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 7"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	if version < 8 {
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN rss_token TEXT`); err != nil {
			return fmt.Errorf("failed to migrate to v8 (rss_token): %w", err)
		}
		if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_rss_token ON users(rss_token) WHERE rss_token IS NOT NULL`); err != nil {
			return fmt.Errorf("failed to create rss_token index: %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 8"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	if version < 9 {
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN page_size INTEGER NOT NULL DEFAULT 10`); err != nil {
			return fmt.Errorf("failed to migrate to v9 (page_size): %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 9"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	if version < 10 {
		if _, err := db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
			return fmt.Errorf("failed to disable FK for v10: %w", err)
		}
		if _, err := db.Exec(`
			CREATE TABLE users_new (
				id             TEXT PRIMARY KEY,
				email          TEXT NOT NULL UNIQUE,
				password_hash  TEXT NOT NULL,
				created_at     TEXT NOT NULL DEFAULT '',
				email_verified INTEGER NOT NULL DEFAULT 0,
				email_digest   INTEGER NOT NULL DEFAULT 0,
				rss_token      TEXT,
				page_size      INTEGER NOT NULL DEFAULT 10
			)
		`); err != nil {
			return fmt.Errorf("failed to migrate to v10 (create users_new): %w", err)
		}
		if _, err := db.Exec(`
			INSERT INTO users_new (id, email, password_hash, created_at, email_verified, email_digest, rss_token, page_size)
			SELECT id, email, password_hash, created_at, email_verified, email_digest, rss_token, page_size
			FROM users
		`); err != nil {
			return fmt.Errorf("failed to migrate to v10 (copy users): %w", err)
		}
		if _, err := db.Exec(`DROP TABLE users`); err != nil {
			return fmt.Errorf("failed to migrate to v10 (drop users): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE users_new RENAME TO users`); err != nil {
			return fmt.Errorf("failed to migrate to v10 (rename users_new): %w", err)
		}
		if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
			return fmt.Errorf("failed to re-enable FK for v10: %w", err)
		}
		if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_rss_token ON users(rss_token) WHERE rss_token IS NOT NULL`); err != nil {
			return fmt.Errorf("failed to migrate to v10 (rss_token index): %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 10"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	if version < 11 {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS gitlab_settings (
				user_id             TEXT PRIMARY KEY REFERENCES users(id),
				gitlab_url          TEXT NOT NULL DEFAULT '',
				gitlab_token        TEXT NOT NULL DEFAULT '',
				awesome_enabled     INTEGER NOT NULL DEFAULT 0,
				awesome_repo_name   TEXT NOT NULL DEFAULT '',
				awesome_gitlab_path TEXT NOT NULL DEFAULT '',
				updated_at          TEXT NOT NULL DEFAULT ''
			);
		`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (gitlab_settings table): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN gitlab_sync_enabled INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (gitlab_sync_enabled): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN gitlab_sync_frequency TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (gitlab_sync_frequency): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN gitlab_project_path TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (gitlab_project_path): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN last_gitlab_sync_at TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (last_gitlab_sync_at): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN last_gitlab_sync_error TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (last_gitlab_sync_error): %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 11"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	if version < 12 {
		if _, err := db.Exec(`ALTER TABLE gitlab_settings ADD COLUMN gitlab_group TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to migrate to v12 (gitlab_group): %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 12"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	if version < 13 {
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN email_immediate INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("failed to migrate to v13 (email_immediate): %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 13"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}

	return nil
}

// ---- VAPID / Push ----

func getOrCreateVAPIDKeys(db *sql.DB) (pub, priv string, err error) {
	db.QueryRow("SELECT value FROM config WHERE key='vapid_public'").Scan(&pub)
	db.QueryRow("SELECT value FROM config WHERE key='vapid_private'").Scan(&priv)
	if pub != "" && priv != "" {
		return pub, priv, nil
	}
	priv, pub, err = webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", err
	}
	db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES ('vapid_public', ?)", pub)
	db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES ('vapid_private', ?)", priv)
	return pub, priv, nil
}

func (s *Store) GetVAPIDPublicKey() string { return s.vapidPub }

func (s *Store) SavePushSubscription(userID, endpoint, p256dh, auth string) error {
	id := fmt.Sprintf("push_%s", generateToken()[:12])
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, endpoint, p256dh, auth, time.Now().Format(time.RFC3339))
	return err
}

func (s *Store) DeletePushSubscription(userID, endpoint string) {
	s.db.Exec(`DELETE FROM push_subscriptions WHERE user_id = ? AND endpoint = ?`, userID, endpoint)
}

func (s *Store) getPushSubscriptionsForRepo(repoID string) []pushSub {
	rows, err := s.db.Query(`
		SELECT ps.user_id, ps.endpoint, ps.p256dh, ps.auth
		FROM push_subscriptions ps
		INNER JOIN user_repos ur ON ur.user_id = ps.user_id
		WHERE ur.repo_id = ? AND ur.push_enabled = 1`, repoID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var subs []pushSub
	for rows.Next() {
		var sub pushSub
		rows.Scan(&sub.UserID, &sub.Endpoint, &sub.P256dh, &sub.Auth)
		subs = append(subs, sub)
	}
	return subs
}

func (s *Store) sendPushNotifications(repoID, projectName string, newReleases []Release) {
	subs := s.getPushSubscriptionsForRepo(repoID)
	if len(subs) == 0 {
		return
	}

	latest := newReleases[0]
	var body string
	if len(newReleases) == 1 {
		body = latest.Version + " is now available"
	} else {
		body = fmt.Sprintf("%d new releases, latest %s", len(newReleases), latest.Version)
	}
	payload, _ := json.Marshal(map[string]string{
		"title": projectName + " — new release",
		"body":  body,
		"url":   latest.URL,
	})

	for _, sub := range subs {
		resp, err := webpush.SendNotification(payload, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys:     webpush.Keys{Auth: sub.Auth, P256dh: sub.P256dh},
		}, &webpush.Options{
			Subscriber:      "mailto:noreply@localhost",
			VAPIDPublicKey:  s.vapidPub,
			VAPIDPrivateKey: s.vapidPriv,
			TTL:             86400,
		})
		if err != nil {
			log.Printf("⚠ Push failed for %.20s: %v", sub.Endpoint, err)
			continue
		}
		resp.Body.Close()
		// 410 Gone or 404 = subscription expired/invalid — remove it
		if resp.StatusCode == 410 || resp.StatusCode == 404 {
			s.DeletePushSubscription(sub.UserID, sub.Endpoint)
		}
	}
}

// ---- Webhooks ----

func (s *Store) AddWebhook(userID, repoID, webhookURL, secret string) (string, error) {
	id := fmt.Sprintf("wh_%s", generateToken()[:12])
	_, err := s.db.Exec(`
		INSERT INTO webhooks (id, user_id, repo_id, url, secret, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, repoID, webhookURL, secret, time.Now().Format(time.RFC3339))
	return id, err
}

func (s *Store) DeleteWebhook(userID, webhookID string) bool {
	res, err := s.db.Exec(`DELETE FROM webhooks WHERE id = ? AND user_id = ?`, webhookID, userID)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

func (s *Store) GetUserWebhooksForRepo(userID, repoID string) []Webhook {
	rows, err := s.db.Query(`
		SELECT id, repo_id, url, secret FROM webhooks
		WHERE user_id = ? AND repo_id = ? ORDER BY created_at ASC`, userID, repoID)
	if err != nil {
		return []Webhook{}
	}
	defer rows.Close()
	return scanWebhooks(rows)
}

func (s *Store) SetProjectPushEnabled(userID, repoID string, enabled bool) (bool, error) {
	val := 0
	if enabled {
		val = 1
	}
	res, err := s.db.Exec(
		"UPDATE user_repos SET push_enabled = ? WHERE user_id = ? AND repo_id = ?",
		val, userID, repoID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) SetProjectEmailImmediate(userID, repoID string, enabled bool) (bool, error) {
	val := 0
	if enabled {
		val = 1
	}
	res, err := s.db.Exec(
		"UPDATE user_repos SET email_immediate = ? WHERE user_id = ? AND repo_id = ?",
		val, userID, repoID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) getImmediateEmailUsersForRepo(repoID string) []User {
	rows, err := s.db.Query(`
		SELECT u.id, u.email, u.email_verified
		FROM users u
		INNER JOIN user_repos ur ON ur.user_id = u.id
		WHERE ur.repo_id = ?
		  AND ur.email_immediate = 1
		  AND u.email_verified = 1
		  AND u.email != ''`, repoID)
	if err != nil {
		log.Printf("⚠ getImmediateEmailUsersForRepo query failed: %v", err)
		return nil
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		var verified int
		if err := rows.Scan(&u.ID, &u.Email, &verified); err != nil {
			log.Printf("⚠ failed to scan user: %v", err)
			continue
		}
		u.EmailVerified = verified != 0
		users = append(users, u)
	}
	return users
}

func (s *Store) sendImmediateEmails(repoID string, project Project, newReleases []Release) {
	for _, release := range newReleases {
		users := s.getImmediateEmailUsersForRepo(repoID)
		if users == nil {
			log.Printf("⚠ sendImmediateEmails: skipping release %s (query failed)", release.Version)
			continue
		}
		for _, u := range users {
			if err := smtpCfg.SendReleaseEmail(u.Email, project, release); err != nil {
				log.Printf("⚠ sendImmediateEmails: failed to send to %s: %v", u.Email, err)
			}
		}
	}
}

// ---- GitLab sync ----

func (s *Store) SaveGitLabSettings(userID, gitlabURL, token, group string) error {
	_, err := s.db.Exec(`
		INSERT INTO gitlab_settings (user_id, gitlab_url, gitlab_token, gitlab_group, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			gitlab_url = excluded.gitlab_url,
			gitlab_token = excluded.gitlab_token,
			gitlab_group = excluded.gitlab_group,
			updated_at = excluded.updated_at`,
		userID, gitlabURL, token, group, time.Now().Format(time.RFC3339))
	return err
}

func (s *Store) GetGitLabSettings(userID string) (GitLabSettings, error) {
	var g GitLabSettings
	var awesomeEnabled int
	err := s.db.QueryRow(`
		SELECT gitlab_url, gitlab_token, gitlab_group, awesome_enabled, awesome_repo_name, awesome_gitlab_path
		FROM gitlab_settings WHERE user_id = ?`, userID,
	).Scan(&g.GitLabURL, &g.GitLabToken, &g.GitLabGroup, &awesomeEnabled, &g.AwesomeRepoName, &g.AwesomeGitLabPath)
	if err == sql.ErrNoRows {
		return GitLabSettings{}, nil
	}
	if err != nil {
		return GitLabSettings{}, err
	}
	g.AwesomeEnabled = awesomeEnabled != 0
	return g, nil
}

// SetAwesomeConfig requires a gitlab_settings row to already exist (the user
// must save their GitLab URL/token before enabling the Awesome page).
// Clears awesome_gitlab_path so a renamed repo gets recreated rather than
// reusing a stale project path.
func (s *Store) SetAwesomeConfig(userID, repoName string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	res, err := s.db.Exec(`
		UPDATE gitlab_settings
		SET awesome_enabled = ?, awesome_repo_name = ?, awesome_gitlab_path = ''
		WHERE user_id = ?`,
		val, repoName, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("configure your gitlab instance before enabling the awesome page")
	}
	return nil
}

func (s *Store) SetAwesomeGitLabPath(userID, httpURL string) error {
	_, err := s.db.Exec(`UPDATE gitlab_settings SET awesome_gitlab_path = ? WHERE user_id = ?`, httpURL, userID)
	return err
}

// SetProjectGitLabSync enables/disables GitLab mirror sync for a project.
// Enabling requires the repo's platform be github or gitlab (the only
// platforms with a git-clonable repo_url). Disabling clears the stored
// frequency so a stale value can't resurface on a future re-enable that
// omits it.
func (s *Store) SetProjectGitLabSync(userID, repoID string, enabled bool, frequency string) (bool, error) {
	if enabled {
		var platform string
		if err := s.db.QueryRow("SELECT platform FROM repos WHERE id = ?", repoID).Scan(&platform); err != nil {
			return false, fmt.Errorf("repo not found: %w", err)
		}
		if platform != "github" && platform != "gitlab" {
			return false, fmt.Errorf("gitlab sync is only supported for github and gitlab projects")
		}
	}
	enabledVal := 0
	freqVal := frequency
	if enabled {
		enabledVal = 1
	} else {
		freqVal = ""
	}
	res, err := s.db.Exec(
		"UPDATE user_repos SET gitlab_sync_enabled = ?, gitlab_sync_frequency = ? WHERE user_id = ? AND repo_id = ?",
		enabledVal, freqVal, userID, repoID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) SetProjectGitLabPath(userID, repoID, httpURL string) error {
	_, err := s.db.Exec(
		"UPDATE user_repos SET gitlab_project_path = ? WHERE user_id = ? AND repo_id = ?",
		httpURL, userID, repoID,
	)
	return err
}

func (s *Store) RecordGitLabSync(userID, repoID string, syncErr error) error {
	errMsg := ""
	if syncErr != nil {
		errMsg = syncErr.Error()
	}
	_, err := s.db.Exec(`
		UPDATE user_repos
		SET last_gitlab_sync_at = ?, last_gitlab_sync_error = ?
		WHERE user_id = ? AND repo_id = ?`,
		time.Now().Format(time.RFC3339), errMsg, userID, repoID)
	return err
}

// GetAllEnabledGitLabSyncTargets returns every user's GitLab-sync-enabled
// projects, across all users. Requires the user to have saved gitlab_settings
// (inner join) — a project enabled before configuring GitLab simply won't
// appear here until settings are saved.
func (s *Store) GetAllEnabledGitLabSyncTargets() []GitLabSyncTarget {
	rows, err := s.db.Query(`
		SELECT ur.user_id, r.id, r.name, r.repo_url, r.platform,
		       gs.gitlab_url, gs.gitlab_token, gs.gitlab_group,
		       ur.gitlab_project_path, ur.gitlab_sync_frequency, ur.last_gitlab_sync_at
		FROM user_repos ur
		INNER JOIN repos r ON r.id = ur.repo_id
		INNER JOIN gitlab_settings gs ON gs.user_id = ur.user_id
		WHERE ur.gitlab_sync_enabled = 1`)
	if err != nil {
		log.Printf("⚠ Failed to query gitlab sync targets: %v", err)
		return nil
	}
	defer rows.Close()
	return scanGitLabSyncTargets(rows)
}

func (s *Store) GetUserGitLabSyncTargets(userID string) []GitLabSyncTarget {
	rows, err := s.db.Query(`
		SELECT ur.user_id, r.id, r.name, r.repo_url, r.platform,
		       gs.gitlab_url, gs.gitlab_token, gs.gitlab_group,
		       ur.gitlab_project_path, ur.gitlab_sync_frequency, ur.last_gitlab_sync_at
		FROM user_repos ur
		INNER JOIN repos r ON r.id = ur.repo_id
		INNER JOIN gitlab_settings gs ON gs.user_id = ur.user_id
		WHERE ur.gitlab_sync_enabled = 1 AND ur.user_id = ?`, userID)
	if err != nil {
		log.Printf("⚠ Failed to query user gitlab sync targets: %v", err)
		return nil
	}
	defer rows.Close()
	return scanGitLabSyncTargets(rows)
}

func scanGitLabSyncTargets(rows *sql.Rows) []GitLabSyncTarget {
	var targets []GitLabSyncTarget
	for rows.Next() {
		var t GitLabSyncTarget
		var lastSync string
		if err := rows.Scan(&t.UserID, &t.RepoID, &t.RepoName, &t.RepoURL, &t.Platform,
			&t.GitLabURL, &t.GitLabToken, &t.GitLabGroup, &t.GitLabProjectPath, &t.Frequency, &lastSync); err != nil {
			log.Printf("⚠ Failed to scan gitlab sync target: %v", err)
			continue
		}
		if lastSync != "" {
			t.LastSyncAt, _ = time.Parse(time.RFC3339, lastSync)
		}
		targets = append(targets, t)
	}
	return targets
}

func (s *Store) getWebhooksForRepo(repoID string) []struct {
	wh     Webhook
	secret string
} {
	rows, err := s.db.Query(`SELECT id, repo_id, url, secret FROM webhooks WHERE repo_id = ?`, repoID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []struct {
		wh     Webhook
		secret string
	}
	for rows.Next() {
		var id, rid, u, sec string
		if err := rows.Scan(&id, &rid, &u, &sec); err != nil {
			continue
		}
		results = append(results, struct {
			wh     Webhook
			secret string
		}{Webhook{ID: id, RepoID: rid, URL: u, HasSecret: sec != ""}, sec})
	}
	return results
}

func scanWebhooks(rows *sql.Rows) []Webhook {
	var hooks []Webhook
	for rows.Next() {
		var id, repoID, u, secret string
		if err := rows.Scan(&id, &repoID, &u, &secret); err != nil {
			continue
		}
		hooks = append(hooks, Webhook{ID: id, RepoID: repoID, URL: u, HasSecret: secret != ""})
	}
	if hooks == nil {
		return []Webhook{}
	}
	return hooks
}

func (s *Store) sendWebhooks(repoID string, project Project, newReleases []Release) {
	hooks := s.getWebhooksForRepo(repoID)
	if len(hooks) == 0 {
		return
	}

	type releasePayload struct {
		Version     string `json:"version"`
		URL         string `json:"url"`
		PublishedAt string `json:"published_at"`
		Description string `json:"description,omitempty"`
	}
	relPayloads := make([]releasePayload, len(newReleases))
	for i, r := range newReleases {
		relPayloads[i] = releasePayload{
			Version:     r.Version,
			URL:         r.URL,
			PublishedAt: r.PublishedAt.Format(time.RFC3339),
			Description: r.Description,
		}
	}
	body, _ := json.Marshal(map[string]interface{}{
		"project": map[string]string{
			"id":       project.ID,
			"name":     project.Name,
			"platform": project.Platform,
			"repo_url": project.RepoURL,
		},
		"releases": relPayloads,
	})

	client := &http.Client{Timeout: 10 * time.Second}
	for _, entry := range hooks {
		req, err := http.NewRequest(http.MethodPost, entry.wh.URL, bytes.NewReader(body))
		if err != nil {
			log.Printf("⚠ Webhook bad URL %s: %v", entry.wh.URL, err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "release-tracker/1.0")
		if entry.secret != "" {
			mac := hmac.New(sha256.New, []byte(entry.secret))
			mac.Write(body)
			req.Header.Set("X-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("⚠ Webhook delivery failed %s: %v", entry.wh.URL, err)
			continue
		}
		resp.Body.Close()
		log.Printf("✓ Webhook delivered to %s (status %d)", entry.wh.URL, resp.StatusCode)
	}
}

// ---- Helpers ----

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func normalizeURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return u
}

// ---- Auth ----

func (s *Store) CreateUser(email, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("user_%s", generateToken()[:12])
	token := generateToken()
	_, err = s.db.Exec(
		"INSERT INTO users (id, email, password_hash, created_at, rss_token) VALUES (?, ?, ?, ?, ?)",
		id, email, string(hash), time.Now().Format(time.RFC3339), token,
	)
	if err != nil {
		return nil, err
	}
	return &User{ID: id, Email: email, RSSToken: token}, nil
}

func (s *Store) AuthenticateUser(email, password string) (*User, error) {
	var u User
	var hash string
	var emailVerified int
	err := s.db.QueryRow(
		"SELECT id, email, password_hash, email_verified, page_size FROM users WHERE email = ?", email,
	).Scan(&u.ID, &u.Email, &hash, &emailVerified, &u.PageSize)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	u.EmailVerified = emailVerified == 1
	return &u, nil
}

func (s *Store) CreateSession(userID string) string {
	id := generateToken()
	expires := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	s.db.Exec("INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)", id, userID, expires)
	return id
}

func (s *Store) GetSessionUser(sessionID string) (string, bool) {
	var userID, expiresAt string
	err := s.db.QueryRow(
		"SELECT user_id, expires_at FROM sessions WHERE id = ?", sessionID,
	).Scan(&userID, &expiresAt)
	if err != nil {
		return "", false
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().After(t) {
		s.db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
		return "", false
	}
	return userID, true
}

func (s *Store) DeleteSession(sessionID string) {
	s.db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
}

func (s *Store) GetUserByID(userID string) (*User, error) {
	var u User
	var emailVerified int
	err := s.db.QueryRow(
		"SELECT id, email, email_verified, COALESCE(rss_token,''), page_size FROM users WHERE id = ?", userID,
	).Scan(&u.ID, &u.Email, &emailVerified, &u.RSSToken, &u.PageSize)
	if err != nil {
		return nil, err
	}
	u.EmailVerified = emailVerified == 1
	return &u, nil
}

func (s *Store) GetUserByRSSToken(token string) (*User, error) {
	var u User
	var emailVerified int
	err := s.db.QueryRow(
		"SELECT id, email, email_verified, COALESCE(rss_token,''), page_size FROM users WHERE rss_token = ?", token,
	).Scan(&u.ID, &u.Email, &emailVerified, &u.RSSToken, &u.PageSize)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.EmailVerified = emailVerified == 1
	return &u, nil
}

func (s *Store) SetEmailDigest(userID string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := s.db.Exec("UPDATE users SET email_digest = ? WHERE id = ?", val, userID)
	return err
}

func (s *Store) SetPageSize(userID string, size int) error {
	if size != 5 && size != 10 && size != 20 {
		return fmt.Errorf("invalid page_size %d: must be 5, 10, or 20", size)
	}
	_, err := s.db.Exec("UPDATE users SET page_size = ? WHERE id = ?", size, userID)
	return err
}

func (s *Store) GetUserDigestEnabled(userID string) (bool, error) {
	var v int
	err := s.db.QueryRow("SELECT email_digest FROM users WHERE id = ?", userID).Scan(&v)
	return v == 1, err
}

func (s *Store) GetDigestUsers() []User {
	rows, err := s.db.Query(`
		SELECT id, email, email_verified, page_size
		FROM users
		WHERE email_digest = 1
		  AND email_verified = 1
		  AND email != ''`)
	if err != nil {
		log.Printf("⚠ GetDigestUsers failed: %v", err)
		return nil
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		var emailVerified int
		if err := rows.Scan(&u.ID, &u.Email, &emailVerified, &u.PageSize); err != nil {
			log.Printf("⚠ GetDigestUsers scan failed: %v", err)
			continue
		}
		u.EmailVerified = emailVerified == 1
		users = append(users, u)
	}
	return users
}

func (s *Store) GetReleasesPublishedOn(userID string, date time.Time) []Release {
	from := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	to := time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	rows, err := s.db.Query(`
		SELECT r.id, r.repo_id, r.name, r.version, r.platform, r.url,
		       r.published_at, r.description, r.release_notes
		FROM releases r
		INNER JOIN user_repos ur ON ur.repo_id = r.repo_id
		WHERE ur.user_id = ?
		  AND r.published_at >= ?
		  AND r.published_at < ?
		ORDER BY r.name ASC, r.published_at DESC`, userID, from, to)
	if err != nil {
		log.Printf("⚠ GetReleasesPublishedOn failed for user %s: %v", userID, err)
		return []Release{}
	}
	defer rows.Close()
	return scanReleases(rows)
}

var (
	ErrTokenNotFound = fmt.Errorf("token not found")
	ErrTokenExpired  = fmt.Errorf("token expired")
)

func (s *Store) CreateVerificationToken(userID string) (string, error) {
	s.db.Exec("DELETE FROM email_verification_tokens WHERE user_id = ?", userID)
	token := generateToken()
	expires := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	_, err := s.db.Exec(
		"INSERT INTO email_verification_tokens (token, user_id, expires_at) VALUES (?, ?, ?)",
		token, userID, expires,
	)
	return token, err
}

func (s *Store) VerifyEmailToken(token string) (string, error) {
	var userID, expiresAt string
	err := s.db.QueryRow(
		"SELECT user_id, expires_at FROM email_verification_tokens WHERE token = ?", token,
	).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return "", ErrTokenNotFound
	}
	if err != nil {
		return "", err
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().After(t) {
		s.db.Exec("DELETE FROM email_verification_tokens WHERE token = ?", token)
		return "", ErrTokenExpired
	}
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", userID); err != nil {
		tx.Rollback()
		return "", err
	}
	if _, err = tx.Exec("DELETE FROM email_verification_tokens WHERE token = ?", token); err != nil {
		tx.Rollback()
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return userID, nil
}

func (s *Store) DeleteVerificationTokensForUser(userID string) error {
	_, err := s.db.Exec("DELETE FROM email_verification_tokens WHERE user_id = ?", userID)
	return err
}

// ---- Repos (shared, deduped by platform+url) ----

// AddRepo finds or creates the shared repo record, then adds it to the user's
// tracking list. Returns the repo ID. Idempotent — safe to call multiple times.
func (s *Store) AddRepo(userID string, p Project) (string, error) {
	normURL := normalizeURL(p.RepoURL)

	var repoID string
	err := s.db.QueryRow(
		"SELECT id FROM repos WHERE platform = ? AND repo_url = ?",
		p.Platform, normURL,
	).Scan(&repoID)

	if err == sql.ErrNoRows {
		repoID = fmt.Sprintf("repo_%s", generateToken()[:12])
		if _, err := s.db.Exec(
			"INSERT INTO repos (id, platform, repo_url, name) VALUES (?, ?, ?, ?)",
			repoID, p.Platform, normURL, p.Name,
		); err != nil {
			return "", fmt.Errorf("failed to create repo: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("failed to look up repo: %w", err)
	}

	s.db.Exec(
		"INSERT OR IGNORE INTO user_repos (user_id, repo_id, created_at) VALUES (?, ?, ?)",
		userID, repoID, time.Now().Format(time.RFC3339),
	)
	return repoID, nil
}

// RemoveUserRepo removes the repo from the user's list. The shared repo and its
// releases are retained for any other users still tracking it.
func (s *Store) RemoveUserRepo(userID, repoID string) bool {
	res, err := s.db.Exec(
		"DELETE FROM user_repos WHERE user_id = ? AND repo_id = ?",
		userID, repoID,
	)
	if err != nil {
		log.Printf("⚠ Failed to remove user_repo %s/%s: %v", userID, repoID, err)
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

func (s *Store) GetUserRepos(userID string) []Project {
	rows, err := s.db.Query(`
		SELECT r.id, r.name, r.platform, r.repo_url, r.last_refresh, r.refresh_count, ur.push_enabled,
		       ur.email_immediate,
		       ur.gitlab_sync_enabled, ur.gitlab_sync_frequency, ur.last_gitlab_sync_at, ur.last_gitlab_sync_error,
		       ur.gitlab_project_path
		FROM repos r
		INNER JOIN user_repos ur ON ur.repo_id = r.id
		WHERE ur.user_id = ?
		ORDER BY ur.created_at ASC`, userID)
	if err != nil {
		log.Printf("⚠ Failed to query user repos: %v", err)
		return []Project{}
	}
	defer rows.Close()
	projects := []Project{}
	for rows.Next() {
		var p Project
		var lastRefresh, lastGitLabSync string
		var pushEnabled, emailImmediate, gitlabSyncEnabled int
		if err := rows.Scan(&p.ID, &p.Name, &p.Platform, &p.RepoURL, &lastRefresh, &p.RefreshCount, &pushEnabled,
			&emailImmediate,
			&gitlabSyncEnabled, &p.GitLabSyncFrequency, &lastGitLabSync, &p.LastGitLabSyncError,
			&p.GitLabProjectPath); err != nil {
			log.Printf("⚠ Failed to scan project: %v", err)
			continue
		}
		if lastRefresh != "" {
			p.LastRefresh, _ = time.Parse(time.RFC3339, lastRefresh)
		}
		if lastGitLabSync != "" {
			p.LastGitLabSyncAt, _ = time.Parse(time.RFC3339, lastGitLabSync)
		}
		p.PushEnabled = pushEnabled != 0
		p.EmailImmediate = emailImmediate != 0
		p.GitLabSyncEnabled = gitlabSyncEnabled != 0
		projects = append(projects, p)
	}
	return projects
}

// GetReleases returns all releases for a specific repo (used for testing).
func (s *Store) GetReleases(repoID string) []Release {
	rows, err := s.db.Query(`
		SELECT id, repo_id, name, version, platform, url, published_at, description, release_notes
		FROM releases WHERE repo_id = ? ORDER BY published_at DESC`, repoID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanReleases(rows)
}

// GetProjects returns all repos (used for admin/testing).
func (s *Store) GetProjects() []Project {
	rows, err := s.db.Query(`SELECT id, name, platform, repo_url, last_refresh, refresh_count FROM repos`)
	if err != nil {
		return []Project{}
	}
	defer rows.Close()
	return scanProjects(rows)
}

func scanProjects(rows *sql.Rows) []Project {
	projects := []Project{}
	for rows.Next() {
		var p Project
		var lastRefresh string
		if err := rows.Scan(&p.ID, &p.Name, &p.Platform, &p.RepoURL, &lastRefresh, &p.RefreshCount); err != nil {
			log.Printf("⚠ Failed to scan project: %v", err)
			continue
		}
		if lastRefresh != "" {
			p.LastRefresh, _ = time.Parse(time.RFC3339, lastRefresh)
		}
		projects = append(projects, p)
	}
	return projects
}

// ---- Releases ----

func (s *Store) GetUserReleases(userID string) []Release {
	rows, err := s.db.Query(`
		SELECT r.id, r.repo_id, r.name, r.version, r.platform, r.url, r.published_at, r.description, r.release_notes
		FROM releases r
		INNER JOIN user_repos ur ON ur.repo_id = r.repo_id
		WHERE ur.user_id = ?
		ORDER BY r.published_at DESC`, userID)
	if err != nil {
		log.Printf("⚠ Failed to query user releases: %v", err)
		return []Release{}
	}
	defer rows.Close()
	return scanReleases(rows)
}

func (s *Store) GetUserReleasesForFeed(userID string, limit int) []Release {
	rows, err := s.db.Query(`
		SELECT r.id, r.repo_id, r.name, r.version, r.platform, r.url, r.published_at, r.description, r.release_notes
		FROM releases r
		INNER JOIN user_repos ur ON ur.repo_id = r.repo_id
		WHERE ur.user_id = ?
		ORDER BY r.published_at DESC
		LIMIT ?`, userID, limit)
	if err != nil {
		log.Printf("⚠ GetUserReleasesForFeed failed for user %s: %v", userID, err)
		return []Release{}
	}
	defer rows.Close()
	return scanReleases(rows)
}

// GetAllReleases returns all releases across all repos (used for testing).
func (s *Store) GetAllReleases() []Release {
	rows, err := s.db.Query(`
		SELECT r.id, r.repo_id, r.name, r.version, r.platform, r.url, r.published_at, r.description, r.release_notes
		FROM releases r
		INNER JOIN repos rp ON rp.id = r.repo_id
		ORDER BY r.published_at DESC`)
	if err != nil {
		log.Printf("⚠ Failed to query all releases: %v", err)
		return []Release{}
	}
	defer rows.Close()
	return scanReleases(rows)
}

func scanReleases(rows *sql.Rows) []Release {
	releases := []Release{}
	for rows.Next() {
		var r Release
		var publishedAt string
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Name, &r.Version, &r.Platform, &r.URL,
			&publishedAt, &r.Description, &r.ReleaseNotes); err != nil {
			log.Printf("⚠ Failed to scan release: %v", err)
			continue
		}
		r.PublishedAt, _ = time.Parse(time.RFC3339, publishedAt)
		releases = append(releases, r)
	}
	return releases
}

func (s *Store) AddRelease(repoID string, r Release) {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO releases
			(id, repo_id, name, version, platform, url, published_at, description, release_notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, repoID, r.Name, r.Version, r.Platform, r.URL,
		r.PublishedAt.Format(time.RFC3339), r.Description, r.ReleaseNotes,
	)
	if err != nil {
		log.Printf("⚠ Failed to add release %s: %v", r.ID, err)
		return
	}

	// Trim oldest beyond the per-repo limit.
	s.db.Exec(`
		DELETE FROM releases
		WHERE repo_id = ?
		  AND rowid NOT IN (
		      SELECT rowid FROM releases
		      WHERE repo_id = ?
		      ORDER BY published_at DESC
		      LIMIT ?
		  )`, repoID, repoID, releaseLimit)
}

// ---- Refresh ----

func (s *Store) MarkRefreshed(repoID string) {
	_, err := s.db.Exec(`
		UPDATE repos SET last_refresh = ?, refresh_count = refresh_count + 1 WHERE id = ?`,
		time.Now().Format(time.RFC3339), repoID,
	)
	if err != nil {
		log.Printf("⚠ Failed to mark repo %s refreshed: %v", repoID, err)
	}
}

func (s *Store) IsStale(repoID string) bool {
	var lastRefresh string
	err := s.db.QueryRow("SELECT last_refresh FROM repos WHERE id = ?", repoID).Scan(&lastRefresh)
	if err != nil || lastRefresh == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, lastRefresh)
	if err != nil {
		return true
	}
	return time.Since(t) > 30*time.Minute
}

// GetStaleRepos returns repos tracked by at least one user that haven't been
// refreshed recently. Deduped so each repo appears once even if multiple users track it.
func (s *Store) GetStaleRepos() []Project {
	cutoff := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	rows, err := s.db.Query(`
		SELECT DISTINCT r.id, r.name, r.platform, r.repo_url, r.last_refresh, r.refresh_count
		FROM repos r
		INNER JOIN user_repos ur ON ur.repo_id = r.id
		WHERE r.last_refresh = '' OR r.last_refresh < ?`, cutoff)
	if err != nil {
		log.Printf("⚠ Failed to query stale repos: %v", err)
		return []Project{}
	}
	defer rows.Close()
	return scanProjects(rows)
}

// RefreshProject fetches fresh releases for a repo and replaces the stored set.
func (s *Store) RefreshProject(repoID string) {
	var project Project
	err := s.db.QueryRow(
		`SELECT id, name, platform, repo_url FROM repos WHERE id = ?`, repoID,
	).Scan(&project.ID, &project.Name, &project.Platform, &project.RepoURL)
	if err != nil {
		return
	}

	var releases []Release
	switch project.Platform {
	case "github":
		releases = fetchGitHubReleases(project)
	case "npm":
		releases = fetchNPMVersions(project)
	case "pypi":
		releases = fetchPyPIReleases(project)
	case "docker":
		releases = fetchDockerTags(project)
	case "gitlab":
		releases = fetchGitLabReleases(project)
	case "helm-artifacthub":
		releases = fetchHelmArtifactHub(project)
	case "helm-repo":
		releases = fetchHelmRepo(project)
	default:
		log.Printf("⚠ Unknown platform: %s", project.Platform)
		return
	}

	if len(releases) == 0 {
		log.Printf("⚠ No releases found for %s (%s)", project.Name, project.Platform)
		return
	}

	// Snapshot existing IDs before the transaction so we can detect new ones.
	existingIDs := map[string]bool{}
	for _, r := range s.GetReleases(repoID) {
		existingIDs[r.ID] = true
	}

	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("⚠ Failed to begin transaction for %s: %v", project.Name, err)
		return
	}

	if _, err := tx.Exec("DELETE FROM releases WHERE repo_id = ?", repoID); err != nil {
		tx.Rollback()
		log.Printf("⚠ Failed to clear releases for %s: %v", project.Name, err)
		return
	}

	for _, r := range releases {
		_, err := tx.Exec(`
			INSERT INTO releases
				(id, repo_id, name, version, platform, url, published_at, description, release_notes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, repoID, r.Name, r.Version, r.Platform, r.URL,
			r.PublishedAt.Format(time.RFC3339), r.Description, r.ReleaseNotes,
		)
		if err != nil {
			tx.Rollback()
			log.Printf("⚠ Failed to insert release for %s: %v", project.Name, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("⚠ Failed to commit releases for %s: %v", project.Name, err)
		return
	}

	// Guard: only mark refreshed if the repo still exists.
	var exists int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM repos WHERE id = ?", repoID).Scan(&exists); err != nil || exists == 0 {
		return
	}
	s.MarkRefreshed(repoID)
	log.Printf("✓ Refreshed %s: %d releases, latest %s", project.Name, len(releases), releases[0].Version)

	// Notify subscribers about genuinely new releases (skip first-ever fetch).
	if len(existingIDs) > 0 {
		var newReleases []Release
		for _, r := range releases {
			if !existingIDs[r.ID] {
				newReleases = append(newReleases, r)
			}
		}
		if len(newReleases) > 0 {
			go s.sendPushNotifications(repoID, project.Name, newReleases)
			go s.sendWebhooks(repoID, project, newReleases)
			if smtpEnabled {
				go s.sendImmediateEmails(repoID, project, newReleases)
			}
		}
	}
}
