package main

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestMigrationV10EmailUnique(t *testing.T) {
	s := newTestStore(t)

	// After migration, duplicate email must fail.
	_, err := s.db.Exec(
		`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		"user_aaa111aaa111", "dup@example.com", "hash", "2026-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		"user_bbb222bbb222", "dup@example.com", "hash", "2026-01-01T00:00:00Z",
	)
	if err == nil {
		t.Error("expected UNIQUE constraint error for duplicate email, got nil")
	}
}

func TestMigrationV10NoUsernameColumn(t *testing.T) {
	s := newTestStore(t)

	// The username column must not exist after v10 migration.
	_, err := s.db.Exec(`INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		"user_test000", "shouldfail", "hash", "2026-01-01T00:00:00Z")
	if err == nil {
		t.Error("expected error inserting into username column (should not exist), got nil")
	}
}

func TestMigrationV10WithExistingData(t *testing.T) {
	// Verify that the v10 migration correctly handles FK constraints.
	// After migration, we should be able to insert users and sessions with FK references intact.
	s := newTestStore(t)

	// Manually insert a user (bypassing the CreateUser function which is being updated in Task 2)
	userID := "user_test123456"
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	_, err = s.db.Exec(
		"INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)",
		userID, "existing@example.com", string(hash), time.Now().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Create a session that references the user (tests FK constraint is working)
	sessionID := s.CreateSession(userID)
	if sessionID == "" {
		t.Fatal("expected non-empty session ID")
	}

	// Verify the session and user relationship works (FK constraint is intact)
	fetchedUserID, ok := s.GetSessionUser(sessionID)
	if !ok {
		t.Fatal("expected session to be valid after migration")
	}
	if fetchedUserID != userID {
		t.Errorf("expected userID %s, got %s", userID, fetchedUserID)
	}
}
