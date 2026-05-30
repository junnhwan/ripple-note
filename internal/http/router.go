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

type InternalRoutes interface {
	RegisterRoutes(router gin.IRouter, internalAuth gin.HandlerFunc)
}

type RouterOptions struct {
	Logger           *slog.Logger
	AccountRoutes    AccountRoutes
	NoteRoutes       AccountRoutes
	UploadRoutes     AccountRoutes
	ReviewRoutes     AccountRoutes
	InternalRoutes   InternalRoutes
	JWTManager       *auth.JWTManager
	InternalToken    string
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

	requireAuth := middleware.AuthRequired(options.JWTManager)

	if options.AccountRoutes != nil {
		options.AccountRoutes.RegisterRoutes(router.Group("/api"), requireAuth)
	}

	if options.UploadRoutes != nil {
		options.UploadRoutes.RegisterRoutes(router.Group("/api"), requireAuth)
	}

	if options.NoteRoutes != nil {
		options.NoteRoutes.RegisterRoutes(router.Group("/api"), requireAuth)
	}

	if options.ReviewRoutes != nil {
		options.ReviewRoutes.RegisterRoutes(router.Group("/api"), requireAuth)
	}

	if options.InternalRoutes != nil {
		internalAuth := middleware.InternalAuth(options.InternalToken)
		options.InternalRoutes.RegisterRoutes(router.Group(""), internalAuth)
	}

	if options.UploadStaticDir != "" {
		router.Static("/uploads/images", options.UploadStaticDir)
	}

	return router
}
