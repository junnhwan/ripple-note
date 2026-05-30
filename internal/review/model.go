package review

import "time"

const (
	TaskStatusPendingAgent   = "pending_agent"
	TaskStatusAgentPassed    = "agent_passed"
	TaskStatusAgentRejected  = "agent_rejected"
	TaskStatusManualRequired = "manual_required"
	TaskStatusAdminApproved  = "admin_approved"
	TaskStatusAdminRejected  = "admin_rejected"

	SourcePublish = "publish"
	SourceEdit    = "edit"
	SourceReport  = "report"

	ActorTypeSystem = "system"
	ActorTypeAgent  = "agent"
	ActorTypeAdmin  = "admin"
)

type ReviewTask struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement"`
	NoteID         uint64     `gorm:"not null;index"`
	AuthorID       uint64     `gorm:"not null;index"`
	Status         string     `gorm:"size:32;not null;index"`
	Source         string     `gorm:"size:32;not null"`
	AgentDecision  *string    `gorm:"size:32"`
	AgentRiskLevel *string    `gorm:"size:32"`
	AgentReason    *string    `gorm:"type:text"`
	AgentTraceID   *string    `gorm:"size:128"`
	AdminDecision  *string    `gorm:"size:32"`
	AdminReason    *string    `gorm:"type:text"`
	DecidedAt      *time.Time `gorm:""`
	CreatedAt      time.Time  `gorm:"not null"`
	UpdatedAt      time.Time  `gorm:"not null"`
}

func (ReviewTask) TableName() string { return "review_tasks" }

type ReviewTaskEvent struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	TaskID      uint64    `gorm:"not null;index"`
	ActorType   string    `gorm:"size:32;not null"`
	ActorID     string    `gorm:"size:128;not null"`
	EventType   string    `gorm:"size:64;not null"`
	PayloadJSON string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (ReviewTaskEvent) TableName() string { return "review_task_events" }
