package upload

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	httpapi "ripple-note/internal/http"
)

type Handler struct {
	imageDir     string
	maxImageSize int64
}

func NewHandler(imageDir string, maxImageSize int64) *Handler {
	return &Handler{imageDir: imageDir, maxImageSize: maxImageSize}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, requireAuth gin.HandlerFunc) {
	router.POST("/uploads/images", requireAuth, h.UploadImage)
}

func (h *Handler) UploadImage(c *gin.Context) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_file", "image file is required")
		return
	}
	defer file.Close()

	if header.Size > h.maxImageSize {
		httpapi.Error(c, http.StatusBadRequest, "file_too_large", fmt.Sprintf("image exceeds maximum size of %d bytes", h.maxImageSize))
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !isAllowedImageExt(ext) {
		httpapi.Error(c, http.StatusBadRequest, "invalid_file_type", "only jpg, jpeg, png, gif, webp images are allowed")
		return
	}

	filename := fmt.Sprintf("%d_%d%s", time.Now().UnixMilli(), time.Now().Nanosecond(), ext)
	if err := os.MkdirAll(h.imageDir, 0755); err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	fullPath := filepath.Join(h.imageDir, filename)
	dst, err := os.Create(fullPath)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	url := "/uploads/images/" + filename
	httpapi.OK(c, gin.H{"url": url})
}

func isAllowedImageExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	}
	return false
}

// Ensure the upload handler satisfies the router interface.
var _ interface {
	RegisterRoutes(gin.IRouter, gin.HandlerFunc)
} = (*Handler)(nil)
