package config

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"strconv"
	"strings"

	"github.com/faiz-gh/fronko/backend/internal/secrets"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
	// CookieSecure marks the session cookie Secure (HTTPS only). Disable only for plain-http local dev.
	CookieSecure bool
	// TrustProxy reads the client IP from X-Real-IP; enable only behind a proxy that sets it (our nginx).
	TrustProxy bool
	// SecretsKey encrypts users' storage credentials at rest. Nil disables file storage.
	SecretsKey []byte
	// CORSAllowedOrigins may call the API from the browser with the session cookie
	// (e.g. the frontend on its own domain). Empty when nginx proxies everything same-origin.
	CORSAllowedOrigins []string
	// StorageAllowPrivate lets storage endpoints use http and private addresses (local MinIO only).
	StorageAllowPrivate bool

	// Outgoing mail for verification and password-reset codes. An empty
	// SMTPHost logs messages instead of sending them (development only).
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// FeedbackNotifyEmail gets an email for each piece of product feedback,
	// and is the Reply-To on admins' replies. Optional.
	FeedbackNotifyEmail string
}

// Load reads configuration from the environment. DATABASE_URL and JWT_SECRET
// are required: an empty JWT secret would let anyone forge tokens.
func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	cfg := &Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		Port:         port,
		CookieSecure: os.Getenv("COOKIE_SECURE") != "false",
		TrustProxy:   os.Getenv("TRUST_PROXY") == "true",

		CORSAllowedOrigins: strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ","),

		StorageAllowPrivate: os.Getenv("STORAGE_ALLOW_PRIVATE_ENDPOINTS") == "true",

		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     587,
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     os.Getenv("SMTP_FROM"),

		FeedbackNotifyEmail: strings.TrimSpace(os.Getenv("FEEDBACK_NOTIFY_EMAIL")),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 16 {
		return nil, errors.New("JWT_SECRET is required and must be at least 16 characters")
	}

	if raw := os.Getenv("SMTP_PORT"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("SMTP_PORT: invalid port %q", raw)
		}
		cfg.SMTPPort = port
	}
	if cfg.SMTPHost != "" && cfg.SMTPFrom == "" {
		return nil, errors.New("SMTP_FROM is required when SMTP_HOST is set")
	}

	if cfg.FeedbackNotifyEmail != "" {
		addr, err := mail.ParseAddress(cfg.FeedbackNotifyEmail)
		if err != nil {
			return nil, fmt.Errorf("FEEDBACK_NOTIFY_EMAIL: %w", err)
		}
		cfg.FeedbackNotifyEmail = addr.Address
	}

	// Optional: without it the app runs, but file storage is switched off.
	if raw := os.Getenv("SECRETS_KEY"); raw != "" {
		key, err := secrets.ParseKey(raw)
		if err != nil {
			return nil, fmt.Errorf("SECRETS_KEY: %w", err)
		}
		cfg.SecretsKey = key
	}

	return cfg, nil
}
