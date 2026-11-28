package main

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stoganet/stoganet/internal/auth"
	"github.com/stoganet/stoganet/internal/clients/jellyfin"
	"github.com/stoganet/stoganet/internal/clients/seerr"
	"github.com/stoganet/stoganet/internal/config"
	"github.com/stoganet/stoganet/internal/db"
	apihttp "github.com/stoganet/stoganet/internal/http"
	"github.com/stoganet/stoganet/internal/media"
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
	seerrClient := seerr.New(cfg.SeerrURL, cfg.SeerrAPIKey)
	authSvc := auth.NewService(auth.Options{
		DB:       database,
		Jellyfin: jellyfin.AsAuthAdapter(jfClient),
		SignKey:  cfg.JWTSigningKey,
	})
	libSvc := media.NewService(jfClient, seerrClient, cfg.ProxyBaseURL, logger)
	go func() {
		if err := libSvc.RefreshTmdbIndex(ctx); err != nil {
			logger.Warn("tmdb index initial build failed, will retry on timer", "err", err)
		}
		libSvc.StartTmdbIndexRefresher(ctx)
	}()

	srv := apihttp.NewServer(authSvc, libSvc, cfg.JellyfinURL, logger)
	httpSrv := &stdhttp.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("stoganet listening", "addr", cfg.ListenAddr)
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
