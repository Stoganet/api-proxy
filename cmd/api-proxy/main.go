package main

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Stoganet/api-proxy/internal/auth"
	"github.com/Stoganet/api-proxy/internal/clients/jellyfin"
	"github.com/Stoganet/api-proxy/internal/config"
	"github.com/Stoganet/api-proxy/internal/db"
	apihttp "github.com/Stoganet/api-proxy/internal/http"
	"github.com/Stoganet/api-proxy/internal/media"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	database, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		logger.Error("db open", "err", err)
		os.Exit(2)
	}
	defer database.Close()

	jfClient := jellyfin.New(cfg.JellyfinURL, cfg.JellyfinAPIKey)
	authSvc := auth.NewService(auth.Options{
		DB:       database,
		Jellyfin: jellyfin.AsAuthAdapter(jfClient),
		SignKey:  cfg.JWTSigningKey,
	})
	libSvc := media.NewService(jfClient, cfg.JellyfinURL, cfg.ProxyBaseURL, logger)

	srv := apihttp.NewServer(authSvc, libSvc, cfg.JellyfinURL, logger)
	httpSrv := &stdhttp.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("api-proxy listening", "addr", cfg.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != stdhttp.ErrServerClosed {
			logger.Error("listen", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
}
