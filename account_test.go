package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetPageSize(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	// valid sizes
	for _, size := range []int{10, 20, 50} {
		if err := s.SetPageSize(userID, size); err != nil {
			t.Fatalf("SetPageSize(%d): %v", size, err)
		}
		user, err := s.GetUserByID(userID)
		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}
		if user.PageSize != size {
			t.Errorf("expected page_size %d, got %d", size, user.PageSize)
		}
	}

	// invalid size rejected
	if err := s.SetPageSize(userID, 7); err == nil {
		t.Error("expected error for invalid page_size 7, got nil")
	}
}

func TestHandleAccountSettingsGET(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	userID, cookie := newTestAuth(t, store)
	_ = userID

	req, _ := http.NewRequest("GET", "/api/account-settings", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["email_digest"].(bool) {
		t.Error("expected email_digest false by default")
	}
}

func TestHandleAccountSettingsPOST(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	userID, cookie := newTestAuth(t, store)

	body := `{"email_digest":true}`
	req, _ := http.NewRequest("POST", "/api/account-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	enabled, err := store.GetUserDigestEnabled(userID)
	if err != nil {
		t.Fatalf("GetUserDigestEnabled: %v", err)
	}
	if !enabled {
		t.Error("expected email_digest enabled after POST")
	}
}

func TestHandleAccountSettingsMethodNotAllowed(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("DELETE", "/api/account-settings", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestHandleMeIncludesRSSToken verifies /api/me response includes rss_token.
func TestHandleMeIncludesRSSToken(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)
	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleMe).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	token, ok := resp["rss_token"].(string)
	if !ok || token == "" {
		t.Errorf("expected non-empty rss_token in /api/me response, got %v", resp["rss_token"])
	}
}

// TestUserPageSizeDefault tests that new users get a default page_size of 10
func TestUserPageSizeDefault(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	user, err := s.GetUserByID(userID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.PageSize != 10 {
		t.Errorf("expected default page_size 10, got %d", user.PageSize)
	}
}

func TestHandleAccountSettingsPageSizeGET(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("GET", "/api/account-settings", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	ps, ok := resp["page_size"]
	if !ok {
		t.Fatal("expected page_size in response")
	}
	if int(ps.(float64)) != 10 {
		t.Errorf("expected default page_size 10, got %v", ps)
	}
}

func TestHandleAccountSettingsPageSizePOST(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	userID, cookie := newTestAuth(t, store)

	body := `{"page_size":50}`
	req, _ := http.NewRequest("POST", "/api/account-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	user, err := store.GetUserByID(userID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.PageSize != 50 {
		t.Errorf("expected page_size 50, got %d", user.PageSize)
	}
}

func TestHandleAccountSettingsPageSizeInvalid(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	_, cookie := newTestAuth(t, store)

	body := `{"page_size":7}`
	req, _ := http.NewRequest("POST", "/api/account-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid page_size, got %d", w.Code)
	}
}
