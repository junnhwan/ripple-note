package review_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ripple-note/internal/account"
	"ripple-note/internal/auth"
	httpapi "ripple-note/internal/http"
	"ripple-note/internal/middleware"
	"ripple-note/internal/note"
	"ripple-note/internal/observability"
	"ripple-note/internal/review"
)

func TestReviewFlowPublishListDecideApprove(t *testing.T) {
	t.Parallel()

	router, db := newReviewTestRouter(t)

	// Register author + admin.
	registerAndLogin(t, router, "author@example.com", "secret123", "Author")
	adminToken := registerAdmin(t, router, db, "admin@example.com", "secret123", "Admin")
	authorToken := login(t, router, "author@example.com", "secret123")

	// Publish a note (creates review task automatically).
	noteResp := postJSON(t, router, "/api/notes", map[string]any{
		"title": "Review me",
		"body":  "Content body",
		"tags":  []string{"test"},
	}, "Bearer "+authorToken)
	if noteResp.Status != http.StatusCreated {
		t.Fatalf("publish: expected 201, got %d: %s", noteResp.Status, string(noteResp.RawBody))
	}
	published := decodeData[note.NoteDTO](t, noteResp.Data)

	// List review tasks as admin.
	listResp := getJSON(t, router, "/api/admin/review/tasks", "Bearer "+adminToken)
	if listResp.Status != http.StatusOK {
		t.Fatalf("list tasks: expected 200, got %d: %s", listResp.Status, string(listResp.RawBody))
	}
	taskList := decodeData[review.TaskListDTO](t, listResp.Data)
	if taskList.Total != 1 {
		t.Fatalf("expected 1 task, got %d", taskList.Total)
	}
	task := taskList.Items[0]
	if task.NoteID != published.ID {
		t.Fatalf("expected note_id %d, got %d", published.ID, task.NoteID)
	}
	if task.Status != review.TaskStatusPendingAgent {
		t.Fatalf("expected status %s, got %s", review.TaskStatusPendingAgent, task.Status)
	}

	// Get task detail.
	detailResp := getJSON(t, router, fmt.Sprintf("/api/admin/review/tasks/%d", task.ID), "Bearer "+adminToken)
	if detailResp.Status != http.StatusOK {
		t.Fatalf("task detail: expected 200, got %d", detailResp.Status)
	}

	// Approve the task.
	decideResp := putJSON(t, router, fmt.Sprintf("/api/admin/review/tasks/%d/decision", task.ID), map[string]any{
		"decision": "approve",
		"reason":   "looks good",
	}, "Bearer "+adminToken)
	if decideResp.Status != http.StatusOK {
		t.Fatalf("decide: expected 200, got %d: %s", decideResp.Status, string(decideResp.RawBody))
	}
	decided := decodeData[review.TaskDTO](t, decideResp.Data)
	if decided.Status != review.TaskStatusAdminApproved {
		t.Fatalf("expected status %s, got %s", review.TaskStatusAdminApproved, decided.Status)
	}

	// Verify note is now published.
	var updatedNote note.Note
	if err := db.First(&updatedNote, published.ID).Error; err != nil {
		t.Fatalf("find note: %v", err)
	}
	if updatedNote.Status != note.StatusPublished {
		t.Fatalf("expected note status %s, got %s", note.StatusPublished, updatedNote.Status)
	}
	if updatedNote.PublishedAt == nil {
		t.Fatal("expected published_at to be set")
	}
}

func TestReviewFlowReject(t *testing.T) {
	t.Parallel()

	router, db := newReviewTestRouter(t)
	registerAndLogin(t, router, "rauthor@example.com", "secret123", "RAuthor")
	adminToken := registerAdmin(t, router, db, "radmin@example.com", "secret123", "RAdmin")
	authorToken := login(t, router, "rauthor@example.com", "secret123")

	noteResp := postJSON(t, router, "/api/notes", map[string]any{
		"title": "Reject me",
		"body":  "Bad content",
	}, "Bearer "+authorToken)
	published := decodeData[note.NoteDTO](t, noteResp.Data)

	listResp := getJSON(t, router, "/api/admin/review/tasks", "Bearer "+adminToken)
	taskList := decodeData[review.TaskListDTO](t, listResp.Data)
	task := taskList.Items[0]

	decideResp := putJSON(t, router, fmt.Sprintf("/api/admin/review/tasks/%d/decision", task.ID), map[string]any{
		"decision": "reject",
		"reason":   "policy violation",
	}, "Bearer "+adminToken)
	if decideResp.Status != http.StatusOK {
		t.Fatalf("decide: expected 200, got %d: %s", decideResp.Status, string(decideResp.RawBody))
	}

	var updatedNote note.Note
	if err := db.First(&updatedNote, published.ID).Error; err != nil {
		t.Fatalf("find note: %v", err)
	}
	if updatedNote.Status != note.StatusRejected {
		t.Fatalf("expected note status %s, got %s", note.StatusRejected, updatedNote.Status)
	}
}

func TestReviewFlowRejectsNonAdmin(t *testing.T) {
	t.Parallel()

	router, _ := newReviewTestRouter(t)
	token := registerAndLogin(t, router, "user@example.com", "secret123", "User")

	resp := getJSON(t, router, "/api/admin/review/tasks", "Bearer "+token)
	if resp.Status != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.Status)
	}
}

