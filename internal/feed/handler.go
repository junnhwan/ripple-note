package feed

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	httpapi "ripple-note/internal/http"
	"ripple-note/internal/middleware"
)

type FeedService interface {
	Latest(ctx context.Context, viewerID uint64, cursor string, limit int) (FeedResult, error)
	Hot(ctx context.Context, viewerID uint64, cursor string, limit int) (FeedResult, error)
	Following(ctx context.Context, userID uint64, cursor string, limit int) (FeedResult, error)
	ByTag(ctx context.Context, viewerID uint64, tagName, cursor string, limit int) (FeedResult, error)
}

type Handler struct {
	service      FeedService
	optionalAuth gin.HandlerFunc
}

func NewHandler(service FeedService, optionalAuth gin.HandlerFunc) *Handler {
	return &Handler{service: service, optionalAuth: optionalAuth}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, requireAuth gin.HandlerFunc) {
	feed := router.Group("/feed")
	feed.GET("/latest", h.Latest)
	feed.GET("/hot", h.Hot)
	feed.GET("/following", requireAuth, h.Following)
	router.GET("/tags/:tagName/feed", h.optionalAuth, h.ByTag)
}

func (h *Handler) viewerID(c *gin.Context) uint64 {
	claims, ok := middleware.AuthClaimsFromContext(c)
	if !ok {
		return 0
	}
	return claims.UserID
}

func (h *Handler) Latest(c *gin.Context) {
	cursor := c.Query("cursor")
	limit := parseLimitQuery(c)

	result, err := h.service.Latest(c.Request.Context(), h.viewerID(c), cursor, limit)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	httpapi.OK(c, result)
}

func (h *Handler) Hot(c *gin.Context) {
	cursor := c.Query("cursor")
	limit := parseLimitQuery(c)

	result, err := h.service.Hot(c.Request.Context(), h.viewerID(c), cursor, limit)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	httpapi.OK(c, result)
}

func (h *Handler) Following(c *gin.Context) {
	claims, ok := middleware.AuthClaimsFromContext(c)
	if !ok {
		httpapi.Error(c, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}

	cursor := c.Query("cursor")
	limit := parseLimitQuery(c)

	result, err := h.service.Following(c.Request.Context(), claims.UserID, cursor, limit)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	httpapi.OK(c, result)
}

func (h *Handler) ByTag(c *gin.Context) {
	tagName := c.Param("tagName")
	cursor := c.Query("cursor")
	limit := parseLimitQuery(c)

	result, err := h.service.ByTag(c.Request.Context(), h.viewerID(c), tagName, cursor, limit)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	httpapi.OK(c, result)
}

func parseLimitQuery(c *gin.Context) int {
	raw := c.Query("limit")
	if raw == "" {
		return DefaultLimit
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return DefaultLimit
	}
	return val
}
