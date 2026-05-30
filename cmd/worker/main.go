package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ripple-note/internal/config"
	"ripple-note/internal/observability"
	"ripple-note/internal/outbox"
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

	if !cfg.MySQL.Enabled {
		logger.Error("mysql must be enabled for the outbox worker")
		os.Exit(1)
	}

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

	outboxRepo := outbox.NewRepository(db)

	// Select publisher: RabbitMQ if enabled, otherwise NopPublisher.
	var publisher outbox.Publisher
	if cfg.RabbitMQ.Enabled {
		rmq, err := outbox.NewRabbitMQPublisher(cfg.RabbitMQ.DSN, cfg.RabbitMQ.Exchange, logger)
		if err != nil {
			logger.Error("connect rabbitmq failed", "error", err)
			os.Exit(1)
		}
		defer rmq.Close()
		publisher = rmq
		logger.Info("outbox worker using rabbitmq publisher")
	} else {
		publisher = &outbox.NopPublisher{}
		logger.Warn("outbox worker using nop publisher (rabbitmq disabled)")
	}

	worker := outbox.NewWorker(outboxRepo, publisher, logger, 5*time.Second, 50)
	worker.Start()

	logger.Info("outbox worker started", "interval", "5s", "batch_size", 50)

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdownCh
	logger.Info("shutdown signal received", "signal", sig.String())
	worker.Stop()
	logger.Info("outbox worker stopped")
}
