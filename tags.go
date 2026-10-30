package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"
)

const (
	maxTagLen         = 40
	maxTagsPerProject = 10
	maxTagDescLen     = 200
)

// cleanTag trims and collapses whitespace, so a tag is always one line.
func cleanTag(s string) string { return strings.Join(strings.Fields(s), " ") }

// normalizeTags cleans tags, drops blanks and case-insensitive duplicates
// (keeping the first spelling) and enforces the length and count limits.
func normalizeTags(in []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range in {
		t := cleanTag(raw)
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		if utf8.RuneCountInString(t) > maxTagLen {
			return nil, fmt.Errorf("tag %q is longer than %d characters", t, maxTagLen)
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	if len(out) > maxTagsPerProject {
		return nil, fmt.Errorf("a project can have at most %d tags", maxTagsPerProject)
	}
	return out, nil
}

// SetProjectTags replaces userID's tags on repoID. False if they don't track it.
func (s *Store) SetProjectTags(userID, repoID string, tags []string) (bool, error) {
	if !s.userTracksRepo(userID, repoID) {
		return false, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM repo_tags WHERE user_id = ? AND repo_id = ?", userID, repoID); err != nil {
		return false, err
	}
	for _, t := range tags {
		if _, err := tx.Exec("INSERT INTO repo_tags (user_id, repo_id, tag) VALUES (?, ?, ?)", userID, repoID, t); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// GetUserTags returns repoID -> tags (alphabetical, case-insensitive) for userID.
func (s *Store) GetUserTags(userID string) map[string][]string {
	out := map[string][]string{}
	rows, err := s.db.Query("SELECT repo_id, tag FROM repo_tags WHERE user_id = ? ORDER BY tag", userID)
	if err != nil {
		log.Printf("⚠ Failed to query tags: %v", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var repoID, tag string
		if rows.Scan(&repoID, &tag) == nil {
			out[repoID] = append(out[repoID], tag)
		}
	}
	return out
}

// GetTagDescriptions returns lower-cased tag -> description for userID.
func (s *Store) GetTagDescriptions(userID string) map[string]string {
	out := map[string]string{}
	rows, err := s.db.Query("SELECT tag, description FROM tag_descriptions WHERE user_id = ? AND description != ''", userID)
	if err != nil {
		log.Printf("⚠ Failed to query tag descriptions: %v", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var tag, desc string
		if rows.Scan(&tag, &desc) == nil {
			out[strings.ToLower(tag)] = desc
		}
	}
	return out
}

func (s *Store) SetTagDescription(userID, tag, desc string) error {
	_, err := s.db.Exec(`
		INSERT INTO tag_descriptions (user_id, tag, description) VALUES (?, ?, ?)
		ON CONFLICT(user_id, tag) DO UPDATE SET description = excluded.description`,
		userID, tag, desc)
	return err
}

type tagInfo struct {
	Tag         string `json:"tag"`
	Description string `json:"description"`
	Count       int    `json:"count"`
}

// GetUserTagList returns every tag userID has in use, with its description and project count.
func (s *Store) GetUserTagList(userID string) []tagInfo {
	descs := s.GetTagDescriptions(userID)
	list := []tagInfo{}
	rows, err := s.db.Query("SELECT tag, COUNT(*) FROM repo_tags WHERE user_id = ? GROUP BY tag ORDER BY tag", userID)
	if err != nil {
		log.Printf("⚠ Failed to query tag list: %v", err)
		return list
	}
	defer rows.Close()
	for rows.Next() {
		var t tagInfo
		if rows.Scan(&t.Tag, &t.Count) == nil {
			t.Description = descs[strings.ToLower(t.Tag)]
			list = append(list, t)
		}
	}
	return list
}

// POST /api/project-tags {repo_id, tags} — replaces the project's tags.
func handleProjectTags(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RepoID string   `json:"repo_id"`
		Tags   []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RepoID == "" {
		http.Error(w, "repo_id is required", http.StatusBadRequest)
		return
	}
	tags, err := normalizeTags(req.Tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ok, err := store.SetProjectTags(userID, req.RepoID, tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"tags": tags})
}

// GET /api/tags — the user's tags in use with descriptions and counts.
// POST /api/tags {tag, description} — sets a tag's description.
func handleTags(w http.ResponseWriter, r *http.Request, userID string) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(store.GetUserTagList(userID))
	case http.MethodPost:
		var req struct {
			Tag         string `json:"tag"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tag, desc := cleanTag(req.Tag), cleanTag(req.Description)
		if tag == "" {
			http.Error(w, "tag is required", http.StatusBadRequest)
			return
		}
		if utf8.RuneCountInString(desc) > maxTagDescLen {
			http.Error(w, fmt.Sprintf("description is longer than %d characters", maxTagDescLen), http.StatusBadRequest)
			return
		}
		if err := store.SetTagDescription(userID, tag, desc); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
