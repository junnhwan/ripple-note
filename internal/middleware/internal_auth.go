package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const InternalTokenHeader = "X-Internal-Token"

func InternalAuth(validToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if validToken == "" {
			abortInternalUnauthorized(c, "internal token is not configured")
			return
		}

		header := c.GetHeader(InternalTokenHeader)
		if header == "" {
			abortInternalUnauthorized(c, "X-Internal-Token header is required")
			return
		}

		if !strings.EqualFold(header, validToken) {
			abortInternalUnauthorized(c, "invalid internal token")
			return
		}

		c.Next()
	}
}

func abortInternalUnauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"data": nil,
		"error": gin.H{
			"code":    "unauthorized",
			"message": message,
		},
		"request_id": RequestIDFromContext(c),
	})
}
