package main

import "time"

type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	RSSToken      string `json:"rss_token"`
	PageSize      int    `json:"page_size"`
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
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Platform            string    `json:"platform"`
	RepoURL             string    `json:"repo_url"`
	LastRefresh         time.Time `json:"last_refresh"`
	RefreshCount        int       `json:"refresh_count"`
	PushEnabled         bool      `json:"push_enabled"`
	EmailImmediate      bool      `json:"email_immediate"`
	GitLabSyncEnabled   bool      `json:"gitlab_sync_enabled"`
	GitLabSyncFrequency string    `json:"gitlab_sync_frequency,omitempty"`
	LastGitLabSyncAt    time.Time `json:"last_gitlab_sync_at,omitempty"`
	LastGitLabSyncError string    `json:"last_gitlab_sync_error,omitempty"`
	GitLabProjectPath   string    `json:"gitlab_project_path,omitempty"`
}

type GitLabSettings struct {
	GitLabURL         string `json:"gitlab_url"`
	GitLabToken       string `json:"-"`
	GitLabGroup       string `json:"gitlab_group"`
	AwesomeEnabled    bool   `json:"awesome_enabled"`
	AwesomeRepoName   string `json:"awesome_repo_name"`
	AwesomeGitLabPath string `json:"awesome_gitlab_path"`
}

type GitLabSyncTarget struct {
	UserID            string
	RepoID            string
	RepoName          string
	RepoURL           string
	Platform          string
	GitLabURL         string
	GitLabToken       string
	GitLabGroup       string
	GitLabProjectPath string
	Frequency         string
	LastSyncAt        time.Time
}
