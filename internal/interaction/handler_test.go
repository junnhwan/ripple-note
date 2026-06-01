package interaction_test

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
	"ripple-note/internal/interaction"
	"ripple-note/internal/middleware"
	"ripple-note/internal/note"
	"ripple-note/internal/observability"
	"ripple-note/internal/review"
)

func TestLikeUnlikeIdempotent(t *testing.T) {
	t.Parallel()

	router, db := newInteractionTestRouter(t)
	token := publishApprovedNote(t, router, db, "like@example.com", "Like Note")

	// Like.
	resp := putJSON(t, router, "/api/notes/1/like", nil, "Bearer "+token)
	if resp.Status != http.StatusOK {
		t.Fatalf("like: expected 200, got %d: %s", resp.Status, string(resp.RawBody))
	}

	// Like again (idempotent).
	resp = putJSON(t, router, "/api/notes/1/like", nil, "Bearer "+token)
	var body struct {
		Data struct {
			Liked   bool   `json:"liked"`
			Message string `json:"message"`
		} `json:"data"`
	}
	_ = json.Unmarshal(resp.RawBody, &body)
	if body.Data.Liked {
		t.Fatal("expected liked=false on duplicate like")
	}

	// Verify count = 1.
	var n note.Note
	db.First(&n, 1)
	if n.LikesCount != 1 {
		t.Fatalf("expected likes_count=1, got %d", n.LikesCount)
	}

	// Unlike.
	resp = reqMethod(t, router, http.MethodDelete, "/api/notes/1/like", nil, "Bearer "+token)
	if resp.Status != http.StatusOK {
		t.Fatalf("unlike: expected 200, got %d", resp.Status)
	}

	db.First(&n, 1)
	if n.LikesCount != 0 {
		t.Fatalf("expected likes_count=0, got %d", n.LikesCount)
	}

	// Unlike again (idempotent — no negative count).
	resp = reqMethod(t, router, http.MethodDelete, "/api/notes/1/like", nil, "Bearer "+token)
	db.First(&n, 1)
	if n.LikesCount != 0 {
		t.Fatalf("expected likes_count still 0, got %d", n.LikesCount)
	}
}

func TestFavoriteIdempotent(t *testing.T) {
	t.Parallel()

	router, db := newInteractionTestRouter(t)
	token := publishApprovedNote(t, router, db, "fav@example.com", "Fav Note")

	putJSON(t, router, "/api/notes/1/favorite", nil, "Bearer "+token)
	putJSON(t, router, "/api/notes/1/favorite", nil, "Bearer "+token) // idempotent

	var n note.Note
	db.First(&n, 1)
	if n.FavoritesCount != 1 {
		t.Fatalf("expected favorites_count=1, got %d", n.FavoritesCount)
	}
}

