package main

import "testing"

func TestDockerTagTime(t *testing.T) {
	if got := dockerTagTime("", "2026-11-13T13:52:22.842717Z", "2020-01-01T00:00:00Z"); got.Year() != 2026 {
		t.Fatalf("want last_updated, got %v", got)
	}
	if got := dockerTagTime("2026-11-13T13:52:22Z", "x"); got.Year() != 2026 {
		t.Fatalf("want tag_last_pushed, got %v", got)
	}
	if !dockerTagTime("", "garbage").IsZero() {
		t.Fatal("want zero")
	}
}
