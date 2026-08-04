package main

import (
	"fmt"
	"log"
)

// syncProjectToGitLab performs one GitLab mirror sync for t and records the
// outcome. Errors are recorded on the target's user_repos row rather than
// returned loudly — callers run this in a goroutine and must never crash
// the process on a bad token or network failure.
//
// This is a method (not a free function reading the global store) so that
// `go store.syncProjectToGitLab(t)` binds the receiver at the `go`
// statement, matching RefreshProject's existing convention — critical so a
// later reassignment of the global `store` var (e.g. test cleanup swapping
// it back) can't race a still-running goroutine into a nil-pointer deref.
func (s *Store) syncProjectToGitLab(t GitLabSyncTarget) {
	err := s.doSyncProjectToGitLab(t)
	if err != nil {
		log.Printf("⚠ gitlab sync failed for user=%s repo=%s: %v", t.UserID, t.RepoID, err)
	}
	if recErr := s.RecordGitLabSync(t.UserID, t.RepoID, err); recErr != nil {
		log.Printf("⚠ failed to record gitlab sync outcome for user=%s repo=%s: %v", t.UserID, t.RepoID, recErr)
	}
	if err == nil {
		if awErr := s.syncAwesomePage(t.UserID); awErr != nil {
			log.Printf("⚠ awesome page sync failed for user=%s: %v", t.UserID, awErr)
		}
	}
}

// doSyncProjectToGitLab ensures the mirror project exists under the user's
// GitLab namespace, then ephemerally clones+pushes the upstream repo. If it
// creates a new mirror project, it persists the resulting path via
// s.SetProjectGitLabPath so future syncs skip project-creation.
func (s *Store) doSyncProjectToGitLab(t GitLabSyncTarget) error {
	if t.GitLabURL == "" || t.GitLabToken == "" {
		return fmt.Errorf("gitlab instance not configured")
	}
	client := newGitLabClient(t.GitLabURL, t.GitLabToken)

	httpURL := t.GitLabProjectPath
	if httpURL == "" {
		path := slugify(t.RepoName)
		exists, err := client.ProjectExists(path)
		if err != nil {
			return fmt.Errorf("check mirror project: %w", err)
		}
		if exists {
			httpURL, err = client.GetProjectHTTPURL(path)
		} else {
			httpURL, err = client.CreateProject(path)
		}
		if err != nil {
			return fmt.Errorf("create/get mirror project: %w", err)
		}
		if err := s.SetProjectGitLabPath(t.UserID, t.RepoID, httpURL); err != nil {
			return fmt.Errorf("save mirror project path: %w", err)
		}
	}

	pushURL, err := client.AuthenticatedPushURL(httpURL)
	if err != nil {
		return err
	}
	return mirrorSyncRepo(t.RepoURL, pushURL)
}
