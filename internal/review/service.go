package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"ripple-note/internal/note"
	"ripple-note/internal/outbox"
)

var (
	ErrInvalidDecision = errors.New("invalid decision")
	ErrAlreadyDecided  = errors.New("task already decided")
	ErrInvalidStatus   = errors.New("invalid status")
)

// CacheInvalidator invalidates caches after review decisions.
type CacheInvalidator interface {
	InvalidateFeedCache(ctx context.Context)
	InvalidateNoteCache(ctx context.Context, noteID uint64)
}

type OutboxEventCreator interface {
	CreateEvent(ctx context.Context, tx *gorm.DB, topic, aggregateType string, aggregateID uint64, payload any) error
}

type Service struct {
	repo   *Repository
	notes  *note.Repository
	cache  CacheInvalidator
	outbox OutboxEventCreator
}

func NewService(repo *Repository, notes *note.Repository, creators ...OutboxEventCreator) *Service {
	service := &Service{repo: repo, notes: notes}
	if len(creators) > 0 {
		service.outbox = creators[0]
	}
	return service
}

// SetCacheInvalidator injects an optional cache invalidation callback.
func (s *Service) SetCacheInvalidator(cache CacheInvalidator) {
	s.cache = cache
}

func (s *Service) CreateInTx(ctx context.Context, tx *gorm.DB, noteID, authorID uint64, source string) (uint64, error) {
	task := &ReviewTask{
		NoteID:   noteID,
		AuthorID: authorID,
		Status:   TaskStatusPendingAgent,
		Source:   source,
	}
	created, err := s.repo.CreateTask(ctx, tx, task)
	if err != nil {
		return 0, err
	}

	payload, _ := json.Marshal(map[string]any{"note_id": noteID, "source": source})
	event := &ReviewTaskEvent{
		TaskID:      created.ID,
		ActorType:   ActorTypeSystem,
		ActorID:     "system",
		EventType:   "task_created",
		PayloadJSON: string(payload),
	}
	if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
		return 0, err
	}

	return created.ID, nil
}

func (s *Service) List(ctx context.Context, status string, limit, offset int) (TaskListDTO, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	tasks, total, err := s.repo.List(ctx, status, limit, offset)
	if err != nil {
		return TaskListDTO{}, err
	}

	items := make([]TaskDTO, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, toTaskDTO(t))
	}

	return TaskListDTO{Items: items, Total: total}, nil
}

