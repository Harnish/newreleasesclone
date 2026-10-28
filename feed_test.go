package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestCreateUserGeneratesRSSToken verifies every new user gets a non-empty, unique rss_token.
func TestCreateUserGeneratesRSSToken(t *testing.T) {
	s := newTestStore(t)

	u1, err := s.CreateUser("alice@example.com", "password1")
	if err != nil {
		t.Fatalf("CreateUser alice: %v", err)
	}
	if u1.RSSToken == "" {
		t.Fatal("expected non-empty RSSToken for new user")
	}

	u2, err := s.CreateUser("bob@example.com", "password2")
	if err != nil {
		t.Fatalf("CreateUser bob: %v", err)
	}
	if u2.RSSToken == "" {
		t.Fatal("expected non-empty RSSToken for new user")
	}
	if u1.RSSToken == u2.RSSToken {
		t.Errorf("expected unique tokens; both got %q", u1.RSSToken)
	}

	// GetUserByID must also return the token
	got, err := s.GetUserByID(u1.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.RSSToken != u1.RSSToken {
		t.Errorf("GetUserByID returned token %q, want %q", got.RSSToken, u1.RSSToken)
	}
}

// TestGetUserByRSSToken verifies lookup by token works and returns nil for unknown tokens.
func TestGetUserByRSSToken(t *testing.T) {
	s := newTestStore(t)

	u, err := s.CreateUser("alice@example.com", "password1")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Found
	found, err := s.GetUserByRSSToken(u.RSSToken)
	if err != nil {
		t.Fatalf("GetUserByRSSToken: %v", err)
	}
	if found == nil || found.ID != u.ID {
		t.Errorf("expected user %q, got %v", u.ID, found)
	}

	// Not found — nil, nil
	missing, err := s.GetUserByRSSToken("doesnotexist00000000000000000000")
	if err != nil {
		t.Fatalf("unexpected error for unknown token: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for unknown token, got %v", missing)
	}
}

// TestHandleFeed tests GET /feed/<token> returns Atom XML for valid tokens.
func TestHandleFeed(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	s := newTestStore(t)
	store = s

	// Create user and add a release
	user, err := s.CreateUser("feeduser@example.com", "password")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	repoID, err := s.AddRepo(user.ID, Project{
		Name:     "myproject",
		Platform: "github",
		RepoURL:  "https://github.com/x/myproject",
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	s.AddRelease(repoID, Release{
		ID:          "gh_1",
		Name:        "myproject",
		Version:     "v1.2.3",
		Platform:    "github",
		URL:         "https://github.com/x/myproject/releases/tag/v1.2.3",
		PublishedAt: time.Now().Add(-time.Hour),
	})

	// Valid token — expect 200 with Atom content type and valid XML
	req, _ := http.NewRequest("GET", "/feed/"+user.RSSToken, nil)
	req.Host = "example.com"
	w := httptest.NewRecorder()
	http.HandlerFunc(handleFeed).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid token, got %d: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/atom+xml") {
		t.Errorf("expected atom+xml content type, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "myproject v1.2.3") {
		t.Errorf("expected release title in feed, got:\n%s", body)
	}
	if !strings.Contains(body, "https://github.com/x/myproject/releases/tag/v1.2.3") {
		t.Errorf("expected release URL in feed, got:\n%s", body)
	}

	// Unknown token — expect 404
	req, _ = http.NewRequest("GET", "/feed/unknowntoken00000000000000000000000000000000000000000000000000", nil)
	w = httptest.NewRecorder()
	http.HandlerFunc(handleFeed).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown token, got %d", w.Code)
	}

	// Empty token path — expect 404
	req, _ = http.NewRequest("GET", "/feed/", nil)
	w = httptest.NewRecorder()
	http.HandlerFunc(handleFeed).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for empty token, got %d", w.Code)
	}
}

// TestHandleFeedEmptyReleases tests that a valid token with no releases returns a valid empty Atom feed.
func TestHandleFeedEmptyReleases(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	s := newTestStore(t)
	store = s

	user, err := s.CreateUser("emptyuser@example.com", "password")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	req, _ := http.NewRequest("GET", "/feed/"+user.RSSToken, nil)
	req.Host = "example.com"
	w := httptest.NewRecorder()
	http.HandlerFunc(handleFeed).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty feed, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<feed") {
		t.Errorf("expected Atom feed element, got:\n%s", body)
	}
}
