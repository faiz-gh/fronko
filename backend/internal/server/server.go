// Package server is the composition root: it builds every module with its
// dependencies, mounts their routes, connects their events and jobs, and runs
// the HTTP server, the job workers and the periodic tasks until shutdown.
// Dependencies are wired by hand here; there is no DI framework.
package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
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
	"github.com/faiz-gh/fronko/backend/internal/integrations"
	"github.com/faiz-gh/fronko/backend/internal/integrations/directory"
	"github.com/faiz-gh/fronko/backend/internal/integrations/providers"
	"github.com/faiz-gh/fronko/backend/internal/integrations/sso"
	"github.com/faiz-gh/fronko/backend/internal/leads"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platform/config"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/events"
	"github.com/faiz-gh/fronko/backend/internal/platform/jobs"
	"github.com/faiz-gh/fronko/backend/internal/platform/mail"
	"github.com/faiz-gh/fronko/backend/internal/platform/netguard"
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

// backgroundGrace is how long shutdown waits for running jobs and tasks to
// stop. Jobs cut short go back in the queue.
const backgroundGrace = 10 * time.Second

// App is everything Build wires: the HTTP handler and the background work
// the modules contribute.
type App struct {
	Handler     http.Handler
	Tasks       []jobs.Task
	JobHandlers map[string]jobs.Handler
}

// Run starts the server and blocks until ctx ends (then shuts down
// gracefully) or the server fails.
func Run(ctx context.Context, cfg *config.Config) error {
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	a, err := Build(ctx, cfg, pool)
	if err != nil {
		return err
	}

	// Background work stops before the pool closes (defers run last first).
	bgCtx, stopBackground := context.WithCancel(ctx)
	var bg sync.WaitGroup
	defer func() {
		stopBackground()
		waitFor(&bg, backgroundGrace)
	}()
	bg.Go(func() { jobs.RunTasks(bgCtx, pool, a.Tasks) })
	bg.Go(func() { jobs.NewWorker(pool, a.JobHandlers, cfg.JobWorkers).Run(bgCtx) })

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           a.Handler,
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

// waitFor waits for wg, giving up after d.
func waitFor(wg *sync.WaitGroup, d time.Duration) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
		log.Printf("background work still running after %s; exiting anyway", d)
	}
}

