package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	log.Println("Starting Release Tracker...")

	var err error
	store, err = NewStore("data/newreleases.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/sw.js", handleServiceWorker)
	http.HandleFunc("/api/me", requireAuth(handleMe))
	http.HandleFunc("/api/register", handleRegister)
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/logout", handleLogout)
	http.HandleFunc("/api/projects", requireAuth(handleProjects))
	http.HandleFunc("/api/releases", requireAuth(handleReleases))
	http.HandleFunc("/api/refresh-check", requireAuth(handleRefreshCheck))
	http.HandleFunc("/api/refresh", requireAuth(handleRefreshProject))
	http.HandleFunc("/api/push/vapid-key", requireAuth(handlePushVapidKey))
	http.HandleFunc("/api/push/subscribe", requireAuth(handlePushSubscribe))

	fmt.Println("Release Tracker running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
