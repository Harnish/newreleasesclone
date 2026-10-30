package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFrontendServed(t *testing.T) {
	cases := []struct{ path, ctype, want string }{
		{"/", "text/html", `src="/static/app.js"`},
		{"/static/app.js", "javascript", "function esc("},
		{"/static/app.css", "text/css", ".pagination"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		mux := http.NewServeMux()
		mux.HandleFunc("/", handleHome)
		mux.Handle("/static/", staticHandler())
		mux.ServeHTTP(rec, httptest.NewRequest("GET", c.path, nil))
		if rec.Code != 200 {
			t.Errorf("%s: status %d", c.path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, c.ctype) {
			t.Errorf("%s: content-type %q, want %q", c.path, ct, c.ctype)
		}
		if !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("%s: body missing %q", c.path, c.want)
		}
	}
}

func TestIndexShowsBuildVersion(t *testing.T) {
	if strings.Contains(string(indexHTML), "__BUILD__") {
		t.Fatal("placeholder not replaced")
	}
	if !strings.Contains(string(indexHTML), "build "+buildVersion()) {
		t.Fatalf("index missing build %q", buildVersion())
	}
}

func TestBuildVersionTruncatesFullSHA(t *testing.T) {
	defer func(v string) { version = v }(version)
	version = "0123456789abcdef0123456789abcdef01234567"
	if got := buildVersion(); got != "0123456" {
		t.Fatalf("got %q", got)
	}
	version = "abc1234"
	if got := buildVersion(); got != "abc1234" {
		t.Fatalf("got %q", got)
	}
}
