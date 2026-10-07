package config

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/platform/secrets"
)

// Environments (FRONKO_ENV). Production is the default, so a deployment
// that forgets to set it gets the safe behaviour.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// Environment returns FRONKO_ENV, defaulting to production.
func Environment() (string, error) {
	switch env := os.Getenv("FRONKO_ENV"); env {
	case "", EnvProduction:
		return EnvProduction, nil
	case EnvDevelopment:
		return env, nil
	default:
		return "", fmt.Errorf("FRONKO_ENV: must be %q or %q, got %q", EnvDevelopment, EnvProduction, env)
	}
}

type Config struct {
	// Env is FRONKO_ENV: development or production.
	Env string
	// PublicURL is where people reach this deployment (PUBLIC_URL), with no
	// trailing slash, e.g. https://cards.example.com. Integrations need it
	// for OAuth redirects, SAML and SCIM endpoints; those that do are shown
	// as unavailable when it is empty.
	PublicURL string
	// JobWorkers is how many background jobs this instance runs at once
	// (JOB_WORKERS, default 2). Zero leaves the queue to other instances.
	JobWorkers int

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

	// AnalyticsRetention is how long card analytics events are kept (ANALYTICS_RETENTION_DAYS, default 395).
	AnalyticsRetention time.Duration
}

// defaultAnalyticsRetentionDays is about 13 months, so a year can be compared with the one before.
const defaultAnalyticsRetentionDays = 395

const (
	defaultJobWorkers = 2
	maxJobWorkers     = 32
)

// Load reads configuration from the environment. DATABASE_URL and JWT_SECRET
// are required: an empty JWT secret would let anyone forge tokens.
func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	env, err := Environment()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Env:        env,
		JobWorkers: defaultJobWorkers,

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

		AnalyticsRetention: defaultAnalyticsRetentionDays * 24 * time.Hour,
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

	if raw := os.Getenv("ANALYTICS_RETENTION_DAYS"); raw != "" {
		days, err := strconv.Atoi(raw)
		if err != nil || days < 1 || days > 3650 {
			return nil, fmt.Errorf("ANALYTICS_RETENTION_DAYS: must be a number of days between 1 and 3650, got %q", raw)
		}
		cfg.AnalyticsRetention = time.Duration(days) * 24 * time.Hour
	}

	if raw := strings.TrimSpace(os.Getenv("PUBLIC_URL")); raw != "" {
		public, err := parsePublicURL(raw)
		if err != nil {
			return nil, fmt.Errorf("PUBLIC_URL: %w", err)
		}
		cfg.PublicURL = public
	}

	if raw := os.Getenv("JOB_WORKERS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > maxJobWorkers {
			return nil, fmt.Errorf("JOB_WORKERS: must be a number between 0 and %d, got %q", maxJobWorkers, raw)
		}
		cfg.JobWorkers = n
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

// parsePublicURL checks that raw is a bare http(s) origin, optionally with a
// path prefix, and returns it without a trailing slash.
func parsePublicURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", fmt.Errorf("must be an absolute http(s) URL such as https://cards.example.com, got %q", raw)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("must not contain credentials, a query or a fragment, got %q", raw)
	}
	return strings.TrimRight(u.String(), "/"), nil
}
