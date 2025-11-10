package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Release struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Version     string    `json:"version" yaml:"version"`
	Platform    string    `json:"platform" yaml:"platform"`
	URL         string    `json:"url" yaml:"url"`
	PublishedAt time.Time `json:"published_at" yaml:"published_at"`
	Description string    `json:"description" yaml:"description"`
}

type Project struct {
	ID           string    `json:"id" yaml:"id"`
	Name         string    `json:"name" yaml:"name"`
	Platform     string    `json:"platform" yaml:"platform"`
	RepoURL      string    `json:"repo_url" yaml:"repo_url"`
	LastRefresh  time.Time `json:"last_refresh" yaml:"last_refresh"`
	RefreshCount int       `json:"refresh_count" yaml:"refresh_count"`
}

// StateFile represents the persisted state on disk
type StateFile struct {
	Projects map[string]Project    `yaml:"projects"`
	Releases map[string][]Release  `yaml:"releases"`
	Refreshed map[string]time.Time `yaml:"refreshed"`
}

type Store struct {
	mu        sync.RWMutex
	projects  map[string]Project
	releases  map[string][]Release
	refreshed map[string]time.Time // Track when each project was last refreshed
	stateFile string                // Path to state.yaml for auto-save
}

func NewStore() *Store {
	return &Store{
		projects:  make(map[string]Project),
		releases:  make(map[string][]Release),
		refreshed: make(map[string]time.Time),
		stateFile: "state.yaml",
	}
}

func (s *Store) AddProject(p Project) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects[p.ID] = p
	go s.asyncSave() // Auto-save in background
}

func (s *Store) GetProjects() []Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ps []Project
	for _, p := range s.projects {
		ps = append(ps, p)
	}
	return ps
}

func (s *Store) AddRelease(projectID string, r Release) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releases[projectID] = append([]Release{r}, s.releases[projectID]...)
	if len(s.releases[projectID]) > 50 {
		s.releases[projectID] = s.releases[projectID][:50]
	}
	go s.asyncSave() // Auto-save in background
}

func (s *Store) GetReleases(projectID string) []Release {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.releases[projectID]
}

func (s *Store) GetAllReleases() []Release {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var all []Release
	for _, rs := range s.releases {
		all = append(all, rs...)
	}
	return all
}

// MarkRefreshed marks a project as refreshed now
func (s *Store) MarkRefreshed(projectID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshed[projectID] = time.Now()
	p := s.projects[projectID]
	p.LastRefresh = time.Now()
	p.RefreshCount++
	s.projects[projectID] = p
	go s.asyncSave() // Auto-save in background
}

// IsStale checks if a project's data is older than 30 minutes
func (s *Store) IsStale(projectID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lastRefresh, exists := s.refreshed[projectID]
	if !exists {
		return true // Never refreshed
	}
	return time.Since(lastRefresh) > 30*time.Minute
}

// GetStaleProjects returns all projects that need refresh
func (s *Store) GetStaleProjects() []Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var stale []Project
	for id, p := range s.projects {
		lastRefresh, exists := s.refreshed[id]
		if !exists || time.Since(lastRefresh) > 30*time.Minute {
			stale = append(stale, p)
		}
	}
	return stale
}

// RefreshProject fetches the latest release data from real APIs
func (s *Store) RefreshProject(projectID string) {
	s.mu.RLock()
	project, exists := s.projects[projectID]
	s.mu.RUnlock()

	if !exists {
		return
	}

	var releases []Release

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

	if len(releases) == 0 {
		log.Printf("⚠ No releases found for %s (%s)", project.Name, project.Platform)
		return
	}

	// Clear old releases and add new ones
	s.mu.Lock()
	s.releases[projectID] = releases
	s.mu.Unlock()

	s.MarkRefreshed(projectID)
	log.Printf("✓ Refreshed %s: found %d releases, latest %s", project.Name, len(releases), releases[0].Version)
}

