package feed_test

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
	"ripple-note/internal/feed"
	"ripple-note/internal/middleware"
	"ripple-note/internal/note"
	httpapi "ripple-note/internal/http"
	"ripple-note/internal/observability"
	"ripple-note/internal/review"
)

func TestFeedLatestReturnsPublishedNotes(t *testing.T) {
	t.Parallel()

	router, db := newFeedTestRouter(t)
	token := registerAndLogin(t, router, "feed@example.com", "secret123", "FeedUser")

	// Publish and approve 3 notes.
	for i := 0; i < 3; i++ {
		resp := postJSON(t, router, "/api/notes", map[string]any{
			"title": fmt.Sprintf("Note %d", i+1),
			"body":  fmt.Sprintf("Body %d", i+1),
			"tags":  []string{"feed"},
		}, "Bearer "+token)
		if resp.Status != http.StatusCreated {
			t.Fatalf("publish %d: expected 201, got %d", i+1, resp.Status)
		}
	}
	// Approve all notes.
	db.Model(&note.Note{}).Where("status = ?", note.StatusPendingReview).Update("status", note.StatusPublished)
	// Set published_at for all approved notes.
	now := time.Now()
	db.Model(&note.Note{}).Where("published_at IS NULL AND status = ?", note.StatusPublished).Update("published_at", now)

	// Fetch latest feed.
	feedResp := getJSON(t, router, "/api/feed/latest?limit=10", "")
	if feedResp.Status != http.StatusOK {
		t.Fatalf("feed: expected 200, got %d: %s", feedResp.Status, string(feedResp.RawBody))
	}

	var result struct {
		Data struct {
			Items []struct {
				ID    uint64 `json:"id"`
				Title string `json:"title"`
			} `json:"items"`
			HasMore bool `json:"has_more"`
		} `json:"data"`
	}
	_ = json.Unmarshal(feedResp.RawBody, &result)
	if len(result.Data.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Data.Items))
	}
}

