package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ripple-note/internal/middleware"
)

type Response struct {
	Data      any       `json:"data"`
	Error     *APIError `json:"error"`
	RequestID string    `json:"request_id"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Data:      data,
		Error:     nil,
		RequestID: middleware.RequestIDFromContext(c),
	})
}

func Error(c *gin.Context, status int, code string, message string) {
	c.JSON(status, Response{
		Data: nil,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
		RequestID: middleware.RequestIDFromContext(c),
	})
}
