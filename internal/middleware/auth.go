package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ripple-note/internal/auth"
)

const AuthClaimsKey = "auth.claims"

func AuthRequired(tokens *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokens == nil {
			writeUnauthorized(c, "authentication is not configured")
			return
		}

		header := c.GetHeader("Authorization")
		if header == "" {
			writeUnauthorized(c, "authentication is required")
			return
		}

		scheme, token, ok := strings.Cut(header, " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			writeUnauthorized(c, "authorization header must use Bearer token")
			return
		}

		claims, err := tokens.Parse(strings.TrimSpace(token))
		if err != nil {
			writeUnauthorized(c, "token is invalid or expired")
			return
		}

		c.Set(AuthClaimsKey, claims)
		c.Next()
	}
}

func AuthClaimsFromContext(c *gin.Context) (auth.UserClaims, bool) {
	value, exists := c.Get(AuthClaimsKey)
	if !exists {
		return auth.UserClaims{}, false
	}
	claims, ok := value.(auth.UserClaims)
	return claims, ok
}

func writeUnauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"data": nil,
		"error": gin.H{
			"code":    "unauthorized",
			"message": message,
		},
		"request_id": RequestIDFromContext(c),
	})
}
