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

	"ripple-note/internal/account"
	"ripple-note/internal/auth"
	"ripple-note/internal/config"
	"ripple-note/internal/feed"
	"ripple-note/internal/follow"
	httpapi "ripple-note/internal/http"
	"ripple-note/internal/interaction"
	"ripple-note/internal/middleware"
	"ripple-note/internal/note"
	"ripple-note/internal/observability"
	"ripple-note/internal/review"
	"ripple-note/internal/storage"
	"ripple-note/internal/upload"
)

type userAuthorProvider struct {
	repo *account.GormUserRepository
}

func (p *userAuthorProvider) FindByID(ctx context.Context, id uint64) (note.AuthorDTO, error) {
	user, err := p.repo.FindByID(ctx, id)
	if err != nil {
		return note.AuthorDTO{}, err
	}
	return note.AuthorDTO{ID: user.ID, Nickname: user.Nickname, AvatarURL: user.AvatarURL}, nil
}

func main() {
	configPath := flag.String("config", "configs/config.local.yaml", "path to YAML config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	logger := observability.NewLogger(cfg.Log.Level)
	jwtManager := auth.NewJWTManager(auth.JWTConfig{
		Secret: cfg.Auth.JWTSecret,
		Issuer: cfg.Auth.JWTIssuer,
		TTL:    cfg.Auth.JWTTTL,
	})

	var (
		accountRoutes     httpapi.AccountRoutes
		noteRoutes        httpapi.AccountRoutes
		uploadRoutes      httpapi.AccountRoutes
		reviewRoutes      httpapi.AccountRoutes
		feedRoutes        httpapi.AccountRoutes
		interactionRoutes httpapi.AccountRoutes
		internalRoutes    httpapi.InternalRoutes
	)

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

		if err := db.AutoMigrate(
			&account.User{},
			&note.Note{},
			&note.NoteImage{},
			&note.Tag{},
			&note.NoteTag{},
			&review.ReviewTask{},
			&review.ReviewTaskEvent{},
			&interaction.NoteLike{},
			&interaction.NoteFavorite{},
			&interaction.Comment{},
			&interaction.Follow{},
		); err != nil {
			logger.Error("auto migrate mysql failed", "error", err)
			os.Exit(1)
		}

		userRepo := account.NewGormUserRepository(db)
		accountService := account.NewService(userRepo, auth.NewBcryptPasswordHasher(), jwtManager)
		accountRoutes = account.NewHandler(accountService)

		authorProvider := &userAuthorProvider{repo: userRepo}
		noteRepo := note.NewRepository(db)

		reviewRepo := review.NewRepository(db)
		reviewService := review.NewService(reviewRepo, noteRepo)
		reviewRoutes = review.NewHandler(reviewService)
		internalRoutes = review.NewInternalHandler(reviewRepo, noteRepo)

		noteService := note.NewService(noteRepo, authorProvider, reviewService)
		optionalAuth := middleware.OptionalAuth(jwtManager)
		noteRoutes = note.NewHandler(noteService, optionalAuth)

		uploadRoutes = upload.NewHandler(cfg.Upload.ImageDir, cfg.Upload.MaxImageSize)

		feedRepo := feed.NewRepository(db)
		interactionRepo := interaction.NewRepository(db)
		followProvider := follow.NewProvider(interactionRepo)
		feedService := feed.NewService(db, feedRepo, noteRepo, authorProvider, followProvider)
		feedRoutes = feed.NewHandler(feedService, optionalAuth)

		interactionRoutes = interaction.NewHandler(interactionRepo)

		logger.Info("mysql connected")
	} else {
		logger.Warn("mysql disabled; account and note endpoints are unavailable")
	}

	server := &http.Server{
		Addr: cfg.Addr(),
		Handler: httpapi.NewRouter(httpapi.RouterOptions{
			Logger:            logger,
			AccountRoutes:     accountRoutes,
			NoteRoutes:        noteRoutes,
			UploadRoutes:      uploadRoutes,
			ReviewRoutes:      reviewRoutes,
			FeedRoutes:        feedRoutes,
			InteractionRoutes: interactionRoutes,
			JWTManager:        jwtManager,
			UploadStaticDir:   cfg.Upload.ImageDir,
			InternalRoutes:    internalRoutes,
			InternalToken:     cfg.Review.InternalToken,
		}),
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