// fetchGitHubReleases fetches latest releases from GitHub API
func fetchGitHubReleases(project Project) []Release {
	// Extract owner/repo from URL
	repoPath := strings.TrimPrefix(project.RepoURL, "https://github.com/")
	repoPath = strings.TrimSuffix(repoPath, ".git")

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=30", repoPath)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("⚠ GitHub API error for %s: %v", project.Name, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("⚠ GitHub API returned %d for %s", resp.StatusCode, project.Name)
		return nil
	}

	var ghReleases []struct {
		ID          int    `json:"id"`
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Draft       bool   `json:"draft"`
		Prerelease  bool   `json:"prerelease"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		HtmlUrl     string `json:"html_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghReleases); err != nil {
		log.Printf("⚠ Failed to parse GitHub response for %s: %v", project.Name, err)
		return nil
	}

	var releases []Release
	// First, try to get stable releases (no draft, no prerelease)
	for _, gh := range ghReleases {
		if gh.Draft {
			continue
		}
		if gh.Prerelease {
			continue // Skip prerelease on first pass
		}

		pubTime, _ := time.Parse(time.RFC3339, gh.PublishedAt)
		releases = append(releases, Release{
			ID:          fmt.Sprintf("gh_%d", gh.ID),
			Name:        project.Name,
			Version:     gh.TagName,
			Platform:    "github",
			URL:         gh.HtmlUrl,
			PublishedAt: pubTime,
			Description: truncate(gh.Body, 200),
		})
	}

	// If no stable releases found, include prerelease
	if len(releases) == 0 {
		for _, gh := range ghReleases {
			if gh.Draft {
				continue
			}

			pubTime, _ := time.Parse(time.RFC3339, gh.PublishedAt)
			releases = append(releases, Release{
				ID:          fmt.Sprintf("gh_%d", gh.ID),
				Name:        project.Name,
				Version:     gh.TagName,
				Platform:    "github",
				URL:         gh.HtmlUrl,
				PublishedAt: pubTime,
				Description: truncate(gh.Body, 200),
			})
		}
	}

	return releases
}

// fetchNPMVersions fetches latest versions from NPM registry
func fetchNPMVersions(project Project) []Release {
	// Extract package name from repo URL or use project name
	pkgName := project.Name
	if strings.Contains(project.RepoURL, "github.com/facebook/react") {
		pkgName = "react"
	}

	url := fmt.Sprintf("https://registry.npmjs.org/%s", pkgName)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("⚠ NPM API error for %s: %v", project.Name, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("⚠ NPM API returned %d for %s", resp.StatusCode, project.Name)
		return nil
	}

	var npmData struct {
		Versions map[string]struct {
			Version string `json:"version"`
		} `json:"versions"`
		DistTags struct {
			Latest string `json:"latest"`
		} `json:"dist-tags"`
		Time map[string]string `json:"time"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&npmData); err != nil {
		log.Printf("⚠ Failed to parse NPM response for %s: %v", project.Name, err)
		return nil
	}

	var versions []string
	for v := range npmData.Versions {
		versions = append(versions, v)
	}

	// Sort versions in descending order
	sort.Slice(versions, func(i, j int) bool {
		return versions[i] > versions[j]
	})

	var releases []Release
	for i, v := range versions {
		if i >= 10 {
			break // Limit to 10 versions
		}

		pubTime := time.Now()
		if t, ok := npmData.Time[v]; ok {
			pubTime, _ = time.Parse(time.RFC3339, t)
		}

		releases = append(releases, Release{
			ID:          fmt.Sprintf("npm_%s", v),
			Name:        project.Name,
			Version:     v,
			Platform:    "npm",
			URL:         fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", pkgName, v),
			PublishedAt: pubTime,
			Description: "NPM Package Version",
		})
	}

	return releases
}

// fetchPyPIReleases fetches releases from PyPI
func fetchPyPIReleases(project Project) []Release {
	pkgName := project.Name

	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", pkgName)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("⚠ PyPI API error for %s: %v", project.Name, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("⚠ PyPI API returned %d for %s", resp.StatusCode, project.Name)
		return nil
	}

	var pypiData struct {
		Releases map[string][]struct {
			UploadTime string `json:"upload_time_iso_8601"`
		} `json:"releases"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&pypiData); err != nil {
		log.Printf("⚠ Failed to parse PyPI response for %s: %v", project.Name, err)
		return nil
	}

	var versions []string
	for v := range pypiData.Releases {
		versions = append(versions, v)
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i] > versions[j]
	})

	var releases []Release
	for i, v := range versions {
		if i >= 10 {
			break
		}

		pubTime := time.Now()
		if uploads, ok := pypiData.Releases[v]; ok && len(uploads) > 0 {
			pubTime, _ = time.Parse(time.RFC3339, uploads[0].UploadTime)
		}

		releases = append(releases, Release{
			ID:          fmt.Sprintf("pypi_%s", v),
			Name:        project.Name,
			Version:     v,
			Platform:    "pypi",
			URL:         fmt.Sprintf("https://pypi.org/project/%s/%s/", pkgName, v),
			PublishedAt: pubTime,
			Description: "PyPI Package Version",
		})
	}

	return releases
}

