package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

var (
	smtpCfg     SMTPConfig
	smtpEnabled bool
)

// version is the git commit this binary was built from. The Dockerfile sets it
// with -ldflags "-X main.version=<sha>"; local `go build` falls back to the
// VCS revision Go embeds, else "dev".
var version = "dev"

func buildVersion() string {
	if version != "dev" {
		if len(version) > 12 { // full SHA from CI
			return version[:7]
		}
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				return s.Value[:7]
			}
		}
	}
	return version
}

func main() {
	log.Printf("Starting Release Tracker (build %s)...", buildVersion())

	var err error
	store, err = NewStore("data/newreleases.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	smtpCfg, smtpEnabled = SMTPConfigFromEnv()
	if !smtpEnabled {
		log.Println("⚠ SMTP not configured — email features disabled")
	}

	if smtpEnabled {
		go runDailyDigest()
	}
	go runGitLabSyncScheduler()

	http.HandleFunc("/", handleHome)
	http.Handle("/static/", staticHandler())
	http.HandleFunc("/sw.js", handleServiceWorker)
	http.HandleFunc("/verify-email", handleVerifyEmail)
	http.HandleFunc("/feed/", handleFeed)
	http.HandleFunc("/api/me", requireAuth(handleMe))
	http.HandleFunc("/api/icon", requireAuth(handleIcon))
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
	http.HandleFunc("/api/project-tags", requireAuth(handleProjectTags))
	http.HandleFunc("/api/tags", requireAuth(handleTags))
	http.HandleFunc("/api/account-settings", requireAuth(handleAccountSettings))
	http.HandleFunc("/api/push/vapid-key", requireAuth(handlePushVapidKey))
	http.HandleFunc("/api/push/subscribe", requireAuth(handlePushSubscribe))
	http.HandleFunc("/api/gitlab-settings", requireAuth(handleGitLabSettings))
	http.HandleFunc("/api/gitlab-settings/awesome", requireAuth(handleGitLabAwesome))
	http.HandleFunc("/api/project-gitlab-sync", requireAuth(handleProjectGitLabSync))
	http.HandleFunc("/api/project-gitlab-sync/sync-now", requireAuth(handleProjectGitLabSyncNow))

	fmt.Println("Release Tracker running on http://localhost:8080")
	// Fetchers use http.Get (DefaultClient), which has no timeout by default.
	http.DefaultClient.Timeout = 30 * time.Second
	srv := &http.Server{Addr: ":8080", ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
