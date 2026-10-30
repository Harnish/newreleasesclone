package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func tinyPNG() []byte {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	return b.Bytes()
}

func TestIconURLFor(t *testing.T) {
	cases := []struct {
		p    Project
		want string
	}{
		{Project{Platform: "github", RepoURL: "https://github.com/kubernetes/kubernetes"}, "https://github.com/kubernetes.png?size=64"},
		{Project{Platform: "github", RepoURL: "https://evil.example.com/kubernetes/kubernetes"}, ""}, // only github.com
		{Project{Platform: "github", RepoURL: "not a url"}, ""},
		{Project{Platform: "npm", RepoURL: "https://www.npmjs.com/package/react"}, ""},
		{Project{Platform: "pypi", RepoURL: "https://pypi.org/project/numpy/"}, ""},
		{Project{Platform: "docker", RepoURL: "https://hub.docker.com/r/library/redis"}, ""},
	}
	for _, c := range cases {
		if got := iconURLFor(c.p); got != c.want {
			t.Errorf("iconURLFor(%s %s) = %q, want %q", c.p.Platform, c.p.RepoURL, got, c.want)
		}
	}
}

func TestFetchIconValidatesContent(t *testing.T) {
	withLoopbackAllowedForTests(t)
	big := bytes.Repeat([]byte{0}, maxIconBytes+1)
	serve := map[string]struct {
		ctype string
		body  []byte
	}{
		"/png-lying-header": {"text/html", tinyPNG()}, // type comes from content, not the header
		"/svg":              {"image/svg+xml", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`)},
		"/svg-undeclared":   {"text/plain", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)},
		"/html":             {"image/png", []byte(`<html><script>alert(1)</script></html>`)},
		"/huge":             {"image/png", big},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := serve[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", c.ctype)
		w.Write(c.body)
	}))
	defer srv.Close()

	if ct, data, err := fetchIcon(srv.URL+"/png-lying-header", ""); err != nil || ct != "image/png" || len(data) == 0 {
		t.Errorf("png: ct=%q err=%v", ct, err)
	}
	if ct, _, err := fetchIcon(srv.URL+"/svg", ""); err != nil || ct != "image/svg+xml" {
		t.Errorf("svg: ct=%q err=%v", ct, err)
	}
	for _, path := range []string{"/svg-undeclared", "/html", "/huge", "/missing"} {
		if _, _, err := fetchIcon(srv.URL+path, ""); err == nil {
			t.Errorf("%s: expected rejection", path)
		}
	}
	for _, bad := range []string{"ftp://example.com/x.png", "file:///etc/passwd", "javascript:alert(1)", ""} {
		if _, _, err := fetchIcon(bad, ""); err == nil {
			t.Errorf("fetchIcon(%q): expected rejection", bad)
		}
	}
}

func TestFetchIconRefusesInternalAddresses(t *testing.T) {
	// No loopback exemption: the guard must stop the download itself.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(tinyPNG()) }))
	defer srv.Close()
	_, _, err := fetchIcon(srv.URL+"/x.png", "")
	if err == nil || !strings.Contains(err.Error(), "disallowed") {
		t.Fatalf("got %v, want the disallowed-address rejection", err)
	}
}

func TestHelmRepoIconFromIndex(t *testing.T) {
	defer disableHTTPSVerificationForTest()()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `apiVersion: v1
entries:
  redis:
  - version: "17.0.0"
    created: "2023-10-01T00:00:00Z"
    icon: https://charts.example.com/redis.png
  other:
  - version: "1.0.0"
    icon: https://charts.example.com/other.png
`)
	}))
	defer srv.Close()
	got := iconURLFor(Project{Name: "redis", Platform: "helm-repo", RepoURL: srv.URL})
	if got != "https://charts.example.com/redis.png" {
		t.Fatalf("got %q", got)
	}
}

func iconFixture(t *testing.T) (*Store, string, string, *http.Cookie, Project) {
	t.Helper()
	s := newTestStore(t)
	userID, cookie := newTestAuth(t, s)
	p := Project{Name: "mychart", Platform: "helm-artifacthub", RepoURL: "https://artifacthub.io/packages/helm/org/mychart"}
	repoID, err := s.AddRepo(userID, p)
	if err != nil {
		t.Fatal(err)
	}
	p.ID = repoID
	return s, userID, repoID, cookie, p
}

func TestRefreshIconStoresAndExposesVersion(t *testing.T) {
	withLoopbackAllowedForTests(t)
	s, userID, repoID, _, p := iconFixture(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/packages/helm/org/mychart", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"logo_image_id": "abc123"}`)
	})
	mux.HandleFunc("/image/abc123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(tinyPNG())
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	old := artifacthubBaseURL
	artifacthubBaseURL = srv.URL
	defer func() { artifacthubBaseURL = old }()

	if repos := s.GetUserRepos(userID); repos[0].IconVersion != 0 {
		t.Fatalf("icon_version = %d before any lookup, want 0", repos[0].IconVersion)
	}
	s.refreshIcon(p)
	ct, data, ok := s.GetIcon(repoID)
	if !ok || ct != "image/png" || len(data) == 0 {
		t.Fatalf("GetIcon ok=%v ct=%q len=%d", ok, ct, len(data))
	}
	if repos := s.GetUserRepos(userID); repos[0].IconVersion == 0 {
		t.Fatal("icon_version should be set once an icon is stored")
	}
	if s.iconStale(repoID) {
		t.Fatal("a just-fetched icon should not be stale")
	}
}

