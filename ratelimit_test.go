package main

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestAllow(t *testing.T) {
	k := "test|" + t.Name()
	for i := 0; i < 3; i++ {
		if !allow(k, 3, time.Minute) {
			t.Fatalf("blocked after %d attempts", i)
		}
	}
	if allow(k, 3, time.Minute) {
		t.Fatal("expected block at max attempts")
	}
	forgetAttempts(k)
	if !allow(k, 3, time.Minute) {
		t.Fatal("expected allow after forget")
	}
}

func TestAllowKeyCapEvicts(t *testing.T) {
	attemptMu.Lock()
	saved := attempts
	attempts = map[string][]time.Time{}
	for i := 0; i < maxAttemptKeys; i++ {
		attempts[time.Duration(i).String()] = []time.Time{time.Now()}
	}
	attemptMu.Unlock()
	defer func() { attemptMu.Lock(); attempts = saved; attemptMu.Unlock() }()
	if !allow("brand-new-key", 3, time.Minute) {
		t.Fatal("expected new key allowed by evicting oldest")
	}
	if len(attempts) > maxAttemptKeys {
		t.Fatalf("map grew past cap: %d", len(attempts))
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.9")
	defer func(v bool) { trustedProxy = v }(trustedProxy)
	trustedProxy = false
	if got := clientIP(r); got != "10.0.0.1" {
		t.Fatalf("untrusted: got %s", got)
	}
	trustedProxy = true
	if got := clientIP(r); got != "203.0.113.9" {
		t.Fatalf("trusted: got %s", got)
	}
}
