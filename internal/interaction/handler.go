package interaction

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	httpapi "ripple-note/internal/http"
	"ripple-note/internal/middleware"
)

// NoteCacheInvalidator invalidates note-related caches after interactions.
type NoteCacheInvalidator interface {
	InvalidateNoteCache(ctx context.Context, noteID uint64)
}

type Handler struct {
	repo     *Repository
	cache    NoteCacheInvalidator
}

func NewHandler(repo *Repository, cache ...NoteCacheInvalidator) *Handler {
	h := &Handler{repo: repo}
	if len(cache) > 0 {
		h.cache = cache[0]
	}
	return h
}

func (h *Handler) RegisterRoutes(router gin.IRouter, requireAuth gin.HandlerFunc) {
	router.PUT("/notes/:noteId/like", requireAuth, h.Like)
	router.DELETE("/notes/:noteId/like", requireAuth, h.Unlike)
	router.PUT("/notes/:noteId/favorite", requireAuth, h.Favorite)
	router.DELETE("/notes/:noteId/favorite", requireAuth, h.Unfavorite)
	router.POST("/notes/:noteId/comments", requireAuth, h.CreateComment)
	router.GET("/notes/:noteId/comments", h.ListComments)
	router.PUT("/users/me/following/:targetUserId", requireAuth, h.Follow)
	router.DELETE("/users/me/following/:targetUserId", requireAuth, h.Unfollow)
}

func (h *Handler) invalidateNote(ctx context.Context, noteID uint64) {
	if h.cache != nil {
		h.cache.InvalidateNoteCache(ctx, noteID)
	}
}

func (h *Handler) Like(c *gin.Context) {
	noteID, err := parseNoteID(c.Param("noteId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_note_id", "note id must be a positive integer")
		return
	}
	if err := h.repo.NoteAvailable(c.Request.Context(), noteID); err != nil {
		if errors.Is(err, ErrNoteNotAvailable) {
			httpapi.Error(c, http.StatusNotFound, "note_not_found", "note not found or not available for interaction")
			return
		}
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	claims, _ := middleware.AuthClaimsFromContext(c)
	created, err := h.repo.UpsertLike(c.Request.Context(), claims.UserID, noteID)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	h.invalidateNote(c.Request.Context(), noteID)
	if created {
		httpapi.OK(c, gin.H{"liked": true})
	} else {
		httpapi.OK(c, gin.H{"liked": false, "message": "already liked"})
	}
}

func (h *Handler) Unlike(c *gin.Context) {
	noteID, err := parseNoteID(c.Param("noteId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_note_id", "note id must be a positive integer")
		return
	}
	claims, _ := middleware.AuthClaimsFromContext(c)
	removed, err := h.repo.DeleteLike(c.Request.Context(), claims.UserID, noteID)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	h.invalidateNote(c.Request.Context(), noteID)
	httpapi.OK(c, gin.H{"unliked": removed})
}

func (h *Handler) Favorite(c *gin.Context) {
	noteID, err := parseNoteID(c.Param("noteId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_note_id", "note id must be a positive integer")
		return
	}
	if err := h.repo.NoteAvailable(c.Request.Context(), noteID); err != nil {
		if errors.Is(err, ErrNoteNotAvailable) {
			httpapi.Error(c, http.StatusNotFound, "note_not_found", "note not found or not available for interaction")
			return
		}
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	claims, _ := middleware.AuthClaimsFromContext(c)
	created, err := h.repo.UpsertFavorite(c.Request.Context(), claims.UserID, noteID)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	h.invalidateNote(c.Request.Context(), noteID)
	if created {
		httpapi.OK(c, gin.H{"favorited": true})
	} else {
		httpapi.OK(c, gin.H{"favorited": false, "message": "already favorited"})
	}
}

func (h *Handler) Unfavorite(c *gin.Context) {
	noteID, err := parseNoteID(c.Param("noteId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_note_id", "note id must be a positive integer")
		return
	}
	claims, _ := middleware.AuthClaimsFromContext(c)
	removed, err := h.repo.DeleteFavorite(c.Request.Context(), claims.UserID, noteID)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	h.invalidateNote(c.Request.Context(), noteID)
	httpapi.OK(c, gin.H{"unfavorited": removed})
}

type createCommentRequest struct {
	Body string `json:"body"`
}

func (h *Handler) CreateComment(c *gin.Context) {
	noteID, err := parseNoteID(c.Param("noteId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_note_id", "note id must be a positive integer")
		return
	}
	if err := h.repo.NoteAvailable(c.Request.Context(), noteID); err != nil {
		if errors.Is(err, ErrNoteNotAvailable) {
			httpapi.Error(c, http.StatusNotFound, "note_not_found", "note not found or not available for interaction")
			return
		}
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	claims, _ := middleware.AuthClaimsFromContext(c)

	var req createCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		httpapi.Error(c, http.StatusBadRequest, "validation_error", "body is required")
		return
	}

	comment, err := h.repo.CreateComment(c.Request.Context(), &Comment{
		NoteID:   noteID,
		AuthorID: claims.UserID,
		Body:     body,
		Status:   CommentStatusVisible,
	})
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	h.invalidateNote(c.Request.Context(), noteID)
	c.JSON(http.StatusCreated, httpapi.Response{
		Data:      CommentDTO{ID: comment.ID, NoteID: comment.NoteID, AuthorID: comment.AuthorID, Body: comment.Body, CreatedAt: comment.CreatedAt},
		Error:     nil,
		RequestID: middleware.RequestIDFromContext(c),
	})
}

func (h *Handler) ListComments(c *gin.Context) {
	noteID, err := parseNoteID(c.Param("noteId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_note_id", "note id must be a positive integer")
		return
	}
	limit := parseQueryInt(c, "limit", 20)
	offset := parseQueryInt(c, "offset", 0)

	comments, total, err := h.repo.ListComments(c.Request.Context(), noteID, limit, offset)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	items := make([]CommentDTO, 0, len(comments))
	for _, c := range comments {
		items = append(items, CommentDTO{
			ID:        c.ID,
			NoteID:    c.NoteID,
			AuthorID:  c.AuthorID,
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
		})
	}
	httpapi.OK(c, CommentListDTO{Items: items, Total: total})
}

func (h *Handler) Follow(c *gin.Context) {
	targetID, err := parseUserID(c.Param("targetUserId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_user_id", "user id must be a positive integer")
		return
	}
	claims, _ := middleware.AuthClaimsFromContext(c)
	if claims.UserID == targetID {
		httpapi.Error(c, http.StatusBadRequest, "invalid_target", "cannot follow yourself")
		return
	}

	created, err := h.repo.UpsertFollow(c.Request.Context(), claims.UserID, targetID)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	if created {
		httpapi.OK(c, gin.H{"following": true})
	} else {
		httpapi.OK(c, gin.H{"following": false, "message": "already following"})
	}
}

func (h *Handler) Unfollow(c *gin.Context) {
	targetID, err := parseUserID(c.Param("targetUserId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_user_id", "user id must be a positive integer")
		return
	}
	claims, _ := middleware.AuthClaimsFromContext(c)
	removed, err := h.repo.DeleteFollow(c.Request.Context(), claims.UserID, targetID)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	httpapi.OK(c, gin.H{"unfollowed": removed})
}

func parseNoteID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid note id")
	}
	return id, nil
}

func parseUserID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid user id")
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
