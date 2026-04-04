package main

import (
	"fmt"
	"net/smtp"
	"os"
	"strconv"
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
func SMTPConfigFromEnv() (SMTPConfig, bool) {
	host := os.Getenv("SMTP_HOST")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")
	if host == "" || user == "" || pass == "" || from == "" {
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

// SendMail sends a plain-text email via SMTP using AUTH PLAIN.
func (c SMTPConfig) SendMail(to, subject, body string) error {
	auth := smtp.PlainAuth("", c.User, c.Pass, c.Host)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		c.From, to, subject, body,
	)
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	return smtp.SendMail(addr, auth, c.From, []string{to}, []byte(msg))
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
