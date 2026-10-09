package config

import (
	"testing"
)

func TestLoadRequiresBaseURL(t *testing.T) {
	t.Setenv("APP_BASE_URL", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !stringsContains(err.Error(), "APP_BASE_URL") {
		t.Fatal(err)
	}
}

func TestLoadRejectsHeaderAuthWithoutProxies(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://reisekosten.example")
	t.Setenv("HEADER_AUTH_ENABLED", "true")
	t.Setenv("TRUSTED_PROXIES", "")
	_, err := Load()
	if err == nil || !stringsContains(err.Error(), "TRUSTED_PROXIES") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadRejectsNonEUS3(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://reisekosten.example")
	t.Setenv("STORAGE_BACKEND", "s3")
	t.Setenv("S3_ENDPOINT", "https://s3.example")
	t.Setenv("S3_REGION", "us-east-1")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_ACCESS_KEY_ID", "key")
	t.Setenv("S3_SECRET_ACCESS_KEY", "secret")
	t.Setenv("S3_DATA_LOCATION", "US")
	_, err := Load()
	if err == nil || !stringsContains(err.Error(), "S3_DATA_LOCATION") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadRejectsAllAuthDisabled(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://reisekosten.example")
	t.Setenv("AUTH_PASSWORD_ENABLED", "false")
	t.Setenv("OIDC_ENABLED", "false")
	t.Setenv("HEADER_AUTH_ENABLED", "false")
	_, err := Load()
	if err == nil || !stringsContains(err.Error(), "login method") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadOK(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://reisekosten.example/")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OriginURL() != "https://reisekosten.example" {
		t.Fatal(cfg.OriginURL())
	}
	if cfg.CookieName("rk_session") != "__Host-rk_session" {
		t.Fatal(cfg.CookieName("rk_session"))
	}
	if cfg.DefaultLocale != "de" || cfg.Argon2MemoryKiB != 65536 {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoadAIRequiresEndpoint(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://reisekosten.example")
	t.Setenv("AI_ENABLED", "true")
	_, err := Load()
	if err == nil || !stringsContains(err.Error(), "AI_BASE_URL") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadAIOffWhenUnset(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://reisekosten.example")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AI.Enabled || cfg.AI.PublicBaseURL() != "" {
		t.Fatalf("%+v", cfg.AI)
	}
}

func TestParseRate(t *testing.T) {
	r, err := ParseRate("20/h")
	if err != nil || r.Count != 20 {
		t.Fatal(r, err)
	}
	if _, err := ParseRate("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || contains(s, sub))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
