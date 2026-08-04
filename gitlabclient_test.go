package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withLoopbackAllowedForTests lets a test's httptest.NewServer (always on
// 127.0.0.1) pass the production dial-time SSRF guard, scoped to just this
// test via t.Cleanup.
func withLoopbackAllowedForTests(t *testing.T) {
	t.Helper()
	allowLoopbackInTests.Store(true)
	t.Cleanup(func() { allowLoopbackInTests.Store(false) })
}

func TestGitLabProjectExistsAndCreateProject(t *testing.T) {
	withLoopbackAllowedForTests(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/my-repo", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"http_url_to_repo": "https://example.com/user/my-repo.git",
		})
	})
	mux.HandleFunc("/api/v4/projects/missing-repo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	var createdBody map[string]any
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&createdBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"http_url_to_repo": "https://example.com/user/new-repo.git",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newGitLabClient(srv.URL, "tok")

	exists, err := c.ProjectExists("my-repo")
	if err != nil || !exists {
		t.Fatalf("ProjectExists() = (%v, %v), want (true, nil)", exists, err)
	}
	exists, err = c.ProjectExists("missing-repo")
	if err != nil || exists {
		t.Fatalf("ProjectExists() = (%v, %v), want (false, nil)", exists, err)
	}

	httpURL, err := c.CreateProject("new-repo")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if httpURL != "https://example.com/user/new-repo.git" {
		t.Errorf("CreateProject() = %q, want the http_url_to_repo value", httpURL)
	}
	if _, hasNamespace := createdBody["namespace_id"]; hasNamespace {
		t.Errorf("CreateProject() sent namespace_id = %v, want it omitted (personal namespace default)", createdBody["namespace_id"])
	}

	got, err := c.GetProjectHTTPURL("my-repo")
	if err != nil || got != "https://example.com/user/my-repo.git" {
		t.Fatalf("GetProjectHTTPURL() = (%q, %v), want the http_url_to_repo value", got, err)
	}
}

func TestGitLabAuthenticatedPushURL(t *testing.T) {
	c := newGitLabClient("https://gitlab.example.com", "glpat-secret")

	got, err := c.AuthenticatedPushURL("https://gitlab.example.com/user/repo.git")
	if err != nil {
		t.Fatalf("AuthenticatedPushURL() error = %v", err)
	}
	want := "https://oauth2:glpat-secret@gitlab.example.com/user/repo.git"
	if got != want {
		t.Errorf("AuthenticatedPushURL() = %q, want %q", got, want)
	}

	if _, err := c.AuthenticatedPushURL("://not a url"); err == nil {
		t.Error("AuthenticatedPushURL() error = nil, want parse error")
	}
}

func TestValidateGitLabURLRejectsInternalAddresses(t *testing.T) {
	bad := []string{
		"http://127.0.0.1/",
		"http://localhost/",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.5/",
		"http://192.168.1.1/",
		"http://[::1]/",
		"ftp://gitlab.example.com",
		"not-a-url",
	}
	for _, u := range bad {
		if err := validateGitLabURL(u); err == nil {
			t.Errorf("validateGitLabURL(%q) = nil, want rejection", u)
		}
	}
}

func TestValidateGitLabURLAcceptsPublicHost(t *testing.T) {
	if err := validateGitLabURL("https://gitlab.com"); err != nil {
		t.Errorf("validateGitLabURL(gitlab.com) = %v, want nil for a public host", err)
	}
}

func TestIsDisallowedIPExpandedRanges(t *testing.T) {
	disallowed := []string{
		"100.64.0.1",     // CGNAT (100.64.0.0/10)
		"100.127.255.254", // top of CGNAT range
		"0.0.0.1",         // 0.0.0.0/8 "this network"
		"::ffff:127.0.0.1", // IPv4-mapped IPv6 loopback
		"::ffff:169.254.169.254", // IPv4-mapped IPv6 link-local
	}
	for _, s := range disallowed {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("net.ParseIP(%q) = nil", s)
		}
		if !isDisallowedIP(ip) {
			t.Errorf("isDisallowedIP(%q) = false, want true", s)
		}
	}

	allowed := []string{"8.8.8.8", "1.1.1.1"}
	for _, s := range allowed {
		ip := net.ParseIP(s)
		if isDisallowedIP(ip) {
			t.Errorf("isDisallowedIP(%q) = true, want false (public address)", s)
		}
	}
}

