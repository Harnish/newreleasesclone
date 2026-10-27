package main

import (
	"testing"
	"time"
)

func TestTooMany(t *testing.T) {
	k := "test|" + t.Name()
	for i := 0; i < 3; i++ {
		if tooMany(k, 3, time.Minute) {
			t.Fatalf("blocked after %d attempts", i)
		}
		recordAttempt(k)
	}
	if !tooMany(k, 3, time.Minute) {
		t.Fatal("expected block at max attempts")
	}
	if tooMany(k, 3, time.Nanosecond) {
		t.Fatal("expected old attempts to expire")
	}
}
