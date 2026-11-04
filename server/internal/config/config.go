package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5"
)

type Config struct {
	DatabaseURL    string
	ListenAddr     string
	AppHost        string
	AppTZ          *time.Location
	SecretKey      []byte
	MailerFrom     string
	SMTPAddress    string
	SMTPPort       int
	SMTPUser       string
	SMTPPass       string
	AssumeSSL      bool
	TrustedProxies []netip.Prefix
}

func Load() (Config, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if _, err := pgx.ParseConfig(databaseURL); err != nil {
		return Config{}, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}

	secretKey := []byte(os.Getenv("SECRET_KEY"))
	if len(secretKey) < 32 {
		return Config{}, fmt.Errorf("SECRET_KEY must be at least 32 bytes")
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return Config{}, fmt.Errorf("invalid PORT: %w", err)
	}
	listenAddr := strings.TrimSpace(os.Getenv("LISTEN_ADDR"))
	if listenAddr == "" {
		listenAddr = ":" + port
	}

	timezone := strings.TrimSpace(os.Getenv("APP_TZ"))
	if timezone == "" {
		timezone = "Europe/Bratislava"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_TZ: %w", err)
	}

	appHost := strings.TrimSpace(os.Getenv("APP_HOST"))
	if appHost == "" {
		appHost = "localhost:8080"
	}
	smtpPort := 587
	if raw := strings.TrimSpace(os.Getenv("SMTP_PORT")); raw != "" {
		smtpPort, err = strconv.Atoi(raw)
		if err != nil || smtpPort < 1 || smtpPort > 65535 {
			return Config{}, fmt.Errorf("invalid SMTP_PORT %q", raw)
		}
	}

	assumeSSL, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("ASSUME_SSL")))
	if err != nil && os.Getenv("ASSUME_SSL") != "" {
		return Config{}, fmt.Errorf("invalid ASSUME_SSL: %w", err)
	}

	var trusted []netip.Prefix
	for _, value := range strings.Split(os.Getenv("TRUSTED_PROXY_CIDRS"), ",") {
		if strings.TrimSpace(value) == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			return Config{}, fmt.Errorf("invalid TRUSTED_PROXY_CIDRS: %w", err)
		}
		trusted = append(trusted, prefix)
	}
	return Config{
		TrustedProxies: trusted,
		DatabaseURL:    databaseURL,
		ListenAddr:     listenAddr,
		AppHost:        appHost,
		AppTZ:          location,
		SecretKey:      secretKey,
		MailerFrom:     envOr("MAILER_FROM", "from@example.com"),
		SMTPAddress:    strings.TrimSpace(os.Getenv("SMTP_ADDRESS")),
		SMTPPort:       smtpPort,
		SMTPUser:       os.Getenv("SMTP_USERNAME"),
		SMTPPass:       os.Getenv("SMTP_PASSWORD"),
		AssumeSSL:      assumeSSL,
	}, nil
}

func ValidateDatabaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" {
		return fmt.Errorf("DATABASE_URL must be a PostgreSQL connection URL")
	}
	return nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
