package note

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

var ErrInvalidInput = errors.New("invalid input")

type AuthorProvider interface {
	FindByID(ctx context.Context, id uint64) (AuthorDTO, error)
}

type ReviewTaskCreator interface {
	CreateInTx(ctx context.Context, tx *gorm.DB, noteID, authorID uint64, source string) (uint64, error)
}

type OutboxEventCreator interface {
	CreateEvent(ctx context.Context, tx *gorm.DB, topic, aggregateType string, aggregateID uint64, payload any) error
}

type Service struct {
	repo              *Repository
	users             AuthorProvider
	reviewTaskCreator ReviewTaskCreator
	outbox            OutboxEventCreator
}

func NewService(repo *Repository, users AuthorProvider, reviewTaskCreator ReviewTaskCreator, outbox OutboxEventCreator) *Service {
	return &Service{repo: repo, users: users, reviewTaskCreator: reviewTaskCreator, outbox: outbox}
}

func (s *Service) Publish(ctx context.Context, input PublishInput, authorID uint64) (NoteDTO, error) {
	if err := validatePublishInput(input); err != nil {
		return NoteDTO{}, err
	}

	var note *Note
	err := s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		note = &Note{
			AuthorID:   authorID,
			Title:      strings.TrimSpace(input.Title),
			Body:       strings.TrimSpace(input.Body),
			Status:     StatusPendingReview,
			Visibility: VisibilityPublic,
		}

		created, err := s.repo.CreateNote(ctx, tx, note)
		if err != nil {
			return err
		}
		note = created

		for _, tagName := range normalizeTags(input.Tags) {
			tag, err := s.repo.FindOrCreateTag(ctx, tx, tagName)
			if err != nil {
				return err
			}
			if err := s.repo.CreateNoteTags(ctx, tx, []*NoteTag{{NoteID: note.ID, TagID: tag.ID}}); err != nil {
				return err
			}
		}

		var images []*NoteImage
		for i, url := range input.ImageURLs {
			images = append(images, &NoteImage{
				NoteID:     note.ID,
				URL:        url,
				StorageKey: ExtractStorageKey(url),
				SortOrder:  i,
			})
		}
		if err := s.repo.CreateNoteImages(ctx, tx, images); err != nil {
			return err
		}
			if s.reviewTaskCreator != nil {
				taskID, err := s.reviewTaskCreator.CreateInTx(ctx, tx, note.ID, authorID, "publish")
				if err != nil {
					return err
				}
				if err := s.repo.UpdateReviewTaskID(ctx, tx, note.ID, taskID); err != nil {
					return err
				}
			}
			if s.outbox != nil {
				_ = s.outbox.CreateEvent(ctx, tx, "note.review_requested", "note", note.ID, map[string]any{"note_id": note.ID, "author_id": authorID})
			}

		return nil
	})
	if err != nil {
		return NoteDTO{}, err
	}

	return s.toDTO(ctx, note)
}

func (s *Service) Detail(ctx context.Context, noteID uint64, viewerID uint64) (NoteDTO, error) {
	note, err := s.repo.FindNoteByID(ctx, noteID)
	if err != nil {
		return NoteDTO{}, err
	}

	if viewerID == 0 || viewerID != note.AuthorID {
		if note.Status != StatusPublished {
			return NoteDTO{}, ErrNoteNotFound
		}
	}

	return s.toDTO(ctx, note)
}

func (s *Service) MyNotes(ctx context.Context, authorID uint64, limit, offset int) (NoteListDTO, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	notes, total, err := s.repo.FindNotesByAuthorID(ctx, authorID, limit, offset)
	if err != nil {
		return NoteListDTO{}, err
	}

	items := make([]NoteDTO, 0, len(notes))
	for _, n := range notes {
		dto, err := s.toDTO(ctx, n)
		if err != nil {
			return NoteListDTO{}, err
		}
		items = append(items, dto)
	}

	return NoteListDTO{Items: items, Total: total}, nil
}

func (s *Service) toDTO(ctx context.Context, note *Note) (NoteDTO, error) {
	author, err := s.users.FindByID(ctx, note.AuthorID)
	if err != nil {
		return NoteDTO{}, fmt.Errorf("find author: %w", err)
	}

	images, err := s.repo.FindImagesByNoteID(ctx, note.ID)
	if err != nil {
		return NoteDTO{}, fmt.Errorf("find images: %w", err)
	}

	tags, err := s.repo.FindTagNamesByNoteID(ctx, note.ID)
	if err != nil {
		return NoteDTO{}, fmt.Errorf("find tags: %w", err)
	}

	imageDTOs := make([]ImageDTO, 0, len(images))
	for _, img := range images {
		imageDTOs = append(imageDTOs, ImageDTO{
			ID:     img.ID,
			URL:    img.URL,
			Width:  img.Width,
			Height: img.Height,
		})
	}

	return NoteDTO{
		ID:             note.ID,
		Title:          note.Title,
		Body:           note.Body,
		Status:         note.Status,
		Visibility:     note.Visibility,
		Author:         author,
		Images:         imageDTOs,
		Tags:           tags,
		LikesCount:     note.LikesCount,
		FavoritesCount: note.FavoritesCount,
		CommentsCount:  note.CommentsCount,
		PublishedAt:    note.PublishedAt,
		CreatedAt:      note.CreatedAt,
		UpdatedAt:      note.UpdatedAt,
	}, nil
}

func validatePublishInput(input PublishInput) error {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	if len(title) > 120 {
		return fmt.Errorf("%w: title must be at most 120 characters", ErrInvalidInput)
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		return fmt.Errorf("%w: body is required", ErrInvalidInput)
	}
	for _, url := range input.ImageURLs {
		if err := ValidateImageURL(url); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
		}
	}
	for _, name := range input.Tags {
		if NormalizeTagName(name) == "" {
			return fmt.Errorf("%w: tag name must not be empty", ErrInvalidInput)
		}
	}
	return nil
}

func normalizeTags(raw []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, name := range raw {
		normalized := NormalizeTagName(name)
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result
}
