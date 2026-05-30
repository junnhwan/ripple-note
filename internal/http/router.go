package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"ripple-note/internal/auth"
	"ripple-note/internal/middleware"
	"ripple-note/internal/observability"
)

type AccountRoutes interface {
	RegisterRoutes(router gin.IRouter, requireAuth gin.HandlerFunc)
}

type RouterOptions struct {
	Logger           *slog.Logger
	AccountRoutes    AccountRoutes
	NoteRoutes       AccountRoutes
	UploadRoutes     AccountRoutes
	JWTManager       *auth.JWTManager
	UploadStaticDir  string
}

func NewRouter(options RouterOptions) http.Handler {
	gin.SetMode(gin.ReleaseMode)

	logger := options.Logger
	if logger == nil {
		logger = observability.NewLogger("info")
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.Logger(logger))

	router.NoRoute(func(c *gin.Context) {
		Error(c, http.StatusNotFound, "not_found", "route not found")
	})

	router.GET("/health", func(c *gin.Context) {
		OK(c, gin.H{"status": "ok"})
	})

	if options.AccountRoutes != nil {
		options.AccountRoutes.RegisterRoutes(router.Group("/api"), middleware.AuthRequired(options.JWTManager))
	}

	if options.UploadRoutes != nil {
		options.UploadRoutes.RegisterRoutes(router.Group("/api"), middleware.AuthRequired(options.JWTManager))
	}

	if options.NoteRoutes != nil {
		options.NoteRoutes.RegisterRoutes(router.Group("/api"), middleware.AuthRequired(options.JWTManager))
	}

	if options.UploadStaticDir != "" {
		router.Static("/uploads/images", options.UploadStaticDir)
	}

	return router
}
