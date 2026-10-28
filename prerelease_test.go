package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsPrerelease(t *testing.T) {
	pre := []string{"1.0.0-rc.1", "2.0.0-beta", "v3.1.0-alpha.2", "1.0a1", "1.0b2", "1.0rc1",
		"1.0.dev3", "nightly", "next", "19.0.0-canary-abc", "0.0.0-experimental-dc9aed36-20261027", "1.2.3-SNAPSHOT"}
	stable := []string{"1.0.0", "v2.3.4", "latest", "1.2.3-alpine", "3.9-slim-bookworm", "1.2.3-alpine3.18",
		"10.0.0", "1.0.0+build5", "arc-1.0"}
	for _, v := range pre {
		if !isPrerelease(v) {
			t.Errorf("isPrerelease(%q) = false, want true", v)
		}
	}
	for _, v := range stable {
		if isPrerelease(v) {
			t.Errorf("isPrerelease(%q) = true, want false", v)
		}
	}
}

func prereleaseFixture(t *testing.T) (*Store, string, string, *http.Cookie) {
	s := newTestStore(t)
	userID, cookie := newTestAuth(t, s)
	repoID, err := s.AddRepo(userID, Project{Name: "P", Platform: "github", RepoURL: "https://github.com/a/b"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	s.AddRelease(repoID, Release{ID: "gh_1", Name: "P", Version: "1.0.0", Platform: "github", URL: "u", PublishedAt: now.Add(-time.Hour)})
	s.AddRelease(repoID, Release{ID: "gh_2", Name: "P", Version: "1.1.0-rc.1", Platform: "github", URL: "u", PublishedAt: now, Prerelease: true})
	return s, userID, repoID, cookie
}

func TestPrereleasesHiddenUntilOptIn(t *testing.T) {
	s, userID, repoID, _ := prereleaseFixture(t)

	if got := s.GetUserReleases(userID); len(got) != 1 || got[0].Version != "1.0.0" {
		t.Fatalf("default: got %+v, want only stable 1.0.0", got)
	}
	if got := s.GetUserReleasesForFeed(userID, 10); len(got) != 1 {
		t.Fatalf("feed default: got %d releases, want 1", len(got))
	}
	if got := s.GetReleasesPublishedOn(userID, time.Now().UTC()); len(got) != 1 {
		t.Fatalf("digest default: got %d releases, want 1", len(got))
	}
	if got := s.GetReleases(repoID); len(got) != 2 {
		t.Fatalf("unfiltered store view: got %d, want 2 (prereleases are stored)", len(got))
	}

	if ok, err := s.SetProjectIncludePrereleases(userID, repoID, true); err != nil || !ok {
		t.Fatalf("SetProjectIncludePrereleases ok=%v err=%v", ok, err)
	}
	got := s.GetUserReleases(userID)
	if len(got) != 2 || !got[0].Prerelease || got[0].Version != "1.1.0-rc.1" {
		t.Fatalf("opted in: got %+v, want rc first and flagged", got)
	}
	if p := s.GetUserRepos(userID); !p[0].IncludePrereleases {
		t.Fatal("GetUserRepos should report include_prereleases=true")
	}

	if ok, _ := s.SetProjectIncludePrereleases("someone-else", repoID, true); ok {
		t.Fatal("must not update another user's project")
	}
}

func TestReleasesForFiltersPerRecipient(t *testing.T) {
	s, userID, repoID, _ := prereleaseFixture(t)
	all := s.GetReleases(repoID)

	if got := s.releasesFor(userID, repoID, all); len(got) != 1 || got[0].Prerelease {
		t.Fatalf("opted out: got %+v, want stable only", got)
	}
	s.SetProjectIncludePrereleases(userID, repoID, true)
	if got := s.releasesFor(userID, repoID, all); len(got) != 2 {
		t.Fatalf("opted in: got %d, want 2", len(got))
	}
	// A user who doesn't track the repo gets stable only (no row => treated as opted out).
	if got := s.releasesFor("nobody", repoID, all); len(got) != 1 {
		t.Fatalf("unknown user: got %d, want 1", len(got))
	}
}

func TestProjectSettingsPrereleasePointerSemantics(t *testing.T) {
	s, userID, repoID, cookie := prereleaseFixture(t)
	old := store
	store = s
	defer func() { store = old }()

	post := func(body map[string]any) int {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/project-settings", bytes.NewReader(b))
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		requireAuth(handleProjectSettings).ServeHTTP(w, req)
		return w.Code
	}
	included := func() bool { return s.GetUserRepos(userID)[0].IncludePrereleases }

	if c := post(map[string]any{"repo_id": repoID, "push_enabled": true, "include_prereleases": true}); c != 200 || !included() {
		t.Fatalf("enable: code=%d included=%v", c, included())
	}
	// Older client that omits the field must not reset it.
	if c := post(map[string]any{"repo_id": repoID, "push_enabled": true}); c != 200 || !included() {
		t.Fatalf("omitted field reset the setting: code=%d included=%v", c, included())
	}
	if c := post(map[string]any{"repo_id": repoID, "push_enabled": true, "include_prereleases": false}); c != 200 || included() {
		t.Fatalf("disable: code=%d included=%v", c, included())
	}
}
