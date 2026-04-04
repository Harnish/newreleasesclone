package main

import (
	"fmt"
	"log"
	"net/http"
)

var (
	smtpCfg     SMTPConfig
	smtpEnabled bool
)

func main() {
	log.Println("Starting Release Tracker...")

	var err error
	store, err = NewStore("data/newreleases.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	smtpCfg, smtpEnabled = SMTPConfigFromEnv()
	if !smtpEnabled {
		log.Println("⚠ SMTP not configured — email features disabled")
	}

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/sw.js", handleServiceWorker)
	http.HandleFunc("/verify-email", handleVerifyEmail)
	http.HandleFunc("/api/me", requireAuth(handleMe))
	http.HandleFunc("/api/register", handleRegister)
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/logout", handleLogout)
	http.HandleFunc("/api/resend-verification", requireAuth(handleResendVerification))
	http.HandleFunc("/api/projects", requireAuth(handleProjects))
	http.HandleFunc("/api/releases", requireAuth(handleReleases))
	http.HandleFunc("/api/refresh-check", requireAuth(handleRefreshCheck))
	http.HandleFunc("/api/refresh", requireAuth(handleRefreshProject))
	http.HandleFunc("/api/webhooks", requireAuth(handleWebhooks))
	http.HandleFunc("/api/project-settings", requireAuth(handleProjectSettings))
	http.HandleFunc("/api/push/vapid-key", requireAuth(handlePushVapidKey))
	http.HandleFunc("/api/push/subscribe", requireAuth(handlePushSubscribe))

	fmt.Println("Release Tracker running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
