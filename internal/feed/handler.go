package feed

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	httpapi "ripple-note/internal/http"
	"ripple-note/internal/middleware"
)

type Handler struct {
	service     *Service
	optionalAuth gin.HandlerFunc
}

func NewHandler(service *Service, optionalAuth gin.HandlerFunc) *Handler {
	return &Handler{service: service, optionalAuth: optionalAuth}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, requireAuth gin.HandlerFunc) {
	feed := router.Group("/feed")
	feed.GET("/latest", h.Latest)
	feed.GET("/hot", h.Hot)
	feed.GET("/following", requireAuth, h.Following)
	router.GET("/tags/:tagName/feed", h.ByTag)
}

func (h *Handler) Latest(c *gin.Context) {
	cursor := c.Query("cursor")
	limit := parseLimitQuery(c)

	result, err := h.service.Latest(c.Request.Context(), cursor, limit)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	httpapi.OK(c, result)
}

func (h *Handler) Hot(c *gin.Context) {
	cursor := c.Query("cursor")
	limit := parseLimitQuery(c)

	result, err := h.service.Hot(c.Request.Context(), cursor, limit)
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

	result, err := h.service.ByTag(c.Request.Context(), tagName, cursor, limit)
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
