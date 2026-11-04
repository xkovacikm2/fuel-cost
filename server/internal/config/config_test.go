package config

import (
	"testing"
	"time"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SECRET_KEY", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded without DATABASE_URL")
	}
}

func TestLoadRequiresStrongSecretKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/spotreba_development")
	t.Setenv("SECRET_KEY", "too-short")

	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded with a short SECRET_KEY")
	}
}

func TestLoadUsesDevelopmentDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/spotreba_development")
	t.Setenv("SECRET_KEY", "development-key-that-is-at-least-32-bytes-long")
	t.Setenv("PORT", "")
	t.Setenv("APP_TZ", "")
	t.Setenv("APP_HOST", "")

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want :8080", got.ListenAddr)
	}
	if got.AppHost != "localhost:8080" {
		t.Errorf("AppHost = %q, want localhost:8080", got.AppHost)
	}
	want, err := time.LoadLocation("Europe/Bratislava")
	if err != nil {
		t.Fatal(err)
	}
	if got.AppTZ.String() != want.String() {
		t.Errorf("AppTZ = %q, want %q", got.AppTZ, want)
	}
}

func TestLoadUsesConfiguredPortAndTimezone(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/spotreba_development")
	t.Setenv("SECRET_KEY", "development-key-that-is-at-least-32-bytes-long")
	t.Setenv("PORT", "9090")
	t.Setenv("APP_TZ", "UTC")
	t.Setenv("APP_HOST", "spotreba.example.com")

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q, want :9090", got.ListenAddr)
	}
	if got.AppHost != "spotreba.example.com" {
		t.Errorf("AppHost = %q, want spotreba.example.com", got.AppHost)
	}
	if got.AppTZ != time.UTC {
		t.Errorf("AppTZ = %q, want UTC", got.AppTZ)
	}
}

func TestLoadParsesTrustedProxyCIDRs(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/spotreba_development")
	t.Setenv("SECRET_KEY", "development-key-that-is-at-least-32-bytes-long")
	t.Setenv("TRUSTED_PROXY_CIDRS", "127.0.0.0/8, 10.1.0.0/16")

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.TrustedProxies) != 2 || got.TrustedProxies[0].String() != "127.0.0.0/8" || got.TrustedProxies[1].String() != "10.1.0.0/16" {
		t.Fatalf("TrustedProxies = %v", got.TrustedProxies)
	}
}

func TestLoadRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/spotreba_development")
	t.Setenv("SECRET_KEY", "development-key-that-is-at-least-32-bytes-long")
	t.Setenv("TRUSTED_PROXY_CIDRS", "not-a-cidr")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an invalid trusted proxy range")
	}
}
