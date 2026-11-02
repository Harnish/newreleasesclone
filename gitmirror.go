package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// mirrorCacheDir holds one bare mirror per (user, project) so a sync only
// fetches what changed. Lives under data/ so it survives container restarts.
var mirrorCacheDir = "data/mirrors"

// mirrorSyncRepo updates the bare mirror at cacheDir from remoteURL (fetching
// only new objects after the first run), then push --mirror to pushURL. The
// remote URL is passed per call and never saved in the repo's config, so a
// token never lands on disk and a rotated token just works. If an existing
// cache fails to update (corrupt, force-pushed history git won't reconcile),
// it is deleted and rebuilt once from scratch.
func mirrorSyncRepo(cacheDir, remoteURL, pushURL string) error {
	_, statErr := os.Stat(cacheDir)
	existed := statErr == nil
	err := mirrorFetch(cacheDir, remoteURL)
	if err != nil && existed {
		log.Printf("gitlab sync: cache refresh failed, re-cloning dir=%s err=%v", cacheDir, err)
		if rmErr := os.RemoveAll(cacheDir); rmErr != nil {
			return fmt.Errorf("reset mirror cache: %w", rmErr)
		}
		err = mirrorFetch(cacheDir, remoteURL)
	}
	if err != nil {
		return err
	}
	log.Printf("gitlab sync: pushing mirror dst=%s", redactArgs([]string{pushURL}))
	if err := gitRun(cacheDir, "push", "--mirror", "--", pushURL); err != nil {
		return err
	}
	log.Printf("gitlab sync: push complete dst=%s", redactArgs([]string{pushURL}))
	return nil
}

// mirrorFetch makes cacheDir a bare repo (if it isn't one) and force-fetches
// every ref from remoteURL into it, pruning refs deleted upstream. A failed
// first fetch removes the half-made dir so it isn't mistaken for a cache.
func mirrorFetch(cacheDir, remoteURL string) error {
	fresh := false
	if _, err := os.Stat(filepath.Join(cacheDir, "HEAD")); err != nil {
		if err := os.MkdirAll(cacheDir, 0o700); err != nil {
			return fmt.Errorf("create mirror cache: %w", err)
		}
		if err := gitRun(cacheDir, "init", "--bare"); err != nil {
			return err
		}
		fresh = true
	}
	log.Printf("gitlab sync: fetching mirror src=%s fresh=%v", redactArgs([]string{remoteURL}), fresh)
	err := gitRun(cacheDir, "fetch", "--prune", "--no-tags", "--", remoteURL, "+refs/*:refs/*")
	if err != nil && fresh {
		os.RemoveAll(cacheDir)
	}
	return err
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

// gitTimeout bounds every git call so a hung push becomes a recorded sync
// error instead of blocking that project's syncs forever.
var gitTimeout = 15 * time.Minute

func gitRun(dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.WaitDelay = 5 * time.Second // don't wait on pipes held open by git's children after the kill
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("git %s: timed out after %s", redactArgs(args), gitTimeout)
	}
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", redactArgs(args), err, strings.TrimSpace(string(out)))
	}
	return nil
}
