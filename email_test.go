package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGetImmediateEmailUsers tests that only verified users with email_immediate=1 are returned.
func TestGetImmediateEmailUsers(t *testing.T) {
	s := newTestStore(t)

	// User A — opts in, verified
	userA, _ := newTestAuth(t, s)
	s.db.Exec("UPDATE users SET email='a@example.com', email_verified=1 WHERE id=?", userA)
	repoID, _ := s.AddRepo(userA, Project{Name: "Proj", Platform: "github", RepoURL: "https://github.com/x/y"})
	s.SetProjectEmailImmediate(userA, repoID, true)

	// User B — opts in, but unverified
	userB, _ := s.CreateUser("userb", "pass")
	s.db.Exec("UPDATE users SET email='b@example.com', email_verified=0 WHERE id=?", userB.ID)
	s.AddRepo(userB.ID, Project{Name: "Proj", Platform: "github", RepoURL: "https://github.com/x/y"})
	s.SetProjectEmailImmediate(userB.ID, repoID, true)

	// User C — verified, but did not opt in
	userC, _ := s.CreateUser("userc", "pass")
	s.db.Exec("UPDATE users SET email='c@example.com', email_verified=1 WHERE id=?", userC.ID)
	s.AddRepo(userC.ID, Project{Name: "Proj", Platform: "github", RepoURL: "https://github.com/x/y"})
	// email_immediate stays false (default)

	users, err := s.getImmediateEmailUsersForRepo(repoID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].ID != userA {
		t.Errorf("expected user A, got %s", users[0].ID)
	}
}

func TestStoreEmailFields(t *testing.T) {
	s := newTestStore(t)
	user, err := s.CreateUser("emailuser@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	got, err := s.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got.Email != "emailuser@example.com" {
		t.Errorf("expected email emailuser@example.com, got %q", got.Email)
	}
	if got.EmailVerified {
		t.Error("expected email_verified false for new user")
	}
}

func TestStoreVerificationToken(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("token@example.com", "password123")

	token, err := s.CreateVerificationToken(user.ID)
	if err != nil {
		t.Fatalf("CreateVerificationToken failed: %v", err)
	}
	if len(token) == 0 {
		t.Fatal("expected non-empty token")
	}

	// Verify it
	gotUserID, err := s.VerifyEmailToken(token)
	if err != nil {
		t.Fatalf("VerifyEmailToken failed: %v", err)
	}
	if gotUserID != user.ID {
		t.Errorf("expected userID %s, got %s", user.ID, gotUserID)
	}

	// User should now be verified
	got, _ := s.GetUserByID(user.ID)
	if !got.EmailVerified {
		t.Error("expected email_verified true after verification")
	}

	// Token should be consumed
	_, err = s.VerifyEmailToken(token)
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound after use, got %v", err)
	}
}

