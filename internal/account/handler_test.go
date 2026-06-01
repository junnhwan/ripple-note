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

func TestAccountRoutesUpdateCurrentUserProfile(t *testing.T) {
	t.Parallel()

	router := newAccountTestRouter(t)
	token := registerAndLogin(t, router, "profile@example.com", "secret123", "Profile")

	update := patchJSON(t, router, "/api/users/me", map[string]any{
		"nickname":   "Updated Profile",
		"avatar_url": "/uploads/images/avatar.jpg",
		"bio":        "Go backend learner",
	}, "Bearer "+token)
	if update.Status != http.StatusOK {
		t.Fatalf("expected update status 200, got %d: %#v", update.Status, update.Error)
	}

	updated := decodeData[account.UserDTO](t, update.Data)
	if updated.Nickname != "Updated Profile" {
		t.Fatalf("expected updated nickname, got %q", updated.Nickname)
	}
	if updated.AvatarURL != "/uploads/images/avatar.jpg" {
		t.Fatalf("expected updated avatar url, got %q", updated.AvatarURL)
	}
	if updated.Bio != "Go backend learner" {
		t.Fatalf("expected updated bio, got %q", updated.Bio)
	}

	current := decodeData[account.UserDTO](t, getJSON(t, router, "/api/users/me", "Bearer "+token).Data)
	if current.Nickname != updated.Nickname || current.AvatarURL != updated.AvatarURL || current.Bio != updated.Bio {
		t.Fatalf("expected current user to reflect profile update, got %#v", current)
	}
}

func TestAccountRoutesPatchProfileKeepsOmittedFields(t *testing.T) {
	t.Parallel()

	router := newAccountTestRouter(t)
	token := registerAndLogin(t, router, "partial-profile@example.com", "secret123", "Partial")

	fullUpdate := patchJSON(t, router, "/api/users/me", map[string]any{
		"nickname":   "Partial One",
		"avatar_url": "/uploads/images/partial-one.jpg",
		"bio":        "first bio",
	}, "Bearer "+token)
	if fullUpdate.Status != http.StatusOK {
		t.Fatalf("expected initial update status 200, got %d: %#v", fullUpdate.Status, fullUpdate.Error)
	}

	partialUpdate := patchJSON(t, router, "/api/users/me", map[string]any{
		"bio": "second bio",
	}, "Bearer "+token)
	if partialUpdate.Status != http.StatusOK {
		t.Fatalf("expected partial update status 200, got %d: %#v", partialUpdate.Status, partialUpdate.Error)
	}

	updated := decodeData[account.UserDTO](t, partialUpdate.Data)
	if updated.Nickname != "Partial One" {
		t.Fatalf("expected omitted nickname to stay unchanged, got %q", updated.Nickname)
	}
	if updated.AvatarURL != "/uploads/images/partial-one.jpg" {
		t.Fatalf("expected omitted avatar to stay unchanged, got %q", updated.AvatarURL)
	}
	if updated.Bio != "second bio" {
		t.Fatalf("expected bio to change, got %q", updated.Bio)
	}
}

func TestAccountRoutesRejectInvalidProfileUpdate(t *testing.T) {
	t.Parallel()

	router := newAccountTestRouter(t)
	token := registerAndLogin(t, router, "invalid-profile@example.com", "secret123", "InvalidProfile")

	emptyNickname := patchJSON(t, router, "/api/users/me", map[string]any{
		"nickname": " ",
	}, "Bearer "+token)
	if emptyNickname.Status != http.StatusBadRequest {
		t.Fatalf("expected empty nickname status 400, got %d", emptyNickname.Status)
	}

	longBio := strings.Repeat("a", 513)
	tooLongBio := patchJSON(t, router, "/api/users/me", map[string]any{
		"bio": longBio,
	}, "Bearer "+token)
	if tooLongBio.Status != http.StatusBadRequest {
		t.Fatalf("expected too long bio status 400, got %d", tooLongBio.Status)
	}

	noAuth := patchJSON(t, router, "/api/users/me", map[string]any{
		"nickname": "NoAuth",
	}, "")
	if noAuth.Status != http.StatusUnauthorized {
		t.Fatalf("expected no auth status 401, got %d", noAuth.Status)
	}
}

