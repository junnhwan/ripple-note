package outbox

import (
	"encoding/json"
	"time"
)

const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"

	TopicNoteReviewRequested = "note.review_requested"
	TopicNoteReviewDecided   = "note.review_decided"
	TopicInteractionCreated  = "interaction.created"
)

type Event struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement"`
	Topic         string     `gorm:"size:128;not null;index"`
	AggregateType string     `gorm:"size:64;not null"`
	AggregateID   uint64     `gorm:"not null"`
	Payload       string     `gorm:"type:text;not null"`
	Status        string     `gorm:"size:32;not null;index"`
	RetryCount    int        `gorm:"not null;default:0"`
	NextRetryAt   *time.Time `gorm:"index"`
	CreatedAt     time.Time  `gorm:"not null"`
	UpdatedAt     time.Time  `gorm:"not null"`
}

func (Event) TableName() string { return "outbox_events" }

type EventPayload struct {
	NoteID  uint64 `json:"note_id,omitempty"`
	UserID  uint64 `json:"user_id,omitempty"`
	TaskID  uint64 `json:"task_id,omitempty"`
	Action  string `json:"action,omitempty"`
	Details any    `json:"details,omitempty"`
}

func NewEvent(topic, aggregateType string, aggregateID uint64, payload EventPayload) (*Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Event{
		Topic:         topic,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       string(data),
		Status:        StatusPending,
	}, nil
}
