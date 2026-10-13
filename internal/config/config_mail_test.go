package config

import (
	"strings"
	"testing"
)

func TestMailConfigDisabledByDefault(t *testing.T) {
	t.Setenv("APP_BASE_URL", "http://127.0.0.1:9")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mail.IMAPHost != "" {
		t.Fatalf("poller should be off without MAIL_IMAP_HOST: %+v", cfg.Mail)
	}
	if cfg.Mail.IMAPPort != "993" || cfg.Mail.IMAPFolder != "INBOX" ||
		cfg.Mail.ProcessedAction != "flag" || cfg.Mail.IMAPInterval.Minutes() != 5 {
		t.Fatalf("defaults: %+v", cfg.Mail)
	}
}

func TestMailConfigValid(t *testing.T) {
	t.Setenv("APP_BASE_URL", "http://127.0.0.1:9")
	t.Setenv("MAIL_IMAP_HOST", "imap.example.com")
	t.Setenv("MAIL_IMAP_USERNAME", "belege@example.com")
	t.Setenv("MAIL_IMAP_PASSWORD", "secret")
	t.Setenv("MAIL_FORWARD_DOMAIN", "@Example.com")
	t.Setenv("MAIL_IMAP_PROCESSED_ACTION", "move:Processed")
	t.Setenv("MAIL_IMAP_INTERVAL", "15m")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mail.IMAPHost != "imap.example.com" || cfg.Mail.IMAPUser != "belege@example.com" ||
		cfg.Mail.IMAPPassword != "secret" || cfg.Mail.ForwardDomain != "example.com" ||
		cfg.Mail.ProcessedAction != "move:Processed" || cfg.Mail.IMAPInterval.Minutes() != 15 {
		t.Fatalf("mail: %+v", cfg.Mail)
	}
}

func TestMailConfigRequiresCredentials(t *testing.T) {
	t.Setenv("APP_BASE_URL", "http://127.0.0.1:9")
	t.Setenv("MAIL_IMAP_HOST", "imap.example.com")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "MAIL_IMAP_USERNAME") {
		t.Fatalf("want a credentials error, got %v", err)
	}
}

func TestMailConfigRejectsBadInterval(t *testing.T) {
	t.Setenv("APP_BASE_URL", "http://127.0.0.1:9")
	t.Setenv("MAIL_IMAP_HOST", "imap.example.com")
	t.Setenv("MAIL_IMAP_USERNAME", "u")
	t.Setenv("MAIL_IMAP_PASSWORD", "p")
	t.Setenv("MAIL_FORWARD_DOMAIN", "@Example.com")
	t.Setenv("MAIL_IMAP_INTERVAL", "0s")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "MAIL_IMAP_INTERVAL") {
		t.Fatalf("want an interval error, got %v", err)
	}
}
