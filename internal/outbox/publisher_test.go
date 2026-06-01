package outbox

import (
	"errors"
	"io"
	"log"
	"log/slog"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestWorkerAbandonsEventAfterMaxRetries(t *testing.T) {
	db := newOutboxTestDB(t)
	repo := NewRepository(db)
	event := createOutboxEvent(t, db, StatusPending, 5)

	worker := NewWorker(repo, failingPublisher{}, discardLogger(), time.Hour, 10)
	worker.processBatch()

	var got Event
	if err := db.First(&got, event.ID).Error; err != nil {
		t.Fatalf("find outbox event: %v", err)
	}
	if got.Status != "abandoned" {
		t.Fatalf("expected abandoned status, got %q", got.Status)
	}
	if got.RetryCount != 6 {
		t.Fatalf("expected retry_count=6, got %d", got.RetryCount)
	}
	if got.NextRetryAt != nil {
		t.Fatalf("expected abandoned event to clear next_retry_at, got %v", got.NextRetryAt)
	}

	pending, err := repo.FindPending(t.Context(), 10)
	if err != nil {
		t.Fatalf("find pending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected abandoned event not to be picked again, got %d events", len(pending))
	}
}

func TestWorkerKeepsFailedEventRetryableBeforeMaxRetries(t *testing.T) {
	db := newOutboxTestDB(t)
	repo := NewRepository(db)
	event := createOutboxEvent(t, db, StatusPending, 2)

	worker := NewWorker(repo, failingPublisher{}, discardLogger(), time.Hour, 10)
	worker.processBatch()

	var got Event
	if err := db.First(&got, event.ID).Error; err != nil {
		t.Fatalf("find outbox event: %v", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("expected failed status, got %q", got.Status)
	}
	if got.RetryCount != 3 {
		t.Fatalf("expected retry_count=3, got %d", got.RetryCount)
	}
	if got.NextRetryAt == nil {
		t.Fatal("expected next_retry_at to be set")
	}
}

type failingPublisher struct{}

func (f failingPublisher) Publish(_ string, _ []byte) error {
	return errors.New("publish failed")
}

func newOutboxTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.New(log.New(io.Discard, "", 0), logger.Config{}),
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	if err := db.AutoMigrate(&Event{}); err != nil {
		t.Fatalf("auto migrate outbox events: %v", err)
	}
	return db
}

func createOutboxEvent(t *testing.T, db *gorm.DB, status string, retryCount int) *Event {
	t.Helper()

	event := &Event{
		Topic:         TopicInteractionCreated,
		AggregateType: "note",
		AggregateID:   1,
		Payload:       `{"note_id":1}`,
		Status:        status,
		RetryCount:    retryCount,
	}
	if err := db.Create(event).Error; err != nil {
		t.Fatalf("create outbox event: %v", err)
	}
	return event
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
