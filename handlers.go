package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// getSessionUser extracts the authenticated user ID from the session cookie.
func getSessionUser(r *http.Request) (string, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return "", false
	}
	return store.GetSessionUser(cookie.Value)
}

// requireAuth wraps a handler that needs a user ID, returning 401 if no valid session.
func requireAuth(h func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := getSessionUser(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r, userID)
	}
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	tmpl.Execute(w, nil)
}

// GET /api/me — returns the current user or 401.
func handleMe(w http.ResponseWriter, r *http.Request, userID string) {
	user, err := store.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// POST /api/register
func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 3 {
		http.Error(w, "Username must be at least 3 characters", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	user, err := store.CreateUser(req.Username, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "Username already taken", http.StatusConflict)
		} else {
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
		}
		return
	}
	sessionID := store.CreateSession(user.ID)
	setSessionCookie(w, sessionID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// POST /api/login
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	user, err := store.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	sessionID := store.CreateSession(user.ID)
	setSessionCookie(w, sessionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// POST /api/logout
func handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if cookie, err := r.Cookie("session"); err == nil {
		store.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func setSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   30 * 24 * 3600,
	})
}

// /api/projects — GET list, POST add, DELETE remove (all require auth).
func handleProjects(w http.ResponseWriter, r *http.Request, userID string) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(store.GetUserRepos(userID))

	case http.MethodPost:
		var p Project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if p.Platform == "" || p.RepoURL == "" || p.Name == "" {
			http.Error(w, "name, platform, and repo_url are required", http.StatusBadRequest)
			return
		}
		repoID, err := store.AddRepo(userID, p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		go store.RefreshProject(repoID)
		p.ID = repoID
		p.LastRefresh = time.Now()
		json.NewEncoder(w).Encode(p)

	case http.MethodDelete:
		repoID := r.URL.Query().Get("id")
		if repoID == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}
		if store.RemoveUserRepo(userID, repoID) {
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": repoID})
		} else {
			http.Error(w, "Project not found", http.StatusNotFound)
		}
	}
}

func handleReleases(w http.ResponseWriter, r *http.Request, userID string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(store.GetUserReleases(userID))
}

func handleRefreshCheck(w http.ResponseWriter, r *http.Request, userID string) {
	w.Header().Set("Content-Type", "application/json")
	stale := store.GetStaleRepos()
	for _, p := range stale {
		go store.RefreshProject(p.ID)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"refreshed_count": len(stale),
		"stale_projects":  stale,
	})
}

// /api/webhooks — GET list for a repo, POST add, DELETE remove.
func handleWebhooks(w http.ResponseWriter, r *http.Request, userID string) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		repoID := r.URL.Query().Get("repo_id")
		if repoID == "" {
			http.Error(w, "missing repo_id", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(store.GetUserWebhooksForRepo(userID, repoID))

	case http.MethodPost:
		var req struct {
			RepoID string `json:"repo_id"`
			URL    string `json:"url"`
			Secret string `json:"secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RepoID == "" || req.URL == "" {
			http.Error(w, "repo_id and url are required", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
			http.Error(w, "url must start with http:// or https://", http.StatusBadRequest)
			return
		}
		id, err := store.AddWebhook(userID, req.RepoID, req.URL, req.Secret)
		if err != nil {
			http.Error(w, "failed to add webhook", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"id": id})

	case http.MethodDelete:
		whID := r.URL.Query().Get("id")
		if whID == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		if store.DeleteWebhook(userID, whID) {
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
		} else {
			http.Error(w, "webhook not found", http.StatusNotFound)
		}

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GET /api/push/vapid-key — returns the VAPID public key for subscription setup.
func handlePushVapidKey(w http.ResponseWriter, r *http.Request, userID string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"publicKey": store.GetVAPIDPublicKey()})
}

// POST /api/push/subscribe — save a push subscription.
// DELETE /api/push/subscribe — remove a push subscription.
func handlePushSubscribe(w http.ResponseWriter, r *http.Request, userID string) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			Endpoint string `json:"endpoint"`
			Keys     struct {
				P256dh string `json:"p256dh"`
				Auth   string `json:"auth"`
			} `json:"keys"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Endpoint == "" {
			http.Error(w, "invalid subscription", http.StatusBadRequest)
			return
		}
		if err := store.SavePushSubscription(userID, body.Endpoint, body.Keys.P256dh, body.Keys.Auth); err != nil {
			http.Error(w, "failed to save subscription", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	case http.MethodDelete:
		var body struct {
			Endpoint string `json:"endpoint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Endpoint == "" {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		store.DeletePushSubscription(userID, body.Endpoint)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// POST /api/project-settings — update per-project settings (push_enabled).
func handleProjectSettings(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RepoID      string `json:"repo_id"`
		PushEnabled bool   `json:"push_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.RepoID == "" {
		http.Error(w, "repo_id is required", http.StatusBadRequest)
		return
	}
	ok, err := store.SetProjectPushEnabled(userID, req.RepoID, req.PushEnabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleRefreshProject(w http.ResponseWriter, r *http.Request, userID string) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	repoID := r.URL.Query().Get("id")
	if repoID == "" {
		http.Error(w, "Missing project id", http.StatusBadRequest)
		return
	}
	store.RefreshProject(repoID)
	json.NewEncoder(w).Encode(map[string]string{"status": "refreshed", "project_id": repoID})
}
