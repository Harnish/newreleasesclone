package main

import "time"

type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

type Release struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"` // repo_id in DB
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Platform     string    `json:"platform"`
	URL          string    `json:"url"`
	PublishedAt  time.Time `json:"published_at"`
	Description  string    `json:"description"`
	ReleaseNotes string    `json:"release_notes"`
}

type Webhook struct {
	ID        string `json:"id"`
	RepoID    string `json:"repo_id"`
	URL       string `json:"url"`
	HasSecret bool   `json:"has_secret"`
}

type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Platform     string    `json:"platform"`
	RepoURL      string    `json:"repo_url"`
	LastRefresh  time.Time `json:"last_refresh"`
	RefreshCount int       `json:"refresh_count"`
	PushEnabled  bool      `json:"push_enabled"`
}
