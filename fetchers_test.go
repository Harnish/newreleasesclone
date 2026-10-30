package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestTruncate tests the truncate function
func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a ..."},
		{"exactly", 7, "exactly"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

// BenchmarkTruncate benchmarks the truncate function
func BenchmarkTruncate(b *testing.B) {
	longString := strings.Repeat("a", 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = truncate(longString, 200)
	}
}

func TestFetchHelmArtifactHub(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/packages/helm/bitnami/redis", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        "redis",
			"description": "Redis chart",
			"available_versions": []map[string]interface{}{
				{"version": "17.0.0", "ts": int64(1699000000), "prerelease": false},
				{"version": "16.9.0", "ts": int64(1698000000), "prerelease": false},
				{"version": "16.0.0-beta.1", "ts": int64(1697000000), "prerelease": true},
			},
		})
	})
	mux.HandleFunc("/api/v1/packages/helm/bitnami/redis/17.0.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"version": "17.0.0", "app_version": "7.2.1"})
	})
	mux.HandleFunc("/api/v1/packages/helm/bitnami/redis/16.9.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"version": "16.9.0", "app_version": "7.0.5"})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	origBase := artifacthubBaseURL
	artifacthubBaseURL = server.URL
	defer func() { artifacthubBaseURL = origBase }()

	project := Project{
		Name:     "redis",
		Platform: "helm-artifacthub",
		RepoURL:  "https://artifacthub.io/packages/helm/bitnami/redis",
	}

	releases := fetchHelmArtifactHub(project)

	if len(releases) != 2 {
		t.Fatalf("expected 2 releases (prerelease filtered), got %d", len(releases))
	}
	if releases[0].Version != "17.0.0 (app 7.2.1)" {
		t.Errorf("expected version %q, got %q", "17.0.0 (app 7.2.1)", releases[0].Version)
	}
	if releases[0].ID != "helm_ah_17.0.0" {
		t.Errorf("expected ID %q, got %q", "helm_ah_17.0.0", releases[0].ID)
	}
	if releases[1].Version != "16.9.0 (app 7.0.5)" {
		t.Errorf("expected version %q, got %q", "16.9.0 (app 7.0.5)", releases[1].Version)
	}
	if releases[0].Platform != "helm-artifacthub" {
		t.Errorf("expected platform %q, got %q", "helm-artifacthub", releases[0].Platform)
	}
}

func TestFetchHelmArtifactHubPrereleaseOnlyFallback(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/packages/helm/org/chart", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        "chart",
			"description": "",
			"available_versions": []map[string]interface{}{
				{"version": "1.0.0-alpha.1", "ts": int64(1699000000), "prerelease": true},
			},
		})
	})
	mux.HandleFunc("/api/v1/packages/helm/org/chart/1.0.0-alpha.1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0-alpha.1", "app_version": "1.0.0"})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	origBase := artifacthubBaseURL
	artifacthubBaseURL = server.URL
	defer func() { artifacthubBaseURL = origBase }()

	releases := fetchHelmArtifactHub(Project{
		Name:     "chart",
		Platform: "helm-artifacthub",
		RepoURL:  "https://artifacthub.io/packages/helm/org/chart",
	})

	if len(releases) != 1 {
		t.Fatalf("expected 1 release (fallback to prerelease), got %d", len(releases))
	}
	if releases[0].Version != "1.0.0-alpha.1 (app 1.0.0)" {
		t.Errorf("got %q", releases[0].Version)
	}
}

func TestFetchHelmRepo(t *testing.T) {
	defer disableHTTPSVerificationForTest()()
	indexYAML := `apiVersion: v1
entries:
  redis:
  - version: "17.0.0"
    appVersion: "7.2.1"
    created: "2023-10-01T00:00:00Z"
    urls:
    - https://charts.example.com/redis-17.0.0.tgz
  - version: "16.9.0"
    appVersion: "7.0.5"
    created: "2023-09-01T00:00:00Z"
    urls:
    - https://charts.example.com/redis-16.9.0.tgz
  - version: "16.0.0-beta.1"
    appVersion: "7.0.0"
    created: "2023-08-01T00:00:00Z"
    urls:
    - https://charts.example.com/redis-16.0.0-beta.1.tgz
generated: "2023-10-01T00:00:00Z"
`
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.yaml" {
			w.Header().Set("Content-Type", "application/yaml")
			fmt.Fprint(w, indexYAML)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	project := Project{
		Name:     "redis",
		Platform: "helm-repo",
		RepoURL:  server.URL,
	}

	releases := fetchHelmRepo(project)

	if len(releases) != 3 {
		t.Fatalf("expected 3 releases (prerelease kept, flagged), got %d", len(releases))
	}
	if releases[0].Prerelease || releases[1].Prerelease || !releases[2].Prerelease || releases[2].Version != "16.0.0-beta.1 (app 7.0.0)" {
		t.Errorf("expected the beta last and flagged, got %+v", releases)
	}
	if releases[0].Version != "17.0.0 (app 7.2.1)" {
		t.Errorf("expected version %q, got %q", "17.0.0 (app 7.2.1)", releases[0].Version)
	}
	if releases[0].ID != "helm_repo_17.0.0" {
		t.Errorf("expected ID %q, got %q", "helm_repo_17.0.0", releases[0].ID)
	}
	if releases[1].Version != "16.9.0 (app 7.0.5)" {
		t.Errorf("expected version %q, got %q", "16.9.0 (app 7.0.5)", releases[1].Version)
	}
	if releases[0].URL != "https://charts.example.com/redis-17.0.0.tgz" {
		t.Errorf("expected URL from index, got %q", releases[0].URL)
	}
	if releases[0].Platform != "helm-repo" {
		t.Errorf("expected platform %q, got %q", "helm-repo", releases[0].Platform)
	}
	if releases[0].Description != "Helm Chart" {
		t.Errorf("expected description %q, got %q", "Helm Chart", releases[0].Description)
	}
}

func TestFetchHelmRepoChartNotFound(t *testing.T) {
	defer disableHTTPSVerificationForTest()()
	indexYAML := `apiVersion: v1
entries:
  nginx:
  - version: "1.0.0"
    appVersion: "1.25.0"
    created: "2023-10-01T00:00:00Z"
    urls:
    - https://charts.example.com/nginx-1.0.0.tgz
generated: "2023-10-01T00:00:00Z"
`
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, indexYAML)
	}))
	defer server.Close()

	releases := fetchHelmRepo(Project{Name: "redis", Platform: "helm-repo", RepoURL: server.URL})

	if releases != nil {
		t.Errorf("expected nil for missing chart, got %v", releases)
	}
}

func TestFetchHelmArtifactHubDetailError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/packages/helm/org/mychart", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        "mychart",
			"description": "",
			"available_versions": []map[string]interface{}{
				{"version": "1.0.0", "ts": int64(1699000000), "prerelease": false},
			},
		})
	})
	// detail endpoint returns 404
	mux.HandleFunc("/api/v1/packages/helm/org/mychart/1.0.0", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "not found"})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	origBase := artifacthubBaseURL
	artifacthubBaseURL = server.URL
	defer func() { artifacthubBaseURL = origBase }()

	releases := fetchHelmArtifactHub(Project{
		Name:     "mychart",
		Platform: "helm-artifacthub",
		RepoURL:  "https://artifacthub.io/packages/helm/org/mychart",
	})

	if len(releases) != 0 {
		t.Errorf("expected 0 releases when detail call returns 404, got %d", len(releases))
	}
}
