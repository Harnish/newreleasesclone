package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"
)

const (
	maxDescription     = 500
	descriptionRecheck = 24 * time.Hour // GitHub's unauthenticated limit is 60 requests/hour, so don't ask every refresh
)

// fetchJSONField GETs rawURL and decodes the JSON body into out; false on any failure.
func fetchJSONField(rawURL string, out any) bool {
	resp, err := fetch(rawURL, maxBody)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200 && json.NewDecoder(resp.Body).Decode(out) == nil
}

// fetchDescription returns the upstream project description, or "" when the
// platform has none or the lookup failed.
func fetchDescription(p Project) string {
	var d struct {
		Description string `json:"description"`
	}
	switch p.Platform {
	case "github":
		path := strings.TrimSuffix(strings.TrimPrefix(p.RepoURL, "https://github.com/"), ".git")
		fetchJSONField("https://api.github.com/repos/"+path, &d)
	case "gitlab":
		path := strings.TrimSuffix(strings.TrimPrefix(p.RepoURL, "https://gitlab.com/"), ".git")
		fetchJSONField("https://gitlab.com/api/v4/projects/"+strings.ReplaceAll(path, "/", "%2F"), &d)
	case "npm":
		fetchJSONField(fmt.Sprintf("%s/%s/latest", npmRegistryURL, npmPackageName(p)), &d)
	case "pypi":
		var info struct {
			Info struct {
				Summary string `json:"summary"`
			} `json:"info"`
		}
		fetchJSONField(fmt.Sprintf("%s/pypi/%s/json", pypiBaseURL, url.PathEscape(p.Name)), &info)
		d.Description = info.Info.Summary
	case "docker":
		fetchJSONField("https://hub.docker.com/v2/repositories/"+dockerRepo(p), &d)
	case "helm-artifacthub":
		path := strings.TrimPrefix(strings.TrimSuffix(p.RepoURL, "/"), "https://artifacthub.io/packages/helm/")
		fetchJSONField(artifacthubBaseURL+"/api/v1/packages/helm/"+path, &d)
	case "helm-repo":
		for _, e := range helmRepoEntries(p) {
			if e.Description != "" {
				d.Description = e.Description
				break
			}
		}
	}
	return truncate(strings.TrimSpace(d.Description), maxDescription)
}

func (s *Store) descriptionStale(repoID string) bool {
	var checked int64
	err := s.db.QueryRow("SELECT description_checked_at FROM repos WHERE id = ?", repoID).Scan(&checked)
	return err == nil && time.Since(time.Unix(checked, 0)) > descriptionRecheck
}

// refreshDescription looks up and stores p's description if it is due. A failed
// lookup keeps the old description but still counts as a check.
func (s *Store) refreshDescription(p Project) {
	if !s.descriptionStale(p.ID) {
		return
	}
	desc := fetchDescription(p)
	var err error
	if desc == "" {
		_, err = s.db.Exec("UPDATE repos SET description_checked_at = ? WHERE id = ?", time.Now().Unix(), p.ID)
	} else {
		_, err = s.db.Exec("UPDATE repos SET description = ?, description_checked_at = ? WHERE id = ?", desc, time.Now().Unix(), p.ID)
	}
	if err != nil {
		log.Printf("⚠ Failed to save description for %s: %v", p.Name, err)
	}
}
