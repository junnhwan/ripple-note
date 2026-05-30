package account

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	httpapi "ripple-note/internal/http"
	"ripple-note/internal/middleware"
)

type Handler struct {
	service *Service
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, requireAuth gin.HandlerFunc) {
	router.POST("/users", h.Register)
	router.POST("/sessions", h.Login)
	router.GET("/users/me", requireAuth, h.CurrentUser)
}

func (h *Handler) Register(c *gin.Context) {
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	user, err := h.service.Register(c.Request.Context(), RegisterInput{
		Email:    request.Email,
		Password: request.Password,
		Nickname: request.Nickname,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusCreated, httpapi.Response{
		Data:      user,
		Error:     nil,
		RequestID: middleware.RequestIDFromContext(c),
	})
}

func (h *Handler) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	session, err := h.service.Login(c.Request.Context(), LoginInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpapi.OK(c, session)
}

func (h *Handler) CurrentUser(c *gin.Context) {
	claims, ok := middleware.AuthClaimsFromContext(c)
	if !ok {
		httpapi.Error(c, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}

	user, err := h.service.CurrentUser(c.Request.Context(), claims.UserID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpapi.OK(c, user)
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		httpapi.Error(c, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, ErrEmailAlreadyRegistered):
		httpapi.Error(c, http.StatusConflict, "email_already_registered", "email is already registered")
	case errors.Is(err, ErrInvalidCredentials):
		httpapi.Error(c, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	case errors.Is(err, ErrUserNotFound):
		httpapi.Error(c, http.StatusNotFound, "user_not_found", "user not found")
	case errors.Is(err, ErrUserDisabled):
		httpapi.Error(c, http.StatusForbidden, "user_disabled", "user is disabled")
	default:
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
