package main

import (
	"crypto/tls"
	"net/http"
	"testing"
)

// newTestStore creates an in-memory SQLite store for testing.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return s
}

// newTestAuth creates a user + session in s and returns the userID and session cookie.
func newTestAuth(t *testing.T, s *Store) (string, *http.Cookie) {
	t.Helper()
	user, err := s.CreateUser("testuser@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	sessionID := s.CreateSession(user.ID)
	cookie := &http.Cookie{Name: "session", Value: sessionID}
	return user.ID, cookie
}

// disableHTTPSVerificationForTest temporarily disables TLS certificate verification
// for the default HTTP client, needed for httptest.NewTLSServer with self-signed certs.
// It does the same for guardedClient (the SSRF-guarded client Helm repo fetches use)
// and lets loopback through its dial guard, since httptest servers bind 127.0.0.1.
func disableHTTPSVerificationForTest() func() {
	oldTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	oldHelm := guardedClient.Transport
	guarded := gitLabHTTPTransport.Clone()
	guarded.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	guardedClient.Transport = guarded
	allowLoopbackInTests.Store(true)
	return func() {
		http.DefaultClient.Transport = oldTransport
		guardedClient.Transport = oldHelm
		allowLoopbackInTests.Store(false)
	}
}
