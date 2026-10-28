package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRegisterWithEmail(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	body := `{"password":"password123","email":"new@example.com"}`
	req, _ := http.NewRequest("POST", "/api/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "localhost:8080")
	w := httptest.NewRecorder()
	handleRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var user User
	if err := json.NewDecoder(w.Body).Decode(&user); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if user.Email != "new@example.com" {
		t.Errorf("expected email new@example.com in response, got %q", user.Email)
	}
	if user.EmailVerified {
		t.Error("expected email_verified false on fresh registration")
	}
}

func TestHandleRegisterMissingEmail(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	body := `{"password":"password123"}`
	req, _ := http.NewRequest("POST", "/api/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleRegister(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateUserByEmail(t *testing.T) {
	s := newTestStore(t)
	user, err := s.CreateUser("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %q", user.Email)
	}
	if user.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("dup@example.com", "password123"); err != nil {
		t.Fatalf("first CreateUser: %v", err)
	}
	_, err := s.CreateUser("dup@example.com", "password123")
	if err == nil {
		t.Error("expected error for duplicate email, got nil")
	}
}

func TestAuthenticateUserByEmail(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("bob@example.com", "secret123"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	user, err := s.AuthenticateUser("bob@example.com", "secret123")
	if err != nil {
		t.Fatalf("AuthenticateUser: %v", err)
	}
	if user.Email != "bob@example.com" {
		t.Errorf("expected email bob@example.com, got %q", user.Email)
	}
}

func TestAuthenticateUserWrongPassword(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("carol@example.com", "rightpass"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	_, err := s.AuthenticateUser("carol@example.com", "wrongpass")
	if err == nil {
		t.Error("expected error for wrong password, got nil")
	}
}
