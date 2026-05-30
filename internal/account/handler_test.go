package account_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ripple-note/internal/account"
	"ripple-note/internal/auth"
	httpapi "ripple-note/internal/http"
	"ripple-note/internal/observability"
)

func TestAccountRoutesRegisterLoginAndGetCurrentUser(t *testing.T) {
	t.Parallel()

	router := newAccountTestRouter(t)

	registerBody := postJSON(t, router, "/api/users", map[string]any{
		"email":    "alice@example.com",
		"password": "secret123",
		"nickname": "Alice",
	}, "")

	if registerBody.Error != nil {
		t.Fatalf("expected register success, got error: %#v", registerBody.Error)
	}
	registered := decodeData[account.UserDTO](t, registerBody.Data)
	if registered.ID == 0 {
		t.Fatal("expected registered user id")
	}
	if registered.Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %q", registered.Email)
	}

	loginBody := postJSON(t, router, "/api/sessions", map[string]any{
		"email":    "alice@example.com",
		"password": "secret123",
	}, "")

	session := decodeData[account.SessionDTO](t, loginBody.Data)
	if session.Token == "" {
		t.Fatal("expected login token")
	}
	if session.User.ID != registered.ID {
		t.Fatalf("expected session user id %d, got %d", registered.ID, session.User.ID)
	}

	meBody := getJSON(t, router, "/api/users/me", "Bearer "+session.Token)
	current := decodeData[account.UserDTO](t, meBody.Data)
	if current.ID != registered.ID {
		t.Fatalf("expected current user id %d, got %d", registered.ID, current.ID)
	}
}

func TestAccountRoutesRejectDuplicateEmail(t *testing.T) {
	t.Parallel()

	router := newAccountTestRouter(t)
	payload := map[string]any{
		"email":    "dupe@example.com",
		"password": "secret123",
		"nickname": "Dupe",
	}

	first := postJSON(t, router, "/api/users", payload, "")
	if first.Error != nil {
		t.Fatalf("expected first register success, got error: %#v", first.Error)
	}

	second := postJSON(t, router, "/api/users", payload, "")
	if second.Status != http.StatusConflict {
		t.Fatalf("expected duplicate email status 409, got %d", second.Status)
	}
	if second.Error == nil || second.Error.Code != "email_already_registered" {
		t.Fatalf("expected email_already_registered error, got %#v", second.Error)
	}
}

func TestAccountRoutesRejectInvalidCredentials(t *testing.T) {
	t.Parallel()

	router := newAccountTestRouter(t)
	postJSON(t, router, "/api/users", map[string]any{
		"email":    "bob@example.com",
		"password": "secret123",
		"nickname": "Bob",
	}, "")

	login := postJSON(t, router, "/api/sessions", map[string]any{
		"email":    "bob@example.com",
		"password": "wrong-password",
	}, "")
	if login.Status != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", login.Status)
	}
	if login.Error == nil || login.Error.Code != "invalid_credentials" {
		t.Fatalf("expected invalid_credentials error, got %#v", login.Error)
	}
}

func TestAccountRoutesRequireBearerTokenForCurrentUser(t *testing.T) {
	t.Parallel()

	router := newAccountTestRouter(t)

	response := getJSON(t, router, "/api/users/me", "")
	if response.Status != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", response.Status)
	}
	if response.Error == nil || response.Error.Code != "unauthorized" {
		t.Fatalf("expected unauthorized error, got %#v", response.Error)
	}
}

func newAccountTestRouter(t *testing.T) http.Handler {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.New(log.New(io.Discard, "", 0), logger.Config{}),
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	if err := db.AutoMigrate(&account.User{}); err != nil {
		t.Fatalf("auto migrate users: %v", err)
	}

	repo := account.NewGormUserRepository(db)
	hasher := auth.NewBcryptPasswordHasher()
	jwtManager := auth.NewJWTManager(auth.JWTConfig{
		Secret: "handler-test-secret",
		Issuer: "ripple-note-test",
		TTL:    time.Hour,
	})
	service := account.NewService(repo, hasher, jwtManager)
	handler := account.NewHandler(service)

	return httpapi.NewRouter(httpapi.RouterOptions{
		Logger:        observability.NewDiscardLogger(),
		AccountRoutes: handler,
		JWTManager:    jwtManager,
	})
}

type apiTestResponse struct {
	Status int
	Data   json.RawMessage `json:"data"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

func postJSON(t *testing.T, handler http.Handler, path string, payload any, authorization string) apiTestResponse {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	return doJSON(t, handler, request)
}

func getJSON(t *testing.T, handler http.Handler, path string, authorization string) apiTestResponse {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	return doJSON(t, handler, request)
}

func doJSON(t *testing.T, handler http.Handler, request *http.Request) apiTestResponse {
	t.Helper()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var body apiTestResponse
	body.Status = response.Code
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body %q: %v", response.Body.String(), err)
	}
	if strings.TrimSpace(response.Body.String()) == "" {
		t.Fatal("expected response body")
	}
	return body
}

func decodeData[T any](t *testing.T, data json.RawMessage) T {
	t.Helper()

	var value T
	if len(data) == 0 || string(data) == "null" {
		t.Fatalf("expected response data, got %s", string(data))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil && err != io.EOF {
		t.Fatalf("decode response data %s: %v", string(data), err)
	}
	return value
}
