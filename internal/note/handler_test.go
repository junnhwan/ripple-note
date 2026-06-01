package note_test

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
)

func TestNoteRoutesPublishDetailAndMyNotes(t *testing.T) {
	t.Parallel()

	router, db := newNoteTestRouter(t)
	token := registerAndLogin(t, router, "author@example.com", "secret123", "Author")

	// Publish a note.
	publishResp := postJSON(t, router, "/api/notes", map[string]any{
		"title":      "My first note",
		"body":       "Hello world content",
		"image_urls": []string{"/uploads/images/test1.jpg"},
		"tags":       []string{"go", "backend"},
	}, "Bearer "+token)
	if publishResp.Status != http.StatusCreated {
		t.Fatalf("expected publish status 201, got %d: %s", publishResp.Status, string(publishResp.RawBody))
	}

	published := decodeData[note.NoteDTO](t, publishResp.Data)
	if published.ID == 0 {
		t.Fatal("expected note id")
	}
	if published.Title != "My first note" {
		t.Fatalf("expected title 'My first note', got %q", published.Title)
	}
	if published.Status != note.StatusPendingReview {
		t.Fatalf("expected status %s, got %s", note.StatusPendingReview, published.Status)
	}
	if len(published.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(published.Tags))
	}
	if len(published.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(published.Images))
	}
	if published.Author.Nickname != "Author" {
		t.Fatalf("expected author nickname 'Author', got %q", published.Author.Nickname)
	}

	// Detail as author: pending_review is visible.
	detailResp := getJSON(t, router, fmt.Sprintf("/api/notes/%d", published.ID), "Bearer "+token)
	if detailResp.Status != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d", detailResp.Status)
	}
	detail := decodeData[note.NoteDTO](t, detailResp.Data)
	if detail.ID != published.ID {
		t.Fatalf("expected note id %d, got %d", published.ID, detail.ID)
	}

	// Detail without auth: pending_review returns 404.
	anonResp := getJSON(t, router, fmt.Sprintf("/api/notes/%d", published.ID), "")
	if anonResp.Status != http.StatusNotFound {
		t.Fatalf("expected anon detail status 404, got %d", anonResp.Status)
	}

	// My notes returns the note.
	myNotesResp := getJSON(t, router, "/api/users/me/notes", "Bearer "+token)
	if myNotesResp.Status != http.StatusOK {
		t.Fatalf("expected my notes status 200, got %d", myNotesResp.Status)
	}
	list := decodeData[note.NoteListDTO](t, myNotesResp.Data)
	if list.Total != 1 {
		t.Fatalf("expected total 1, got %d", list.Total)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list.Items))
	}
	if list.Items[0].ID != published.ID {
		t.Fatalf("expected note id %d, got %d", published.ID, list.Items[0].ID)
	}

	// Simulate approval: set status to published.
	if err := db.Model(&note.Note{}).Where("id = ?", published.ID).Update("status", note.StatusPublished).Error; err != nil {
		t.Fatalf("update note status: %v", err)
	}

	// Now anonymous can see it.
	approvedResp := getJSON(t, router, fmt.Sprintf("/api/notes/%d", published.ID), "")
	if approvedResp.Status != http.StatusOK {
		t.Fatalf("expected approved detail status 200, got %d", approvedResp.Status)
	}
}

func TestNoteDetailHidesPrivatePublishedNoteFromNonAuthor(t *testing.T) {
	t.Parallel()

	router, db := newNoteTestRouter(t)
	authorToken := registerAndLogin(t, router, "private-author@example.com", "secret123", "PrivateAuthor")
	otherToken := registerAndLogin(t, router, "private-other@example.com", "secret123", "PrivateOther")
	noteID := publishNote(t, router, authorToken, "Private published note")

	if err := db.Model(&note.Note{}).Where("id = ?", noteID).Updates(map[string]any{
		"status":       note.StatusPublished,
		"visibility":   note.VisibilityPrivate,
		"published_at": time.Now(),
	}).Error; err != nil {
		t.Fatalf("make note private published: %v", err)
	}

	authorResp := getJSON(t, router, fmt.Sprintf("/api/notes/%d", noteID), "Bearer "+authorToken)
	if authorResp.Status != http.StatusOK {
		t.Fatalf("expected author detail status 200, got %d", authorResp.Status)
	}

	anonResp := getJSON(t, router, fmt.Sprintf("/api/notes/%d", noteID), "")
	if anonResp.Status != http.StatusNotFound {
		t.Fatalf("expected anonymous detail status 404, got %d", anonResp.Status)
	}

	otherResp := getJSON(t, router, fmt.Sprintf("/api/notes/%d", noteID), "Bearer "+otherToken)
	if otherResp.Status != http.StatusNotFound {
		t.Fatalf("expected other user detail status 404, got %d", otherResp.Status)
	}
}

