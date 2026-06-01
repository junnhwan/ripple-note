package ratelimit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMiddlewareRejectsRequestsAfterLimit(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	limiter := NewLimiter(store, []Rule{
		{
			Name:   "login",
			Method: http.MethodPost,
			Path:   "/api/sessions",
			Limit:  2,
			Window: time.Minute,
			KeyFunc: func(c *gin.Context) string {
				return "rate:auth:ip:" + c.ClientIP()
			},
		},
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "req_rate_limit")
	})
	router.Use(limiter.Middleware())
	router.POST("/api/sessions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 2; i++ {
		response := performRequest(router, http.MethodPost, "/api/sessions")
		if response.Code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d body=%s", i+1, response.Code, response.Body.String())
		}
	}

	response := performRequest(router, http.MethodPost, "/api/sessions")
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d body=%s", response.Code, response.Body.String())
	}

	var body struct {
		Data  any `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "rate_limited" {
		t.Fatalf("expected rate_limited error code, got %q", body.Error.Code)
	}
	if body.RequestID != "req_rate_limit" {
		t.Fatalf("expected request id req_rate_limit, got %q", body.RequestID)
	}
}

func TestMiddlewareUsesIndependentRules(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	limiter := NewLimiter(store, []Rule{
		{
			Name:    "login",
			Method:  http.MethodPost,
			Path:    "/api/sessions",
			Limit:   1,
			Window:  time.Minute,
			KeyFunc: IPKey("rate:auth:login:ip"),
		},
		{
			Name:    "publish",
			Method:  http.MethodPost,
			Path:    "/api/notes",
			Limit:   1,
			Window:  time.Minute,
			KeyFunc: IPKey("rate:publish:ip"),
		},
	})

	router := gin.New()
	router.Use(limiter.Middleware())
	router.POST("/api/sessions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.POST("/api/notes", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	firstLogin := performRequest(router, http.MethodPost, "/api/sessions")
	if firstLogin.Code != http.StatusOK {
		t.Fatalf("expected first login status 200, got %d", firstLogin.Code)
	}
	secondLogin := performRequest(router, http.MethodPost, "/api/sessions")
	if secondLogin.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second login status 429, got %d", secondLogin.Code)
	}

	publish := performRequest(router, http.MethodPost, "/api/notes")
	if publish.Code != http.StatusCreated {
		t.Fatalf("expected publish to use independent rule and return 201, got %d", publish.Code)
	}
}

func TestMemoryStoreResetsAfterWindow(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	ctx := context.Background()

	count, err := store.Increment(ctx, "rate:test", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("first increment: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	time.Sleep(20 * time.Millisecond)

	count, err = store.Increment(ctx, "rate:test", time.Minute)
	if err != nil {
		t.Fatalf("second increment: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count reset to 1, got %d", count)
	}
}

func performRequest(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.RemoteAddr = "192.0.2.10:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
