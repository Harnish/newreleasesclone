package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestStoreAddRepo tests adding a repo to the store
func TestStoreAddRepo(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}
	if repoID == "" {
		t.Fatal("expected non-empty repoID")
	}

	projects := s.GetUserRepos(userID)
	if len(projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "Test Project" {
		t.Errorf("Expected project name 'Test Project', got %s", projects[0].Name)
	}
}

// TestStoreAddRepoDedupe tests that two users adding the same repo share one record
func TestStoreAddRepoDedupe(t *testing.T) {
	s := newTestStore(t)

	user1, _ := s.CreateUser("user1@example.com", "password123")
	user2, _ := s.CreateUser("user2@example.com", "password123")

	id1, err := s.AddRepo(user1.ID, Project{Name: "React", Platform: "npm", RepoURL: "react"})
	if err != nil {
		t.Fatalf("AddRepo user1 failed: %v", err)
	}
	id2, err := s.AddRepo(user2.ID, Project{Name: "React", Platform: "npm", RepoURL: "react"})
	if err != nil {
		t.Fatalf("AddRepo user2 failed: %v", err)
	}

	if id1 != id2 {
		t.Errorf("Expected shared repo ID, got %s and %s", id1, id2)
	}
	if len(s.GetProjects()) != 1 {
		t.Errorf("Expected 1 shared repo, got %d", len(s.GetProjects()))
	}
}

// TestStoreAddRelease tests adding releases to the store
func TestStoreAddRelease(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	s.AddRelease(repoID, Release{
		ID:           "rel-1",
		Name:         "Test Project",
		Version:      "v1.0.0",
		Platform:     "github",
		URL:          "https://github.com/test/repo/releases/v1.0.0",
		PublishedAt:  time.Now(),
		Description:  "Test release",
		ReleaseNotes: "Full release notes here",
	})

	releases := s.GetReleases(repoID)
	if len(releases) != 1 {
		t.Errorf("Expected 1 release, got %d", len(releases))
	}
	if releases[0].Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got %s", releases[0].Version)
	}
	if releases[0].ReleaseNotes != "Full release notes here" {
		t.Errorf("Expected release notes preserved, got %s", releases[0].ReleaseNotes)
	}
}