func TestFeedLatestExcludesPendingReview(t *testing.T) {
	t.Parallel()

	router, _ := newFeedTestRouter(t)
	token := registerAndLogin(t, router, "excl@example.com", "secret123", "ExclUser")

	// Publish but don't approve.
	postJSON(t, router, "/api/notes", map[string]any{
		"title": "Pending Note",
		"body":  "Should not appear",
	}, "Bearer "+token)

	feedResp := getJSON(t, router, "/api/feed/latest", "")
	var result struct {
		Data struct {
			Items []any `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(feedResp.RawBody, &result)
	if len(result.Data.Items) != 0 {
		t.Fatalf("expected 0 items for pending notes, got %d", len(result.Data.Items))
	}
}

func TestFeedByTag(t *testing.T) {
	t.Parallel()

	router, db := newFeedTestRouter(t)
	token := registerAndLogin(t, router, "tag@example.com", "secret123", "TagUser")

	postJSON(t, router, "/api/notes", map[string]any{
		"title": "Tagged Note",
		"body":  "Has tags",
		"tags":  []string{"go", "backend"},
	}, "Bearer "+token)

	now := time.Now()
	db.Model(&note.Note{}).Where("status = ?", note.StatusPendingReview).Updates(map[string]any{
		"status":       note.StatusPublished,
		"published_at": now,
	})

	feedResp := getJSON(t, router, "/api/tags/go/feed", "")
	if feedResp.Status != http.StatusOK {
		t.Fatalf("tag feed: expected 200, got %d", feedResp.Status)
	}
	var result struct {
		Data struct {
			Items []struct {
				ID uint64 `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(feedResp.RawBody, &result)
	if len(result.Data.Items) != 1 {
		t.Fatalf("expected 1 tagged note, got %d", len(result.Data.Items))
	}

	// Nonexistent tag returns empty.
	emptyResp := getJSON(t, router, "/api/tags/nonexistent/feed", "")
	var emptyResult struct {
		Data struct {
			Items []any `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(emptyResp.RawBody, &emptyResult)
	if len(emptyResult.Data.Items) != 0 {
		t.Fatalf("expected 0 for nonexistent tag, got %d", len(emptyResult.Data.Items))
	}
}

func TestFeedCursorPagination(t *testing.T) {
	t.Parallel()

	router, db := newFeedTestRouter(t)
	token := registerAndLogin(t, router, "page@example.com", "secret123", "Pager")

	// Publish 5 notes.
	for i := 0; i < 5; i++ {
		postJSON(t, router, "/api/notes", map[string]any{
			"title": fmt.Sprintf("Page Note %d", i+1),
			"body":  "Content",
		}, "Bearer "+token)
	}
	now := time.Now()
	db.Model(&note.Note{}).Where("status = ?", note.StatusPendingReview).Updates(map[string]any{
		"status":       note.StatusPublished,
		"published_at": now,
	})

	// Page 1: limit=2.
	page1 := getJSON(t, router, "/api/feed/latest?limit=2", "")
	var result1 struct {
		Data struct {
			Items      []struct{ ID uint64 `json:"id"` } `json:"items"`
			NextCursor string `json:"next_cursor"`
			HasMore    bool   `json:"has_more"`
		} `json:"data"`
	}
	_ = json.Unmarshal(page1.RawBody, &result1)
	if len(result1.Data.Items) != 2 {
		t.Fatalf("page 1: expected 2, got %d", len(result1.Data.Items))
	}
	if !result1.Data.HasMore {
		t.Fatal("page 1: expected has_more=true")
	}

	// Page 2: use cursor.
	page2 := getJSON(t, router, "/api/feed/latest?limit=2&cursor="+result1.Data.NextCursor, "")
	var result2 struct {
		Data struct {
			Items      []struct{ ID uint64 `json:"id"` } `json:"items"`
			NextCursor string `json:"next_cursor"`
			HasMore    bool   `json:"has_more"`
		} `json:"data"`
	}
	_ = json.Unmarshal(page2.RawBody, &result2)
	if len(result2.Data.Items) != 2 {
		t.Fatalf("page 2: expected 2, got %d", len(result2.Data.Items))
	}

	// Ensure no overlap between pages.
	if result1.Data.Items[0].ID == result2.Data.Items[0].ID {
		t.Fatal("page overlap detected")
	}
}

func newFeedTestRouter(t *testing.T) (http.Handler, *gorm.DB) {
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
		Secret: "feed-test-secret",
		Issuer: "ripple-note-test",
		TTL:    time.Hour,
	})

	userRepo := account.NewGormUserRepository(db)
	accountService := account.NewService(userRepo, auth.NewBcryptPasswordHasher(), jwtManager)
	accountHandler := account.NewHandler(accountService)

	noteRepo := note.NewRepository(db)
	reviewRepo := review.NewRepository(db)
	reviewService := review.NewService(reviewRepo, noteRepo)

	authorProvider := &testFeedAuthorProvider{repo: userRepo}
	noteService := note.NewService(noteRepo, authorProvider, reviewService, nil)
	optionalAuth := middleware.OptionalAuth(jwtManager)
	noteHandler := note.NewHandler(noteService, optionalAuth)

	feedRepo := feed.NewRepository(db)
	followProvider := &stubFollowProvider{}
	feedService := feed.NewService(db, feedRepo, noteRepo, authorProvider, followProvider, nil)
	feedHandler := feed.NewHandler(feedService, optionalAuth)

	router := httpapi.NewRouter(httpapi.RouterOptions{
		Logger:        observability.NewDiscardLogger(),
		AccountRoutes: accountHandler,
		NoteRoutes:    noteHandler,
		FeedRoutes:    feedHandler,
		JWTManager:    jwtManager,
	})

	return router, db
}

type testFeedAuthorProvider struct {
	repo *account.GormUserRepository
}

func (p *testFeedAuthorProvider) FindByID(ctx context.Context, id uint64) (note.AuthorDTO, error) {
	user, err := p.repo.FindByID(ctx, id)
	if err != nil {
		return note.AuthorDTO{}, err
	}
	return note.AuthorDTO{ID: user.ID, Nickname: user.Nickname, AvatarURL: user.AvatarURL}, nil
}

type stubFollowProvider struct{}

func (s *stubFollowProvider) FollowingIDs(_ context.Context, _ uint64) ([]uint64, error) {
	return nil, nil
}

func registerAndLogin(t *testing.T, handler http.Handler, email, password, nickname string) string {
	t.Helper()
	resp := postJSON(t, handler, "/api/users", map[string]any{
		"email": email, "password": password, "nickname": nickname,
	}, "")
	if resp.Status != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.Status)
	}
	resp = postJSON(t, handler, "/api/sessions", map[string]any{
		"email": email, "password": password,
	}, "")
	if resp.Status != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", resp.Status)
	}
	var session struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(resp.RawBody, &session)
	return session.Data.Token
}

type apiResp struct {
	Status  int
	RawBody []byte
}

func postJSON(t *testing.T, handler http.Handler, path string, payload any, auth string) apiResp {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return doReq(t, handler, req)
}

func getJSON(t *testing.T, handler http.Handler, path string, auth string) apiResp {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return doReq(t, handler, req)
}

func doReq(t *testing.T, handler http.Handler, req *http.Request) apiResp {
	t.Helper()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return apiResp{Status: w.Code, RawBody: w.Body.Bytes()}
}
