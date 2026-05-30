package outbox

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"
)

type Helper struct {
	repo *Repository
}

func NewHelper(repo *Repository) *Helper {
	return &Helper{repo: repo}
}

func (h *Helper) CreateEvent(ctx context.Context, tx *gorm.DB, topic, aggregateType string, aggregateID uint64, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	event := &Event{
		Topic:         topic,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       string(data),
		Status:        StatusPending,
	}
	return h.repo.CreateInTx(ctx, tx, event)
}
