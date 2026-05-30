package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	httpapi "ripple-note/internal/http"
	"ripple-note/internal/observability"
)

func TestHealthReturnsUnifiedResponseWithProvidedRequestID(t *testing.T) {
	t.Parallel()

	router := httpapi.NewRouter(httpapi.RouterOptions{
		Logger: observability.NewDiscardLogger(),
	})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("X-Request-ID", "req_test_123")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("X-Request-ID"); got != "req_test_123" {
		t.Fatalf("expected X-Request-ID header to be req_test_123, got %q", got)
	}

	var body struct {
		Data  map[string]string `json:"data"`
		Error any               `json:"error"`
		ID    string            `json:"request_id"`
	}
	decodeJSON(t, response.Body.Bytes(), &body)

	if body.ID != "req_test_123" {
		t.Fatalf("expected response request_id to be req_test_123, got %q", body.ID)
	}
	if body.Error != nil {
		t.Fatalf("expected error to be null, got %#v", body.Error)
	}
	if body.Data["status"] != "ok" {
		t.Fatalf("expected health status ok, got %#v", body.Data)
	}
}

func TestHealthGeneratesRequestIDWhenMissing(t *testing.T) {
	t.Parallel()

	router := httpapi.NewRouter(httpapi.RouterOptions{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	headerID := response.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Fatal("expected generated X-Request-ID header")
	}

	var body struct {
		ID string `json:"request_id"`
	}
	decodeJSON(t, response.Body.Bytes(), &body)

	if body.ID == "" {
		t.Fatal("expected generated request_id in response body")
	}
	if body.ID != headerID {
		t.Fatalf("expected response request_id %q to match header %q", body.ID, headerID)
	}
}

func TestNotFoundUsesUnifiedErrorResponse(t *testing.T) {
	t.Parallel()

	router := httpapi.NewRouter(httpapi.RouterOptions{
		Logger: observability.NewDiscardLogger(),
	})

	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	request.Header.Set("X-Request-ID", "req_missing")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d with body %s", response.Code, response.Body.String())
	}

	var body struct {
		Data  any `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		ID string `json:"request_id"`
	}
	decodeJSON(t, response.Body.Bytes(), &body)

	if body.Data != nil {
		t.Fatalf("expected data to be null, got %#v", body.Data)
	}
	if body.Error.Code != "not_found" {
		t.Fatalf("expected error code not_found, got %q", body.Error.Code)
	}
	if body.ID != "req_missing" {
		t.Fatalf("expected request_id req_missing, got %q", body.ID)
	}
}

func decodeJSON(t *testing.T, data []byte, target any) {
	t.Helper()

	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("failed to decode JSON %s: %v", string(data), err)
	}
}
