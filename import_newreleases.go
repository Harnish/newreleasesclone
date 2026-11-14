package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// newReleasesAPI is a var so tests can point it at a fake.
var newReleasesAPI = "https://api.newreleases.io/v1"

type nrProject struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// toProject maps a newreleases.io project onto ours; false if the provider is unsupported.
func (p nrProject) toProject() (Project, bool) {
	n := strings.Trim(strings.TrimSpace(p.Name), "/")
	if n == "" {
		return Project{}, false
	}
	switch strings.ToLower(p.Provider) {
	case "github", "gitlab":
		parts := strings.Split(n, "/")
		if len(parts) < 2 {
			return Project{}, false
		}
		return Project{Name: parts[len(parts)-1], Platform: strings.ToLower(p.Provider),
			RepoURL: "https://" + strings.ToLower(p.Provider) + ".com/" + n}, true
	case "npm":
		return Project{Name: n, Platform: "npm", RepoURL: "https://www.npmjs.com/package/" + n}, true
	case "pypi":
		return Project{Name: n, Platform: "pypi", RepoURL: "https://pypi.org/project/" + n}, true
	case "docker", "dockerhub":
		ref := n
		if !strings.Contains(ref, "/") {
			ref = "library/" + ref
		}
		return Project{Name: n, Platform: "docker", RepoURL: "https://hub.docker.com/r/" + ref}, true
	}
	return Project{}, false
}

var errNRKey = errors.New("newreleases.io rejected the API key")

// fetchNewReleasesProjects pages through GET /projects. The key is only ever sent upstream.
func fetchNewReleasesProjects(key string) ([]nrProject, error) {
	var all []nrProject
	for page := 1; page <= 100; page++ { // ponytail: 100-page cap (~half a million projects)
		req, _ := http.NewRequest("GET", fmt.Sprintf("%s/projects?page=%d", newReleasesAPI, page), nil)
		req.Header.Set("X-Key", key)
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			return nil, fmt.Errorf("could not reach newreleases.io")
		}
		var out struct {
			Projects   []nrProject `json:"projects"`
			TotalPages int         `json:"total_pages"`
		}
		err = nil
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			err = errNRKey
		} else if resp.StatusCode != 200 {
			err = fmt.Errorf("newreleases.io returned %d", resp.StatusCode)
		} else if e := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&out); e != nil {
			err = fmt.Errorf("unexpected response from newreleases.io")
		}
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		all = append(all, out.Projects...)
		if page >= out.TotalPages {
			break
		}
	}
	return all, nil
}

// POST /api/import-newreleases {"api_key": "..."} — adds every supported project
// from the user's newreleases.io account. The key is not stored or logged.
func handleImportNewReleases(w http.ResponseWriter, r *http.Request, userID string) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		APIKey string `json:"api_key"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req) != nil || strings.TrimSpace(req.APIKey) == "" {
		http.Error(w, "api_key is required", http.StatusBadRequest)
		return
	}
	if !allow("nrimport:"+userID, 5, time.Hour) {
		http.Error(w, "Too many imports; try again later.", http.StatusTooManyRequests)
		return
	}
	projects, err := fetchNewReleasesProjects(strings.TrimSpace(req.APIKey))
	if err != nil {
		code := http.StatusBadGateway
		if errors.Is(err, errNRKey) {
			code = http.StatusUnauthorized
		}
		http.Error(w, err.Error(), code)
		return
	}
	var added, existing int
	skipped := []string{}
	for _, np := range projects {
		p, ok := np.toProject()
		if !ok {
			skipped = append(skipped, np.Name+" ("+np.Provider+")")
			continue
		}
		id, err := store.AddRepo(userID, p)
		switch {
		case errors.Is(err, errAlreadyTracked):
			existing++
		case err != nil:
			skipped = append(skipped, np.Name+" (error)")
		default:
			added++
			go store.RefreshProject(id)
		}
	}
	json.NewEncoder(w).Encode(map[string]any{"added": added, "existing": existing, "skipped": skipped})
}
