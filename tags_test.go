package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	got, err := normalizeTags([]string{"  Self   hosted ", "self hosted", "", "Emulator"})
	if err != nil || len(got) != 2 || got[0] != "Self hosted" || got[1] != "Emulator" {
		t.Fatalf("got %q, %v; want [Self hosted Emulator] (trimmed, deduped case-insensitively, first spelling kept)", got, err)
	}
	if _, err := normalizeTags([]string{strings.Repeat("x", maxTagLen+1)}); err == nil {
		t.Error("an over-long tag was accepted")
	}
	many := make([]string, maxTagsPerProject+1)
	for i := range many {
		many[i] = string(rune('a' + i))
	}
	if _, err := normalizeTags(many); err == nil {
		t.Error("too many tags were accepted")
	}
}

func tagFixture(t *testing.T) (*Store, string, string) {
	t.Helper()
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)
	repoID, err := s.AddRepo(userID, Project{Name: "retroarch", Platform: "github", RepoURL: "https://github.com/libretro/RetroArch"})
	if err != nil {
		t.Fatal(err)
	}
	return s, userID, repoID
}

func TestProjectTagsRoundTripAndAreCaseInsensitive(t *testing.T) {
	s, userID, repoID := tagFixture(t)
	if ok, err := s.SetProjectTags(userID, repoID, []string{"Emulator", "Self hosted"}); !ok || err != nil {
		t.Fatalf("SetProjectTags = %v, %v", ok, err)
	}
	if got := s.GetUserRepos(userID)[0].Tags; len(got) != 2 || got[0] != "Emulator" || got[1] != "Self hosted" {
		t.Fatalf("project tags = %q", got)
	}
	// Replacing swaps the whole set; a different case of an existing tag doesn't duplicate it.
	s.SetProjectTags(userID, repoID, []string{"emulator"})
	if got := s.GetUserTagList(userID); len(got) != 1 || got[0].Count != 1 {
		t.Fatalf("tag list = %+v, want one tag used once", got)
	}
}

func TestTagsArePerUserAndGoWithTheTrackedRepo(t *testing.T) {
	s, userID, repoID := tagFixture(t)
	other, err := s.CreateUser("other@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	s.AddRepo(other.ID, Project{Name: "retroarch", Platform: "github", RepoURL: "https://github.com/libretro/RetroArch"})

	s.SetProjectTags(userID, repoID, []string{"Emulator"})
	if got := s.GetUserTags(other.ID); len(got) != 0 {
		t.Fatalf("another user sees tags: %v", got)
	}
	if ok, _ := s.SetProjectTags(other.ID, "repo_not_tracked", []string{"x"}); ok {
		t.Fatal("tagged a project the user doesn't track")
	}

	s.RemoveUserRepo(userID, repoID)
	if got := s.GetUserTags(userID); len(got) != 0 {
		t.Fatalf("tags survived untracking the repo: %v", got)
	}
}

func TestTagDescriptions(t *testing.T) {
	s, userID, repoID := tagFixture(t)
	s.SetProjectTags(userID, repoID, []string{"Emulator"})
	if err := s.SetTagDescription(userID, "emulator", "Old consoles"); err != nil { // other case, same tag
		t.Fatal(err)
	}
	if got := s.GetUserTagList(userID); got[0].Description != "Old consoles" {
		t.Fatalf("description = %q", got[0].Description)
	}
	s.SetTagDescription(userID, "Emulator", "")
	if got := s.GetTagDescriptions(userID); len(got) != 0 {
		t.Fatalf("cleared description still returned: %v", got)
	}
}

func TestTagHandlers(t *testing.T) {
	s, userID, repoID := tagFixture(t)
	old := store
	store = s
	defer func() { store = old }()

	post := func(h func(http.ResponseWriter, *http.Request, string), body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest("POST", "/", strings.NewReader(body)), userID)
		return rec
	}
	if rec := post(handleProjectTags, `{"repo_id":"`+repoID+`","tags":["Emulator"," Self  hosted "]}`); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Self hosted") {
		t.Fatalf("set tags: %d %s", rec.Code, rec.Body)
	}
	if rec := post(handleProjectTags, `{"repo_id":"nope","tags":["x"]}`); rec.Code != 404 {
		t.Errorf("untracked project: %d, want 404", rec.Code)
	}
	if rec := post(handleProjectTags, `{"repo_id":"`+repoID+`","tags":["`+strings.Repeat("x", 41)+`"]}`); rec.Code != 400 {
		t.Errorf("long tag: %d, want 400", rec.Code)
	}
	if rec := post(handleTags, `{"tag":"Emulator","description":"Old consoles"}`); rec.Code != 200 {
		t.Fatalf("set description: %d %s", rec.Code, rec.Body)
	}
	if rec := post(handleTags, `{"tag":"","description":"x"}`); rec.Code != 400 {
		t.Errorf("blank tag: %d, want 400", rec.Code)
	}
	rec := httptest.NewRecorder()
	handleTags(rec, httptest.NewRequest("GET", "/", nil), userID)
	if !strings.Contains(rec.Body.String(), `"description":"Old consoles"`) || !strings.Contains(rec.Body.String(), `"count":1`) {
		t.Fatalf("GET /api/tags = %s", rec.Body)
	}
}

func TestBuildAwesomeReadmeGroupsByTag(t *testing.T) {
	targets := []GitLabSyncTarget{
		{RepoID: "1", RepoName: "RetroArch", RepoURL: "https://x/retroarch", Platform: "github"},
		{RepoID: "2", RepoName: "Jellyfin", RepoURL: "https://x/jellyfin", Platform: "github"},
		{RepoID: "3", RepoName: "kubernetes", RepoURL: "https://x/k8s", Platform: "github"},
	}
	tags := map[string][]string{"1": {"Emulator", "Self hosted"}, "2": {"self hosted"}}
	descs := map[string]string{"emulator": "Old consoles"}
	out := buildAwesomeReadme(targets, tags, descs)

	order := []string{"## Emulator\n\nOld consoles\n\n- [RetroArch]", "## Self hosted\n\n- [Jellyfin](https://x/jellyfin)\n- [RetroArch]", "## Untagged\n\n- [kubernetes]"}
	last := -1
	for _, want := range order {
		i := strings.Index(out, want)
		if i <= last {
			t.Fatalf("missing or out of order %q in:\n%s", want, out)
		}
		last = i
	}
	if strings.Contains(out, "## Github") {
		t.Errorf("grouped by platform while tags exist:\n%s", out)
	}
}