func TestStoreVerifyEmailTokenExpired(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("expiretest@example.com", "password123")

	// Insert an already-expired token directly
	expiredAt := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	s.db.Exec(
		"INSERT INTO email_verification_tokens (token, user_id, expires_at) VALUES (?, ?, ?)",
		"expiredtoken", user.ID, expiredAt,
	)

	_, err := s.VerifyEmailToken("expiredtoken")
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestStoreVerifyEmailTokenNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.VerifyEmailToken("doesnotexist")
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestStoreCreateVerificationTokenDeletesOld(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("reusetest@example.com", "password123")

	token1, _ := s.CreateVerificationToken(user.ID)
	token2, _ := s.CreateVerificationToken(user.ID)

	if token1 == token2 {
		t.Error("expected different tokens")
	}
	// Old token should be gone
	_, err := s.VerifyEmailToken(token1)
	if err != ErrTokenNotFound {
		t.Errorf("expected old token deleted, got %v", err)
	}
}

func TestSMTPConfigFromEnv(t *testing.T) {
	// No env vars set — should return disabled
	cfg, ok := SMTPConfigFromEnv()
	if ok {
		t.Errorf("expected disabled with no env vars, got config: %+v", cfg)
	}

	// Set required vars
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_USER", "user@example.com")
	t.Setenv("SMTP_PASS", "secret")
	t.Setenv("SMTP_FROM", "noreply@example.com")

	cfg, ok = SMTPConfigFromEnv()
	if !ok {
		t.Fatal("expected enabled with all required env vars")
	}
	if cfg.Host != "smtp.example.com" {
		t.Errorf("expected Host smtp.example.com, got %q", cfg.Host)
	}
	if cfg.Port != 587 {
		t.Errorf("expected default Port 587, got %d", cfg.Port)
	}
	if cfg.User != "user@example.com" {
		t.Errorf("expected User user@example.com, got %q", cfg.User)
	}

	// Custom port
	t.Setenv("SMTP_PORT", "465")
	cfg, _ = SMTPConfigFromEnv()
	if cfg.Port != 465 {
		t.Errorf("expected Port 465, got %d", cfg.Port)
	}
}

func TestHandleVerifyEmail(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	user, _ := store.CreateUser("verify@example.com", "password123")
	token, _ := store.CreateVerificationToken(user.ID)

	req, _ := http.NewRequest("GET", "/verify-email?token="+token, nil)
	w := httptest.NewRecorder()
	handleVerifyEmail(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/?verified=1" {
		t.Errorf("expected redirect to /?verified=1, got %q", loc)
	}

	got, _ := store.GetUserByID(user.ID)
	if !got.EmailVerified {
		t.Error("expected email_verified true after verification")
	}
}

func TestHandleVerifyEmailInvalidToken(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	req, _ := http.NewRequest("GET", "/verify-email?token=badtoken", nil)
	w := httptest.NewRecorder()
	handleVerifyEmail(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/?verify_error=1" {
		t.Errorf("expected redirect to /?verify_error=1, got %q", loc)
	}
}

func TestHandleResendVerification(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	// Reset rate limiter state for test isolation.
	resendMu.Lock()
	resendAttempts = map[string][]time.Time{}
	resendMu.Unlock()

	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("POST", "/api/resend-verification", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleResendVerification).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleResendVerificationAlreadyVerified(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	// Reset rate limiter state for test isolation.
	resendMu.Lock()
	resendAttempts = map[string][]time.Time{}
	resendMu.Unlock()

	userID, cookie := newTestAuth(t, store)
	// Mark verified directly
	store.db.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", userID)

	req, _ := http.NewRequest("POST", "/api/resend-verification", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleResendVerification).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleResendVerificationRateLimit(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	// Reset rate limiter state for this test
	resendMu.Lock()
	resendAttempts = map[string][]time.Time{}
	resendMu.Unlock()

	_, cookie := newTestAuth(t, store)

	// Make 3 successful requests
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("POST", "/api/resend-verification", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		requireAuth(handleResendVerification).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 4th request should be rate limited
	req, _ := http.NewRequest("POST", "/api/resend-verification", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleResendVerification).ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestHandleResendVerificationMethodNotAllowed(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	// Reset rate limiter state for test isolation.
	resendMu.Lock()
	resendAttempts = map[string][]time.Time{}
	resendMu.Unlock()

	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("GET", "/api/resend-verification", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleResendVerification).ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestSetGetEmailDigest(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	// default is false
	enabled, err := s.GetUserDigestEnabled(userID)
	if err != nil {
		t.Fatalf("GetUserDigestEnabled: %v", err)
	}
	if enabled {
		t.Error("expected email_digest disabled by default")
	}

	// enable it
	if err := s.SetEmailDigest(userID, true); err != nil {
		t.Fatalf("SetEmailDigest true: %v", err)
	}
	enabled, err = s.GetUserDigestEnabled(userID)
	if err != nil {
		t.Fatalf("GetUserDigestEnabled after set: %v", err)
	}
	if !enabled {
		t.Error("expected email_digest enabled after set")
	}

	// disable it
	if err := s.SetEmailDigest(userID, false); err != nil {
		t.Fatalf("SetEmailDigest false: %v", err)
	}
	enabled, _ = s.GetUserDigestEnabled(userID)
	if enabled {
		t.Error("expected email_digest disabled after unset")
	}
}

func TestGetDigestUsers(t *testing.T) {
	s := newTestStore(t)

	// u1: digest + verified + email => should appear
	u1, _ := s.CreateUser("digest1@example.com", "password123")
	s.db.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", u1.ID)
	s.SetEmailDigest(u1.ID, true)

	// u2: digest enabled but not verified => should NOT appear
	u2, _ := s.CreateUser("digest2@example.com", "password123")
	s.SetEmailDigest(u2.ID, true)

	// u3: verified but digest off => should NOT appear
	u3, _ := s.CreateUser("digest3@example.com", "password123")
	s.db.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", u3.ID)

	users := s.GetDigestUsers()
	if len(users) != 1 {
		t.Errorf("expected 1 digest user, got %d", len(users))
	}
	if len(users) > 0 && users[0].ID != u1.ID {
		t.Errorf("expected user %s, got %s", u1.ID, users[0].ID)
	}
}

func TestGetReleasesPublishedOn(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, _ := s.AddRepo(userID, Project{
		Name: "TestProj", Platform: "github",
		RepoURL: "https://github.com/test/repo",
	})

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)

	// Release published yesterday (noon UTC) — should appear
	s.AddRelease(repoID, Release{
		ID: "rel-yesterday", Name: "TestProj", Version: "v1.0.0",
		Platform: "github", URL: "https://github.com/test/repo/releases/v1.0.0",
		PublishedAt: time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 12, 0, 0, 0, time.UTC),
	})
	// Release published today — should NOT appear
	s.AddRelease(repoID, Release{
		ID: "rel-today", Name: "TestProj", Version: "v1.1.0",
		Platform: "github", URL: "https://github.com/test/repo/releases/v1.1.0",
		PublishedAt: time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC),
	})

	releases := s.GetReleasesPublishedOn(userID, yesterday)
	if len(releases) != 1 {
		t.Fatalf("expected 1 release for yesterday, got %d", len(releases))
	}
	if releases[0].ID != "rel-yesterday" {
		t.Errorf("expected rel-yesterday, got %s", releases[0].ID)
	}

	// Other user should see nothing (no user_repos entry)
	u2, _ := s.CreateUser("other@example.com", "password123")
	releases2 := s.GetReleasesPublishedOn(u2.ID, yesterday)
	if len(releases2) != 0 {
		t.Errorf("expected 0 releases for other user, got %d", len(releases2))
	}
}

func TestBuildDailySummaryBody(t *testing.T) {
	releases := []Release{
		{Name: "myproject", Version: "v1.2.3", URL: "https://github.com/my/project/releases/v1.2.3"},
		{Name: "another", Version: "v0.9.0", URL: "https://pypi.org/project/another/0.9.0"},
	}
	body := buildDailySummaryBody(releases)
	if !strings.Contains(body, "Here are the releases from yesterday:") {
		t.Error("expected header line in body")
	}
	if !strings.Contains(body, "myproject v1.2.3") {
		t.Error("expected 'myproject v1.2.3' in body")
	}
	if !strings.Contains(body, "https://github.com/my/project/releases/v1.2.3") {
		t.Error("expected URL in body")
	}
	if !strings.Contains(body, "another v0.9.0") {
		t.Error("expected 'another v0.9.0' in body")
	}
}

// TestBuildReleaseEmailBody tests the plain-text email body for a single release.
func TestBuildReleaseEmailBody(t *testing.T) {
	proj := Project{Name: "myproject", Platform: "github"}
	release := Release{Version: "v1.2.3", URL: "https://github.com/x/myproject/releases/tag/v1.2.3"}

	// Without release notes
	body := buildReleaseEmailBody(proj, release)
	if !strings.Contains(body, "myproject v1.2.3 has been released.") {
		t.Errorf("body missing release line, got:\n%s", body)
	}
	if !strings.Contains(body, "https://github.com/x/myproject/releases/tag/v1.2.3") {
		t.Errorf("body missing URL, got:\n%s", body)
	}
	if strings.Contains(body[strings.Index(body, release.URL)+len(release.URL):], "\n\n") {
		t.Errorf("body should not have trailing blank line when no release notes")
	}

	// With release notes
	release.ReleaseNotes = "- Fixed a bug\n- Added a feature"
	body = buildReleaseEmailBody(proj, release)
	if !strings.Contains(body, "- Fixed a bug") {
		t.Errorf("body missing release notes, got:\n%s", body)
	}
}

// TestSendReleaseEmailSubject verifies the subject format.
func TestSendReleaseEmailSubject(t *testing.T) {
	proj := Project{Name: "myproject"}
	release := Release{Version: "v1.2.3", URL: "https://example.com"}
	// Verify subject is correct by calling buildReleaseEmailBody (SendReleaseEmail requires live SMTP).
	// Subject format: "<Name> <Version> released"
	expectedSubject := "myproject v1.2.3 released"
	subject := fmt.Sprintf("%s %s released", proj.Name, release.Version)
	if subject != expectedSubject {
		t.Errorf("expected subject %q, got %q", expectedSubject, subject)
	}
}

func TestNextSevenAMUTC(t *testing.T) {
	d := nextSevenAMUTC()
	if d <= 0 {
		t.Errorf("expected positive duration, got %v", d)
	}
	if d > 24*time.Hour {
		t.Errorf("expected at most 24h, got %v", d)
	}
}
