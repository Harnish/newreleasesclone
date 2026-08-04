package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// allowLoopbackInTests lets test doubles (httptest.NewServer, which always
// binds 127.0.0.1) run through the same dial path production traffic uses,
// without weakening the SSRF guard for the real server. Always false in
// production — only test code (via withLoopbackAllowedForTests) sets it.
// atomic because dial Control callbacks can run on a background goroutine
// concurrently with test cleanup.
var allowLoopbackInTests atomic.Bool

// gitLabAllowedHosts holds an operator-configured set of hostnames (from
// GITLAB_ALLOWED_HOSTS, comma-separated) that bypass the disallowed-address
// checks entirely, both at registration (validateGitLabURL) and at dial
// time. This exists because a self-hosted GitLab instance — the exact use
// case this feature targets — commonly resolves to a private address via
// split-horizon DNS or a Kubernetes service name when reached from inside
// the same cluster/network as newreleases, which the general per-user SSRF
// guard must otherwise reject (it can't distinguish "attacker-supplied
// internal target" from "operator's own instance" from IP alone). This is
// deliberately operator-controlled (an env var set by whoever deploys
// newreleases), not something a newreleases *user* can set via the API —
// that would just reopen the SSRF hole for a multi-tenant deployment.
// An atomic.Pointer swap (not a mutable map) so concurrent dial-time reads
// from background sync goroutines are race-free even while tests swap it.
var gitLabAllowedHosts atomic.Pointer[map[string]bool]

func init() {
	m := parseAllowedHosts(os.Getenv("GITLAB_ALLOWED_HOSTS"))
	gitLabAllowedHosts.Store(&m)
}

func parseAllowedHosts(raw string) map[string]bool {
	set := map[string]bool{}
	for _, h := range strings.Split(raw, ",") {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			set[h] = true
		}
	}
	return set
}

func isAllowedHost(host string) bool {
	m := gitLabAllowedHosts.Load()
	if m == nil {
		return false
	}
	return (*m)[strings.ToLower(host)]
}

