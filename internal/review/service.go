package review

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"ripple-note/internal/note"
)

var (
	ErrInvalidDecision = errors.New("invalid decision")
	ErrAlreadyDecided  = errors.New("task already decided")
)

type Service struct {
	repo  *Repository
	notes *note.Repository
}

func NewService(repo *Repository, notes *note.Repository) *Service {
	return &Service{repo: repo, notes: notes}
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

	event := &ReviewTaskEvent{
		TaskID:      created.ID,
		ActorType:   ActorTypeSystem,
		ActorID:     "system",
		EventType:   "task_created",
		PayloadJSON: fmt.Sprintf(`{"note_id": %d, "source": "%s"}`, noteID, source),
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

func (s *Service) GetByID(ctx context.Context, id uint64) (TaskDTO, error) {
	task, err := s.repo.FindByID(ctx, nil, id)
	if err != nil {
		return TaskDTO{}, err
	}
	return toTaskDTO(task), nil
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

		if task.Status == TaskStatusAdminApproved || task.Status == TaskStatusAdminRejected {
			return ErrAlreadyDecided
		}

		var newStatus, noteStatus string
		var publishedAt *time.Time

		switch input.Decision {
		case "approve":
			newStatus = TaskStatusAdminApproved
			noteStatus = note.StatusPublished
			now := time.Now()
			publishedAt = &now
		case "reject":
			newStatus = TaskStatusAdminRejected
			noteStatus = note.StatusRejected
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

		event := &ReviewTaskEvent{
			TaskID:      task.ID,
			ActorType:   ActorTypeAdmin,
			ActorID:     fmt.Sprintf("%d", input.AdminID),
			EventType:   "admin_decided",
			PayloadJSON: fmt.Sprintf(`{"decision": "%s", "reason": "%s"}`, input.Decision, input.Reason),
		}
		return s.repo.CreateEvent(ctx, tx, event)
	})
	if err != nil {
		return TaskDTO{}, err
	}

	return toTaskDTO(task), nil
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