// Build wires every module and returns the HTTP handler and the background
// work the modules contribute.
func Build(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) (*App, error) {
	authService := auth.NewService(cfg.JWTSecret)
	bus := events.New()

	log.Printf("Environment: %s", cfg.Env)
	if cfg.PublicURL == "" {
		log.Println("PUBLIC_URL not set (nor FRONTEND_URL): integrations that need a callback URL (OAuth, SAML, SCIM) are unavailable")
	}
	if cfg.PublicAPIURL != cfg.PublicURL {
		log.Printf("API reached at %s (PUBLIC_API_URL); OAuth callbacks, SAML and SCIM use it", cfg.PublicAPIURL)
	}
	if cfg.JobWorkers == 0 {
		log.Println("JOB_WORKERS is 0: this instance queues background jobs but doesn't run them")
	}

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
			return nil, fmt.Errorf("SMTP_FROM: %w", err)
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
			return nil, err
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
		leadStore      = leads.NewStore(pool, bus)
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
	// In development, integrations may call private addresses (a webhook
	// receiver on localhost); in production they never can.
	devOutbound := cfg.Env == config.EnvDevelopment
	if devOutbound {
		log.Println("FRONKO_ENV=development: integrations may call private and local addresses")
	}
	registry := integrations.NewRegistry()
	providers.All(registry)
	integrationSvc := integrations.NewService(integrations.NewStore(pool), registry, leadStore, orgStore,
		deriveKey(cfg.JWTSecret, "integrations oauth state"),
		integrations.Options{PublicURL: cfg.PublicURL, APIURL: cfg.PublicAPIURL, Box: box, AllowPrivate: devOutbound})
	// Single sign-on and SCIM build on the integrations core.
	ssoStore := sso.NewStore(pool)
	ssoPolicy := sso.NewPolicy(integrationSvc, ssoStore, cfg.PublicURL)
	ssoHandler := sso.NewHandler(integrationSvc, ssoStore, userStore, orgStore, authService, codes, sso.Options{
		PublicURL:    cfg.PublicURL,
		APIURL:       cfg.PublicAPIURL,
		CookieSecure: cfg.CookieSecure,
		StateKey:     deriveKey(cfg.JWTSecret, "sso sign-in state"),
		HTTP:         netguard.Client(netguard.Options{AllowPrivate: devOutbound, Timeout: 20 * time.Second}),
		Resolver:     net.DefaultResolver,
	})
	scim := directory.NewHandler(integrationSvc, directory.NewStore(pool), userStore, orgStore, teamStore,
		authService, codes, ssoPolicy, cfg.PublicAPIURL)
	bookings := func(ctx context.Context, orgID int64) (func(int64) *cards.Booking, error) {
		set, err := integrationSvc.Bookings(ctx, orgID)
		if err != nil {
			return nil, err
		}
		return func(holderID int64) *cards.Booking {
			b := set.For(holderID)
			if b == nil {
				return nil
			}
			return &cards.Booking{Provider: b.Provider, Name: b.Name, URL: b.URL, Prefill: b.Prefill, Scope: string(b.Scope)}
		}, nil
	}
	admin := platformadmin.NewAdminHandler(adminStore, feedbackStore, authService, mailer, cfg.FeedbackNotifyEmail, cfg.CookieSecure)

	modules := []app.Module{
		account.NewAuthHandler(userStore, orgStore, teamStore, codes, authService, ssoPolicy, cfg.CookieSecure),
		orgs.NewOrgHandler(orgStore, userStore, codes, authService),
		teams.NewTeamHandler(teamStore, userStore),
		branding.NewHandler(brandingStore, fileStore),
		files.Module{
			Files:   files.NewFileHandler(storageSvc, fileStore, userStore, teamStore, orgStore),
			Storage: files.NewStorageHandler(storageSvc),
		},
		cards.NewProfileHandler(cardStore, fileStore, brandingStore, userStore, cfg.CORSAllowedOrigins, events, bookings),
		leads.NewLeadHandler(leadStore, cardStore, events),
		analytics.Module{
			Handler:   analytics.NewAnalyticsHandler(analyticsStore, cardStore, events),
			Store:     analyticsStore,
			Retention: cfg.AnalyticsRetention,
		},
		feedback.NewFeedbackHandler(feedbackStore, userStore, orgStore, mailer, cfg.FeedbackNotifyEmail),
		integrations.Module{Service: integrationSvc, Handler: integrations.NewHandler(integrationSvc, cfg.CookieSecure)},
		ssoHandler,
		scim,
		admin,
	}

	routes := app.NewRoutes(ctx, auth.JWTMiddleware(authService, sessions), cfg.TrustProxy, authInterval, authBurst)
	routes.Public("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	a := &App{
		Tasks:       jobs.MaintenanceTasks(pool),
		JobHandlers: map[string]jobs.Handler{},
	}
	for _, m := range modules {
		m.Routes(routes)
		if s, ok := m.(app.Subscriber); ok {
			s.Subscribe(bus)
		}
		if w, ok := m.(app.Worker); ok {
			for kind, h := range w.JobHandlers() {
				if _, dup := a.JobHandlers[kind]; dup {
					return nil, fmt.Errorf("two modules handle %q jobs", kind)
				}
				a.JobHandlers[kind] = h
			}
		}
		if s, ok := m.(app.Scheduled); ok {
			a.Tasks = append(a.Tasks, s.Tasks()...)
		}
	}

	sameOrigin := func(h http.Handler) http.Handler { return web.SameOrigin(cfg.CORSAllowedOrigins, h) }
	a.Handler = web.CORS(cfg.CORSAllowedOrigins, routes.Handler(admin.Guard(), sameOrigin))
	return a, nil
}

// deriveKey derives a purpose-specific key from a server secret, so one
// secret never signs two kinds of thing.
func deriveKey(secret, purpose string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("fronko:" + purpose))
	return mac.Sum(nil)
}
