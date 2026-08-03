package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitLabClient is a minimal GitLab REST v4 client, ported from
// /home/jharnish/Work/syncrepos/syncrepos/internal/gitlab/client.go.
// ponytail: dropped GroupExists/EnsureGroupPath/SetToken/HasToken from the
// original — newreleases creates each mirror project directly under the
// token owner's personal GitLab namespace (namespace_id omitted from
// CreateProject), so subgroup management isn't needed, and a fresh client
// is constructed per sync rather than reused with a live-swappable token.
type GitLabClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newGitLabClient(baseURL, token string) *GitLabClient {
	return &GitLabClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *GitLabClient) ProjectExists(fullPath string) (bool, error) {
	status, err := c.do(http.MethodGet, "/projects/"+url.PathEscape(fullPath), nil, nil)
	if err != nil {
		return false, err
	}
	if status == http.StatusNotFound {
		return false, nil
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("gitlab: unexpected status %d checking project %s", status, fullPath)
	}
	return true, nil
}

// GetProjectHTTPURL fetches an existing project's http_url_to_repo, used
// instead of CreateProject when ProjectExists already reported true.
func (c *GitLabClient) GetProjectHTTPURL(fullPath string) (string, error) {
	var body struct {
		HTTPURLToRepo string `json:"http_url_to_repo"`
	}
	status, err := c.do(http.MethodGet, "/projects/"+url.PathEscape(fullPath), nil, &body)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("gitlab: unexpected status %d getting project %s", status, fullPath)
	}
	return body.HTTPURLToRepo, nil
}

// CreateProject creates a private project named name directly under the
// token owner's personal GitLab namespace (namespace_id omitted — GitLab
// defaults to the authenticated user's own namespace).
func (c *GitLabClient) CreateProject(name string) (httpURLToRepo string, err error) {
	reqBody := map[string]any{
		"name":       name,
		"path":       name,
		"visibility": "private",
	}
	var body struct {
		HTTPURLToRepo string `json:"http_url_to_repo"`
	}
	status, err := c.do(http.MethodPost, "/projects", reqBody, &body)
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated {
		return "", fmt.Errorf("gitlab: create project %s failed, status %d", name, status)
	}
	return body.HTTPURLToRepo, nil
}

// AuthenticatedPushURL embeds the client's token in a project's
// http_url_to_repo so git can push over HTTPS without SSH keys. The result
// contains a secret — keep it out of logs and error messages.
func (c *GitLabClient) AuthenticatedPushURL(httpURLToRepo string) (string, error) {
	u, err := url.Parse(httpURLToRepo)
	if err != nil {
		return "", fmt.Errorf("parse gitlab http url %q: %w", httpURLToRepo, err)
	}
	u.User = url.UserPassword("oauth2", c.token)
	return u.String(), nil
}

func (c *GitLabClient) do(method, path string, reqBody, out any) (int, error) {
	var bodyReader *bytes.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return 0, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, c.baseURL+"/api/v4"+path, bodyReader)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("gitlab request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if out != nil && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response for %s %s: %w", method, path, err)
		}
	}
	return resp.StatusCode, nil
}

// slugify lowercases name and collapses runs of non-alphanumeric characters
// into a single hyphen, producing a GitLab-safe project path segment.
func slugify(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
