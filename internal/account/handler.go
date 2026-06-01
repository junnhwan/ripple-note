package account

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

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type updateProfileRequest struct {
	Nickname  *string `json:"nickname"`
	AvatarURL *string `json:"avatar_url"`
	Bio       *string `json:"bio"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, requireAuth gin.HandlerFunc) {
	router.POST("/users", h.Register)
	router.POST("/sessions", h.Login)
	router.DELETE("/sessions/current", requireAuth, h.LogoutCurrentSession)
	router.GET("/users/me", requireAuth, h.CurrentUser)
	router.PATCH("/users/me", requireAuth, h.UpdateProfile)
	router.GET("/users/:userId", h.PublicProfile)
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

func (h *Handler) UpdateProfile(c *gin.Context) {
	claims, ok := middleware.AuthClaimsFromContext(c)
	if !ok {
		httpapi.Error(c, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}

	var request updateProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	user, err := h.service.UpdateProfile(c.Request.Context(), claims.UserID, UpdateProfileInput{
		Nickname:  request.Nickname,
		AvatarURL: request.AvatarURL,
		Bio:       request.Bio,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpapi.OK(c, user)
}

func (h *Handler) PublicProfile(c *gin.Context) {
	userID, err := parseUserID(c.Param("userId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_user_id", "user id must be a positive integer")
		return
	}

	user, err := h.service.PublicProfile(c.Request.Context(), userID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpapi.OK(c, user)
}

func (h *Handler) LogoutCurrentSession(c *gin.Context) {
	if _, ok := middleware.AuthClaimsFromContext(c); !ok {
		httpapi.Error(c, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}
	httpapi.OK(c, gin.H{"logged_out": true})
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

func parseUserID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid user id")
	}
	return id, nil
}
