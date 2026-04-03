package main

import "time"

type Release struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Platform     string    `json:"platform"`
	URL          string    `json:"url"`
	PublishedAt  time.Time `json:"published_at"`
	Description  string    `json:"description"`
	ReleaseNotes string    `json:"release_notes"`
}

type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Platform     string    `json:"platform"`
	RepoURL      string    `json:"repo_url"`
	LastRefresh  time.Time `json:"last_refresh"`
	RefreshCount int       `json:"refresh_count"`
}
