package main

import (
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

func TestAllowKeyCap(t *testing.T) {
	attemptMu.Lock()
	saved := attempts
	attempts = map[string][]time.Time{}
	for i := 0; i < maxAttemptKeys; i++ {
		attempts[string(rune('a'))+time.Duration(i).String()] = []time.Time{time.Now()}
	}
	attemptMu.Unlock()
	defer func() { attemptMu.Lock(); attempts = saved; attemptMu.Unlock() }()
	if allow("brand-new-key", 3, time.Minute) {
		t.Fatal("expected new key rejected when map is full")
	}
}
