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
	"ripple-note/internal/cache"
	"ripple-note/internal/config"
	"ripple-note/internal/feed"
	"ripple-note/internal/follow"
	httpapi "ripple-note/internal/http"
	"ripple-note/internal/interaction"
	"ripple-note/internal/middleware"
	"ripple-note/internal/note"
	"ripple-note/internal/observability"
	"ripple-note/internal/outbox"
	"ripple-note/internal/review"
	"ripple-note/internal/storage"
	"ripple-note/internal/upload"
)

// --- Adapter types for cross-module interfaces ---

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

// authorInfoAdapter implements review.AuthorInfoProvider using account and note repos.
type authorInfoAdapter struct {
	userRepo *account.GormUserRepository
	noteRepo *note.Repository
}

func (a *authorInfoAdapter) FindAuthorInfo(ctx context.Context, userID uint64) (review.AuthorInfo, error) {
	user, err := a.userRepo.FindByID(ctx, userID)
	if err != nil {
		return review.AuthorInfo{}, err
	}

	var notesCount, publishedCount, rejectedCount int64
	a.noteRepo.DB().WithContext(ctx).Model(&note.Note{}).Where("author_id = ?", userID).Count(&notesCount)
	a.noteRepo.DB().WithContext(ctx).Model(&note.Note{}).Where("author_id = ? AND status = ?", userID, note.StatusPublished).Count(&publishedCount)
	a.noteRepo.DB().WithContext(ctx).Model(&note.Note{}).Where("author_id = ? AND status = ?", userID, note.StatusRejected).Count(&rejectedCount)

	registeredDays := int(time.Since(user.CreatedAt).Hours() / 24)

	return review.AuthorInfo{
		ID:             user.ID,
		Nickname:       user.Nickname,
		AvatarURL:      user.AvatarURL,
		Bio:            user.Bio,
		NotesCount:     notesCount,
		PublishedCount: publishedCount,
		RejectedCount:  rejectedCount,
		RegisteredDays: registeredDays,
	}, nil
}

// cacheInvalidatorAdapter bridges cache.FeedCache to review.CacheInvalidator and interaction.CacheInvalidator.
type cacheInvalidatorAdapter struct {
	cache *cache.FeedCache
}

func (c *cacheInvalidatorAdapter) InvalidateFeedCache(ctx context.Context) {
	c.cache.InvalidateFeedCache(ctx)
}

func (c *cacheInvalidatorAdapter) InvalidateNoteCache(ctx context.Context, noteID uint64) {
	c.cache.InvalidateNoteCache(ctx, noteID)
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
			&outbox.Event{},
		); err != nil {
			logger.Error("auto migrate mysql failed", "error", err)
			os.Exit(1)
		}

		// --- Account ---
		userRepo := account.NewGormUserRepository(db)
		accountService := account.NewService(userRepo, auth.NewBcryptPasswordHasher(), jwtManager)
		accountRoutes = account.NewHandler(accountService)

		// --- Note + Upload ---
		authorProvider := &userAuthorProvider{repo: userRepo}
		noteRepo := note.NewRepository(db)
		uploadRoutes = upload.NewHandler(cfg.Upload.ImageDir, cfg.Upload.MaxImageSize)

		// --- Review ---
		reviewRepo := review.NewRepository(db)
		reviewService := review.NewService(reviewRepo, noteRepo)
		reviewRoutes = review.NewHandler(reviewService)

		// Internal handler with real author info provider.
		authorInfo := &authorInfoAdapter{userRepo: userRepo, noteRepo: noteRepo}
		internalRoutes = review.NewInternalHandler(reviewRepo, noteRepo, authorInfo)

		// --- Outbox ---
		outboxRepo := outbox.NewRepository(db)
		outboxHelper := outbox.NewHelper(outboxRepo)
		noteService := note.NewService(noteRepo, authorProvider, reviewService, outboxHelper)
		optionalAuth := middleware.OptionalAuth(jwtManager)
		noteRoutes = note.NewHandler(noteService, optionalAuth)

		// --- Feed ---
		feedRepo := feed.NewRepository(db)
		interactionRepo := interaction.NewRepository(db)
		followProvider := follow.NewProvider(interactionRepo)
		feedService := feed.NewService(db, feedRepo, noteRepo, authorProvider, followProvider, interactionRepo)

		// --- Redis Cache ---
		var feedHandler feed.FeedService = feedService
		var cacheInvalidator *cacheInvalidatorAdapter
		if cfg.Redis.Enabled {
			redisClient, err := cache.NewClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
			if err != nil {
				logger.Error("connect redis failed", "error", err)
				os.Exit(1)
			}
			defer redisClient.Close()
			feedCache := cache.NewFeedCache(redisClient, feedService)
			feedHandler = feedCache
			cacheInvalidator = &cacheInvalidatorAdapter{cache: feedCache}
			logger.Info("redis connected")
		}

		feedRoutes = feed.NewHandler(feedHandler, optionalAuth)
		if cacheInvalidator != nil {
			interactionRoutes = interaction.NewHandler(interactionRepo, cacheInvalidator)
		} else {
			interactionRoutes = interaction.NewHandler(interactionRepo)
		}

		// --- Outbox Worker (in-process for API server; use cmd/worker for production) ---
		var publisher outbox.Publisher
		if cfg.RabbitMQ.Enabled {
			rmq, err := outbox.NewRabbitMQPublisher(cfg.RabbitMQ.DSN, cfg.RabbitMQ.Exchange, logger)
			if err != nil {
				logger.Error("connect rabbitmq failed", "error", err)
				os.Exit(1)
			}
			defer rmq.Close()
			publisher = rmq
			logger.Info("rabbitmq connected")
		} else {
			publisher = &outbox.NopPublisher{}
			logger.Warn("rabbitmq disabled; outbox using nop publisher")
		}
		outboxWorker := outbox.NewWorker(outboxRepo, publisher, logger, 5*time.Second, 50)
		outboxWorker.Start()
		defer outboxWorker.Stop()

		// Wire cache invalidation into review service.
		if cacheInvalidator != nil {
			reviewService.SetCacheInvalidator(cacheInvalidator)
		}

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
