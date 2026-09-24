package main

import (
	"context"
	"expert-listing/internal/config"
	"expert-listing/internal/db"
	"expert-listing/internal/handler"
	"expert-listing/internal/repository"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	godotenv.Load() //nolint:errcheck

	logger := newLogger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		logger.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	repo := repository.New(pool)
	listingHandler := handler.NewListingHandler(repo, logger)
	swaggerHandler := handler.SwaggerHandler{}

	mux := http.NewServeMux()
	listingHandler.RegisterRoutes(mux)
	swaggerHandler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Infof("server listening on %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("server shutdown error: %v", err)
	}
	logger.Info("server stopped")
}

func newLogger() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	if os.Getenv("APP_ENV") != "production" {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}
	return log
}
