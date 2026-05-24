package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	gomail "gopkg.in/gomail.v2"
)

// SMTPConfig holds SMTP connection settings read from environment variables.
type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// SMTPConfigFromEnv reads SMTP settings from environment variables.
// Returns (config, true) if all required vars are present, (zero, false) otherwise.
// Accepts SMTP_USERNAME as a fallback for SMTP_USER, and FROM_EMAIL as a fallback for SMTP_FROM.
func SMTPConfigFromEnv() (SMTPConfig, bool) {
	host := os.Getenv("SMTP_HOST")
	user := os.Getenv("SMTP_USER")
	if user == "" {
		user = os.Getenv("SMTP_USERNAME")
	}
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = os.Getenv("FROM_EMAIL")
	}
	if from == "" {
		from = user
	}
	if host == "" || user == "" || pass == "" {
		return SMTPConfig{}, false
	}
	port := 587
	if p := os.Getenv("SMTP_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	return SMTPConfig{Host: host, Port: port, User: user, Pass: pass, From: from}, true
}

// SendMail sends a plain-text email via SMTP using gomail.
// Port 465 uses implicit TLS; port 587 uses STARTTLS.
func (c SMTPConfig) SendMail(to, subject, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", c.From)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	d := gomail.NewDialer(c.Host, c.Port, c.User, c.Pass)
	d.SSL = c.Port == 465

	log.Printf("smtp: sending to=%s subject=%q", to, subject)
	if err := d.DialAndSend(m); err != nil {
		log.Printf("smtp: send failed to=%s subject=%q error=%v", to, subject, err)
		return err
	}
	log.Printf("smtp: sent ok to=%s subject=%q", to, subject)
	return nil
}

// SendVerificationEmail sends a verification link email to the given address.
// baseURL should be e.g. "https://example.com" (no trailing slash).
func (c SMTPConfig) SendVerificationEmail(to, token, baseURL string) error {
	link := baseURL + "/verify-email?token=" + token
	body := fmt.Sprintf(
		"Hello,\n\nPlease verify your email address by clicking the link below:\n\n%s\n\nThis link expires in 24 hours.\n\nIf you did not create an account, you can ignore this email.\n",
		link,
	)
	return c.SendMail(to, "Verify your Release Tracker email", body)
}

// buildDailySummaryBody formats the plain-text body for the daily digest email.
// Each release is rendered as: "<Name> <Version> — <URL>"
func buildDailySummaryBody(releases []Release) string {
	var buf strings.Builder
	buf.WriteString("Here are the releases from yesterday:\n\n")
	for _, r := range releases {
		fmt.Fprintf(&buf, "%s %s — %s\n", r.Name, r.Version, r.URL)
	}
	return buf.String()
}

// SendDailySummary sends the daily release digest to the given address.
func (c SMTPConfig) SendDailySummary(to string, releases []Release) error {
	return c.SendMail(to, "Your daily release summary", buildDailySummaryBody(releases))
}
