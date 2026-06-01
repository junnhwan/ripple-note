package interaction_test

import (
	"encoding/json"
	"io"
	"log"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ripple-note/internal/interaction"
	"ripple-note/internal/note"
	"ripple-note/internal/outbox"
)

func TestRepositoryFindsBatchViewerState(t *testing.T) {
	t.Parallel()

	db := newInteractionTestDB(t)
	repo := interaction.NewRepository(db)
	now := time.Now()

	if err := db.Create([]*interaction.NoteLike{
		{UserID: 1, NoteID: 10, CreatedAt: now},
		{UserID: 1, NoteID: 20, CreatedAt: now},
		{UserID: 2, NoteID: 30, CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create likes: %v", err)
	}
	if err := db.Create([]*interaction.NoteFavorite{
		{UserID: 1, NoteID: 20, CreatedAt: now},
		{UserID: 2, NoteID: 10, CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create favorites: %v", err)
	}
	if err := db.Create([]*interaction.Follow{
		{FollowerID: 1, FolloweeID: 100, CreatedAt: now},
		{FollowerID: 1, FolloweeID: 200, CreatedAt: now},
		{FollowerID: 2, FolloweeID: 300, CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create follows: %v", err)
	}

	liked, err := repo.LikedNoteIDs(t.Context(), 1, []uint64{10, 20, 30})
	if err != nil {
		t.Fatalf("liked note ids: %v", err)
	}
	if !liked[10] || !liked[20] || liked[30] {
		t.Fatalf("unexpected liked map: %#v", liked)
	}

	favorited, err := repo.FavoritedNoteIDs(t.Context(), 1, []uint64{10, 20, 30})
	if err != nil {
		t.Fatalf("favorited note ids: %v", err)
	}
	if favorited[10] || !favorited[20] || favorited[30] {
		t.Fatalf("unexpected favorited map: %#v", favorited)
	}

	following, err := repo.FollowingAuthorIDs(t.Context(), 1, []uint64{100, 200, 300})
	if err != nil {
		t.Fatalf("following author ids: %v", err)
	}
	if !following[100] || !following[200] || following[300] {
		t.Fatalf("unexpected following map: %#v", following)
	}
}

func TestRepositoryCreatesOutboxEventOnlyWhenLikeStateChanges(t *testing.T) {
	t.Parallel()

	db := newInteractionTestDB(t)
	repo := interaction.NewRepository(db, outbox.NewHelper(outbox.NewRepository(db)))
	createPublishedNote(t, db, 10, 100)

	created, err := repo.UpsertLike(t.Context(), 100, 10)
	if err != nil {
		t.Fatalf("upsert like: %v", err)
	}
	if !created {
		t.Fatal("expected first like to create state")
	}
	assertOutboxEvent(t, db, 1, outbox.TopicInteractionCreated, "note", 10, map[string]any{
		"note_id": float64(10),
		"user_id": float64(100),
		"action":  "like",
	})

	created, err = repo.UpsertLike(t.Context(), 100, 10)
	if err != nil {
		t.Fatalf("upsert duplicate like: %v", err)
	}
	if created {
		t.Fatal("expected duplicate like to be idempotent")
	}
	assertOutboxCount(t, db, 1)

	removed, err := repo.DeleteLike(t.Context(), 100, 10)
	if err != nil {
		t.Fatalf("delete like: %v", err)
	}
	if !removed {
		t.Fatal("expected unlike to remove state")
	}
	assertOutboxEvent(t, db, 2, outbox.TopicInteractionRemoved, "note", 10, map[string]any{
		"note_id": float64(10),
		"user_id": float64(100),
		"action":  "unlike",
	})

	removed, err = repo.DeleteLike(t.Context(), 100, 10)
	if err != nil {
		t.Fatalf("delete duplicate like: %v", err)
	}
	if removed {
		t.Fatal("expected duplicate unlike to be idempotent")
	}
	assertOutboxCount(t, db, 2)
}

func TestRepositoryCreatesOutboxEventsForFavoriteCommentAndFollow(t *testing.T) {
	t.Parallel()

	db := newInteractionTestDB(t)
	repo := interaction.NewRepository(db, outbox.NewHelper(outbox.NewRepository(db)))
	createPublishedNote(t, db, 20, 200)

	if created, err := repo.UpsertFavorite(t.Context(), 101, 20); err != nil || !created {
		t.Fatalf("upsert favorite created=%v err=%v", created, err)
	}
	if removed, err := repo.DeleteFavorite(t.Context(), 101, 20); err != nil || !removed {
		t.Fatalf("delete favorite removed=%v err=%v", removed, err)
	}
	comment, err := repo.CreateComment(t.Context(), &interaction.Comment{
		NoteID:   20,
		AuthorID: 102,
		Body:     "good",
		Status:   interaction.CommentStatusVisible,
	})
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if comment.ID == 0 {
		t.Fatal("expected comment id to be assigned")
	}
	if created, err := repo.UpsertFollow(t.Context(), 103, 203); err != nil || !created {
		t.Fatalf("upsert follow created=%v err=%v", created, err)
	}
	if removed, err := repo.DeleteFollow(t.Context(), 103, 203); err != nil || !removed {
		t.Fatalf("delete follow removed=%v err=%v", removed, err)
	}

	assertOutboxEvent(t, db, 1, outbox.TopicInteractionCreated, "note", 20, map[string]any{
		"note_id": float64(20),
		"user_id": float64(101),
		"action":  "favorite",
	})
	assertOutboxEvent(t, db, 2, outbox.TopicInteractionRemoved, "note", 20, map[string]any{
		"note_id": float64(20),
		"user_id": float64(101),
		"action":  "unfavorite",
	})
	assertOutboxEvent(t, db, 3, outbox.TopicInteractionCreated, "comment", comment.ID, map[string]any{
		"note_id":    float64(20),
		"user_id":    float64(102),
		"comment_id": float64(comment.ID),
		"action":     "comment",
	})
	assertOutboxEvent(t, db, 4, outbox.TopicInteractionCreated, "follow", 203, map[string]any{
		"user_id":     float64(103),
		"follower_id": float64(103),
		"followee_id": float64(203),
		"action":      "follow",
	})
	assertOutboxEvent(t, db, 5, outbox.TopicInteractionRemoved, "follow", 203, map[string]any{
		"user_id":     float64(103),
		"follower_id": float64(103),
		"followee_id": float64(203),
		"action":      "unfollow",
	})
}

func TestRepositoryCreatesOutboxEventOnlyWhenCommentDeleted(t *testing.T) {
	t.Parallel()

	db := newInteractionTestDB(t)
	repo := interaction.NewRepository(db, outbox.NewHelper(outbox.NewRepository(db)))
	createPublishedNote(t, db, 30, 300)

	comment, err := repo.CreateComment(t.Context(), &interaction.Comment{
		NoteID:   30,
		AuthorID: 301,
		Body:     "remove me",
		Status:   interaction.CommentStatusVisible,
	})
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	assertOutboxCount(t, db, 1)

	removed, noteID, err := repo.DeleteComment(t.Context(), 301, comment.ID)
	if err != nil {
		t.Fatalf("delete comment: %v", err)
	}
	if !removed {
		t.Fatal("expected comment to be removed")
	}
	if noteID != 30 {
		t.Fatalf("expected note id 30, got %d", noteID)
	}
	assertOutboxEvent(t, db, 2, outbox.TopicInteractionRemoved, "comment", comment.ID, map[string]any{
		"note_id":    float64(30),
		"user_id":    float64(301),
		"comment_id": float64(comment.ID),
		"action":     "delete_comment",
	})

	removed, noteID, err = repo.DeleteComment(t.Context(), 301, comment.ID)
	if err != nil {
		t.Fatalf("delete duplicate comment: %v", err)
	}
	if removed {
		t.Fatal("expected duplicate delete to be idempotent")
	}
	if noteID != 0 {
		t.Fatalf("expected no note id for duplicate delete, got %d", noteID)
	}
	assertOutboxCount(t, db, 2)
}

func newInteractionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.New(log.New(io.Discard, "", 0), logger.Config{}),
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	if err := db.AutoMigrate(&note.Note{}, &interaction.NoteLike{}, &interaction.NoteFavorite{}, &interaction.Comment{}, &interaction.Follow{}, &outbox.Event{}); err != nil {
		t.Fatalf("auto migrate interaction tables: %v", err)
	}
	return db
}

func createPublishedNote(t *testing.T, db *gorm.DB, noteID, authorID uint64) {
	t.Helper()

	publishedAt := time.Now()
	if err := db.Create(&note.Note{
		ID:          noteID,
		AuthorID:    authorID,
		Title:       "published note",
		Body:        "content",
		Status:      note.StatusPublished,
		Visibility:  note.VisibilityPublic,
		PublishedAt: &publishedAt,
	}).Error; err != nil {
		t.Fatalf("create published note: %v", err)
	}
}

func assertOutboxCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()

	var got int64
	if err := db.Model(&outbox.Event{}).Count(&got).Error; err != nil {
		t.Fatalf("count outbox events: %v", err)
	}
	if got != want {
		t.Fatalf("expected %d outbox events, got %d", want, got)
	}
}

func assertOutboxEvent(t *testing.T, db *gorm.DB, id uint64, topic, aggregateType string, aggregateID uint64, wantPayload map[string]any) {
	t.Helper()

	var event outbox.Event
	if err := db.First(&event, id).Error; err != nil {
		t.Fatalf("find outbox event %d: %v", id, err)
	}
	if event.Topic != topic {
		t.Fatalf("event %d topic: expected %q, got %q", id, topic, event.Topic)
	}
	if event.AggregateType != aggregateType {
		t.Fatalf("event %d aggregate type: expected %q, got %q", id, aggregateType, event.AggregateType)
	}
	if event.AggregateID != aggregateID {
		t.Fatalf("event %d aggregate id: expected %d, got %d", id, aggregateID, event.AggregateID)
	}
	if event.Status != outbox.StatusPending {
		t.Fatalf("event %d status: expected %q, got %q", id, outbox.StatusPending, event.Status)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		t.Fatalf("unmarshal event %d payload: %v", id, err)
	}
	for key, want := range wantPayload {
		got, ok := payload[key]
		if !ok {
			t.Fatalf("event %d payload missing key %q in %#v", id, key, payload)
		}
		if got != want {
			t.Fatalf("event %d payload[%s]: expected %#v, got %#v", id, key, want, got)
		}
	}
}