func TestSaveIconKeepsExistingOnFailedLookup(t *testing.T) {
	s, _, repoID, _, _ := iconFixture(t)
	s.saveIcon(repoID, "image/png", tinyPNG())
	s.saveIcon(repoID, "", nil) // a later lookup that found nothing
	if _, data, ok := s.GetIcon(repoID); !ok || len(data) == 0 {
		t.Fatal("a failed lookup erased a good icon")
	}
}

func TestIconStalenessWindows(t *testing.T) {
	s, _, repoID, _, _ := iconFixture(t)
	if !s.iconStale(repoID) {
		t.Fatal("never-checked repo should be stale")
	}
	age := func(d time.Duration) {
		s.db.Exec("UPDATE repo_icons SET checked_at = ? WHERE repo_id = ?", time.Now().Add(-d).Unix(), repoID)
	}
	s.saveIcon(repoID, "", nil) // checked, found nothing
	age(2 * time.Hour)
	if s.iconStale(repoID) {
		t.Fatal("empty lookup 2h ago should not be retried yet")
	}
	age(25 * time.Hour)
	if !s.iconStale(repoID) {
		t.Fatal("empty lookup 25h ago should be retried")
	}
	s.saveIcon(repoID, "image/png", tinyPNG())
	age(3 * 24 * time.Hour)
	if s.iconStale(repoID) {
		t.Fatal("icon fetched 3 days ago should not be rechecked")
	}
	age(8 * 24 * time.Hour)
	if !s.iconStale(repoID) {
		t.Fatal("icon fetched 8 days ago should be rechecked")
	}
}

func TestHandleIcon(t *testing.T) {
	s, _, repoID, cookie, _ := iconFixture(t)
	old := store
	store = s
	defer func() { store = old }()

	get := func(id string, withCookie bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/icon?id="+id, nil)
		if withCookie {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		requireAuth(handleIcon).ServeHTTP(w, req)
		return w
	}

	if w := get(repoID, true); w.Code != http.StatusNotFound {
		t.Fatalf("no icon stored: code %d, want 404", w.Code)
	}
	s.saveIcon(repoID, "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	w := get(repoID, true)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("code=%d ct=%q", w.Code, w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("missing hardening headers: %v", w.Header())
	}
	if w := get("some-repo-the-user-does-not-track", true); w.Code != http.StatusNotFound {
		t.Errorf("untracked repo: code %d, want 404", w.Code)
	}
	if w := get(repoID, false); w.Code != http.StatusUnauthorized {
		t.Errorf("no session: code %d, want 401", w.Code)
	}
}

func TestFetchIconIgnoresAllowlistUnlessSameHostAsRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(tinyPNG()) }))
	defer srv.Close()
	withAllowedHostForTests(t, "127.0.0.1") // operator allowlisted this host (and loopback is NOT otherwise permitted)

	// An icon URL from third-party content must not inherit the exemption.
	if _, _, err := fetchIcon(srv.URL+"/x.png", ""); err == nil || !strings.Contains(err.Error(), "disallowed") {
		t.Fatalf("icon on an allowlisted host with no trust: got %v, want the disallowed-address rejection", err)
	}
	// A different repo host does not vouch for it either.
	if _, _, err := fetchIcon(srv.URL+"/x.png", "charts.example.com"); err == nil {
		t.Fatal("icon on a host other than the repo's must stay strict")
	}
	// The repo's own host, allowlisted by the operator, may serve its own icon.
	if ct, _, err := fetchIcon(srv.URL+"/x.png", "127.0.0.1"); err != nil || ct != "image/png" {
		t.Fatalf("same-host icon: ct=%q err=%v", ct, err)
	}
}