// GitLabClient is a minimal GitLab REST v4 client, ported from
// /home/jharnish/Work/syncrepos/syncrepos/internal/gitlab/client.go.
// ponytail: dropped SetToken/HasToken from the original — a fresh client is
// constructed per sync rather than reused with a live-swappable token.
type GitLabClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newGitLabClient(baseURL, token string) *GitLabClient {
	return &GitLabClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: gitLabHTTPTransport,
			// Don't follow redirects: a validated gitlab_url (see
			// validateGitLabURL) could otherwise redirect requests to an
			// internal address after the fact, bypassing the SSRF check.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
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

// CreateProject creates a private project named name. If namespaceID is 0,
// GitLab defaults to the token owner's personal namespace; otherwise the
// project is created under that group/subgroup.
func (c *GitLabClient) CreateProject(namespaceID int, name string) (httpURLToRepo string, err error) {
	reqBody := map[string]any{
		"name":       name,
		"path":       name,
		"visibility": "private",
	}
	if namespaceID != 0 {
		reqBody["namespace_id"] = namespaceID
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

// GroupExists looks up a group (or nested subgroup, e.g. "upstream/owner")
// by its full path.
func (c *GitLabClient) GroupExists(fullPath string) (id int, exists bool, err error) {
	var body struct {
		ID int `json:"id"`
	}
	status, err := c.do(http.MethodGet, "/groups/"+url.PathEscape(fullPath), nil, &body)
	if err != nil {
		return 0, false, err
	}
	if status == http.StatusNotFound {
		return 0, false, nil
	}
	if status != http.StatusOK {
		return 0, false, fmt.Errorf("gitlab: unexpected status %d checking group %s", status, fullPath)
	}
	return body.ID, true, nil
}

// EnsureGroupPath returns the id of the group at fullPath, creating any
// missing subgroups beneath the top-level segment. The top-level segment
// (the operator-configured "upstream group") must already exist in GitLab —
// newreleases only creates subgroups beneath it, never a new top-level group.
func (c *GitLabClient) EnsureGroupPath(fullPath string) (int, error) {
	if id, exists, err := c.GroupExists(fullPath); err != nil {
		return 0, err
	} else if exists {
		return id, nil
	}

	parent := path.Dir(fullPath)
	name := path.Base(fullPath)
	if parent == "." || parent == fullPath {
		return 0, fmt.Errorf("gitlab: top-level group %q does not exist; create it manually first", fullPath)
	}

	parentID, err := c.EnsureGroupPath(parent)
	if err != nil {
		return 0, err
	}
	return c.createSubgroup(parentID, name)
}

func (c *GitLabClient) createSubgroup(parentID int, name string) (int, error) {
	reqBody := map[string]any{"name": name, "path": name, "parent_id": parentID}
	var body struct {
		ID int `json:"id"`
	}
	status, err := c.do(http.MethodPost, "/groups", reqBody, &body)
	if err != nil {
		return 0, err
	}
	if status != http.StatusCreated {
		return 0, fmt.Errorf("gitlab: create subgroup %s failed, status %d", name, status)
	}
	return body.ID, nil
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

var disallowedIPRanges = mustParseCIDRs(
	"100.64.0.0/10", // CGNAT
	"0.0.0.0/8",     // "this network"
)

func mustParseCIDRs(cidrs ...string) []*net.IPNet {
	nets := make([]*net.IPNet, len(cidrs))
	for i, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic(err)
		}
		nets[i] = n
	}
	return nets
}

// isDisallowedIP reports whether ip must not be reached by outbound
// requests to a user-supplied GitLab URL: loopback, link-local, private
// (RFC1918/ULA), unspecified, multicast, CGNAT, and the 0.0.0.0/8 "this
// network" range. IPv4-mapped IPv6 addresses are normalized via To4() first
// so ::ffff:127.0.0.1-style addresses can't bypass the IPv4-range checks.
func isDisallowedIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	for _, n := range disallowedIPRanges {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// gitLabGuardedDialer re-validates the resolved IP on every connection, not
// just once at registration — closing a DNS-rebinding/TOCTOU gap that a
// registration-time-only check (validateGitLabURL) leaves open: a
// user-controlled hostname can resolve to a public IP when saved and to an
// internal one by the time a sync actually connects, arbitrarily later.
// net.Dialer.Control runs after DNS resolution but before the connect()
// syscall, so it sees the real destination IP for every request.
var gitLabGuardedDialer = &net.Dialer{
	Timeout: 10 * time.Second,
	Control: func(network, address string, c syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("refusing to dial non-IP address %q", host)
		}
		if isDisallowedIP(ip) && !(ip.IsLoopback() && allowLoopbackInTests.Load()) {
			return fmt.Errorf("refusing to dial disallowed address %s", ip)
		}
		return nil
	},
}

// gitLabUnguardedDialer is used only for hostnames in gitLabAllowedHosts —
// the operator has already vouched for the destination, so no IP-range
// check applies (that's the whole point of the allowlist: it needs to
// permit private addresses).
var gitLabUnguardedDialer = &net.Dialer{Timeout: 10 * time.Second}

// gitLabHTTPTransport is shared by every GitLabClient. Its DialContext
// checks the allowlist against the pre-resolution hostname (Control, on the
// dialer itself, only ever sees the post-resolution IP — too late to know
// which hostname it came from) and picks the guarded or unguarded dialer
// accordingly.
var gitLabHTTPTransport = &http.Transport{
	DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}
		if isAllowedHost(host) {
			return gitLabUnguardedDialer.DialContext(ctx, network, addr)
		}
		return gitLabGuardedDialer.DialContext(ctx, network, addr)
	},
}

// validateGitLabURL rejects URLs that don't resolve to a public address at
// registration time. This is a fast-fail UX check, not the security
// boundary — gitLabHTTPTransport's dial-time Control enforces the same
// denylist on every actual connection, which is what prevents a
// DNS-rebinding bypass of this check. gitlab_url is user-supplied (any
// newreleases account can register one via POST /api/gitlab-settings), and
// the server subsequently makes authenticated-looking requests to it
// (project lookup/creation) and mirror git-pushes to whatever project URL
// it returns — SSRF if unvalidated.
func validateGitLabURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid gitlab_url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("gitlab_url must start with http:// or https://")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("gitlab_url must include a host")
	}
	if isAllowedHost(host) {
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("could not resolve gitlab_url host %q: %w", host, err)
	}
	for _, ip := range ips {
		if isDisallowedIP(ip) {
			return fmt.Errorf("gitlab_url resolves to a disallowed address")
		}
	}
	return nil
}

// repoOwner extracts the account/org segment from a repo URL, e.g.
// "https://github.com/DrewThomasson/ebook2audiobook" -> "DrewThomasson".
// Used to namespace mirror projects as <group>/<owner>/<repo> so mirrors
// from different upstream accounts don't collide on repo name alone.
func repoOwner(repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("parse repo url %q: %w", repoURL, err)
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segs) == 0 || segs[0] == "" {
		return "", fmt.Errorf("repo url %q has no owner segment", repoURL)
	}
	return segs[0], nil
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