// fetchDockerTags fetches image tags from Docker Hub
func fetchDockerTags(project Project) []Release {
	// Extract repo name from URL (e.g., docker.com/library/nginx)
	repoParts := strings.Split(project.Name, "/")
	var repo string
	if len(repoParts) == 1 {
		repo = fmt.Sprintf("library/%s", project.Name)
	} else {
		repo = project.Name
	}

	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags?page_size=10", repo)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("⚠ Docker Hub API error for %s: %v", project.Name, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("⚠ Docker Hub API returned %d for %s", resp.StatusCode, project.Name)
		return nil
	}

	var dockerData struct {
		Results []struct {
			Name       string `json:"name"`
			LastPushed string `json:"last_pushed"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&dockerData); err != nil {
		log.Printf("⚠ Failed to parse Docker response for %s: %v", project.Name, err)
		return nil
	}

	var releases []Release
	for _, tag := range dockerData.Results {
		pubTime, _ := time.Parse(time.RFC3339, tag.LastPushed)
		releases = append(releases, Release{
			ID:          fmt.Sprintf("docker_%s", tag.Name),
			Name:        project.Name,
			Version:     tag.Name,
			Platform:    "docker",
			URL:         fmt.Sprintf("https://hub.docker.com/r/%s/tags", repo),
			PublishedAt: pubTime,
			Description: "Docker Image Tag",
		})
	}

	return releases
}

// fetchGitLabReleases fetches releases from GitLab
func fetchGitLabReleases(project Project) []Release {
	// Extract project ID from URL
	repoPath := strings.TrimPrefix(project.RepoURL, "https://gitlab.com/")
	repoPath = strings.TrimSuffix(repoPath, ".git")
	projectID := strings.ReplaceAll(repoPath, "/", "%2F")

	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/releases?per_page=10", projectID)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("⚠ GitLab API error for %s: %v", project.Name, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("⚠ GitLab API returned %d for %s", resp.StatusCode, project.Name)
		return nil
	}

	var glReleases []struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Description string `json:"description"`
		CreatedAt   string `json:"created_at"`
		WebUrl      string `json:"web_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glReleases); err != nil {
		log.Printf("⚠ Failed to parse GitLab response for %s: %v", project.Name, err)
		return nil
	}

	var releases []Release
	for _, gl := range glReleases {
		pubTime, _ := time.Parse(time.RFC3339, gl.CreatedAt)
		releases = append(releases, Release{
			ID:          fmt.Sprintf("gl_%s", gl.TagName),
			Name:        project.Name,
			Version:     gl.TagName,
			Platform:    "gitlab",
			URL:         gl.WebUrl,
			PublishedAt: pubTime,
			Description: truncate(gl.Description, 200),
		})
	}

	return releases
}

