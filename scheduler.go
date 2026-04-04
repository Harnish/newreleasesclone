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