func (s *Service) SearchNotes(ctx context.Context, status, keyword string, limit, offset int) (AdminNoteListDTO, error) {
	if !isValidNoteStatusFilter(status) {
		return AdminNoteListDTO{}, ErrInvalidStatus
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	notes, total, err := s.repo.SearchNotes(ctx, status, keyword, limit, offset)
	if err != nil {
		return AdminNoteListDTO{}, err
	}

	items := make([]AdminNoteDTO, 0, len(notes))
	for _, n := range notes {
		items = append(items, toAdminNoteDTO(n))
	}
	return AdminNoteListDTO{Items: items, Total: total}, nil
}

func (s *Service) GetByID(ctx context.Context, id uint64) (TaskDTO, error) {
	task, err := s.repo.FindByID(ctx, nil, id)
	if err != nil {
		return TaskDTO{}, err
	}
	return toTaskDTO(task), nil
}

func isValidNoteStatusFilter(status string) bool {
	switch status {
	case "", note.StatusPendingReview, note.StatusPublished, note.StatusRejected, note.StatusRemoved:
		return true
	default:
		return false
	}
}

type DecideInput struct {
	Decision string
	Reason   string
	AdminID  uint64
}

func (s *Service) Decide(ctx context.Context, taskID uint64, input DecideInput) (TaskDTO, error) {
	var task *ReviewTask
	err := s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		task, err = s.repo.FindByID(ctx, tx, taskID)
		if err != nil {
			return err
		}

		var newStatus, noteStatus string
		var publishedAt *time.Time

		switch input.Decision {
		case "approve":
			if isAdminFinalStatus(task.Status) {
				return ErrAlreadyDecided
			}
			newStatus = TaskStatusAdminApproved
			noteStatus = note.StatusPublished
			now := time.Now()
			publishedAt = &now
		case "reject":
			if isAdminFinalStatus(task.Status) {
				return ErrAlreadyDecided
			}
			newStatus = TaskStatusAdminRejected
			noteStatus = note.StatusRejected
		case "remove":
			if task.Status == TaskStatusAdminRemoved {
				return ErrAlreadyDecided
			}
			newStatus = TaskStatusAdminRemoved
			noteStatus = note.StatusRemoved
		default:
			return ErrInvalidDecision
		}

		task.Status = newStatus
		adminDecision := input.Decision
		task.AdminDecision = &adminDecision
		if input.Reason != "" {
			task.AdminReason = &input.Reason
		}
		now := time.Now()
		task.DecidedAt = &now

		if err := s.repo.UpdateTask(ctx, tx, task); err != nil {
			return err
		}

		if err := s.notes.UpdateNoteStatus(ctx, tx, task.NoteID, noteStatus, publishedAt); err != nil {
			return err
		}

		payload, _ := json.Marshal(map[string]any{"decision": input.Decision, "reason": input.Reason})
		event := &ReviewTaskEvent{
			TaskID:      task.ID,
			ActorType:   ActorTypeAdmin,
			ActorID:     fmt.Sprintf("%d", input.AdminID),
			EventType:   "admin_decided",
			PayloadJSON: string(payload),
		}
		if err := s.repo.CreateEvent(ctx, tx, event); err != nil {
			return err
		}
		if err := s.createReviewDecidedEvent(ctx, tx, task, input.AdminID, noteStatus); err != nil {
			return fmt.Errorf("create outbox event: %w", err)
		}
		return nil
	})
	if err != nil {
		return TaskDTO{}, err
	}

	// Invalidate caches after successful transaction commit.
	if s.cache != nil {
		s.cache.InvalidateFeedCache(ctx)
		s.cache.InvalidateNoteCache(ctx, task.NoteID)
	}

	return toTaskDTO(task), nil
}

func isAdminFinalStatus(status string) bool {
	switch status {
	case TaskStatusAdminApproved, TaskStatusAdminRejected, TaskStatusAdminRemoved:
		return true
	default:
		return false
	}
}

func (s *Service) createReviewDecidedEvent(ctx context.Context, tx *gorm.DB, task *ReviewTask, adminID uint64, noteStatus string) error {
	if s.outbox == nil {
		return nil
	}
	return s.outbox.CreateEvent(ctx, tx, outbox.TopicNoteReviewDecided, "note", task.NoteID, map[string]any{
		"note_id":     task.NoteID,
		"task_id":     task.ID,
		"author_id":   task.AuthorID,
		"decision":    valueOrEmpty(task.AdminDecision),
		"actor_type":  ActorTypeAdmin,
		"actor_id":    adminID,
		"note_status": noteStatus,
	})
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func toTaskDTO(task *ReviewTask) TaskDTO {
	return TaskDTO{
		ID:             task.ID,
		NoteID:         task.NoteID,
		AuthorID:       task.AuthorID,
		Status:         task.Status,
		Source:         task.Source,
		AgentDecision:  task.AgentDecision,
		AgentRiskLevel: task.AgentRiskLevel,
		AgentReason:    task.AgentReason,
		AgentTraceID:   task.AgentTraceID,
		AdminDecision:  task.AdminDecision,
		AdminReason:    task.AdminReason,
		DecidedAt:      task.DecidedAt,
		CreatedAt:      task.CreatedAt,
		UpdatedAt:      task.UpdatedAt,
	}
}

func toAdminNoteDTO(n *note.Note) AdminNoteDTO {
	return AdminNoteDTO{
		ID:             n.ID,
		AuthorID:       n.AuthorID,
		Title:          n.Title,
		Body:           n.Body,
		Status:         n.Status,
		Visibility:     n.Visibility,
		ReviewTaskID:   n.ReviewTaskID,
		LikesCount:     n.LikesCount,
		FavoritesCount: n.FavoritesCount,
		CommentsCount:  n.CommentsCount,
		PublishedAt:    n.PublishedAt,
		CreatedAt:      n.CreatedAt,
		UpdatedAt:      n.UpdatedAt,
	}
}
