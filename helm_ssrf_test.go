package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHelmRepoClientRefusesPrivateAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	defer srv.Close()

	resp, err := fetchVia(helmRepoClient, srv.URL, 1024) // httptest listens on 127.0.0.1
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected loopback to be refused")
	}
	if !strings.Contains(err.Error(), "disallowed") {
		t.Fatalf("got %v, want the disallowed-address rejection (not a generic failure)", err)
	}
}

func TestHelmRepoClientHonoursOperatorAllowlist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	defer srv.Close()
	withAllowedHostForTests(t, "127.0.0.1")

	resp, err := fetchVia(helmRepoClient, srv.URL, 1024)
	if err != nil {
		t.Fatalf("allowlisted host should be reachable: %v", err)
	}
	resp.Body.Close()
}

func TestHelmRepoClientRefusesNonHTTPSRedirect(t *testing.T) {
	withLoopbackAllowedForTests(t) // isolate the redirect rule from the address rule
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound) // same plain-http scheme
	}))
	defer srv.Close()

	resp, err := fetchVia(helmRepoClient, srv.URL, 1024)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected redirect to a non-https URL to be refused")
	}
	if !strings.Contains(err.Error(), "non-https") {
		t.Fatalf("got %v, want the non-https redirect rejection", err)
	}
}

func TestAddHelmRepoRejectsUnsafeURLs(t *testing.T) {
	s := newTestStore(t)
	old := store
	store = s
	defer func() { store = old }()
	userID, cookie := newTestAuth(t, s)

	post := func(repoURL string) (int, string) {
		b, _ := json.Marshal(map[string]string{"name": "x", "platform": "helm-repo", "repo_url": repoURL})
		req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(b))
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		requireAuth(handleProjects).ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}

	cases := []struct{ url, want string }{
		{"http://charts.example.com", "https://"},
		{"https://127.0.0.1/charts", "disallowed"},
		{"https://169.254.169.254/latest", "disallowed"},
		{"https://10.0.0.5", "disallowed"},
	}
	for _, c := range cases {
		code, body := post(c.url)
		if code != http.StatusBadRequest || !strings.Contains(body, c.want) {
			t.Errorf("POST %s: code=%d body=%q, want 400 containing %q", c.url, code, body, c.want)
		}
	}
	if n := len(s.GetUserRepos(userID)); n != 0 {
		t.Errorf("rejected URLs still created %d project(s)", n)
	}
}
