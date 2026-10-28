package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Project icons are fetched server-side, validated, and served from our own
// origin, so browsers never contact third-party hosts (no viewer IP leak) and
// a Helm repo author can't point viewers at an arbitrary URL.

const (
	maxIconBytes   = 512 << 10
	iconRecheck    = 7 * 24 * time.Hour // re-fetch an icon we have
	iconRetryEmpty = 24 * time.Hour     // retry a lookup that found nothing
)

// iconURLFor returns where a project's icon lives, or "" when its platform has
// none (npm, PyPI, Docker: the UI shows a letter badge instead).
func iconURLFor(p Project) string {
	switch p.Platform {
	case "github":
		u, err := url.Parse(p.RepoURL)
		if err != nil || u.Host != "github.com" {
			return ""
		}
		owner, err := repoOwner(p.RepoURL)
		if err != nil {
			return ""
		}
		return "https://github.com/" + url.PathEscape(owner) + ".png?size=64"
	case "gitlab":
		return gitlabIconURL(p)
	case "helm-artifacthub":
		return artifactHubIconURL(p)
	case "helm-repo":
		for _, e := range helmRepoEntries(p) {
			if icon := strings.TrimSpace(e.Icon); icon != "" {
				return icon // entries share one chart icon; the first non-empty will do
			}
		}
	}
	return ""
}

// gitlabIconURL returns the project avatar, else its namespace avatar.
func gitlabIconURL(p Project) string {
	path := strings.TrimSuffix(strings.TrimPrefix(p.RepoURL, "https://gitlab.com/"), ".git")
	resp, err := fetch("https://gitlab.com/api/v4/projects/"+strings.ReplaceAll(path, "/", "%2F"), maxBody)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	var data struct {
		AvatarURL string `json:"avatar_url"`
		Namespace struct {
			AvatarURL string `json:"avatar_url"`
		} `json:"namespace"`
	}
	if json.NewDecoder(resp.Body).Decode(&data) != nil {
		return ""
	}
	for _, a := range []string{data.AvatarURL, data.Namespace.AvatarURL} {
		if a == "" {
			continue
		}
		if strings.HasPrefix(a, "/") { // namespace avatars come back relative
			a = "https://gitlab.com" + a
		}
		return a
	}
	return ""
}

func artifactHubIconURL(p Project) string {
	path := strings.TrimPrefix(strings.TrimSuffix(p.RepoURL, "/"), "https://artifacthub.io/packages/helm/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	resp, err := fetch(fmt.Sprintf("%s/api/v1/packages/helm/%s/%s", artifacthubBaseURL, parts[0], parts[1]), maxBody)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	var data struct {
		LogoImageID string `json:"logo_image_id"`
	}
	if json.NewDecoder(resp.Body).Decode(&data) != nil || data.LogoImageID == "" {
		return ""
	}
	return artifacthubBaseURL + "/image/" + url.PathEscape(data.LogoImageID)
}

// fetchIcon downloads rawURL through the SSRF-guarded client and returns the
// bytes with a content type we vouch for. The type comes from the content, not
// the server's header, except SVG, which must also be declared as SVG.
func fetchIcon(rawURL string) (contentType string, data []byte, err error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", nil, fmt.Errorf("unusable icon URL %q", rawURL)
	}
	resp, err := fetchVia(guardedClient, rawURL, maxIconBytes)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("icon HTTP %d", resp.StatusCode)
	}
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	switch sniffed := http.DetectContentType(data); {
	case sniffed == "image/png", sniffed == "image/jpeg", sniffed == "image/gif", sniffed == "image/webp":
		return sniffed, data, nil
	case strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "image/svg+xml") && looksLikeSVG(data):
		return "image/svg+xml", data, nil
	}
	return "", nil, fmt.Errorf("not a supported image (server said %q)", resp.Header.Get("Content-Type"))
}

func looksLikeSVG(b []byte) bool {
	head := b
	if len(head) > 2048 {
		head = head[:2048]
	}
	return bytes.Contains(bytes.ToLower(head), []byte("<svg"))
}

// ---- Store ----

// iconStale reports whether repoID's icon should be (re)looked-up.
func (s *Store) iconStale(repoID string) bool {
	var checkedAt int64
	var size sql.NullInt64
	err := s.db.QueryRow("SELECT checked_at, length(data) FROM repo_icons WHERE repo_id = ?", repoID).Scan(&checkedAt, &size)
	if err != nil {
		return true // no row yet
	}
	wait := iconRetryEmpty
	if size.Int64 > 0 {
		wait = iconRecheck
	}
	return time.Since(time.Unix(checkedAt, 0)) > wait
}

// saveIcon records a lookup. An empty data keeps any icon we already have and
// just notes that we checked, so a flaky upstream can't erase a good icon.
func (s *Store) saveIcon(repoID, contentType string, data []byte) {
	now := time.Now().Unix()
	var err error
	if len(data) == 0 {
		_, err = s.db.Exec(`INSERT INTO repo_icons (repo_id, checked_at) VALUES (?, ?)
			ON CONFLICT(repo_id) DO UPDATE SET checked_at = excluded.checked_at`, repoID, now)
	} else {
		_, err = s.db.Exec(`INSERT INTO repo_icons (repo_id, content_type, data, checked_at, fetched_at) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(repo_id) DO UPDATE SET content_type = excluded.content_type, data = excluded.data,
			checked_at = excluded.checked_at, fetched_at = excluded.fetched_at`, repoID, contentType, data, now, now)
	}
	if err != nil {
		log.Printf("⚠ Failed to save icon for %s: %v", repoID, err)
	}
}

// refreshIcon looks up and stores p's icon if it is missing or due for a recheck.
func (s *Store) refreshIcon(p Project) {
	if !s.iconStale(p.ID) {
		return
	}
	var contentType string
	var data []byte
	if iconURL := iconURLFor(p); iconURL != "" {
		ct, d, err := fetchIcon(iconURL)
		if err != nil {
			log.Printf("⚠ Icon for %s: %v", p.Name, err)
		} else {
			contentType, data = ct, d
		}
	}
	s.saveIcon(p.ID, contentType, data)
}

// GetIcon returns repoID's stored icon, if any.
func (s *Store) GetIcon(repoID string) (contentType string, data []byte, ok bool) {
	err := s.db.QueryRow("SELECT content_type, data FROM repo_icons WHERE repo_id = ? AND length(data) > 0", repoID).Scan(&contentType, &data)
	return contentType, data, err == nil
}

func (s *Store) userTracksRepo(userID, repoID string) bool {
	var one int
	return s.db.QueryRow("SELECT 1 FROM user_repos WHERE user_id = ? AND repo_id = ?", userID, repoID).Scan(&one) == nil
}

// GET /api/icon?id=<repo id> — the cached icon for a repo the user tracks.
func handleIcon(w http.ResponseWriter, r *http.Request, userID string) {
	repoID := r.URL.Query().Get("id")
	if repoID == "" || !store.userTracksRepo(userID, repoID) {
		http.NotFound(w, r)
		return
	}
	contentType, data, ok := store.GetIcon(repoID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// SVG can carry script; this CSP neutralises it if the URL is opened directly.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Write(data)
}
