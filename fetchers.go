package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func fetchGitHubReleases(project Project) []Release {
	repoPath := strings.TrimPrefix(project.RepoURL, "https://github.com/")
	repoPath = strings.TrimSuffix(repoPath, ".git")

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=30", repoPath)
	resp, err := fetch(url, maxBody)
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
	for _, gh := range ghReleases {
		if gh.Draft {
			continue
		}
		pubTime, _ := time.Parse(time.RFC3339, gh.PublishedAt)
		releases = append(releases, Release{
			ID:           fmt.Sprintf("gh_%d", gh.ID),
			Name:         project.Name,
			Version:      gh.TagName,
			Platform:     "github",
			URL:          gh.HtmlUrl,
			PublishedAt:  pubTime,
			Description:  truncate(gh.Body, 200),
			ReleaseNotes: gh.Body,
			Prerelease:   gh.Prerelease,
		})
	}

	return releases
}

func fetchNPMVersions(project Project) []Release {
	pkgName := project.Name
	if strings.HasPrefix(project.RepoURL, "https://www.npmjs.com/package/") {
		pkgName = strings.TrimPrefix(project.RepoURL, "https://www.npmjs.com/package/")
	} else if strings.Contains(project.RepoURL, "github.com/facebook/react") {
		pkgName = "react" // legacy entry
	}

	url := fmt.Sprintf("https://registry.npmjs.org/%s", pkgName)
	resp, err := fetch(url, maxBodyLarge)
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

	sort.Slice(versions, func(i, j int) bool {
		return versions[i] > versions[j]
	})

	var releases []Release
	for i, v := range versions {
		if i >= 10 {
			break
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
			Prerelease:  isPrerelease(v),
		})
	}

	return releases
}

func fetchPyPIReleases(project Project) []Release {
	pkgName := project.Name

	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", pkgName)
	resp, err := fetch(url, maxBody)
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
			Prerelease:  isPrerelease(v),
		})
	}

	return releases
}

func fetchDockerTags(project Project) []Release {
	var repo string
	if strings.HasPrefix(project.RepoURL, "https://hub.docker.com/r/") {
		repo = strings.TrimSuffix(strings.TrimPrefix(project.RepoURL, "https://hub.docker.com/r/"), "/")
	} else {
		repoParts := strings.Split(project.Name, "/")
		if len(repoParts) == 1 {
			repo = "library/" + project.Name
		} else {
			repo = project.Name
		}
	}

	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags?page_size=10", repo)
	resp, err := fetch(url, maxBody)
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
			Prerelease:  isPrerelease(tag.Name),
		})
	}

	return releases
}

