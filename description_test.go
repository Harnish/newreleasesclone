package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchDescriptionPerPlatform(t *testing.T) {
	withLoopbackAllowedForTests(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/left-pad/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"description": "  pads strings  "}`)
	})
	mux.HandleFunc("/pypi/requests/json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"info": {"summary": "HTTP for humans"}}`)
	})
	mux.HandleFunc("/api/v1/packages/helm/org/mychart", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"description": "A chart"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	oldN, oldP, oldA := npmRegistryURL, pypiBaseURL, artifacthubBaseURL
	npmRegistryURL, pypiBaseURL, artifacthubBaseURL = srv.URL, srv.URL, srv.URL
	defer func() { npmRegistryURL, pypiBaseURL, artifacthubBaseURL = oldN, oldP, oldA }()

	for _, c := range []struct {
		p    Project
		want string
	}{
		{Project{Name: "left-pad", Platform: "npm"}, "pads strings"},
		{Project{Name: "requests", Platform: "pypi"}, "HTTP for humans"},
		{Project{Name: "mychart", Platform: "helm-artifacthub", RepoURL: "https://artifacthub.io/packages/helm/org/mychart"}, "A chart"},
		{Project{Name: "missing", Platform: "npm"}, ""}, // 404 -> empty, no panic
	} {
		if got := fetchDescription(c.p); got != c.want {
			t.Errorf("%s/%s: got %q, want %q", c.p.Platform, c.p.Name, got, c.want)
		}
	}
}

func TestFetchDescriptionHelmRepoAndCap(t *testing.T) {
	defer disableHTTPSVerificationForTest()()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "apiVersion: v1\nentries:\n  redis:\n  - version: \"1.0.0\"\n    created: \"2023-10-01T00:00:00Z\"\n    description: %q\n", strings.Repeat("x", 800))
	}))
	defer srv.Close()
	got := fetchDescription(Project{Name: "redis", Platform: "helm-repo", RepoURL: srv.URL})
	if len(got) != maxDescription+len("...") {
		t.Fatalf("len = %d, want capped at %d", len(got), maxDescription+3)
	}
}

func TestRefreshDescriptionStoresAndThrottles(t *testing.T) {
	withLoopbackAllowedForTests(t)
	s, userID, repoID, _, p := iconFixture(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"description": "A chart"}`)
	}))
	defer srv.Close()
	old := artifacthubBaseURL
	artifacthubBaseURL = srv.URL
	defer func() { artifacthubBaseURL = old }()

	if got := s.GetUserRepos(userID)[0].Description; got != "" {
		t.Fatalf("description = %q before lookup", got)
	}
	s.refreshDescription(p)
	if got := s.GetUserRepos(userID)[0].Description; got != "A chart" {
		t.Fatalf("description = %q after lookup", got)
	}
	s.refreshDescription(p) // fresh: must not hit upstream again
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1 (throttled)", calls)
	}

	// A failed later lookup keeps the stored description.
	s.db.Exec("UPDATE repos SET description_checked_at = 0 WHERE id = ?", repoID)
	artifacthubBaseURL = "http://127.0.0.1:1"
	s.refreshDescription(p)
	if got := s.GetUserRepos(userID)[0].Description; got != "A chart" {
		t.Fatalf("failed lookup erased the description: %q", got)
	}
}
