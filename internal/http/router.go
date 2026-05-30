package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"ripple-note/internal/middleware"
	"ripple-note/internal/observability"
)

type RouterOptions struct {
	Logger *slog.Logger
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

	return router
}