func TestNoteRoutesRejectInvalidPublish(t *testing.T) {
	t.Parallel()

	router, _ := newNoteTestRouter(t)
	token := registerAndLogin(t, router, "val@example.com", "secret123", "Val")

	// Missing title.
	resp := postJSON(t, router, "/api/notes", map[string]any{
		"body": "some content",
	}, "Bearer "+token)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing title, got %d", resp.Status)
	}

	// Missing body.
	resp = postJSON(t, router, "/api/notes", map[string]any{
		"title": "some title",
	}, "Bearer "+token)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing body, got %d", resp.Status)
	}

	// Invalid image URL.
	resp = postJSON(t, router, "/api/notes", map[string]any{
		"title":      "Title",
		"body":       "Body",
		"image_urls": []string{"http://evil.com/img.jpg"},
	}, "Bearer "+token)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid image url, got %d", resp.Status)
	}

	// No auth.
	resp = postJSON(t, router, "/api/notes", map[string]any{
		"title": "Title",
		"body":  "Body",
	}, "")
	if resp.Status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", resp.Status)
	}
}

func TestNoteRoutesDetailNotFound(t *testing.T) {
	t.Parallel()

	router, _ := newNoteTestRouter(t)
	token := registerAndLogin(t, router, "nf@example.com", "secret123", "NF")

	resp := getJSON(t, router, "/api/notes/99999", "Bearer "+token)
	if resp.Status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Status)
	}
}

func TestPublicAuthorNotesOnlyReturnsPublishedPublicNotes(t *testing.T) {
	t.Parallel()

	router, db := newNoteTestRouter(t)
	authorToken := registerAndLogin(t, router, "public-author@example.com", "secret123", "PublicAuthor")
	otherToken := registerAndLogin(t, router, "public-other@example.com", "secret123", "OtherAuthor")

	publishedID := publishNote(t, router, authorToken, "Published public note")
	pendingID := publishNote(t, router, authorToken, "Pending note")
	privateID := publishNote(t, router, authorToken, "Private published note")
	removedID := publishNote(t, router, authorToken, "Removed note")
	otherAuthorID := publishNote(t, router, otherToken, "Other author published note")

	now := time.Now()
	if err := db.Model(&note.Note{}).Where("id IN ?", []uint64{publishedID, privateID, removedID, otherAuthorID}).Updates(map[string]any{
		"status":       note.StatusPublished,
		"published_at": now,
	}).Error; err != nil {
		t.Fatalf("publish notes: %v", err)
	}
	if err := db.Model(&note.Note{}).Where("id = ?", privateID).Update("visibility", note.VisibilityPrivate).Error; err != nil {
		t.Fatalf("make note private: %v", err)
	}
	if err := db.Model(&note.Note{}).Where("id = ?", removedID).Update("status", note.StatusRemoved).Error; err != nil {
		t.Fatalf("remove note: %v", err)
	}

	resp := getJSON(t, router, "/api/users/1/notes", "")
	if resp.Status != http.StatusOK {
		t.Fatalf("expected public author notes status 200, got %d: %s", resp.Status, string(resp.RawBody))
	}

	list := decodeData[note.NoteListDTO](t, resp.Data)
	if list.Total != 1 {
		t.Fatalf("expected total 1, got %d", list.Total)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list.Items))
	}
	if list.Items[0].ID != publishedID {
		t.Fatalf("expected only published public note %d, got %d", publishedID, list.Items[0].ID)
	}
	if list.Items[0].ID == pendingID || list.Items[0].ID == privateID || list.Items[0].ID == removedID || list.Items[0].ID == otherAuthorID {
		t.Fatalf("public author notes leaked non-public note id %d", list.Items[0].ID)
	}
}

