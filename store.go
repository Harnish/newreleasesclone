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
	"sync"
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

	gitlabSyncing sync.Map // "user/repo" -> struct{} while a GitLab sync for it runs
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

	for _, sub := range subs {
		rels := s.releasesFor(sub.UserID, repoID, newReleases)
		if len(rels) == 0 {
			continue
		}
		latest := rels[0]
		var body string
		if len(rels) == 1 {
			body = latest.Version + " is now available"
		} else {
			body = fmt.Sprintf("%d new releases, latest %s", len(rels), latest.Version)
		}
		payload, _ := json.Marshal(map[string]string{
			"title": projectName + " — new release",
			"body":  body,
			"url":   latest.URL,
		})
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

func (s *Store) SetProjectIncludePrereleases(userID, repoID string, enabled bool) (bool, error) {
	res, err := s.db.Exec(
		"UPDATE user_repos SET include_prereleases = ? WHERE user_id = ? AND repo_id = ?",
		boolInt(enabled), userID, repoID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// releasesFor drops prereleases unless the user opted in for this project.
// Notification senders call it per recipient, since releases are shared.
func (s *Store) releasesFor(userID, repoID string, rels []Release) []Release {
	var include int
	s.db.QueryRow("SELECT include_prereleases FROM user_repos WHERE user_id = ? AND repo_id = ?",
		userID, repoID).Scan(&include)
	if include != 0 {
		return rels
	}
	var stable []Release
	for _, r := range rels {
		if !r.Prerelease {
			stable = append(stable, r)
		}
	}
	return stable
}

func (s *Store) getImmediateEmailUsersForRepo(repoID string) ([]User, error) {
	rows, err := s.db.Query(`
		SELECT u.id, u.email, u.email_verified
		FROM users u
		INNER JOIN user_repos ur ON ur.user_id = u.id
		WHERE ur.repo_id = ?
		  AND ur.email_immediate = 1
		  AND u.email_verified = 1
		  AND u.email != ''`, repoID)
	if err != nil {
		return nil, err
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
	return users, nil
}

func (s *Store) sendImmediateEmails(repoID string, project Project, newReleases []Release) {
	users, err := s.getImmediateEmailUsersForRepo(repoID)
	if err != nil {
		log.Printf("⚠ sendImmediateEmails: failed to query users: %v", err)
		return
	}
	if len(users) == 0 {
		return
	}

	for _, u := range users {
		for _, release := range s.releasesFor(u.ID, repoID, newReleases) {
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
			token_invalid = 0,
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
	if syncErr == nil {
		s.db.Exec("UPDATE gitlab_settings SET token_invalid = 0 WHERE user_id = ?", userID) // it worked, so the token is fine
	} else {
		s.noteGitLabAuth(userID, syncErr)
	}
	return err
}

// noteGitLabAuth flags userID's saved token as no longer working if err says so.
func (s *Store) noteGitLabAuth(userID string, err error) {
	if isGitLabAuthError(err) {
		s.db.Exec("UPDATE gitlab_settings SET token_invalid = 1 WHERE user_id = ?", userID)
	}
}

// GitLabTokenInvalidURL returns the page on the user's GitLab where they can
// create a new token, or "" while the saved token works (or none is saved).
func (s *Store) GitLabTokenInvalidURL(userID string) string {
	var base string
	err := s.db.QueryRow("SELECT gitlab_url FROM gitlab_settings WHERE user_id = ? AND token_invalid = 1", userID).Scan(&base)
	if err != nil || base == "" {
		return ""
	}
	return strings.TrimSuffix(base, "/") + "/-/user_settings/personal_access_tokens?name=newreleases&scopes=api"
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

type webhookTarget struct {
	wh     Webhook
	secret string
	userID string
}

func (s *Store) getWebhooksForRepo(repoID string) []webhookTarget {
	rows, err := s.db.Query(`SELECT id, repo_id, url, secret, user_id FROM webhooks WHERE repo_id = ?`, repoID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []webhookTarget
	for rows.Next() {
		var id, rid, u, sec, uid string
		if err := rows.Scan(&id, &rid, &u, &sec, &uid); err != nil {
			continue
		}
		results = append(results, webhookTarget{Webhook{ID: id, RepoID: rid, URL: u, HasSecret: sec != ""}, sec, uid})
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
		Prerelease  bool   `json:"prerelease"`
	}
	payloadFor := func(rels []Release) []byte {
		relPayloads := make([]releasePayload, len(rels))
		for i, r := range rels {
			relPayloads[i] = releasePayload{
				Version:     r.Version,
				URL:         r.URL,
				PublishedAt: r.PublishedAt.Format(time.RFC3339),
				Description: r.Description,
				Prerelease:  r.Prerelease,
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
		return body
	}

	// Reuse the GitLab SSRF-guarded transport: webhook URLs are user-supplied.
	client := &http.Client{
		Timeout:       10 * time.Second,
		Transport:     gitLabHTTPTransport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	for _, entry := range hooks {
		rels := s.releasesFor(entry.userID, repoID, newReleases)
		if len(rels) == 0 {
			continue
		}
		body := payloadFor(rels)
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
	if size != 10 && size != 20 && size != 50 {
		return fmt.Errorf("invalid page_size %d: must be 10, 20, or 50", size)
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
		       r.published_at, r.description, r.release_notes, r.is_prerelease
		FROM releases r
		INNER JOIN user_repos ur ON ur.repo_id = r.repo_id
		WHERE ur.user_id = ?
		  AND (r.is_prerelease = 0 OR ur.include_prereleases = 1)
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
	tags := s.GetUserTags(userID) // before the query below: the DB has one connection, so it can't run while rows is open
	rows, err := s.db.Query(`
		SELECT r.id, r.name, r.platform, r.repo_url, r.last_refresh, r.refresh_count, ur.push_enabled,
		       ur.email_immediate, ur.include_prereleases,
		       ur.gitlab_sync_enabled, ur.gitlab_sync_frequency, ur.last_gitlab_sync_at, ur.last_gitlab_sync_error,
		       ur.gitlab_project_path, r.description,
		       CASE WHEN length(ri.data) > 0 THEN ri.fetched_at ELSE 0 END
		FROM repos r
		INNER JOIN user_repos ur ON ur.repo_id = r.id
		LEFT JOIN repo_icons ri ON ri.repo_id = r.id
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
		var pushEnabled, emailImmediate, includePre, gitlabSyncEnabled int
		if err := rows.Scan(&p.ID, &p.Name, &p.Platform, &p.RepoURL, &lastRefresh, &p.RefreshCount, &pushEnabled,
			&emailImmediate, &includePre,
			&gitlabSyncEnabled, &p.GitLabSyncFrequency, &lastGitLabSync, &p.LastGitLabSyncError,
			&p.GitLabProjectPath, &p.Description, &p.IconVersion); err != nil {
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
		p.IncludePrereleases = includePre != 0
		p.GitLabSyncEnabled = gitlabSyncEnabled != 0
		if p.Tags = tags[p.ID]; p.Tags == nil {
			p.Tags = []string{}
		}
		projects = append(projects, p)
	}
	return projects
}

// GetReleases returns all releases for a specific repo (used for testing).
func (s *Store) GetReleases(repoID string) []Release {
	rows, err := s.db.Query(`
		SELECT id, repo_id, name, version, platform, url, published_at, description, release_notes, is_prerelease
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
		SELECT r.id, r.repo_id, r.name, r.version, r.platform, r.url, r.published_at, r.description, r.release_notes, r.is_prerelease
		FROM releases r
		INNER JOIN user_repos ur ON ur.repo_id = r.repo_id
		WHERE ur.user_id = ?
		  AND (r.is_prerelease = 0 OR ur.include_prereleases = 1)
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
		SELECT r.id, r.repo_id, r.name, r.version, r.platform, r.url, r.published_at, r.description, r.release_notes, r.is_prerelease
		FROM releases r
		INNER JOIN user_repos ur ON ur.repo_id = r.repo_id
		WHERE ur.user_id = ?
		  AND (r.is_prerelease = 0 OR ur.include_prereleases = 1)
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
		SELECT r.id, r.repo_id, r.name, r.version, r.platform, r.url, r.published_at, r.description, r.release_notes, r.is_prerelease
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
		var prerelease int
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Name, &r.Version, &r.Platform, &r.URL,
			&publishedAt, &r.Description, &r.ReleaseNotes, &prerelease); err != nil {
			log.Printf("⚠ Failed to scan release: %v", err)
			continue
		}
		r.PublishedAt, _ = time.Parse(time.RFC3339, publishedAt)
		r.Prerelease = prerelease != 0
		releases = append(releases, r)
	}
	return releases
}

func (s *Store) AddRelease(repoID string, r Release) {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO releases
			(id, repo_id, name, version, platform, url, published_at, description, release_notes, is_prerelease)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, repoID, r.Name, r.Version, r.Platform, r.URL,
		r.PublishedAt.Format(time.RFC3339), r.Description, r.ReleaseNotes, boolInt(r.Prerelease),
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
				(id, repo_id, name, version, platform, url, published_at, description, release_notes, is_prerelease)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, repoID, r.Name, r.Version, r.Platform, r.URL,
			r.PublishedAt.Format(time.RFC3339), r.Description, r.ReleaseNotes, boolInt(r.Prerelease),
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
	go s.refreshIcon(project)
	go s.refreshDescription(project)

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
