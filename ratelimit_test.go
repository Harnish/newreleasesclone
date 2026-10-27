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