// TestStoreReleaseLimit tests that the store limits releases per project to releaseLimit
func TestStoreReleaseLimit(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	for i := 0; i < releaseLimit+10; i++ {
		s.AddRelease(repoID, Release{
			ID:          fmt.Sprintf("rel-%d", i),
			Name:        "Test Project",
			Version:     fmt.Sprintf("v%d.0.0", i),
			Platform:    "github",
			PublishedAt: time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	releases := s.GetReleases(repoID)
	if len(releases) != releaseLimit {
		t.Errorf("Expected %d releases (limit), got %d", releaseLimit, len(releases))
	}
}

// TestStoreMarkRefreshed tests marking a project as refreshed
func TestStoreMarkRefreshed(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	s.MarkRefreshed(repoID)

	if s.IsStale(repoID) {
		t.Error("Expected project to not be stale after refresh")
	}

	projects := s.GetProjects()
	if projects[0].RefreshCount != 1 {
		t.Errorf("Expected RefreshCount to be 1, got %d", projects[0].RefreshCount)
	}
}

// TestStoreIsStale tests stale data detection
func TestStoreIsStale(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	if !s.IsStale(repoID) {
		t.Error("Expected new project to be stale")
	}

	s.MarkRefreshed(repoID)
	if s.IsStale(repoID) {
		t.Error("Expected refreshed project to not be stale")
	}
}

// TestStoreGetAllReleases tests getting all releases across projects
func TestStoreGetAllReleases(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	for p := 1; p <= 3; p++ {
		repoID, err := s.AddRepo(userID, Project{
			Name:     fmt.Sprintf("Project %d", p),
			Platform: "github",
			RepoURL:  fmt.Sprintf("https://github.com/test/repo%d", p),
		})
		if err != nil {
			t.Fatalf("AddRepo failed: %v", err)
		}
		for r := 1; r <= 2; r++ {
			s.AddRelease(repoID, Release{
				ID:       fmt.Sprintf("rel-p%d-r%d", p, r),
				Name:     fmt.Sprintf("Project %d", p),
				Version:  fmt.Sprintf("v%d.%d.0", p, r),
				Platform: "github",
			})
		}
	}

	allReleases := s.GetAllReleases()
	if len(allReleases) != 6 {
		t.Errorf("Expected 6 total releases, got %d", len(allReleases))
	}
}

// TestStorePersistence tests that data survives closing and reopening the DB
func TestStorePersistence(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"

	s1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	user, err := s1.CreateUser("persist_user@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	repoID, err := s1.AddRepo(user.ID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	s1.MarkRefreshed(repoID)
	s1.AddRelease(repoID, Release{
		ID:           "rel-1",
		Name:         "Test Project",
		Version:      "v1.0.0",
		Platform:     "github",
		URL:          "https://github.com/test/repo/releases/v1.0.0",
		PublishedAt:  time.Now(),
		ReleaseNotes: "Full release notes here",
	})
	s1.db.Close()

	s2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}

	projects := s2.GetProjects()
	if len(projects) != 1 {
		t.Errorf("Expected 1 project after reopen, got %d", len(projects))
	}
	if projects[0].Name != "Test Project" {
		t.Errorf("Project name not preserved: got %s", projects[0].Name)
	}

	releases := s2.GetReleases(repoID)
	if len(releases) != 1 {
		t.Errorf("Expected 1 release after reopen, got %d", len(releases))
	}
	if releases[0].ReleaseNotes != "Full release notes here" {
		t.Errorf("Release notes not preserved: got %s", releases[0].ReleaseNotes)
	}
}

// TestReleaseJSONSerialization tests Release struct JSON tags
func TestReleaseJSONSerialization(t *testing.T) {
	release := Release{
		ID:           "gh_123",
		Name:         "Test",
		Version:      "v1.0.0",
		Platform:     "github",
		URL:          "https://example.com",
		PublishedAt:  time.Now(),
		Description:  "Short desc",
		ReleaseNotes: "Full notes here",
	}

	jsonBytes, err := json.Marshal(release)
	if err != nil {
		t.Fatalf("Failed to marshal Release: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("Failed to unmarshal Release: %v", err)
	}

	for _, field := range []string{"id", "name", "version", "platform", "url", "published_at", "description", "release_notes"} {
		if _, ok := result[field]; !ok {
			t.Errorf("Expected field '%s' in JSON output", field)
		}
	}

	if notes, ok := result["release_notes"].(string); !ok || notes != "Full notes here" {
		t.Error("ReleaseNotes field not properly serialized")
	}
}

// TestConcurrentAddRepo tests concurrent repo additions
func TestConcurrentAddRepo(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)
	numGoroutines := 10

	done := make(chan struct{}, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			s.AddRepo(userID, Project{
				Name:     fmt.Sprintf("Project %d", id),
				Platform: "github",
				RepoURL:  fmt.Sprintf("https://github.com/test/repo%d", id),
			})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	projects := s.GetUserRepos(userID)
	if len(projects) != numGoroutines {
		t.Errorf("Expected %d projects from concurrent adds, got %d", numGoroutines, len(projects))
	}
}

// TestConcurrentAddRelease tests concurrent release additions
func TestConcurrentAddRelease(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	numGoroutines := 20
	done := make(chan struct{}, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			s.AddRelease(repoID, Release{
				ID:       fmt.Sprintf("rel-%d", id),
				Name:     "Test Project",
				Version:  fmt.Sprintf("v%d.0.0", id),
				Platform: "github",
			})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	releases := s.GetReleases(repoID)
	if len(releases) > releaseLimit {
		t.Errorf("Expected max %d releases, got %d", releaseLimit, len(releases))
	}
}

// TestStoreSetProjectPushEnabled tests per-project push notification toggle.
func TestStoreSetProjectPushEnabled(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	// Default is enabled — verify via GetUserRepos
	projects := s.GetUserRepos(userID)
	if len(projects) != 1 || !projects[0].PushEnabled {
		t.Errorf("Expected push_enabled=true by default, got %v", projects[0].PushEnabled)
	}

	// Disable push
	ok, err := s.SetProjectPushEnabled(userID, repoID, false)
	if err != nil || !ok {
		t.Fatalf("SetProjectPushEnabled(false) returned ok=%v err=%v", ok, err)
	}
	projects = s.GetUserRepos(userID)
	if projects[0].PushEnabled {
		t.Error("Expected push_enabled=false after disabling")
	}

	// Re-enable push
	ok, err = s.SetProjectPushEnabled(userID, repoID, true)
	if err != nil || !ok {
		t.Fatalf("SetProjectPushEnabled(true) returned ok=%v err=%v", ok, err)
	}
	projects = s.GetUserRepos(userID)
	if !projects[0].PushEnabled {
		t.Error("Expected push_enabled=true after re-enabling")
	}

	// Unowned repo returns ok=false
	ok, err = s.SetProjectPushEnabled(userID, "repo_doesnotexist", false)
	if err != nil {
		t.Fatalf("unexpected error for unowned repo: %v", err)
	}
	if ok {
		t.Error("Expected ok=false for unowned repo ID")
	}
}

// TestStoreSetProjectEmailImmediate tests per-project immediate email toggle.
func TestStoreSetProjectEmailImmediate(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	// Default is disabled
	projects := s.GetUserRepos(userID)
	if len(projects) != 1 || projects[0].EmailImmediate {
		t.Errorf("expected email_immediate=false by default, got %v", projects[0].EmailImmediate)
	}

	// Enable
	ok, err := s.SetProjectEmailImmediate(userID, repoID, true)
	if err != nil || !ok {
		t.Fatalf("SetProjectEmailImmediate(true) returned ok=%v err=%v", ok, err)
	}
	projects = s.GetUserRepos(userID)
	if !projects[0].EmailImmediate {
		t.Error("expected email_immediate=true after enabling")
	}

	// Disable
	ok, err = s.SetProjectEmailImmediate(userID, repoID, false)
	if err != nil || !ok {
		t.Fatalf("SetProjectEmailImmediate(false) returned ok=%v err=%v", ok, err)
	}
	projects = s.GetUserRepos(userID)
	if projects[0].EmailImmediate {
		t.Error("expected email_immediate=false after disabling")
	}

	// Unowned repo returns ok=false
	ok, err = s.SetProjectEmailImmediate(userID, "repo_doesnotexist", false)
	if err != nil {
		t.Fatalf("unexpected error for unowned repo: %v", err)
	}
	if ok {
		t.Error("expected ok=false for unknown repo")
	}
}

// BenchmarkStoreAddRepo benchmarks adding repos
func BenchmarkStoreAddRepo(b *testing.B) {
	s, _ := NewStore(":memory:")
	user, _ := s.CreateUser("benchuser@example.com", "password123")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.AddRepo(user.ID, Project{
			Name:     fmt.Sprintf("Project %d", i),
			Platform: "github",
			RepoURL:  fmt.Sprintf("https://github.com/bench/repo%d", i),
		})
	}
}

// BenchmarkStoreAddRelease benchmarks adding releases
func BenchmarkStoreAddRelease(b *testing.B) {
	s, _ := NewStore(":memory:")
	user, _ := s.CreateUser("benchuser@example.com", "password123")
	repoID, _ := s.AddRepo(user.ID, Project{
		Name:     "Bench Project",
		Platform: "github",
		RepoURL:  "https://github.com/bench/repo",
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.AddRelease(repoID, Release{
			ID:       fmt.Sprintf("rel-%d", i),
			Name:     "Bench Project",
			Version:  fmt.Sprintf("v%d.0.0", i),
			Platform: "github",
		})
	}
}