func fetchGitLabReleases(project Project) []Release {
	repoPath := strings.TrimPrefix(project.RepoURL, "https://gitlab.com/")
	repoPath = strings.TrimSuffix(repoPath, ".git")
	projectID := strings.ReplaceAll(repoPath, "/", "%2F")

	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/releases?per_page=10", projectID)
	resp, err := fetch(url, maxBody)
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
		Upcoming    bool   `json:"upcoming_release"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glReleases); err != nil {
		log.Printf("⚠ Failed to parse GitLab response for %s: %v", project.Name, err)
		return nil
	}

	var releases []Release
	for _, gl := range glReleases {
		pubTime, _ := time.Parse(time.RFC3339, gl.CreatedAt)
		releases = append(releases, Release{
			ID:           fmt.Sprintf("gl_%s", gl.TagName),
			Name:         project.Name,
			Version:      gl.TagName,
			Platform:     "gitlab",
			URL:          gl.WebUrl,
			PublishedAt:  pubTime,
			Description:  truncate(gl.Description, 200),
			ReleaseNotes: gl.Description,
			Prerelease:   gl.Upcoming || isPrerelease(gl.TagName),
		})
	}

	return releases
}

var (
	// prereleaseWord: alpha/beta/rc/... delimited by non-letters, e.g. "1.0.0-rc.1", "2.0-beta", "nightly".
	prereleaseWord = regexp.MustCompile(`(?i)(^|[^a-z])(alpha|beta|rc|pre|preview|dev|canary|nightly|snapshot|next|unstable)([^a-z]|$)`)
	// prereleaseShort: PEP 440 style "1.0a1", "1.0b2", "1.0rc1".
	prereleaseShort = regexp.MustCompile(`(?i)\d(a|b|rc)\d+($|[.\-+_])`)
)

// isPrerelease guesses from a version/tag string for platforms whose API has
// no prerelease flag (npm, PyPI, Docker, Helm repos). Docker tags like
// "1.2.3-alpine" or "latest" are deliberately not treated as prereleases.
func isPrerelease(version string) bool {
	return prereleaseWord.MatchString(version) || prereleaseShort.MatchString(version)
}

const (
	maxBody      = 16 << 20 // GitHub, PyPI, Docker, GitLab, Artifact Hub (largest seen: ~3 MB)
	maxBodyLarge = 64 << 20 // npm packuments and Helm index.yaml (largest seen: ~16 MB)
)

// fetch GETs url and caps the response body at limit bytes, so a hostile or
// broken upstream can't exhaust memory. Reading past the cap fails with
// *http.MaxBytesError, which the callers' decoders surface as a parse error.
func fetch(url string, limit int64) (*http.Response, error) {
	return fetchVia(http.DefaultClient, url, limit)
}

// helmRepoClient is for Helm repo index URLs, the one fetch whose host is
// user-supplied. Its transport re-checks the resolved IP on every connection
// (including redirect hops) and honours GITLAB_ALLOWED_HOSTS for internal
// repos. Redirects must stay on https.
var helmRepoClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: gitLabHTTPTransport,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("refusing redirect to non-https URL %s", req.URL)
		}
		if len(via) >= 5 {
			return fmt.Errorf("stopped after 5 redirects")
		}
		return nil
	},
}

func fetchVia(client *http.Client, url string, limit int64) (*http.Response, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.ContentLength > limit {
		resp.Body.Close()
		return nil, fmt.Errorf("response too large: %d bytes (limit %d)", resp.ContentLength, limit)
	}
	resp.Body = http.MaxBytesReader(nil, resp.Body, limit) // nil writer is fine; Close closes the original body
	return resp, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

var artifacthubBaseURL = "https://artifacthub.io"

func fetchHelmArtifactHub(project Project) []Release {
	rawURL := strings.TrimSuffix(project.RepoURL, "/")
	path := strings.TrimPrefix(rawURL, "https://artifacthub.io/packages/helm/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		log.Printf("⚠ Invalid Artifact Hub URL for %s: %s", project.Name, project.RepoURL)
		return nil
	}
	repoName, chartName := parts[0], parts[1]

	listURL := fmt.Sprintf("%s/api/v1/packages/helm/%s/%s", artifacthubBaseURL, repoName, chartName)
	resp, err := fetch(listURL, maxBody)
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

	// Up to 10 stable plus 5 prereleases, so a run of rc's can't crowd stable out.
	var stable []versionEntry
	nStable, nPre := 0, 0
	for _, v := range pkgData.AvailableVersions {
		if v.Prerelease {
			if nPre < 5 {
				nPre++
				stable = append(stable, versionEntry{v.Version, v.TS, true})
			}
		} else if nStable < 10 {
			nStable++
			stable = append(stable, versionEntry{v.Version, v.TS, false})
		}
	}

	var releases []Release
	for _, v := range stable {
		detailURL := fmt.Sprintf("%s/api/v1/packages/helm/%s/%s/%s", artifacthubBaseURL, repoName, chartName, v.version)
		detailResp, err := fetch(detailURL, maxBody)
		if err != nil {
			log.Printf("⚠ Artifact Hub version detail error for %s@%s: %v", project.Name, v.version, err)
			continue
		}
		if detailResp.StatusCode != 200 {
			log.Printf("⚠ Artifact Hub version detail returned %d for %s@%s", detailResp.StatusCode, project.Name, v.version)
			detailResp.Body.Close()
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
			Prerelease:  v.prerelease,
		})
	}

	return releases
}

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
	if !strings.HasPrefix(project.RepoURL, "https://") {
		log.Printf("⚠ Helm repo URL must use HTTPS: %s", project.RepoURL)
		return nil
	}
	indexURL := strings.TrimSuffix(project.RepoURL, "/") + "/index.yaml"
	resp, err := fetchVia(helmRepoClient, indexURL, maxBodyLarge)
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
			Prerelease:  isPrerelease(e.Version),
		})
	}

	return releases
}