func TestCommentAndList(t *testing.T) {
	t.Parallel()

	router, db := newInteractionTestRouter(t)
	token := publishApprovedNote(t, router, db, "comment@example.com", "Comment Note")

	// Create comment.
	resp := postJSON(t, router, "/api/notes/1/comments", map[string]any{
		"body": "Great post!",
	}, "Bearer "+token)
	if resp.Status != http.StatusCreated {
		t.Fatalf("comment: expected 201, got %d: %s", resp.Status, string(resp.RawBody))
	}

	// List comments.
	resp = getJSON(t, router, "/api/notes/1/comments", "")
	if resp.Status != http.StatusOK {
		t.Fatalf("list comments: expected 200, got %d", resp.Status)
	}
	var list struct {
		Data struct {
			Items []struct {
				Body string `json:"body"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(resp.RawBody, &list)
	if list.Data.Total != 1 {
		t.Fatalf("expected total=1, got %d", list.Data.Total)
	}
	if len(list.Data.Items) != 1 || list.Data.Items[0].Body != "Great post!" {
		t.Fatal("comment body mismatch")
	}

	var n note.Note
	db.First(&n, 1)
	if n.CommentsCount != 1 {
		t.Fatalf("expected comments_count=1, got %d", n.CommentsCount)
	}
}

func TestDeleteCommentIdempotentForAuthor(t *testing.T) {
	t.Parallel()

	router, db := newInteractionTestRouter(t)
	token := publishApprovedNote(t, router, db, "delete-comment@example.com", "Delete Comment Note")

	resp := postJSON(t, router, "/api/notes/1/comments", map[string]any{
		"body": "Remove this comment",
	}, "Bearer "+token)
	if resp.Status != http.StatusCreated {
		t.Fatalf("comment: expected 201, got %d: %s", resp.Status, string(resp.RawBody))
	}
	var created struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(resp.RawBody, &created)
	if created.Data.ID == 0 {
		t.Fatal("expected created comment id")
	}

	resp = reqMethod(t, router, http.MethodDelete, fmt.Sprintf("/api/comments/%d", created.Data.ID), nil, "Bearer "+token)
	if resp.Status != http.StatusOK {
		t.Fatalf("delete comment: expected 200, got %d: %s", resp.Status, string(resp.RawBody))
	}

	var body struct {
		Data struct {
			Deleted bool `json:"deleted"`
		} `json:"data"`
	}
	_ = json.Unmarshal(resp.RawBody, &body)
	if !body.Data.Deleted {
		t.Fatal("expected deleted=true on first delete")
	}

	var n note.Note
	db.First(&n, 1)
	if n.CommentsCount != 0 {
		t.Fatalf("expected comments_count=0, got %d", n.CommentsCount)
	}

	resp = reqMethod(t, router, http.MethodDelete, fmt.Sprintf("/api/comments/%d", created.Data.ID), nil, "Bearer "+token)
	if resp.Status != http.StatusOK {
		t.Fatalf("duplicate delete: expected 200, got %d: %s", resp.Status, string(resp.RawBody))
	}
	_ = json.Unmarshal(resp.RawBody, &body)
	if body.Data.Deleted {
		t.Fatal("expected deleted=false on duplicate delete")
	}
	db.First(&n, 1)
	if n.CommentsCount != 0 {
		t.Fatalf("expected comments_count to stay 0, got %d", n.CommentsCount)
	}
}

func TestDeleteCommentRejectsNonAuthor(t *testing.T) {
	t.Parallel()

	router, db := newInteractionTestRouter(t)
	authorToken := publishApprovedNote(t, router, db, "comment-owner@example.com", "Owner Comment Note")
	otherToken := registerAndLogin(t, router, "comment-other@example.com", "secret123", "Other")

	resp := postJSON(t, router, "/api/notes/1/comments", map[string]any{
		"body": "Owned comment",
	}, "Bearer "+authorToken)
	if resp.Status != http.StatusCreated {
		t.Fatalf("comment: expected 201, got %d: %s", resp.Status, string(resp.RawBody))
	}
	var created struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(resp.RawBody, &created)

	resp = reqMethod(t, router, http.MethodDelete, fmt.Sprintf("/api/comments/%d", created.Data.ID), nil, "Bearer "+otherToken)
	if resp.Status != http.StatusForbidden {
		t.Fatalf("delete other user's comment: expected 403, got %d: %s", resp.Status, string(resp.RawBody))
	}

	var n note.Note
	db.First(&n, 1)
	if n.CommentsCount != 1 {
		t.Fatalf("expected comments_count=1 after forbidden delete, got %d", n.CommentsCount)
	}
}

func TestFollowUnfollowIdempotent(t *testing.T) {
	t.Parallel()

	router, _ := newInteractionTestRouter(t)
	token1 := registerAndLogin(t, router, "follower@example.com", "secret123", "Follower")
	registerAndLogin(t, router, "followee@example.com", "secret123", "Followee")

	// Follow user 2.
	resp := putJSON(t, router, "/api/users/me/following/2", nil, "Bearer "+token1)
	if resp.Status != http.StatusOK {
		t.Fatalf("follow: expected 200, got %d: %s", resp.Status, string(resp.RawBody))
	}

	// Follow again (idempotent).
	resp = putJSON(t, router, "/api/users/me/following/2", nil, "Bearer "+token1)
	var body struct {
		Data struct {
			Following bool `json:"following"`
		} `json:"data"`
	}
	_ = json.Unmarshal(resp.RawBody, &body)
	if body.Data.Following {
		t.Fatal("expected following=false on duplicate follow")
	}

	// Cannot follow self.
	resp = putJSON(t, router, "/api/users/me/following/1", nil, "Bearer "+token1)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("self-follow: expected 400, got %d", resp.Status)
	}
}

func publishApprovedNote(t *testing.T, router http.Handler, db *gorm.DB, email, title string) string {
	t.Helper()
	token := registerAndLogin(t, router, email, "secret123", "Author")
	resp := postJSON(t, router, "/api/notes", map[string]any{
		"title": title,
		"body":  "Content for testing",
	}, "Bearer "+token)
	if resp.Status != http.StatusCreated {
		t.Fatalf("publish: expected 201, got %d", resp.Status)
	}
	now := time.Now()
	db.Model(&note.Note{}).Where("id = 1").Updates(map[string]any{
		"status":       note.StatusPublished,
		"published_at": now,
	})
	return token
}

func newInteractionTestRouter(t *testing.T) (http.Handler, *gorm.DB) {
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
		&interaction.NoteLike{},
		&interaction.NoteFavorite{},
		&interaction.Comment{},
		&interaction.Follow{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	jwtManager := auth.NewJWTManager(auth.JWTConfig{
		Secret: "interaction-test-secret",
		Issuer: "ripple-note-test",
		TTL:    time.Hour,
	})

	userRepo := account.NewGormUserRepository(db)
	accountService := account.NewService(userRepo, auth.NewBcryptPasswordHasher(), jwtManager)
	accountHandler := account.NewHandler(accountService)

	noteRepo := note.NewRepository(db)
	reviewRepo := review.NewRepository(db)
	reviewService := review.NewService(reviewRepo, noteRepo)

	authorProvider := &testAuthorProvider{repo: userRepo}
	noteService := note.NewService(noteRepo, authorProvider, reviewService, nil)
	optionalAuth := middleware.OptionalAuth(jwtManager)
	noteHandler := note.NewHandler(noteService, optionalAuth)

	interactionRepo := interaction.NewRepository(db)
	interactionHandler := interaction.NewHandler(interactionRepo)

	router := httpapi.NewRouter(httpapi.RouterOptions{
		Logger:            observability.NewDiscardLogger(),
		AccountRoutes:     accountHandler,
		NoteRoutes:        noteHandler,
		InteractionRoutes: interactionHandler,
		JWTManager:        jwtManager,
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
	return note.AuthorDTO{ID: user.ID, Nickname: user.Nickname, AvatarURL: user.AvatarURL}, nil
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
	return reqBody(t, handler, http.MethodPost, path, payload, auth)
}

func putJSON(t *testing.T, handler http.Handler, path string, payload any, auth string) apiResp {
	t.Helper()
	return reqBody(t, handler, http.MethodPut, path, payload, auth)
}

func getJSON(t *testing.T, handler http.Handler, path string, auth string) apiResp {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return doReq(t, handler, req)
}

func reqBody(t *testing.T, handler http.Handler, method, path string, payload any, auth string) apiResp {
	t.Helper()
	var body []byte
	if payload != nil {
		body, _ = json.Marshal(payload)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return doReq(t, handler, req)
}

func reqMethod(t *testing.T, handler http.Handler, method, path string, payload any, auth string) apiResp {
	t.Helper()
	return reqBody(t, handler, method, path, payload, auth)
}

func doReq(t *testing.T, handler http.Handler, req *http.Request) apiResp {
	t.Helper()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return apiResp{Status: w.Code, RawBody: w.Body.Bytes()}
}

// Ensure unused import doesn't cause issues.
var _ = fmt.Sprintf
