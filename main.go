package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	log.Println("🚀 Starting Release Tracker...")

	var err error
	store, err = NewStore("data/newreleases.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	projects := store.GetProjects()
	log.Printf("📊 Found %d projects in database", len(projects))

	if len(projects) == 0 {
		log.Println("Empty database — seeding demo projects...")
		seedDemoData()
	}

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/api/projects", handleProjects)
	http.HandleFunc("/api/releases", handleReleases)
	http.HandleFunc("/api/refresh-check", handleRefreshCheck)
	http.HandleFunc("/api/refresh", handleRefreshProject)

	fmt.Println("🚀 Release Tracker running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func seedDemoData() {
	projects := []Project{
		{ID: "1", Name: "kubernetes", Platform: "github", RepoURL: "https://github.com/kubernetes/kubernetes"},
		{ID: "2", Name: "react", Platform: "npm", RepoURL: "https://github.com/facebook/react"},
		{ID: "3", Name: "golang", Platform: "github", RepoURL: "https://github.com/golang/go"},
	}
	for _, p := range projects {
		store.AddProject(p)
		go store.RefreshProject(p.ID)
	}
	log.Println("📦 Seeding projects with live API data...")
}
