// Package server is the composition root: it builds every module with its
// dependencies, mounts their routes, and runs the HTTP server and the
// background tasks until shutdown. Dependencies are wired by hand here;
// there is no DI framework.
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/faiz-gh/fronko/backend/internal/account"
	"github.com/faiz-gh/fronko/backend/internal/analytics"
	"github.com/faiz-gh/fronko/backend/internal/app"
	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/branding"
	"github.com/faiz-gh/fronko/backend/internal/cards"
	"github.com/faiz-gh/fronko/backend/internal/feedback"
	"github.com/faiz-gh/fronko/backend/internal/files"
	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platform/config"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/mail"
	"github.com/faiz-gh/fronko/backend/internal/platform/schedule"
	"github.com/faiz-gh/fronko/backend/internal/platform/secrets"
	"github.com/faiz-gh/fronko/backend/internal/platform/storage"
	"github.com/faiz-gh/fronko/backend/internal/platform/web"
	"github.com/faiz-gh/fronko/backend/internal/platformadmin"
	"github.com/faiz-gh/fronko/backend/internal/teams"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

// The per-IP budget for endpoints that check a password or send an email code.
const (
	authBurst    = 10
	authInterval = 10 * time.Second
)

// Run starts the server and blocks until ctx ends (then shuts down
// gracefully) or the server fails.
func Run(ctx context.Context, cfg *config.Config) error {
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	handler, tasks, err := Build(ctx, cfg, pool)
	if err != nil {
		return err
	}
	go schedule.Run(ctx, tasks)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

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

// Build wires every module and returns the HTTP handler and the periodic
// tasks the modules contribute.
func Build(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) (http.Handler, []schedule.Task, error) {
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
			return nil, nil, fmt.Errorf("SMTP_FROM: %w", err)
		}
		mailer = smtpSender
	} else {
		log.Println("SMTP_HOST not set: emails (verification and reset codes) are logged, not sent")
	}

	// File storage needs SECRETS_KEY to encrypt organisations' bucket keys;
	// without it the feature is off.
	var box *secrets.Box
	if cfg.SecretsKey != nil {
		var err error
		if box, err = secrets.New(cfg.SecretsKey); err != nil {
			return nil, nil, err
		}
	} else {
		log.Println("SECRETS_KEY not set: file storage is disabled")
	}
	if cfg.StorageAllowPrivate {
		log.Println("STORAGE_ALLOW_PRIVATE_ENDPOINTS is on: storage may reach private addresses (development only)")
	}

	// Stores: each module's SQL.
	var (
		userStore      = users.NewStore(pool)
		orgStore       = orgs.NewStore(pool)
		teamStore      = teams.NewStore(pool)
		brandingStore  = branding.NewStore(pool)
		fileStore      = files.NewStore(pool)
		cardStore      = cards.NewStore(pool)
		leadStore      = leads.NewStore(pool)
		analyticsStore = analytics.NewStore(pool)
		feedbackStore  = feedback.NewStore(pool)
		adminStore     = platformadmin.NewStore(pool)
	)

	sessions := auth.SessionCheckerFunc(func(ctx context.Context, userID int64) (auth.SessionState, error) {
		state, err := userStore.GetSessionState(ctx, userID)
		if errors.Is(err, database.ErrNotFound) {
			err = auth.ErrSessionUserNotFound
		}
		return state, err
	})
	codes := users.NewCodes(userStore, authService, mailer)
	events := analytics.NewEventRecorder(analyticsStore, authService, sessions, cfg.TrustProxy)
	storageSvc := files.NewStorageService(fileStore, userStore, box, storage.NewS3(cfg.StorageAllowPrivate), cfg.StorageAllowPrivate)
	admin := platformadmin.NewAdminHandler(adminStore, feedbackStore, authService, mailer, cfg.FeedbackNotifyEmail, cfg.CookieSecure)

	modules := []app.Module{
		account.NewAuthHandler(userStore, orgStore, teamStore, codes, authService, cfg.CookieSecure),
		orgs.NewOrgHandler(orgStore, userStore, codes, authService),
		teams.NewTeamHandler(teamStore, userStore),
		branding.NewHandler(brandingStore, fileStore),
		files.Module{
			Files:   files.NewFileHandler(storageSvc, fileStore, userStore, teamStore, orgStore),
			Storage: files.NewStorageHandler(storageSvc),
		},
		cards.NewProfileHandler(cardStore, fileStore, brandingStore, userStore, cfg.CORSAllowedOrigins, events),
		leads.NewLeadHandler(leadStore, cardStore, events),
		analytics.Module{
			Handler:   analytics.NewAnalyticsHandler(analyticsStore, cardStore, events),
			Store:     analyticsStore,
			Retention: cfg.AnalyticsRetention,
		},
		feedback.NewFeedbackHandler(feedbackStore, userStore, orgStore, mailer, cfg.FeedbackNotifyEmail),
		admin,
	}

	routes := app.NewRoutes(ctx, auth.JWTMiddleware(authService, sessions), cfg.TrustProxy, authInterval, authBurst)
	routes.Public("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	var tasks []schedule.Task
	for _, m := range modules {
		m.Routes(routes)
		if s, ok := m.(app.Scheduled); ok {
			tasks = append(tasks, s.Tasks()...)
		}
	}

	handler := web.CORS(cfg.CORSAllowedOrigins, web.SameOrigin(cfg.CORSAllowedOrigins, routes.Handler(admin.Guard())))
	return handler, tasks, nil
}
