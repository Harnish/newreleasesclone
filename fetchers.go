package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

func fetchGitHubReleases(project Project) []Release {
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
	for _, gh := range ghReleases {
		if gh.Draft || gh.Prerelease {
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
		})
	}

	// If no stable releases, fall back to including prereleases
	if len(releases) == 0 {
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
			})
		}
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
		})
	}

	return releases
}

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

func fetchGitLabReleases(project Project) []Release {
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
			ID:           fmt.Sprintf("gl_%s", gl.TagName),
			Name:         project.Name,
			Version:      gl.TagName,
			Platform:     "gitlab",
			URL:          gl.WebUrl,
			PublishedAt:  pubTime,
			Description:  truncate(gl.Description, 200),
			ReleaseNotes: gl.Description,
		})
	}

	return releases
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
