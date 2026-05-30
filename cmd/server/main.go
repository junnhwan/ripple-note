package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ripple-note/internal/config"
	httpapi "ripple-note/internal/http"
	"ripple-note/internal/observability"
	"ripple-note/internal/storage"
)

func main() {
	configPath := flag.String("config", "configs/config.local.yaml", "path to YAML config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	logger := observability.NewLogger(cfg.Log.Level)

	if cfg.MySQL.Enabled {
		db, err := storage.OpenMySQL(cfg.MySQL)
		if err != nil {
			logger.Error("connect mysql failed", "error", err)
			os.Exit(1)
		}
		sqlDB, err := db.DB()
		if err != nil {
			logger.Error("get mysql sql db failed", "error", err)
			os.Exit(1)
		}
		defer sqlDB.Close()
		logger.Info("mysql connected")
	} else {
		logger.Warn("mysql disabled; only infrastructure endpoints are available")
	}

	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      httpapi.NewRouter(httpapi.RouterOptions{Logger: logger}),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server starting", "addr", server.Addr, "app", cfg.App.Name, "env", cfg.App.Env)
		errCh <- server.ListenAndServe()
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case sig := <-shutdownCh:
		logger.Info("shutdown signal received", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("http server shutdown failed", "error", err)
			os.Exit(1)
		}
	}

	logger.Info("http server stopped")
}
