package review

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	httpapi "ripple-note/internal/http"
	"ripple-note/internal/note"
)

type InternalHandler struct {
	reviewRepo *Repository
	noteRepo   *note.Repository
}

func NewInternalHandler(reviewRepo *Repository, noteRepo *note.Repository) *InternalHandler {
	return &InternalHandler{reviewRepo: reviewRepo, noteRepo: noteRepo}
}

func (h *InternalHandler) RegisterRoutes(router gin.IRouter, internalAuth gin.HandlerFunc) {
	router.GET("/internal/review/tasks/pending", internalAuth, h.ListPending)
	router.GET("/internal/review/tasks/:taskId", internalAuth, h.GetTask)
	router.GET("/internal/notes/:noteId/review-context", internalAuth, h.GetReviewContext)
	router.PUT("/internal/review/tasks/:taskId/agent-result", internalAuth, h.SubmitAgentResult)
}

func (h *InternalHandler) ListPending(c *gin.Context) {
	limit := parseQueryInt(c, "limit", 10)
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	tasks, total, err := h.reviewRepo.List(c.Request.Context(), TaskStatusPendingAgent, limit, 0)
	if err != nil {
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	items := make([]TaskDTO, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, toTaskDTO(t))
	}

	httpapi.OK(c, gin.H{"items": items, "total": total})
}

func (h *InternalHandler) GetTask(c *gin.Context) {
	taskID, err := parseTaskID(c.Param("taskId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_task_id", "task id must be a positive integer")
		return
	}

	task, err := h.reviewRepo.FindByID(c.Request.Context(), nil, taskID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpapi.OK(c, toTaskDTO(task))
}

type ReviewContextDTO struct {
	Note   NoteContextDTO `json:"note"`
	Author AuthorInfoDTO  `json:"author"`
	Images []ImageInfoDTO `json:"images"`
}

type NoteContextDTO struct {
	ID     uint64 `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Status string `json:"status"`
}

type AuthorInfoDTO struct {
	ID       uint64 `json:"id"`
	Nickname string `json:"nickname"`
}

type ImageInfoDTO struct {
	URL string `json:"url"`
}

func (h *InternalHandler) GetReviewContext(c *gin.Context) {
	noteID, err := parseNoteIDParam(c.Param("noteId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_note_id", "note id must be a positive integer")
		return
	}

	n, err := h.noteRepo.FindNoteByID(c.Request.Context(), noteID)
	if err != nil {
		httpapi.Error(c, http.StatusNotFound, "note_not_found", "note not found")
		return
	}

	images, _ := h.noteRepo.FindImagesByNoteID(c.Request.Context(), noteID)
	imageDTOs := make([]ImageInfoDTO, 0, len(images))
	for _, img := range images {
		imageDTOs = append(imageDTOs, ImageInfoDTO{URL: img.URL})
	}

	httpapi.OK(c, ReviewContextDTO{
		Note: NoteContextDTO{
			ID:     n.ID,
			Title:  n.Title,
			Body:   n.Body,
			Status: n.Status,
		},
		Author: AuthorInfoDTO{
			ID:       n.AuthorID,
			Nickname: fmt.Sprintf("user_%d", n.AuthorID),
		},
		Images: imageDTOs,
	})
}

type agentResultRequest struct {
	Decision   string  `json:"decision"`
	RiskLevel  string  `json:"risk_level"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
	TraceID    string  `json:"trace_id"`
}

func (h *InternalHandler) SubmitAgentResult(c *gin.Context) {
	taskID, err := parseTaskID(c.Param("taskId"))
	if err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_task_id", "task id must be a positive integer")
		return
	}

	var req agentResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	var newTaskStatus, noteStatus string
	switch req.Decision {
	case "pass":
		newTaskStatus = TaskStatusAgentPassed
		noteStatus = note.StatusPublished
	case "reject":
		newTaskStatus = TaskStatusAgentRejected
		noteStatus = note.StatusRejected
	case "manual_review":
		newTaskStatus = TaskStatusManualRequired
	default:
		httpapi.Error(c, http.StatusBadRequest, "invalid_decision", "decision must be pass, reject, or manual_review")
		return
	}

	var task *ReviewTask
	err = h.reviewRepo.DB().WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		task, err = h.reviewRepo.FindByID(c.Request.Context(), tx, taskID)
		if err != nil {
			return err
		}

		if task.Status != TaskStatusPendingAgent {
			return ErrAlreadyDecided
		}

		task.Status = newTaskStatus
		task.AgentDecision = &req.Decision
		task.AgentRiskLevel = &req.RiskLevel
		task.AgentReason = &req.Reason
		task.AgentTraceID = &req.TraceID

		if err := h.reviewRepo.UpdateTask(c.Request.Context(), tx, task); err != nil {
			return err
		}

		if noteStatus != "" {
			var publishedAt *time.Time
			if noteStatus == note.StatusPublished {
				now := time.Now()
				publishedAt = &now
			}
			if err := h.noteRepo.UpdateNoteStatus(c.Request.Context(), tx, task.NoteID, noteStatus, publishedAt); err != nil {
				return err
			}
		}

		event := &ReviewTaskEvent{
			TaskID:      task.ID,
			ActorType:   ActorTypeAgent,
			ActorID:     "ripple-guard-agent",
			EventType:   "agent_decided",
			PayloadJSON: fmt.Sprintf(`{"decision":"%s","risk_level":"%s","trace_id":"%s"}`, req.Decision, req.RiskLevel, req.TraceID),
		}
		return h.reviewRepo.CreateEvent(c.Request.Context(), tx, event)
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	httpapi.OK(c, toTaskDTO(task))
}

func (h *InternalHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTaskNotFound):
		httpapi.Error(c, http.StatusNotFound, "task_not_found", "review task not found")
	case errors.Is(err, ErrAlreadyDecided):
		httpapi.Error(c, http.StatusConflict, "already_decided", "review task has already been decided")
	default:
		httpapi.Error(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func parseNoteIDParam(raw string) (uint64, error) {
	return parseTaskID(raw)
}
