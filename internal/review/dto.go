package review

import "time"

type TaskDTO struct {
	ID             uint64     `json:"id"`
	NoteID         uint64     `json:"note_id"`
	AuthorID       uint64     `json:"author_id"`
	Status         string     `json:"status"`
	Source         string     `json:"source"`
	AgentDecision  *string    `json:"agent_decision,omitempty"`
	AgentRiskLevel *string    `json:"agent_risk_level,omitempty"`
	AgentReason    *string    `json:"agent_reason,omitempty"`
	AgentTraceID   *string    `json:"agent_trace_id,omitempty"`
	AdminDecision  *string    `json:"admin_decision,omitempty"`
	AdminReason    *string    `json:"admin_reason,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type TaskListDTO struct {
	Items []TaskDTO `json:"items"`
	Total int64     `json:"total"`
}

type AdminNoteDTO struct {
	ID             uint64     `json:"id"`
	AuthorID       uint64     `json:"author_id"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	Status         string     `json:"status"`
	Visibility     string     `json:"visibility"`
	ReviewTaskID   *uint64    `json:"review_task_id,omitempty"`
	LikesCount     uint64     `json:"likes_count"`
	FavoritesCount uint64     `json:"favorites_count"`
	CommentsCount  uint64     `json:"comments_count"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type AdminNoteListDTO struct {
	Items []AdminNoteDTO `json:"items"`
	Total int64          `json:"total"`
}

type DecisionInput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}