func TestReviewFlowRejectsInvalidDecision(t *testing.T) {
	t.Parallel()

	router, db := newReviewTestRouter(t)
	registerAndLogin(t, router, "iauthor@example.com", "secret123", "IAuthor")
	adminToken := registerAdmin(t, router, db, "iadmin@example.com", "secret123", "IAdmin")
	authorToken := login(t, router, "iauthor@example.com", "secret123")

	postJSON(t, router, "/api/notes", map[string]any{
		"title": "Bad decision",
		"body":  "Test",
	}, "Bearer "+authorToken)

	listResp := getJSON(t, router, "/api/admin/review/tasks", "Bearer "+adminToken)
	taskList := decodeData[review.TaskListDTO](t, listResp.Data)

	decideResp := putJSON(t, router, fmt.Sprintf("/api/admin/review/tasks/%d/decision", taskList.Items[0].ID), map[string]any{
		"decision": "invalid",
		"reason":   "test",
	}, "Bearer "+adminToken)
	if decideResp.Status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", decideResp.Status, string(decideResp.RawBody))
	}
}

func newReviewTestRouter(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.New(log.New(io.Discard, "", 0), logger.Config{}),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&account.User{},
		&note.Note{},
		&note.NoteImage{},
		&note.Tag{},
		&note.NoteTag{},
		&review.ReviewTask{},
		&review.ReviewTaskEvent{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	jwtManager := auth.NewJWTManager(auth.JWTConfig{
		Secret: "review-test-secret",
		Issuer: "ripple-note-test",
		TTL:    time.Hour,
	})

	userRepo := account.NewGormUserRepository(db)
	accountService := account.NewService(userRepo, auth.NewBcryptPasswordHasher(), jwtManager)
	accountHandler := account.NewHandler(accountService)

	noteRepo := note.NewRepository(db)
	reviewRepo := review.NewRepository(db)
	reviewService := review.NewService(reviewRepo, noteRepo)
	reviewHandler := review.NewHandler(reviewService)

	authorProvider := &testAuthorProvider{repo: userRepo}
	noteService := note.NewService(noteRepo, authorProvider, reviewService)
	optionalAuth := middleware.OptionalAuth(jwtManager)
	noteHandler := note.NewHandler(noteService, optionalAuth)

	router := httpapi.NewRouter(httpapi.RouterOptions{
		Logger:        observability.NewDiscardLogger(),
		AccountRoutes: accountHandler,
		NoteRoutes:    noteHandler,
		ReviewRoutes:  reviewHandler,
		JWTManager:    jwtManager,
	})

	return router, db
}

type testAuthorProvider struct {
	repo *account.GormUserRepository
}

func (p *testAuthorProvider) FindByID(ctx context.Context, id uint64) (note.AuthorDTO, error) {
	user, err := p.repo.FindByID(ctx, id)
	if err != nil {
		return note.AuthorDTO{}, err
	}
	return note.AuthorDTO{
		ID:        user.ID,
		Nickname:  user.Nickname,
		AvatarURL: user.AvatarURL,
	}, nil
}

func registerAndLogin(t *testing.T, handler http.Handler, email, password, nickname string) string {
	t.Helper()
	resp := postJSON(t, handler, "/api/users", map[string]any{
		"email": email, "password": password, "nickname": nickname,
	}, "")
	if resp.Status != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.Status)
	}
	return login(t, handler, email, password)
}

func login(t *testing.T, handler http.Handler, email, password string) string {
	t.Helper()
	resp := postJSON(t, handler, "/api/sessions", map[string]any{
		"email": email, "password": password,
	}, "")
	if resp.Status != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", resp.Status)
	}
	session := decodeData[account.SessionDTO](t, resp.Data)
	return session.Token
}

func registerAdmin(t *testing.T, handler http.Handler, db *gorm.DB, email, password, nickname string) string {
	t.Helper()
	registerAndLogin(t, handler, email, password, nickname)
	var user account.User
	if err := db.Where("email = ?", email).First(&user).Error; err != nil {
		t.Fatalf("find admin user: %v", err)
	}
	if err := db.Model(&user).Update("role", "admin").Error; err != nil {
		t.Fatalf("update admin role: %v", err)
	}
	// Login again to get token with admin role in JWT.
	return login(t, handler, email, password)
}

type apiTestResponse struct {
	Status  int
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RawBody []byte
}

func postJSON(t *testing.T, handler http.Handler, path string, payload any, authorization string) apiTestResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	return doRequest(t, handler, req)
}

func putJSON(t *testing.T, handler http.Handler, path string, payload any, authorization string) apiTestResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	return doRequest(t, handler, req)
}

func getJSON(t *testing.T, handler http.Handler, path string, authorization string) apiTestResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	return doRequest(t, handler, req)
}

func doRequest(t *testing.T, handler http.Handler, req *http.Request) apiTestResponse {
	t.Helper()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var resp apiTestResponse
	resp.Status = w.Code
	resp.RawBody = w.Body.Bytes()
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp
}

func decodeData[T any](t *testing.T, data json.RawMessage) T {
	t.Helper()
	var value T
	if len(data) == 0 || string(data) == "null" {
		t.Fatalf("expected response data, got %s", string(data))
	}
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode %s: %v", string(data), err)
	}
	return value
}
