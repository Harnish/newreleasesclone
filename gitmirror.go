package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// mirrorSyncRepo does a full ephemeral mirror sync: bare mirror-clone
// remoteURL to a temp dir, push --mirror to pushURL, then remove the temp
// dir. Every sync transfers the full repo; there is no incremental fetch
// and no state carried between syncs.
// ponytail: no persistent REPOS_DIR (unlike syncrepos) — sync cadence here
// is daily/weekly/monthly, so re-cloning each time costs nothing that
// matters and avoids the drift/repair problem persistent clones create.
func mirrorSyncRepo(remoteURL, pushURL string) error {
	dir, err := os.MkdirTemp("", "glsync-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := gitRun("", "clone", "--mirror", "--", remoteURL, dir); err != nil {
		return err
	}
	if err := gitRun(dir, "push", "--mirror", "--", pushURL); err != nil {
		return err
	}
	return nil
}

// pushGeneratedContent commits files (path -> content) into a fresh repo
// and force-pushes it to pushURL as the sole commit on main. Used for the
// Awesome-page README, which is regenerated fresh on every sync rather than
// incrementally edited, so a force-push is expected and safe.
func pushGeneratedContent(pushURL string, files map[string]string) error {
	dir, err := os.MkdirTemp("", "glsync-gen-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := gitRun(dir, "init", "-b", "main"); err != nil {
		return err
	}
	if err := gitRun(dir, "config", "user.email", "newreleases@localhost"); err != nil {
		return err
	}
	if err := gitRun(dir, "config", "user.name", "newreleases"); err != nil {
		return err
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		if err := gitRun(dir, "add", "--", name); err != nil {
			return err
		}
	}
	if err := gitRun(dir, "commit", "-m", "Update: "+time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return gitRun(dir, "push", "--force", "--", pushURL, "HEAD:main")
}

// redactArgs strips userinfo passwords out of any URL-shaped argument so a
// failed git command can't echo the gitlab token into an error string. Git
// redacts credentials in its own output already; the args are ours to scrub.
func redactArgs(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a
		if !strings.Contains(a, "://") || !strings.Contains(a, "@") {
			continue
		}
		u, err := url.Parse(a)
		if err != nil {
			out[i] = "[redacted]"
			continue
		}
		if u.User != nil {
			u.User = url.User(u.User.Username())
			out[i] = u.String()
		}
	}
	return strings.Join(out, " ")
}

func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", redactArgs(args), err, strings.TrimSpace(string(out)))
	}
	return nil
}