// truncate limits string length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// SaveState persists the current store state to state.yaml
func (s *Store) SaveState(filename string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state := StateFile{
		Projects:  s.projects,
		Releases:  s.releases,
		Refreshed: s.refreshed,
	}

	data, err := yaml.Marshal(&state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	log.Printf("✓ State saved to %s", filename)
	return nil
}

// LoadState restores store state from state.yaml
func (s *Store) LoadState(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("No state file found at %s, starting fresh", filename)
			return nil
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state StateFile
	if err := yaml.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.projects = state.Projects
	s.releases = state.Releases
	s.refreshed = state.Refreshed

	log.Printf("✓ State loaded from %s (%d projects, %d total releases)", filename, len(s.projects), countReleases(s.releases))
	return nil
}

// countReleases helper to count total releases across all projects
func countReleases(releases map[string][]Release) int {
	total := 0
	for _, rels := range releases {
		total += len(rels)
	}
	return total
}

// asyncSave saves state in background to avoid blocking
func (s *Store) asyncSave() {
	if err := s.SaveState(s.stateFile); err != nil {
		log.Printf("⚠ Failed to auto-save state: %v", err)
	}
}

var store = NewStore()

var tmpl = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>Release Tracker</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
            background: #0f172a;
            color: #e2e8f0;
        }
        .header {
            background: #1e293b;
            padding: 1.5rem 2rem;
            border-bottom: 1px solid #334155;
        }
        .header h1 { font-size: 1.5rem; color: #60a5fa; }
        .container { max-width: 1200px; margin: 0 auto; padding: 2rem; }
        .tabs {
            display: flex;
            gap: 1rem;
            margin-bottom: 2rem;
            border-bottom: 1px solid #334155;
        }
        .tab {
            padding: 0.75rem 1.5rem;
            cursor: pointer;
            background: none;
            border: none;
            color: #94a3b8;
            font-size: 1rem;
            border-bottom: 2px solid transparent;
            transition: all 0.2s;
        }
        .tab:hover { color: #e2e8f0; }
        .tab.active {
            color: #60a5fa;
            border-bottom-color: #60a5fa;
        }
        .tab-content { display: none; }
        .tab-content.active { display: block; }
        .card {
            background: #1e293b;
            border: 1px solid #334155;
            border-radius: 8px;
            padding: 1.5rem;
            margin-bottom: 1rem;
        }
        .release-header {
            display: flex;
            justify-content: space-between;
            align-items: start;
            margin-bottom: 0.5rem;
        }
        .release-title {
            font-size: 1.125rem;
            font-weight: 600;
            color: #e2e8f0;
        }
        .release-version {
            background: #3b82f6;
            color: white;
            padding: 0.25rem 0.75rem;
            border-radius: 4px;
            font-size: 0.875rem;
        }
        .release-meta {
            color: #94a3b8;
            font-size: 0.875rem;
            margin-bottom: 0.75rem;
        }
        .platform-badge {
            display: inline-block;
            background: #334155;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            margin-right: 0.5rem;
        }
        .release-desc {
            color: #cbd5e1;
            line-height: 1.5;
        }
        .form-group {
            margin-bottom: 1rem;
        }
        .form-group label {
            display: block;
            margin-bottom: 0.5rem;
            color: #e2e8f0;
        }
        .form-group input, .form-group select {
            width: 100%;
            padding: 0.75rem;
            background: #0f172a;
            border: 1px solid #334155;
            border-radius: 4px;
            color: #e2e8f0;
            font-size: 1rem;
        }
        .btn {
            background: #3b82f6;
            color: white;
            padding: 0.75rem 1.5rem;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 1rem;
            transition: background 0.2s;
        }
        .btn:hover { background: #2563eb; }
        .empty-state {
            text-align: center;
            padding: 4rem 2rem;
            color: #64748b;
        }
        .project-list {
            display: grid;
            gap: 1rem;
        }
        .project-card {
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚀 Release Tracker</h1>
    </div>
    <div class="container">
        <div class="tabs">
            <button class="tab active" onclick="showTab('releases')">Latest Releases</button>
            <button class="tab" onclick="showTab('projects')">Projects</button>
            <button class="tab" onclick="showTab('add')">Add Project</button>
        </div>

        <div id="releases" class="tab-content active">
            <div id="releases-list"></div>
        </div>

        <div id="projects" class="tab-content">
            <div id="projects-list"></div>
        </div>

        <div id="add" class="tab-content">
            <div class="card">
                <h2 style="margin-bottom: 1.5rem;">Add New Project</h2>
                <form id="add-project-form">
                    <div class="form-group">
                        <label>Project Name</label>
                        <input type="text" name="name" required placeholder="e.g., kubernetes">
                    </div>
                    <div class="form-group">
                        <label>Platform</label>
                        <select name="platform" required>
                            <option value="github">GitHub</option>
                            <option value="gitlab">GitLab</option>
                            <option value="npm">NPM</option>
                            <option value="pypi">PyPI</option>
                            <option value="docker">Docker Hub</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label>Repository URL</label>
                        <input type="url" name="repo_url" required placeholder="https://github.com/user/repo">
                    </div>
                    <button type="submit" class="btn">Add Project</button>
                </form>
            </div>
        </div>
    </div>

    <script>
        function showTab(tab) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
            event.target.classList.add('active');
            document.getElementById(tab).classList.add('active');
            
            if (tab === 'releases') loadReleases();
            if (tab === 'projects') loadProjects();
        }

        async function loadReleases() {
            const res = await fetch('/api/releases');
            const releases = await res.json();
            const container = document.getElementById('releases-list');
            
            if (!releases || releases.length === 0) {
                container.innerHTML = '<div class="empty-state"><h3>No releases yet</h3><p>Add projects to start tracking releases</p></div>';
                return;
            }

            container.innerHTML = releases.map(r => ` + "`" + ` 
                <div class="card">
                    <div class="release-header">
                        <div class="release-title">${r.name}</div>
                        <div class="release-version">${r.version}</div>
                    </div>
                    <div class="release-meta">
                        <span class="platform-badge">${r.platform}</span>
                        <span>${new Date(r.published_at).toLocaleDateString()}</span>
                    </div>
                    <div class="release-desc">${r.description || 'No description available'}</div>
                </div>
            `+ "`" + `).join('');
        }

        async function loadProjects() {
            const res = await fetch('/api/projects');
            const projects = await res.json();
            const container = document.getElementById('projects-list');
            
            if (!projects || projects.length === 0) {
                container.innerHTML = '<div class="empty-state"><h3>No projects tracked</h3><p>Add your first project to get started</p></div>';
                return;
            }

            container.innerHTML = projects.map(p => ` + "`" + `
                <div class="card project-card">
                    <div style="flex: 1;">
                        <div class="release-title">${p.name}</div>
                        <div class="release-meta">
                            <span class="platform-badge">${p.platform}</span>
                        </div>
                        <div class="release-meta" style="margin-top: 0.5rem; font-size: 0.75rem;">
                            Last refreshed: ${p.last_refresh ? new Date(p.last_refresh).toLocaleString() : 'Never'}
                        </div>
                    </div>
                    <div style="display: flex; gap: 0.5rem;">
                        <button class="btn" style="background: #10b981; padding: 0.5rem 1rem;" onclick="refreshProject('${p.id}', event)">Refresh</button>
                        <a href="${p.repo_url}" target="_blank" class="btn">View</a>
                    </div>
                </div>
            `+ "`" + `).join('');
        }

        async function refreshProject(projectId, event) {
            event.preventDefault();
            const res = await fetch('/api/refresh?id=' + projectId, { method: 'POST' });
            if (res.ok) {
                alert('Refreshing project...');
                setTimeout(() => {
                    loadProjects();
                    loadReleases();
                }, 500);
            }
        }

        document.getElementById('add-project-form').addEventListener('submit', async (e) => {
            e.preventDefault();
            const form = new FormData(e.target);
            const data = Object.fromEntries(form);
            
            await fetch('/api/projects', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
            
            e.target.reset();
            alert('Project added successfully!');
            setTimeout(() => {
                loadProjects();
                loadReleases();
            }, 500);
        });

        // On page load: check for stale projects and refresh them
        async function initializePage() {
            const checkRes = await fetch('/api/refresh-check');
            if (checkRes.ok) {
                const checkData = await checkRes.json();
                console.log('Auto-refreshed ' + checkData.refreshed_count + ' stale projects');
            }
            loadReleases();
            loadProjects();
        }

        initializePage();
    </script>
</body>
</html>
`))

func main() {
	// Load state from disk, or seed with demo data if none exists
	if err := store.LoadState("state.yaml"); err != nil {
		log.Printf("⚠ Error loading state: %v", err)
	}

	// If no projects loaded, seed with demo data
	if len(store.GetProjects()) == 0 {
		log.Println("No existing state found, seeding demo data...")
		seedDemoData()
		store.SaveState("state.yaml")
	}

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/api/projects", handleProjects)
	http.HandleFunc("/api/releases", handleReleases)
	http.HandleFunc("/api/refresh-check", handleRefreshCheck)
	http.HandleFunc("/api/refresh", handleRefreshProject)

	fmt.Println("🚀 Release Tracker running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	tmpl.Execute(w, nil)
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {
		projects := store.GetProjects()
		json.NewEncoder(w).Encode(projects)
		return
	}

	if r.Method == "POST" {
		var p Project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p.ID = fmt.Sprintf("proj_%d", time.Now().UnixNano())
		p.LastRefresh = time.Now()
		store.AddProject(p)
		// Trigger immediate refresh on add
		go store.RefreshProject(p.ID)
		json.NewEncoder(w).Encode(p)
		return
	}
}

func handleReleases(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	releases := store.GetAllReleases()
	json.NewEncoder(w).Encode(releases)
}

// handleRefreshCheck checks for stale projects and auto-refreshes them
// Called on page load to ensure fresh data
func handleRefreshCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	staleProjects := store.GetStaleProjects()
	for _, p := range staleProjects {
		go store.RefreshProject(p.ID)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"refreshed_count": len(staleProjects),
		"stale_projects":  staleProjects,
	})
}

// handleRefreshProject forces a refresh of a specific project
func handleRefreshProject(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("id")
	if projectID == "" {
		http.Error(w, "Missing project id", http.StatusBadRequest)
		return
	}

	store.RefreshProject(projectID)

	json.NewEncoder(w).Encode(map[string]string{
		"status":     "refreshed",
		"project_id": projectID,
	})
}

func seedDemoData() {
	projects := []Project{
		{ID: "1", Name: "kubernetes", Platform: "github", RepoURL: "https://github.com/kubernetes/kubernetes"},
		{ID: "2", Name: "react", Platform: "npm", RepoURL: "https://github.com/facebook/react"},
		{ID: "3", Name: "golang", Platform: "github", RepoURL: "https://github.com/golang/go"},
	}

	for _, p := range projects {
		store.AddProject(p)
		// Fetch real data from APIs
		go store.RefreshProject(p.ID)
	}

	log.Println("📦 Seeding projects with live API data...")
}
