# Helm Chart Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two new platforms — `helm-artifacthub` (Artifact Hub REST API) and `helm-repo` (raw Helm `index.yaml`) — so users can track Helm chart releases alongside existing platforms.

**Architecture:** Two new fetcher functions in `fetchers.go`, two new `case` entries in `store.go`'s `RefreshProject` switch, and UI additions in `ui.go`. Version is stored as `<chart_version> (app <app_version>)` in the existing `version` TEXT column — no schema changes.

**Tech Stack:** Go, `gopkg.in/yaml.v3` (new dep for index.yaml parsing), Artifact Hub REST API, Helm repo HTTP index.

---

## File Map

| File | Change |
|---|---|
| `go.mod` / `go.sum` | Add `gopkg.in/yaml.v3` |
| `fetchers.go` | Add `artifacthubBaseURL` var, `helmIndex`/`helmChartVersion` structs, `fetchHelmArtifactHub`, `fetchHelmRepo` |
| `store.go` | Add 2 cases to `RefreshProject` switch |
| `ui.go` | Dropdown options, badge CSS, platformConfig, detectPlatform, extractNameFromURL |
| `main_test.go` | Add `TestFetchHelmArtifactHub`, `TestFetchHelmRepo` |

---

## Task 1: Add yaml dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the dependency**

```bash
cd /home/jharnish/Work/newreleases && go get gopkg.in/yaml.v3
```

Expected output: a line like `go: added gopkg.in/yaml.v3 v3.0.1`

- [ ] **Step 2: Verify go.mod updated**

```bash
grep yaml go.mod
```

Expected: `gopkg.in/yaml.v3 v3.0.x`

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add gopkg.in/yaml.v3 for Helm index parsing"
```

---

## Task 2: fetchHelmArtifactHub — tests first

**Files:**
- Modify: `main_test.go`
- Modify: `fetchers.go`

- [ ] **Step 1: Write the failing tests**

Add to the bottom of `main_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -v -run "TestFetchHelmArtifactHub" ./...
```

Expected: `undefined: fetchHelmArtifactHub` or `undefined: artifacthubBaseURL`

- [ ] **Step 3: Implement fetchHelmArtifactHub in fetchers.go**

Add after the existing imports block (add `"gopkg.in/yaml.v3"` to imports at the same time — needed for Task 3 but safe to add now):

In `fetchers.go`, the existing imports are:
```go
import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)
```

Change to:
```go
import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)
```

Then add after the `truncate` function at the bottom of `fetchers.go`:

```go
var artifacthubBaseURL = "https://artifacthub.io"

