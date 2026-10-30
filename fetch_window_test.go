package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewestReleasesOrdersByPublishTimeNotVersionString(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := newestReleases([]Release{
		{Version: "9.0.0", PublishedAt: base},
		{Version: "10.0.0", PublishedAt: base.Add(24 * time.Hour)}, // string order puts this below 9.0.0
		{Version: "8.0.0", PublishedAt: base.Add(-24 * time.Hour)},
	})
	want := []string{"10.0.0", "9.0.0", "8.0.0"}
	for i, w := range want {
		if got[i].Version != w {
			t.Fatalf("position %d = %s, want %s (full: %+v)", i, got[i].Version, w, got)
		}
	}
}

func TestNewestReleasesWindowAndDeterminism(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var in []Release
	for i := 0; i < 14; i++ { // 14 stable
		in = append(in, Release{Version: fmt.Sprintf("1.%d.0", i), PublishedAt: base.Add(time.Duration(i) * time.Hour)})
	}
	for i := 0; i < 9; i++ { // 9 prereleases, all newer than every stable
		in = append(in, Release{Version: fmt.Sprintf("2.0.0-rc.%d", i), Prerelease: true, PublishedAt: base.Add(time.Duration(100+i) * time.Hour)})
	}
	got := newestReleases(in)
	stable, pre := 0, 0
	for _, r := range got {
		if r.Prerelease {
			pre++
		} else {
			stable++
		}
	}
	if stable != keepStable || pre != keepPrerelease {
		t.Fatalf("kept %d stable + %d pre, want %d + %d", stable, pre, keepStable, keepPrerelease)
	}
	// Prereleases are newer than all stable, yet stable must not be crowded out:
	// the newest stable (1.13.0) has to be present.
	found := false
	for _, r := range got {
		if r.Version == "1.13.0" {
			found = true
		}
	}
	if !found {
		t.Fatal("newest stable release was crowded out by prereleases")
	}

	// Ties on publish time resolve by version, so output is stable run to run.
	tied := func() []Release {
		return newestReleases([]Release{{Version: "1.0.0", PublishedAt: base}, {Version: "1.1.0", PublishedAt: base}, {Version: "0.9.0", PublishedAt: base}})
	}
	a, b := tied(), tied()
	for i := range a {
		if a[i].Version != b[i].Version {
			t.Fatal("tie ordering is not deterministic")
		}
	}
	if a[0].Version != "1.1.0" {
		t.Fatalf("tie-break picked %s first, want 1.1.0", a[0].Version)
	}
}

func TestFetchNPMVersionsPicksNewestNotHighestString(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	versions := map[string]any{}
	times := map[string]string{"created": base.Format(time.RFC3339), "modified": base.Format(time.RFC3339)}
	for i := 1; i <= 12; i++ { // 1.0.0 … 12.0.0, published in order; string order would rank 9.0.0 first
		v := fmt.Sprintf("%d.0.0", i)
		versions[v] = map[string]string{"version": v}
		times[v] = base.Add(time.Duration(i) * 24 * time.Hour).Format(time.RFC3339)
	}
	versions["13.0.0-beta.1"] = map[string]string{"version": "13.0.0-beta.1"}
	times["13.0.0-beta.1"] = base.Add(30 * 24 * time.Hour).Format(time.RFC3339)
	versions["0.0.1"] = map[string]string{"version": "0.0.1"} // no time entry: must be ignored, not stamped "now"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"versions": versions, "time": times})
	}))
	defer srv.Close()
	old := npmRegistryURL
	npmRegistryURL = srv.URL
	defer func() { npmRegistryURL = old }()

	got := fetchNPMVersions(Project{Name: "pkg", Platform: "npm"})
	if len(got) == 0 || got[0].Version != "13.0.0-beta.1" || !got[0].Prerelease {
		t.Fatalf("first = %+v, want the newest publish (the beta)", got)
	}
	has := map[string]bool{}
	for _, r := range got {
		has[r.Version] = true
	}
	for _, v := range []string{"12.0.0", "11.0.0", "10.0.0", "3.0.0"} {
		if !has[v] {
			t.Errorf("missing %s; 10 newest stable should be 3.0.0…12.0.0", v)
		}
	}
	if has["2.0.0"] || has["1.0.0"] {
		t.Error("kept versions older than the 10 newest stable")
	}
	if has["0.0.1"] {
		t.Error("kept a version with no publish time")
	}
}

func TestFetchPyPIReleasesUsesFirstUploadAndSkipsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"releases": {
			"9.0":  [{"upload_time_iso_8601": "2025-01-01T00:00:00.000000Z"}],
			"10.0": [{"upload_time_iso_8601": "2025-03-01T00:00:00.000000Z"}, {"upload_time_iso_8601": "2025-02-01T00:00:00.000000Z"}],
			"11.0": [],
			"12.0rc1": [{"upload_time_iso_8601": "2025-04-01T00:00:00.000000Z"}]
		}}`)
	}))
	defer srv.Close()
	old := pypiBaseURL
	pypiBaseURL = srv.URL
	defer func() { pypiBaseURL = old }()

	got := fetchPyPIReleases(Project{Name: "pkg", Platform: "pypi"})
	if len(got) != 3 {
		t.Fatalf("got %d releases, want 3 (11.0 has no files): %+v", len(got), got)
	}
	if got[0].Version != "12.0rc1" || !got[0].Prerelease || got[1].Version != "10.0" || got[2].Version != "9.0" {
		t.Fatalf("order = %s, %s, %s; want 12.0rc1, 10.0, 9.0", got[0].Version, got[1].Version, got[2].Version)
	}
	if want := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC); !got[1].PublishedAt.Equal(want) {
		t.Errorf("10.0 published %v, want earliest upload %v", got[1].PublishedAt, want)
	}
}

func TestFetchHelmRepoWindowAndUndated(t *testing.T) {
	defer disableHTTPSVerificationForTest()()
	var idx strings.Builder
	idx.WriteString("apiVersion: v1\nentries:\n  redis:\n")
	for i := 1; i <= 12; i++ { // 12 stable, oldest first
		fmt.Fprintf(&idx, "  - version: \"%d.0.0\"\n    created: \"2026-01-%02dT00:00:00Z\"\n", i, i)
	}
	for i := 1; i <= 7; i++ { // 7 prereleases, all newer than every stable
		fmt.Fprintf(&idx, "  - version: \"13.0.0-rc.%d\"\n    created: \"2026-02-%02dT00:00:00Z\"\n", i, i)
	}
	idx.WriteString("  - version: \"0.0.1\"\n") // no created: must be ignored, not stamped "now"
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, idx.String()) }))
	defer srv.Close()

	got := fetchHelmRepo(Project{Name: "redis", Platform: "helm-repo", RepoURL: srv.URL})
	stable, pre := 0, 0
	has := map[string]bool{}
	for _, r := range got {
		has[r.Version] = true
		if r.Prerelease {
			pre++
		} else {
			stable++
		}
	}
	if stable != keepStable || pre != keepPrerelease {
		t.Fatalf("kept %d stable + %d pre, want %d + %d", stable, pre, keepStable, keepPrerelease)
	}
	if !has["12.0.0"] || !has["3.0.0"] || has["2.0.0"] {
		t.Errorf("stable window wrong: want 3.0.0…12.0.0, got %v", has)
	}
	if has["0.0.1"] {
		t.Error("kept an undated version")
	}
	if got[0].Version != "13.0.0-rc.7" {
		t.Errorf("newest first: got %s", got[0].Version)
	}
}
