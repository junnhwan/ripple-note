package note

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	httpapi "ripple-note/internal/http"
	"ripple-note/internal/middleware"
)

type Handler struct {
	service      *Service
	optionalAuth gin.HandlerFunc
}

func NewHandler(service *Service, optionalAuth gin.HandlerFunc) *Handler {
	return &Handler{service: service, optionalAuth: optionalAuth}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, requireAuth gin.HandlerFunc) {
	router.POST("/notes", requireAuth, h.Publish)
	router.GET("/notes/:noteId", h.optionalAuth, h.Detail)
	router.GET("/users/me/notes", requireAuth, h.MyNotes)
}

type publishRequest struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	ImageURLs []string `json:"image_urls"`
	Tags      []string `json:"tags"`
}

func (h *Handler) Publish(c *gin.Context) {
	var req publishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	claims, ok := middleware.AuthClaimsFromContext(c)
	if !ok {
		httpapi.Error(c, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}

	note, err := h.service.Publish(c.Request.Context(), PublishInput{
		Title:     req.Title,
		Body:      req.Body,
		ImageURLs: req.ImageURLs,
		Tags:      req.Tags,
	}, claims.UserID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusCreated, httpapi.Response{
		Data:      note,
		Error:     nil,
		RequestID: middleware.RequestIDFromContext(c),
	})
}

func (h *Handler) Detail(c *gin.Context) {
	noteID, err := parseNoteID(c.Param("noteId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_note_id", "note id must be a positive integer")
		return
	}

	var viewerID uint64
	if claims, ok := middleware.AuthClaimsFromContext(c); ok {
		viewerID = claims.UserID
	}

	note, err := h.service.Detail(c.Request.Context(), noteID, viewerID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpapi.OK(c, note)
}

func (h *Handler) MyNotes(c *gin.Context) {
	claims, ok := middleware.AuthClaimsFromContext(c)
	if !ok {
		httpapi.Error(c, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}

	limit := parseQueryInt(c, "limit", 20)
	offset := parseQueryInt(c, "offset", 0)

	result, err := h.service.MyNotes(c.Request.Context(), claims.UserID, limit, offset)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpapi.OK(c, result)
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNoteNotFound):
		httpapi.Error(c, http.StatusNotFound, "note_not_found", "note not found")
	case errors.Is(err, ErrInvalidInput):
		httpapi.Error(c, http.StatusBadRequest, "validation_error", err.Error())
	default:
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func parseNoteID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid note id")
	}
	return id, nil
}

func parseQueryInt(c *gin.Context, key string, defaultValue int) int {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return val
}