func fetchHelmArtifactHub(project Project) []Release {
	path := strings.TrimPrefix(project.RepoURL, "https://artifacthub.io/packages/helm/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		log.Printf("⚠ Invalid Artifact Hub URL for %s: %s", project.Name, project.RepoURL)
		return nil
	}
	repoName, chartName := parts[0], parts[1]

	listURL := fmt.Sprintf("%s/api/v1/packages/helm/%s/%s", artifacthubBaseURL, repoName, chartName)
	resp, err := http.Get(listURL)
	if err != nil {
		log.Printf("⚠ Artifact Hub API error for %s: %v", project.Name, err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("⚠ Artifact Hub API returned %d for %s", resp.StatusCode, project.Name)
		return nil
	}

	var pkgData struct {
		Description       string `json:"description"`
		AvailableVersions []struct {
			Version    string `json:"version"`
			TS         int64  `json:"ts"`
			Prerelease bool   `json:"prerelease"`
		} `json:"available_versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pkgData); err != nil {
		log.Printf("⚠ Failed to parse Artifact Hub response for %s: %v", project.Name, err)
		return nil
	}

	type versionEntry struct {
		version    string
		ts         int64
		prerelease bool
	}

	var stable []versionEntry
	for _, v := range pkgData.AvailableVersions {
		if !v.Prerelease {
			stable = append(stable, versionEntry{v.Version, v.TS, v.Prerelease})
		}
	}
	if len(stable) == 0 {
		for _, v := range pkgData.AvailableVersions {
			stable = append(stable, versionEntry{v.Version, v.TS, v.Prerelease})
		}
	}
	if len(stable) > 10 {
		stable = stable[:10]
	}

	var releases []Release
	for _, v := range stable {
		detailURL := fmt.Sprintf("%s/api/v1/packages/helm/%s/%s/%s", artifacthubBaseURL, repoName, chartName, v.version)
		detailResp, err := http.Get(detailURL)
		if err != nil {
			log.Printf("⚠ Artifact Hub version detail error for %s@%s: %v", project.Name, v.version, err)
			continue
		}
		var detail struct {
			AppVersion string `json:"app_version"`
		}
		err = json.NewDecoder(detailResp.Body).Decode(&detail)
		detailResp.Body.Close()
		if err != nil {
			log.Printf("⚠ Failed to parse Artifact Hub version detail for %s@%s: %v", project.Name, v.version, err)
			continue
		}

		version := v.version
		if detail.AppVersion != "" {
			version = fmt.Sprintf("%s (app %s)", v.version, detail.AppVersion)
		}
		releases = append(releases, Release{
			ID:          fmt.Sprintf("helm_ah_%s", v.version),
			Name:        project.Name,
			Version:     version,
			Platform:    "helm-artifacthub",
			URL:         project.RepoURL,
			PublishedAt: time.Unix(v.ts, 0),
			Description: truncate(pkgData.Description, 200),
		})
	}

	return releases
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -v -run "TestFetchHelmArtifactHub" ./...
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add fetchers.go main_test.go
git commit -m "feat: add fetchHelmArtifactHub with tests"
```

---

## Task 3: fetchHelmRepo — tests first

**Files:**
- Modify: `main_test.go`
- Modify: `fetchers.go`

- [ ] **Step 1: Write the failing tests**

Add to the bottom of `main_test.go`:

```go
func TestFetchHelmRepo(t *testing.T) {
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	if len(releases) != 2 {
		t.Fatalf("expected 2 releases (prerelease filtered), got %d", len(releases))
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
}

func TestFetchHelmRepoChartNotFound(t *testing.T) {
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, indexYAML)
	}))
	defer server.Close()

	releases := fetchHelmRepo(Project{Name: "redis", Platform: "helm-repo", RepoURL: server.URL})

	if releases != nil {
		t.Errorf("expected nil for missing chart, got %v", releases)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -v -run "TestFetchHelmRepo" ./...
```

Expected: `undefined: fetchHelmRepo`

- [ ] **Step 3: Implement fetchHelmRepo in fetchers.go**

Add after `fetchHelmArtifactHub` in `fetchers.go`:

```go
type helmIndex struct {
	Entries map[string][]helmChartVersion `yaml:"entries"`
}

type helmChartVersion struct {
	Version    string   `yaml:"version"`
	AppVersion string   `yaml:"appVersion"`
	Created    string   `yaml:"created"`
	URLs       []string `yaml:"urls"`
}

func fetchHelmRepo(project Project) []Release {
	indexURL := strings.TrimSuffix(project.RepoURL, "/") + "/index.yaml"
	resp, err := http.Get(indexURL)
	if err != nil {
		log.Printf("⚠ Helm repo index error for %s: %v", project.Name, err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("⚠ Helm repo index returned %d for %s", resp.StatusCode, project.Name)
		return nil
	}

	var idx helmIndex
	if err := yaml.NewDecoder(resp.Body).Decode(&idx); err != nil {
		log.Printf("⚠ Failed to parse Helm index for %s: %v", project.Name, err)
		return nil
	}

	entries, ok := idx.Entries[project.Name]
	if !ok || len(entries) == 0 {
		log.Printf("⚠ Chart %q not found in Helm repo %s", project.Name, project.RepoURL)
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Created > entries[j].Created
	})

	var stable []helmChartVersion
	for _, e := range entries {
		if !strings.Contains(e.Version, "-") {
			stable = append(stable, e)
		}
	}
	if len(stable) == 0 {
		stable = entries
	}
	if len(stable) > 10 {
		stable = stable[:10]
	}

	var releases []Release
	for _, e := range stable {
		pubTime := time.Now()
		if e.Created != "" {
			pubTime, _ = time.Parse(time.RFC3339, e.Created)
		}
		url := project.RepoURL
		if len(e.URLs) > 0 {
			url = e.URLs[0]
		}
		version := e.Version
		if e.AppVersion != "" {
			version = fmt.Sprintf("%s (app %s)", e.Version, e.AppVersion)
		}
		releases = append(releases, Release{
			ID:          fmt.Sprintf("helm_repo_%s", e.Version),
			Name:        project.Name,
			Version:     version,
			Platform:    "helm-repo",
			URL:         url,
			PublishedAt: pubTime,
			Description: "Helm Chart",
		})
	}

	return releases
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -v -run "TestFetchHelmRepo" ./...
```

Expected: `PASS`

- [ ] **Step 5: Run all tests to verify nothing broken**

```bash
go test -v ./...
```

Expected: all `PASS`

- [ ] **Step 6: Commit**

```bash
git add fetchers.go main_test.go
git commit -m "feat: add fetchHelmRepo with tests"
```

---

## Task 4: Wire Helm platforms into RefreshProject

**Files:**
- Modify: `store.go`

- [ ] **Step 1: Add cases to RefreshProject switch**

In `store.go`, find the switch block (around line 1007):

```go
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
	default:
		log.Printf("⚠ Unknown platform: %s", project.Platform)
		return
	}
```

Change to:

```go
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
```

- [ ] **Step 2: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no output (success)

- [ ] **Step 3: Run all tests**

```bash
go test -v ./...
```

Expected: all `PASS`

- [ ] **Step 4: Commit**

```bash
git add store.go
git commit -m "feat: wire helm-artifacthub and helm-repo into RefreshProject"
```

---

## Task 5: UI — add Helm platform support

**Files:**
- Modify: `ui.go`

- [ ] **Step 1: Add dropdown options**

Find this block in `ui.go` (around line 520):

```html
                        <select name="platform" id="add-platform" required onchange="onPlatformChange()">
                            <option value="github">GitHub</option>
                            <option value="gitlab">GitLab</option>
                            <option value="npm">NPM</option>
                            <option value="pypi">PyPI</option>
                            <option value="docker">Docker Hub</option>
                            <option value="other">Other / Custom URL</option>
                        </select>
```

Change to:

```html
                        <select name="platform" id="add-platform" required onchange="onPlatformChange()">
                            <option value="github">GitHub</option>
                            <option value="gitlab">GitLab</option>
                            <option value="npm">NPM</option>
                            <option value="pypi">PyPI</option>
                            <option value="docker">Docker Hub</option>
                            <option value="helm-artifacthub">Helm (Artifact Hub)</option>
                            <option value="helm-repo">Helm (Repo)</option>
                            <option value="other">Other / Custom URL</option>
                        </select>
```

- [ ] **Step 2: Add badge CSS classes**

Find this block in `ui.go` (around line 286):

```css
.badge.github { background: #1e3a5f; color: #60a5fa; }
.badge.gitlab { background: #2e1f50; color: #c084fc; }
.badge.npm    { background: #103a26; color: #4ade80; }
.badge.pypi   { background: #3b2000; color: #fb923c; }
.badge.docker { background: #0c2a42; color: #38bdf8; }
```

Change to:

```css
.badge.github          { background: #1e3a5f; color: #60a5fa; }
.badge.gitlab          { background: #2e1f50; color: #c084fc; }
.badge.npm             { background: #103a26; color: #4ade80; }
.badge.pypi            { background: #3b2000; color: #fb923c; }
.badge.docker          { background: #0c2a42; color: #38bdf8; }
.badge.helm-artifacthub { background: #1a2e1a; color: #4ade80; }
.badge.helm-repo        { background: #1a2e1a; color: #86efac; }
```

- [ ] **Step 3: Add platformConfig entries**

Find this block in `ui.go` (around line 1295):

```js
    docker: {
        label: 'Image name',
        placeholder: 'e.g., nginx  or  username/image',
        hint: 'Docker Hub image name (official images: just the name)'
    },
    other: {
```

Change to:

```js
    docker: {
        label: 'Image name',
        placeholder: 'e.g., nginx  or  username/image',
        hint: 'Docker Hub image name (official images: just the name)'
    },
    'helm-artifacthub': {
        label: 'Artifact Hub URL',
        placeholder: 'https://artifacthub.io/packages/helm/bitnami/redis',
        hint: 'Paste the full Artifact Hub chart URL'
    },
    'helm-repo': {
        label: 'Repo base URL',
        placeholder: 'https://charts.bitnami.com/bitnami',
        hint: 'Base URL of the Helm repo — enter the chart name in the Name field above'
    },
    other: {
```

- [ ] **Step 4: Add detectPlatform pattern**

Find this block in `ui.go` (around line 1319):

```js
function detectPlatform(url) {
    if (/github\.com\//.test(url))           return 'github';
    if (/gitlab\.com\//.test(url))           return 'gitlab';
    if (/npmjs\.com\/package\//.test(url))   return 'npm';
    if (/pypi\.org\/project\//.test(url))    return 'pypi';
    if (/hub\.docker\.com\/r\//.test(url))   return 'docker';
    return null;
}
```

Change to:

```js
function detectPlatform(url) {
    if (/github\.com\//.test(url))                         return 'github';
    if (/gitlab\.com\//.test(url))                         return 'gitlab';
    if (/npmjs\.com\/package\//.test(url))                 return 'npm';
    if (/pypi\.org\/project\//.test(url))                  return 'pypi';
    if (/hub\.docker\.com\/r\//.test(url))                 return 'docker';
    if (/artifacthub\.io\/packages\/helm\//.test(url))     return 'helm-artifacthub';
    return null;
}
```

- [ ] **Step 5: Add extractNameFromURL case**

Find this block in `ui.go` (around line 1341):

```js
        case 'docker':
            return url.replace(/^.*hub\.docker\.com\/r\//, '').replace(/\/.*$/, '');
    }
    return '';
}
```

Change to:

```js
        case 'docker':
            return url.replace(/^.*hub\.docker\.com\/r\//, '').replace(/\/.*$/, '');
        case 'helm-artifacthub':
            return url.replace(/^.*artifacthub\.io\/packages\/helm\/[^/]+\//, '').split('/')[0];
    }
    return '';
}
```

- [ ] **Step 6: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no output (success)

- [ ] **Step 7: Run all tests**

```bash
go test -v ./...
```

Expected: all `PASS`

- [ ] **Step 8: Commit**

```bash
git add ui.go
git commit -m "feat: add Helm platform options to UI"
```

---

## Task 6: Manual smoke test

- [ ] **Step 1: Start the server**

```bash
go run . &
```

- [ ] **Step 2: Open browser at http://localhost:8080**

Log in (or register), click Add Project, and verify:
- Platform dropdown shows `Helm (Artifact Hub)` and `Helm (Repo)` options
- Selecting `Helm (Artifact Hub)` shows label "Artifact Hub URL" with the correct placeholder
- Selecting `Helm (Repo)` shows label "Repo base URL" with the correct placeholder
- Pasting `https://artifacthub.io/packages/helm/bitnami/redis` into the URL field auto-fills the Name as `redis` and selects `Helm (Artifact Hub)`

- [ ] **Step 3: Add a Helm Artifact Hub chart**

Add platform `Helm (Artifact Hub)`, name `redis`, URL `https://artifacthub.io/packages/helm/bitnami/redis`. Verify releases appear with versions like `17.x.x (app 7.x.x)` and badges show `helm-artifacthub`.

- [ ] **Step 4: Add a Helm repo chart**

Add platform `Helm (Repo)`, name `redis`, URL `https://charts.bitnami.com/bitnami`. Verify releases appear.

- [ ] **Step 5: Stop the server**

```bash
kill %1
```
