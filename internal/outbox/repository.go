package outbox

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateInTx(ctx context.Context, tx *gorm.DB, event *Event) error {
	d := r.tx(tx)
	return d.WithContext(ctx).Create(event).Error
}

// FindPending fetches events that are due for processing:
// status = pending, or status = failed with next_retry_at reached.
func (r *Repository) FindPending(ctx context.Context, limit int) ([]*Event, error) {
	var events []*Event
	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("(status = ? OR (status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?))", StatusPending, StatusFailed, now).
		Order("id ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func (r *Repository) MarkSent(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&Event{}).Where("id IN ?", ids).
		Updates(map[string]any{"status": StatusSent, "updated_at": time.Now()}).Error
}

func (r *Repository) MarkFailed(ctx context.Context, id uint64, retryCount int, nextRetryAt time.Time) error {
	return r.db.WithContext(ctx).Model(&Event{}).Where("id = ?", id).
		Updates(map[string]any{
			"status":        StatusFailed,
			"retry_count":   retryCount,
			"next_retry_at": nextRetryAt,
			"updated_at":    time.Now(),
		}).Error
}

func (r *Repository) DB() *gorm.DB { return r.db }

func (r *Repository) tx(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}
