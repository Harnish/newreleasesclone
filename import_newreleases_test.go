package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImportNewReleases(t *testing.T) {
	store = newTestStore(t)
	uid, _ := newTestAuth(t, store)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Key") != "k" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"projects":[{"name":"kubernetes/kubernetes","provider":"github"},{"name":"react","provider":"npm"},{"name":"nginx","provider":"docker"},{"name":"x","provider":"rubygems"}],"total_pages":1}`))
	}))
	defer srv.Close()
	newReleasesAPI = srv.URL

	do := func(key string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handleImportNewReleases(rec, httptest.NewRequest("POST", "/", strings.NewReader(`{"api_key":"`+key+`"}`)), uid)
		return rec
	}
	if rec := do("bad"); rec.Code != 401 {
		t.Fatalf("bad key: %d", rec.Code)
	}
	rec := do("k")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"added":3`) || !strings.Contains(rec.Body.String(), "rubygems") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if rec := do("k"); !strings.Contains(rec.Body.String(), `"existing":3`) {
		t.Fatalf("rerun: %s", rec.Body.String())
	}
}
