package main

import (
	"log"
	"time"
)

// nextSevenAMUTC returns the duration from now until the next 7:00:00 UTC.
// Always returns a value in the range (0, 24h].
func nextSevenAMUTC() time.Duration {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return time.Until(next)
}

// runDailyDigest sleeps until the next 7:00 UTC then fires the daily digest,
// repeating every 24 hours. Intended to run as a goroutine.
func runDailyDigest() {
	time.Sleep(nextSevenAMUTC())
	sendDailyDigestToAll()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		sendDailyDigestToAll()
	}
}

// sendDailyDigestToAll queries all opted-in verified users and sends each a
// summary of releases published the previous UTC calendar day. Skips users
// with no releases that day.
func sendDailyDigestToAll() {
	users := store.GetDigestUsers()
	if len(users) == 0 {
		return
	}
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	for _, u := range users {
		releases := store.GetReleasesPublishedOn(u.ID, yesterday)
		if len(releases) == 0 {
			continue
		}
		if err := smtpCfg.SendDailySummary(u.Email, releases); err != nil {
			log.Printf("⚠ digest send failed for %s: %v", u.Email, err)
		}
	}
}

// ---- GitLab sync scheduling ----

func gitlabSyncFrequencyDuration(freq string) time.Duration {
	switch freq {
	case "weekly":
		return 7 * 24 * time.Hour
	case "monthly":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

func gitlabSyncDue(lastSyncAt time.Time, freq string) bool {
	if lastSyncAt.IsZero() {
		return true
	}
	return time.Since(lastSyncAt) > gitlabSyncFrequencyDuration(freq)
}

// runGitLabSyncScheduler checks hourly for GitLab-sync-enabled projects that
// are due per their daily/weekly/monthly frequency and fires a background
// sync for each. Hourly (not once-daily like runDailyDigest) because
// frequency is per-project and the coarsest granularity (daily) needs finer
// polling than a once-a-day tick to fire promptly.
func runGitLabSyncScheduler() {
	checkDueGitLabSyncs()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		checkDueGitLabSyncs()
	}
}

func checkDueGitLabSyncs() {
	for _, t := range store.GetAllEnabledGitLabSyncTargets() {
		if gitlabSyncDue(t.LastSyncAt, t.Frequency) {
			go store.syncProjectToGitLab(t)
		}
	}
}
