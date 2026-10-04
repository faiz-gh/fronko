package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/config"
	"github.com/faiz-gh/fronko/backend/internal/database"
	"github.com/faiz-gh/fronko/backend/internal/handlers"
	"github.com/faiz-gh/fronko/backend/internal/mail"
	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/repository"
	"github.com/faiz-gh/fronko/backend/internal/secrets"
	"github.com/faiz-gh/fronko/backend/internal/storage"
	"golang.org/x/time/rate"
)

// Per-IP budgets for unauthenticated endpoints that are easy to abuse.
const (
	leadBurst    = 5
	leadInterval = 15 * time.Second // one more lead per interval after the burst
	authBurst    = 10
	authInterval = 10 * time.Second
	// Uploads are authenticated, but each one costs bandwidth and a bucket write.
	uploadBurst    = 10
	uploadInterval = 6 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run holds the real startup logic so deferred cleanup (like closing the pool)
// still happens when we exit with an error.
func run() error {
	log.Println("Starting Fronko backend...")

	// 1. Load config from os.Getenv
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 2. Initialize DB (pgxpool)
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// 3. Initialize repositories, auth service, and handlers (Manual DI)
	repo := repository.New(pool)
	authService := auth.NewService(cfg.JWTSecret)

	var mailer mail.Sender = mail.LogSender{}
	if cfg.SMTPHost != "" {
		smtpSender, err := mail.NewSMTPSender(mail.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
		})
		if err != nil {
			return fmt.Errorf("SMTP_FROM: %w", err)
		}
		mailer = smtpSender
	} else {
		log.Println("SMTP_HOST not set: emails (verification and reset codes) are logged, not sent")
	}

	authHandler := handlers.NewAuthHandler(repo, authService, mailer, cfg.CookieSecure)
	profileHandler := handlers.NewProfileHandler(repo)
	leadHandler := handlers.NewLeadHandler(repo)

	// File storage needs SECRETS_KEY to encrypt users' bucket keys; without it the feature is off.
	var box *secrets.Box
	if cfg.SecretsKey != nil {
		if box, err = secrets.New(cfg.SecretsKey); err != nil {
			return err
		}
	} else {
		log.Println("SECRETS_KEY not set: file storage is disabled")
	}
	if cfg.StorageAllowPrivate {
		log.Println("STORAGE_ALLOW_PRIVATE_ENDPOINTS is on: storage may reach private addresses (development only)")
	}
	storageSvc := handlers.NewStorageService(repo, box, storage.NewS3(cfg.StorageAllowPrivate), cfg.StorageAllowPrivate)
	storageHandler := handlers.NewStorageHandler(storageSvc)
	fileHandler := handlers.NewFileHandler(storageSvc, repo)

	sessions := middleware.SessionCheckerFunc(func(ctx context.Context, userID int64) (int, bool, error) {
		version, verified, err := repo.GetSessionState(ctx, userID)
		if errors.Is(err, repository.ErrNotFound) {
			err = middleware.ErrSessionUserNotFound
		}
		return version, verified, err
	})
	jwtMiddleware := middleware.JWTMiddleware(authService, sessions)
	leadLimiter := middleware.NewRateLimiter(ctx, rate.Every(leadInterval), leadBurst, cfg.TrustProxy)
	authLimiter := middleware.NewRateLimiter(ctx, rate.Every(authInterval), authBurst, cfg.TrustProxy)
	uploadLimiter := middleware.NewRateLimiter(ctx, rate.Every(uploadInterval), uploadBurst, cfg.TrustProxy)

	// 4. Setup net/http ServeMux with routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Public Routes
	mux.HandleFunc("POST /auth/login", authLimiter.Limit(authHandler.Login))
	mux.HandleFunc("POST /auth/register", authLimiter.Limit(authHandler.Register))
	mux.HandleFunc("POST /auth/password/forgot", authLimiter.Limit(authHandler.ForgotPassword))
	mux.HandleFunc("POST /auth/password/reset", authLimiter.Limit(authHandler.ResetPassword))
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/profiles/{slug}", profileHandler.GetProfileBySlug)
	mux.HandleFunc("POST /api/profiles/{id}/leads", leadLimiter.Limit(leadHandler.SubmitLead))
	mux.HandleFunc("GET /api/files/{id}", fileHandler.Serve)

	// Signed-in routes that still work before the email is verified, so the
	// user can see who they are and finish verification. These patterns are
	// more specific than "/api/me/" below, so they take precedence.
	account := func(h http.HandlerFunc) http.Handler { return jwtMiddleware(h) }
	mux.Handle("GET /api/me/user", account(authHandler.Me))
	mux.Handle("PUT /api/me/email", account(authLimiter.Limit(authHandler.SetEmail)))
	mux.Handle("POST /api/me/email/verify", account(authLimiter.Limit(authHandler.VerifyEmail)))
	mux.Handle("POST /api/me/email/resend", account(authLimiter.Limit(authHandler.ResendVerification)))

	// Protected Routes (Grouped under /api/me/, verified email required)
	protected := http.NewServeMux()
	protected.HandleFunc("PUT /api/me/password", authLimiter.Limit(authHandler.ChangePassword))
	protected.HandleFunc("POST /api/me/email/change", authLimiter.Limit(authHandler.RequestEmailChange))
	protected.HandleFunc("POST /api/me/email/change/confirm", authLimiter.Limit(authHandler.ConfirmEmailChange))
	protected.HandleFunc("GET /api/me/profiles", profileHandler.GetMyProfiles)
	protected.HandleFunc("POST /api/me/profiles", profileHandler.CreateProfile)
	protected.HandleFunc("GET /api/me/profiles/{id}", profileHandler.GetMyProfile)
	protected.HandleFunc("PUT /api/me/profiles/{id}", profileHandler.UpdateProfile)
	protected.HandleFunc("DELETE /api/me/profiles/{id}", profileHandler.DeleteProfile)
	protected.HandleFunc("GET /api/me/profiles/{id}/leads", leadHandler.GetLeads)
	protected.HandleFunc("GET /api/me/leads", leadHandler.ListLeads)
	protected.HandleFunc("GET /api/me/storage", storageHandler.Get)
	protected.HandleFunc("PUT /api/me/storage", storageHandler.Put)
	protected.HandleFunc("POST /api/me/storage/test", storageHandler.Test)
	protected.HandleFunc("DELETE /api/me/storage", storageHandler.Delete)
	protected.HandleFunc("GET /api/me/files", fileHandler.List)
	protected.HandleFunc("POST /api/me/files", uploadLimiter.Limit(fileHandler.Upload))
	protected.HandleFunc("PATCH /api/me/files/{id}", fileHandler.Rename)
	protected.HandleFunc("DELETE /api/me/files/{id}", fileHandler.Delete)

	// Mount protected routes with middleware
	mux.Handle("/api/me/", jwtMiddleware(middleware.RequireVerified(protected)))

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           middleware.CORS(cfg.CORSAllowedOrigins, middleware.SameOrigin(cfg.CORSAllowedOrigins, mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 5. Start HTTP Server with graceful shutdown
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Server listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	log.Println("Server exiting")
	return nil
}