func TestAccountRoutesGetPublicProfile(t *testing.T) {
	t.Parallel()

	router := newAccountTestRouter(t)
	token := registerAndLogin(t, router, "public-profile@example.com", "secret123", "PublicProfile")

	patchJSON(t, router, "/api/users/me", map[string]any{
		"nickname":   "Public Name",
		"avatar_url": "/uploads/images/public-avatar.jpg",
		"bio":        "Public bio",
	}, "Bearer "+token)

	resp := getJSON(t, router, "/api/users/1", "")
	if resp.Status != http.StatusOK {
		t.Fatalf("expected public profile status 200, got %d: %#v", resp.Status, resp.Error)
	}

	profile := decodeData[account.PublicUserDTO](t, resp.Data)
	if profile.ID != 1 {
		t.Fatalf("expected public profile id 1, got %d", profile.ID)
	}
	if profile.Nickname != "Public Name" {
		t.Fatalf("expected public nickname, got %q", profile.Nickname)
	}
	if profile.AvatarURL != "/uploads/images/public-avatar.jpg" {
		t.Fatalf("expected public avatar url, got %q", profile.AvatarURL)
	}
	if profile.Bio != "Public bio" {
		t.Fatalf("expected public bio, got %q", profile.Bio)
	}

	raw := string(resp.Data)
	for _, forbiddenField := range []string{"email", "role", "status"} {
		if strings.Contains(raw, forbiddenField) {
			t.Fatalf("public profile leaked %s in %s", forbiddenField, raw)
		}
	}
}

func TestAccountRoutesPublicProfileNotFound(t *testing.T) {
	t.Parallel()

	router := newAccountTestRouter(t)

	resp := getJSON(t, router, "/api/users/404", "")
	if resp.Status != http.StatusNotFound {
		t.Fatalf("expected public profile status 404, got %d", resp.Status)
	}
}

func TestAccountRoutesLogoutCurrentSession(t *testing.T) {
	t.Parallel()

	router := newAccountTestRouter(t)
	token := registerAndLogin(t, router, "logout@example.com", "secret123", "Logout")

	resp := deleteJSON(t, router, "/api/sessions/current", "Bearer "+token)
	if resp.Status != http.StatusOK {
		t.Fatalf("expected logout status 200, got %d: %#v", resp.Status, resp.Error)
	}

	data := decodeData[map[string]bool](t, resp.Data)
	if !data["logged_out"] {
		t.Fatalf("expected logged_out=true, got %#v", data)
	}

	noAuth := deleteJSON(t, router, "/api/sessions/current", "")
	if noAuth.Status != http.StatusUnauthorized {
		t.Fatalf("expected no auth logout status 401, got %d", noAuth.Status)
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

func registerAndLogin(t *testing.T, handler http.Handler, email, password, nickname string) string {
	t.Helper()

	resp := postJSON(t, handler, "/api/users", map[string]any{
		"email":    email,
		"password": password,
		"nickname": nickname,
	}, "")
	if resp.Status != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %#v", resp.Status, resp.Error)
	}

	resp = postJSON(t, handler, "/api/sessions", map[string]any{
		"email":    email,
		"password": password,
	}, "")
	if resp.Status != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %#v", resp.Status, resp.Error)
	}
	session := decodeData[account.SessionDTO](t, resp.Data)
	if session.Token == "" {
		t.Fatal("expected login token")
	}
	return session.Token
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

func patchJSON(t *testing.T, handler http.Handler, path string, payload any, authorization string) apiTestResponse {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request payload: %v", err)
	}

	request := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(body))
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

func deleteJSON(t *testing.T, handler http.Handler, path string, authorization string) apiTestResponse {
	t.Helper()

	request := httptest.NewRequest(http.MethodDelete, path, nil)
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
