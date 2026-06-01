package review_test

import (
	"encoding/json"
	"io"
	"log"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ripple-note/internal/note"
	"ripple-note/internal/outbox"
	"ripple-note/internal/review"
)

func TestServiceDecideCreatesReviewDecidedOutboxEvent(t *testing.T) {
	t.Parallel()

	db := newReviewServiceTestDB(t)
	noteRepo := note.NewRepository(db)
	reviewRepo := review.NewRepository(db)
	outboxHelper := outbox.NewHelper(outbox.NewRepository(db))
	service := review.NewService(reviewRepo, noteRepo, outboxHelper)

	publishedAt := time.Now()
	n := &note.Note{
		ID:          100,
		AuthorID:    200,
		Title:       "review note",
		Body:        "content",
		Status:      note.StatusPendingReview,
		Visibility:  note.VisibilityPublic,
		PublishedAt: &publishedAt,
	}
	if err := db.Create(n).Error; err != nil {
		t.Fatalf("create note: %v", err)
	}
	task := &review.ReviewTask{
		NoteID:   n.ID,
		AuthorID: n.AuthorID,
		Status:   review.TaskStatusPendingAgent,
		Source:   review.SourcePublish,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create review task: %v", err)
	}

	decided, err := service.Decide(t.Context(), task.ID, review.DecideInput{
		Decision: "approve",
		Reason:   "ok",
		AdminID:  300,
	})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if decided.Status != review.TaskStatusAdminApproved {
		t.Fatalf("expected admin approved, got %s", decided.Status)
	}

	var event outbox.Event
	if err := db.Where("topic = ?", outbox.TopicNoteReviewDecided).First(&event).Error; err != nil {
		t.Fatalf("find outbox event: %v", err)
	}
	if event.AggregateType != "note" {
		t.Fatalf("expected aggregate type note, got %q", event.AggregateType)
	}
	if event.AggregateID != n.ID {
		t.Fatalf("expected aggregate id %d, got %d", n.ID, event.AggregateID)
	}
	if event.Status != outbox.StatusPending {
		t.Fatalf("expected pending event, got %q", event.Status)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	assertPayloadValue(t, payload, "note_id", float64(n.ID))
	assertPayloadValue(t, payload, "task_id", float64(task.ID))
	assertPayloadValue(t, payload, "author_id", float64(n.AuthorID))
	assertPayloadValue(t, payload, "decision", "approve")
	assertPayloadValue(t, payload, "actor_type", review.ActorTypeAdmin)
	assertPayloadValue(t, payload, "actor_id", float64(300))
	assertPayloadValue(t, payload, "note_status", note.StatusPublished)
}

func newReviewServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.New(log.New(io.Discard, "", 0), logger.Config{}),
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	if err := db.AutoMigrate(&note.Note{}, &review.ReviewTask{}, &review.ReviewTaskEvent{}, &outbox.Event{}); err != nil {
		t.Fatalf("auto migrate review service tables: %v", err)
	}
	return db
}

func assertPayloadValue(t *testing.T, payload map[string]any, key string, want any) {
	t.Helper()

	got, ok := payload[key]
	if !ok {
		t.Fatalf("payload missing key %q in %#v", key, payload)
	}
	if got != want {
		t.Fatalf("payload[%s]: expected %#v, got %#v", key, want, got)
	}
}
