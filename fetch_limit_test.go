package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchCapsBody(t *testing.T) {
	big := strings.Repeat("x", 2048)

	t.Run("under the cap reads fully", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
		defer srv.Close()
		resp, err := fetch(srv.URL, 1024)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if b, err := io.ReadAll(resp.Body); err != nil || string(b) != "ok" {
			t.Fatalf("got %q, %v", b, err)
		}
	})

	t.Run("declared Content-Length over the cap is rejected up front", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, big) }))
		defer srv.Close()
		if resp, err := fetch(srv.URL, 1024); err == nil {
			resp.Body.Close()
			t.Fatal("expected error for oversized Content-Length")
		}
	})

	t.Run("chunked body over the cap fails mid-read", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.(http.Flusher).Flush() // forces chunked encoding, so no Content-Length
			io.WriteString(w, big)
		}))
		defer srv.Close()
		resp, err := fetch(srv.URL, 1024)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		_, err = io.ReadAll(resp.Body)
		var tooBig *http.MaxBytesError
		if !errors.As(err, &tooBig) {
			t.Fatalf("got %v, want *http.MaxBytesError", err)
		}
	})
}
