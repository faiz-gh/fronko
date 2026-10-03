package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/faiz-gh/credensync/backend/internal/auth"
	"github.com/faiz-gh/credensync/backend/internal/config"
	"github.com/faiz-gh/credensync/backend/internal/database"
	"github.com/faiz-gh/credensync/backend/internal/handlers"
	"github.com/faiz-gh/credensync/backend/internal/middleware"
	"github.com/faiz-gh/credensync/backend/internal/repository"
)

func main() {
	log.Println("Starting CredenSync backend...")

	// 1. Load config from os.Getenv
	cfg := config.Load()

	// 2. Initialize DB (pgxpool)
	// Temporarily comment out DB connection if DATABASE_URL is empty so we can build and run for now
	ctx := context.Background()
	var repo *repository.Repository
	if cfg.DatabaseURL != "" {
		pool, err := database.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		defer pool.Close()
		repo = repository.New(pool)
	} else {
		log.Println("Warning: DATABASE_URL not set, running without DB for testing")
		repo = repository.New(nil)
	}

	// 3. Initialize repositories, auth service, and handlers (Manual DI)
	authService := auth.NewService(cfg.JWTSecret)
	
	authHandler := handlers.NewAuthHandler(repo, authService)
	profileHandler := handlers.NewProfileHandler(repo)

	jwtMiddleware := middleware.JWTMiddleware(authService)

	// 4. Setup net/http ServeMux with routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.HandleFunc("POST /auth/register", authHandler.Register)

	mux.HandleFunc("GET /api/profiles/{id}", profileHandler.GetProfile)
	
	// Example of protected route using middleware
	mux.Handle("GET /api/me", jwtMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(middleware.UserIDKey).(string)
		w.Write([]byte("Hello user: " + userID))
	})))

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	// 5. Start HTTP Server with graceful shutdown
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen error: %s\n", err)
		}
	}()
	log.Println("Server started on :8080")

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
