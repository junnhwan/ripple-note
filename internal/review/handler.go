package review

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	httpapi "ripple-note/internal/http"
	"ripple-note/internal/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, requireAuth gin.HandlerFunc) {
	admin := router.Group("/admin")
	admin.Use(requireAuth)
	admin.Use(h.requireAdmin)
	admin.GET("/review/tasks", h.List)
	admin.GET("/review/tasks/:taskId", h.Get)
	admin.PUT("/review/tasks/:taskId/decision", h.Decide)
	admin.GET("/notes", h.SearchNotes)
}

func (h *Handler) requireAdmin(c *gin.Context) {
	claims, ok := middleware.AuthClaimsFromContext(c)
	if !ok || claims.Role != "admin" {
		httpapi.Error(c, http.StatusForbidden, "forbidden", "admin access required")
		c.Abort()
		return
	}
	c.Next()
}

func (h *Handler) List(c *gin.Context) {
	status := c.Query("status")
	limit := parseQueryInt(c, "limit", 20)
	offset := parseQueryInt(c, "offset", 0)

	result, err := h.service.List(c.Request.Context(), status, limit, offset)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	httpapi.OK(c, result)
}

func (h *Handler) SearchNotes(c *gin.Context) {
	status := c.Query("status")
	keyword := c.Query("q")
	limit := parseQueryInt(c, "limit", 20)
	offset := parseQueryInt(c, "offset", 0)

	result, err := h.service.SearchNotes(c.Request.Context(), status, keyword, limit, offset)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpapi.OK(c, result)
}

func (h *Handler) Get(c *gin.Context) {
	taskID, err := parseTaskID(c.Param("taskId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_task_id", "task id must be a positive integer")
		return
	}

	task, err := h.service.GetByID(c.Request.Context(), taskID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpapi.OK(c, task)
}

type decisionRequest struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func (h *Handler) Decide(c *gin.Context) {
	taskID, err := parseTaskID(c.Param("taskId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_task_id", "task id must be a positive integer")
		return
	}

	var req decisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	claims, ok := middleware.AuthClaimsFromContext(c)
	if !ok {
		httpapi.Error(c, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}

	task, err := h.service.Decide(c.Request.Context(), taskID, DecideInput{
		Decision: req.Decision,
		Reason:   req.Reason,
		AdminID:  claims.UserID,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpapi.OK(c, task)
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTaskNotFound):
		httpapi.Error(c, http.StatusNotFound, "task_not_found", "review task not found")
	case errors.Is(err, ErrInvalidDecision):
		httpapi.Error(c, http.StatusBadRequest, "invalid_decision", "decision must be approve or reject")
	case errors.Is(err, ErrAlreadyDecided):
		httpapi.Error(c, http.StatusConflict, "already_decided", "review task has already been decided")
	case errors.Is(err, ErrInvalidStatus):
		httpapi.Error(c, http.StatusBadRequest, "invalid_status", "status filter is invalid")
	default:
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func parseTaskID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid task id")
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
