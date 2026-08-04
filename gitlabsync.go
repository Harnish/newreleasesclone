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
	log.Printf("gitlab sync: begin user=%s repo=%s", t.UserID, t.RepoID)
	err := s.doSyncProjectToGitLab(t)
	if err != nil {
		log.Printf("⚠ gitlab sync failed for user=%s repo=%s: %v", t.UserID, t.RepoID, err)
	}
	if recErr := s.RecordGitLabSync(t.UserID, t.RepoID, err); recErr != nil {
		log.Printf("⚠ failed to record gitlab sync outcome for user=%s repo=%s: %v", t.UserID, t.RepoID, recErr)
	}
	if err == nil {
		log.Printf("gitlab sync: syncing awesome page user=%s", t.UserID)
		if awErr := s.syncAwesomePage(t.UserID); awErr != nil {
			log.Printf("⚠ awesome page sync failed for user=%s: %v", t.UserID, awErr)
		} else {
			log.Printf("gitlab sync: awesome page sync complete user=%s", t.UserID)
		}
	}
}

// doSyncProjectToGitLab ensures the mirror project exists, then ephemerally
// clones+pushes the upstream repo. If it creates a new mirror project, it
// persists the resulting path via s.SetProjectGitLabPath so future syncs
// skip project-creation.
//
// If t.GitLabGroup is set, the mirror project is namespaced
// <group>/<upstream-owner>/<repo>, creating the <group>/<owner> subgroup on
// first use (EnsureGroupPath) — the top-level group must already exist in
// GitLab. With no group configured, the project is created directly under
// the token owner's personal namespace, named <repo>, matching the tool's
// original (pre-group) behavior.
func (s *Store) doSyncProjectToGitLab(t GitLabSyncTarget) error {
	if t.GitLabURL == "" || t.GitLabToken == "" {
		return fmt.Errorf("gitlab instance not configured")
	}
	client := newGitLabClient(t.GitLabURL, t.GitLabToken)

	httpURL := t.GitLabProjectPath
	if httpURL == "" {
		name := slugify(t.RepoName)
		fullPath := name
		namespaceID := 0
		if t.GitLabGroup != "" {
			owner, err := repoOwner(t.RepoURL)
			if err != nil {
				return fmt.Errorf("determine upstream owner: %w", err)
			}
			groupPath := t.GitLabGroup + "/" + slugify(owner)
			log.Printf("gitlab sync: ensuring group user=%s repo=%s group=%s", t.UserID, t.RepoID, groupPath)
			namespaceID, err = client.EnsureGroupPath(groupPath)
			if err != nil {
				return fmt.Errorf("ensure gitlab group %s: %w", groupPath, err)
			}
			fullPath = groupPath + "/" + name
		}

		log.Printf("gitlab sync: checking mirror project user=%s repo=%s path=%s", t.UserID, t.RepoID, fullPath)
		exists, err := client.ProjectExists(fullPath)
		if err != nil {
			return fmt.Errorf("check mirror project: %w", err)
		}
		if exists {
			log.Printf("gitlab sync: mirror project exists user=%s repo=%s path=%s", t.UserID, t.RepoID, fullPath)
			httpURL, err = client.GetProjectHTTPURL(fullPath)
		} else {
			log.Printf("gitlab sync: creating mirror project user=%s repo=%s path=%s", t.UserID, t.RepoID, fullPath)
			httpURL, err = client.CreateProject(namespaceID, name)
		}
		if err != nil {
			return fmt.Errorf("create/get mirror project: %w", err)
		}
		if err := s.SetProjectGitLabPath(t.UserID, t.RepoID, httpURL); err != nil {
			return fmt.Errorf("save mirror project path: %w", err)
		}
		log.Printf("gitlab sync: mirror project path saved user=%s repo=%s url=%s", t.UserID, t.RepoID, httpURL)
	}

	pushURL, err := client.AuthenticatedPushURL(httpURL)
	if err != nil {
		return err
	}
	log.Printf("gitlab sync: starting mirror sync user=%s repo=%s src=%s dst=%s", t.UserID, t.RepoID, t.RepoURL, httpURL)
	if err := mirrorSyncRepo(t.RepoURL, pushURL); err != nil {
		return err
	}
	log.Printf("gitlab sync: mirror sync complete user=%s repo=%s", t.UserID, t.RepoID)
	return nil
}