// TestGitLabClientRefusesToDialDisallowedIP proves the connection-time
// guard (not just the registration-time validateGitLabURL check) actually
// blocks requests. This is the layer that closes the DNS-rebinding/TOCTOU
// gap: a hostname could resolve to a public IP when validateGitLabURL ran
// and to an internal one by the time a request actually connects.
// newGitLabClient is called directly here (bypassing validateGitLabURL
// entirely) to isolate and prove this second layer on its own.
func TestGitLabClientRefusesToDialDisallowedIP(t *testing.T) {
	c := newGitLabClient("http://127.0.0.1:1", "tok")
	_, err := c.ProjectExists("whatever")
	if err == nil {
		t.Fatal("ProjectExists() against 127.0.0.1 = nil error, want the dial rejected")
	}
	if !strings.Contains(err.Error(), "disallowed") {
		t.Errorf("error = %v, want it to mention the disallowed-address rejection specifically (not e.g. a generic connection-refused)", err)
	}
}

// withAllowedHostForTests swaps gitLabAllowedHosts to trust exactly host,
// scoped to this test via t.Cleanup. Mirrors withLoopbackAllowedForTests's
// atomic-swap pattern so concurrent dial-time reads stay race-free.
func withAllowedHostForTests(t *testing.T, host string) {
	t.Helper()
	old := gitLabAllowedHosts.Load()
	m := map[string]bool{strings.ToLower(host): true}
	gitLabAllowedHosts.Store(&m)
	t.Cleanup(func() { gitLabAllowedHosts.Store(old) })
}

func TestParseAllowedHosts(t *testing.T) {
	got := parseAllowedHosts(" Gitlab.Example.com , other.host ,, ")
	want := map[string]bool{"gitlab.example.com": true, "other.host": true}
	if len(got) != len(want) {
		t.Fatalf("parseAllowedHosts() = %v, want %v", got, want)
	}
	for h := range want {
		if !got[h] {
			t.Errorf("parseAllowedHosts() missing %q", h)
		}
	}
}

func TestParseAllowedHostsEmpty(t *testing.T) {
	got := parseAllowedHosts("")
	if len(got) != 0 {
		t.Errorf("parseAllowedHosts(\"\") = %v, want empty", got)
	}
}

// TestValidateGitLabURLAllowsAllowlistedHost proves an operator-allowlisted
// hostname bypasses the IP-based rejection even when it resolves to a
// disallowed address (localhost -> 127.0.0.1) — the split-horizon-DNS /
// same-cluster-service-name case this allowlist exists for.
func TestValidateGitLabURLAllowsAllowlistedHost(t *testing.T) {
	withAllowedHostForTests(t, "localhost")
	if err := validateGitLabURL("http://localhost"); err != nil {
		t.Errorf("validateGitLabURL(localhost) with host allowlisted = %v, want nil", err)
	}
}

// TestGitLabAllowedHostsBypassesDialGuard proves the allowlist is also
// consulted at dial time (not just registration) — the actual security
// boundary. If it works, dialing localhost:1 (nothing listening) fails on
// connection-refused, NOT on our "disallowed address" rejection.
func TestGitLabAllowedHostsBypassesDialGuard(t *testing.T) {
	withAllowedHostForTests(t, "localhost")
	c := newGitLabClient("http://localhost:1", "tok")
	_, err := c.ProjectExists("whatever")
	if err == nil {
		t.Fatal("ProjectExists() against a closed port = nil error, want a connection error")
	}
	if strings.Contains(err.Error(), "disallowed") {
		t.Errorf("error = %v, want the allowlisted host to bypass the dial guard entirely", err)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct{ in, want string }{
		{"My Repo", "my-repo"},
		{"kubernetes", "kubernetes"},
		{"foo_bar/baz", "foo-bar-baz"},
		{"  leading and trailing  ", "leading-and-trailing"},
	}
	for _, tt := range tests {
		if got := slugify(tt.in); got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
