package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func handleHome(w http.ResponseWriter, r *http.Request) {
	tmpl.Execute(w, nil)
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(store.GetProjects())

	case http.MethodPost:
		var p Project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p.ID = fmt.Sprintf("proj_%d", time.Now().UnixNano())
		p.LastRefresh = time.Now()
		store.AddProject(p)
		go store.RefreshProject(p.ID)
		json.NewEncoder(w).Encode(p)

	case http.MethodDelete:
		projectID := r.URL.Query().Get("id")
		if projectID == "" {
			http.Error(w, "Missing project id parameter", http.StatusBadRequest)
			return
		}
		if store.DeleteProject(projectID) {
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": projectID})
		} else {
			http.Error(w, "Project not found", http.StatusNotFound)
		}
	}
}

func handleReleases(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(store.GetAllReleases())
}

func handleRefreshCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	staleProjects := store.GetStaleProjects()
	for _, p := range staleProjects {
		go store.RefreshProject(p.ID)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"refreshed_count": len(staleProjects),
		"stale_projects":  staleProjects,
	})
}

func handleRefreshProject(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	projectID := r.URL.Query().Get("id")
	if projectID == "" {
		http.Error(w, "Missing project id", http.StatusBadRequest)
		return
	}
	store.RefreshProject(projectID)
	json.NewEncoder(w).Encode(map[string]string{"status": "refreshed", "project_id": projectID})
}
