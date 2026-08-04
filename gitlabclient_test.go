package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitLabProjectExistsAndCreateProject(t *testing.T) {
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
