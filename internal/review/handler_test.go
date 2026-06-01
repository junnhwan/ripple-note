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

func TestAdminNotesSearchFiltersByStatusAndKeyword(t *testing.T) {
	t.Parallel()

	router, db := newReviewTestRouter(t)
	adminToken := registerAdmin(t, router, db, "notes-admin@example.com", "secret123", "NotesAdmin")
	authorToken := registerAndLogin(t, router, "notes-author@example.com", "secret123", "NotesAuthor")

	goNoteID := publishNoteForReview(t, router, authorToken, "Go backend internship", "Feed and review workflow")
	rejectedNoteID := publishNoteForReview(t, router, authorToken, "Go rejected draft", "Should not match published")
	javaNoteID := publishNoteForReview(t, router, authorToken, "Java backend notes", "Should not match keyword")

	if err := db.Model(&note.Note{}).Where("id = ?", goNoteID).Update("status", note.StatusPublished).Error; err != nil {
		t.Fatalf("publish go note: %v", err)
	}
	if err := db.Model(&note.Note{}).Where("id = ?", rejectedNoteID).Update("status", note.StatusRejected).Error; err != nil {
		t.Fatalf("reject note: %v", err)
	}
	if err := db.Model(&note.Note{}).Where("id = ?", javaNoteID).Update("status", note.StatusPublished).Error; err != nil {
		t.Fatalf("publish java note: %v", err)
	}

	resp := getJSON(t, router, "/api/admin/notes?status=published&q=go", "Bearer "+adminToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("expected admin notes status 200, got %d: %s", resp.Status, string(resp.RawBody))
	}

	list := decodeData[review.AdminNoteListDTO](t, resp.Data)
	if list.Total != 1 {
		t.Fatalf("expected total 1, got %d", list.Total)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list.Items))
	}
	item := list.Items[0]
	if item.ID != goNoteID {
		t.Fatalf("expected only go note %d, got %d", goNoteID, item.ID)
	}
	if item.Status != note.StatusPublished {
		t.Fatalf("expected published status, got %s", item.Status)
	}
	if item.Title != "Go backend internship" {
		t.Fatalf("expected title from note, got %q", item.Title)
	}
	if item.ReviewTaskID == nil {
		t.Fatal("expected review task id in admin note item")
	}
}

func TestAdminNotesRejectsNonAdmin(t *testing.T) {
	t.Parallel()

	router, _ := newReviewTestRouter(t)
	token := registerAndLogin(t, router, "notes-user@example.com", "secret123", "NotesUser")

	resp := getJSON(t, router, "/api/admin/notes", "Bearer "+token)
	if resp.Status != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.Status)
	}
}

func TestAdminNotesRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	router, db := newReviewTestRouter(t)
	adminToken := registerAdmin(t, router, db, "notes-invalid-admin@example.com", "secret123", "NotesInvalidAdmin")

	resp := getJSON(t, router, "/api/admin/notes?status=unknown", "Bearer "+adminToken)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Status, string(resp.RawBody))
	}
	if resp.Error == nil || resp.Error.Code != "invalid_status" {
		t.Fatalf("expected invalid_status, got %#v", resp.Error)
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
	testAuthorInfo := &testAuthorInfoProvider{repo: userRepo, db: db}
	internalHandler := review.NewInternalHandler(reviewRepo, noteRepo, testAuthorInfo)

	authorProvider := &testAuthorProvider{repo: userRepo}
	noteService := note.NewService(noteRepo, authorProvider, reviewService, nil)
	optionalAuth := middleware.OptionalAuth(jwtManager)
	noteHandler := note.NewHandler(noteService, optionalAuth)

	router := httpapi.NewRouter(httpapi.RouterOptions{
		Logger:         observability.NewDiscardLogger(),
		AccountRoutes:  accountHandler,
		NoteRoutes:     noteHandler,
		ReviewRoutes:   reviewHandler,
		JWTManager:     jwtManager,
		InternalRoutes: internalHandler,
		InternalToken:  "local-dev-internal-token",
	})

	return router, db
}

type testAuthorInfoProvider struct {
	repo *account.GormUserRepository
	db   *gorm.DB
}