func TestDeleteOwnNoteSoftRemovesNote(t *testing.T) {
	t.Parallel()

	router, db := newNoteTestRouter(t)
	authorToken := registerAndLogin(t, router, "delete-author@example.com", "secret123", "DeleteAuthor")
	noteID := publishNote(t, router, authorToken, "Delete me")

	if err := db.Model(&note.Note{}).Where("id = ?", noteID).Updates(map[string]any{
		"status":       note.StatusPublished,
		"published_at": time.Now(),
	}).Error; err != nil {
		t.Fatalf("publish note: %v", err)
	}

	resp := deleteJSON(t, router, fmt.Sprintf("/api/notes/%d", noteID), "Bearer "+authorToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d: %s", resp.Status, string(resp.RawBody))
	}

	var stored note.Note
	if err := db.First(&stored, noteID).Error; err != nil {
		t.Fatalf("find removed note: %v", err)
	}
	if stored.Status != note.StatusRemoved {
		t.Fatalf("expected note status %s, got %s", note.StatusRemoved, stored.Status)
	}

	anonResp := getJSON(t, router, fmt.Sprintf("/api/notes/%d", noteID), "")
	if anonResp.Status != http.StatusNotFound {
		t.Fatalf("expected anonymous detail 404 after delete, got %d", anonResp.Status)
	}

	authorResp := getJSON(t, router, fmt.Sprintf("/api/notes/%d", noteID), "Bearer "+authorToken)
	if authorResp.Status != http.StatusNotFound {
		t.Fatalf("expected author detail 404 after delete, got %d", authorResp.Status)
	}

	repeatResp := deleteJSON(t, router, fmt.Sprintf("/api/notes/%d", noteID), "Bearer "+authorToken)
	if repeatResp.Status != http.StatusOK {
		t.Fatalf("expected repeated delete status 200, got %d: %s", repeatResp.Status, string(repeatResp.RawBody))
	}
}

func TestDeleteOwnNoteRejectsNonAuthor(t *testing.T) {
	t.Parallel()

	router, db := newNoteTestRouter(t)
	authorToken := registerAndLogin(t, router, "forbid-author@example.com", "secret123", "ForbidAuthor")
	otherToken := registerAndLogin(t, router, "forbid-other@example.com", "secret123", "ForbidOther")
	noteID := publishNote(t, router, authorToken, "Not yours")

	resp := deleteJSON(t, router, fmt.Sprintf("/api/notes/%d", noteID), "Bearer "+otherToken)
	if resp.Status != http.StatusForbidden {
		t.Fatalf("expected delete status 403 for non-author, got %d: %s", resp.Status, string(resp.RawBody))
	}

	var stored note.Note
	if err := db.First(&stored, noteID).Error; err != nil {
		t.Fatalf("find note: %v", err)
	}
	if stored.Status != note.StatusPendingReview {
		t.Fatalf("expected note to keep status %s, got %s", note.StatusPendingReview, stored.Status)
	}
}

func newNoteTestRouter(t *testing.T) (http.Handler, *gorm.DB) {
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
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	jwtManager := auth.NewJWTManager(auth.JWTConfig{
		Secret: "note-test-secret",
		Issuer: "ripple-note-test",
		TTL:    time.Hour,
	})

	userRepo := account.NewGormUserRepository(db)
	accountService := account.NewService(userRepo, auth.NewBcryptPasswordHasher(), jwtManager)
	accountHandler := account.NewHandler(accountService)

	authorProvider := &testAuthorProvider{repo: userRepo}
	noteRepo := note.NewRepository(db)
	noteService := note.NewService(noteRepo, authorProvider, nil, nil)
	optionalAuth := middleware.OptionalAuth(jwtManager)
	noteHandler := note.NewHandler(noteService, optionalAuth)

	router := httpapi.NewRouter(httpapi.RouterOptions{
		Logger:        observability.NewDiscardLogger(),
		AccountRoutes: accountHandler,
		NoteRoutes:    noteHandler,
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
		"email":    email,
		"password": password,
		"nickname": nickname,
	}, "")
	if resp.Status != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", resp.Status, string(resp.RawBody))
	}

	resp = postJSON(t, handler, "/api/sessions", map[string]any{
		"email":    email,
		"password": password,
	}, "")
	if resp.Status != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", resp.Status, string(resp.RawBody))
	}

	session := decodeData[account.SessionDTO](t, resp.Data)
	return session.Token
}

func publishNote(t *testing.T, handler http.Handler, token, title string) uint64 {
	t.Helper()

	resp := postJSON(t, handler, "/api/notes", map[string]any{
		"title": title,
		"body":  "Body for " + title,
	}, "Bearer "+token)
	if resp.Status != http.StatusCreated {
		t.Fatalf("publish %q: expected 201, got %d: %s", title, resp.Status, string(resp.RawBody))
	}
	published := decodeData[note.NoteDTO](t, resp.Data)
	if published.ID == 0 {
		t.Fatalf("publish %q: expected note id", title)
	}
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

func getJSON(t *testing.T, handler http.Handler, path string, authorization string) apiTestResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	return doRequest(t, handler, req)
}

func deleteJSON(t *testing.T, handler http.Handler, path string, authorization string) apiTestResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodDelete, path, nil)
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
