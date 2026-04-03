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
		WHERE ur.repo_id = ?`, repoID)
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

func (s *Store) CreateUser(username, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("user_%s", generateToken()[:12])
	_, err = s.db.Exec(
		"INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)",
		id, username, string(hash), time.Now().Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &User{ID: id, Username: username}, nil
}

func (s *Store) AuthenticateUser(username, password string) (*User, error) {
	var u User
	var hash string
	err := s.db.QueryRow(
		"SELECT id, username, password_hash FROM users WHERE username = ?", username,
	).Scan(&u.ID, &u.Username, &hash)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
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
	err := s.db.QueryRow("SELECT id, username FROM users WHERE id = ?", userID).Scan(&u.ID, &u.Username)
	if err != nil {
		return nil, err
	}
	return &u, nil
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
		SELECT r.id, r.name, r.platform, r.repo_url, r.last_refresh, r.refresh_count
		FROM repos r
		INNER JOIN user_repos ur ON ur.repo_id = r.id
		WHERE ur.user_id = ?
		ORDER BY ur.created_at ASC`, userID)
	if err != nil {
		log.Printf("⚠ Failed to query user repos: %v", err)
		return []Project{}
	}
	defer rows.Close()
	return scanProjects(rows)
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
		}
	}
}