func (a *testAuthorInfoProvider) FindAuthorInfo(ctx context.Context, userID uint64) (review.AuthorInfo, error) {
	user, err := a.repo.FindByID(ctx, userID)
	if err != nil {
		return review.AuthorInfo{}, err
	}
	return review.AuthorInfo{
		ID:        user.ID,
		Nickname:  user.Nickname,
		AvatarURL: user.AvatarURL,
	}, nil
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

func publishNoteForReview(t *testing.T, handler http.Handler, token, title, body string) uint64 {
	t.Helper()
	resp := postJSON(t, handler, "/api/notes", map[string]any{
		"title": title,
		"body":  body,
	}, "Bearer "+token)
	if resp.Status != http.StatusCreated {
		t.Fatalf("publish note: expected 201, got %d: %s", resp.Status, string(resp.RawBody))
	}
	published := decodeData[note.NoteDTO](t, resp.Data)
	return published.ID
}

type apiTestResponse struct {
	Status int
	Data   json.RawMessage `json:"data"`
	Error  *struct {
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

func TestInternalAPIPullPendingAndGetContext(t *testing.T) {
	t.Parallel()

	router, _ := newReviewTestRouter(t)
	registerAndLogin(t, router, "intauthor@example.com", "secret123", "IntAuthor")
	authorToken := login(t, router, "intauthor@example.com", "secret123")

	postJSON(t, router, "/api/notes", map[string]any{
		"title": "Agent review me",
		"body":  "Some content",
	}, "Bearer "+authorToken)

	pendingResp := getJSONWithHeader(t, router, "/internal/review/tasks/pending", "X-Internal-Token", "local-dev-internal-token")
	if pendingResp.Status != http.StatusOK {
		t.Fatalf("pending: expected 200, got %d: %s", pendingResp.Status, string(pendingResp.RawBody))
	}

	noTokenResp := getJSON(t, router, "/internal/review/tasks/pending", "")
	if noTokenResp.Status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", noTokenResp.Status)
	}
}

func TestInternalAPIAgentResultPass(t *testing.T) {
	t.Parallel()

	router, db := newReviewTestRouter(t)
	registerAndLogin(t, router, "passauth@example.com", "secret123", "PassAuthor")
	authorToken := login(t, router, "passauth@example.com", "secret123")

	postJSON(t, router, "/api/notes", map[string]any{
		"title": "Agent pass me",
		"body":  "Good content",
	}, "Bearer "+authorToken)

	pendingResp := getJSONWithHeader(t, router, "/internal/review/tasks/pending", "X-Internal-Token", "local-dev-internal-token")
	var pendingBody struct {
		Data struct {
			Items []struct {
				ID uint64 `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(pendingResp.RawBody, &pendingBody)
	if len(pendingBody.Data.Items) == 0 {
		t.Fatal("expected at least one pending task")
	}
	taskID := pendingBody.Data.Items[0].ID

	resultResp := putJSONWithHeader(t, router, fmt.Sprintf("/internal/review/tasks/%d/agent-result", taskID), map[string]any{
		"decision":   "pass",
		"risk_level": "low",
		"reason":     "Content is safe.",
		"confidence": 0.95,
		"trace_id":   "rg_trace_test_001",
	}, "X-Internal-Token", "local-dev-internal-token")
	if resultResp.Status != http.StatusOK {
		t.Fatalf("agent result: expected 200, got %d: %s", resultResp.Status, string(resultResp.RawBody))
	}

	var resultBody struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	_ = json.Unmarshal(resultResp.RawBody, &resultBody)
	if resultBody.Data.Status != "agent_passed" {
		t.Fatalf("expected agent_passed, got %s", resultBody.Data.Status)
	}

	var n note.Note
	if err := db.Order("id DESC").First(&n).Error; err != nil {
		t.Fatalf("find note: %v", err)
	}
	if n.Status != note.StatusPublished {
		t.Fatalf("expected note published, got %s", n.Status)
	}
}

func TestInternalAPIAgentResultReject(t *testing.T) {
	t.Parallel()

	router, db := newReviewTestRouter(t)
	registerAndLogin(t, router, "rejauth@example.com", "secret123", "RejAuthor")
	authorToken := login(t, router, "rejauth@example.com", "secret123")

	postJSON(t, router, "/api/notes", map[string]any{
		"title": "Agent reject me",
		"body":  "Bad content",
	}, "Bearer "+authorToken)

	pendingResp := getJSONWithHeader(t, router, "/internal/review/tasks/pending", "X-Internal-Token", "local-dev-internal-token")
	var pendingBody struct {
		Data struct {
			Items []struct {
				ID uint64 `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(pendingResp.RawBody, &pendingBody)
	taskID := pendingBody.Data.Items[0].ID

	resultResp := putJSONWithHeader(t, router, fmt.Sprintf("/internal/review/tasks/%d/agent-result", taskID), map[string]any{
		"decision":   "reject",
		"risk_level": "high",
		"reason":     "Policy violation.",
		"confidence": 0.89,
		"trace_id":   "rg_trace_test_002",
	}, "X-Internal-Token", "local-dev-internal-token")
	if resultResp.Status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resultResp.Status, string(resultResp.RawBody))
	}

	var n note.Note
	if err := db.Order("id DESC").First(&n).Error; err != nil {
		t.Fatalf("find note: %v", err)
	}
	if n.Status != note.StatusRejected {
		t.Fatalf("expected note rejected, got %s", n.Status)
	}
}

func getJSONWithHeader(t *testing.T, handler http.Handler, path, headerKey, headerValue string) apiTestResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set(headerKey, headerValue)
	return doRequest(t, handler, req)
}

func putJSONWithHeader(t *testing.T, handler http.Handler, path string, payload any, headerKey, headerValue string) apiTestResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerKey, headerValue)
	return doRequest(t, handler, req)
}
