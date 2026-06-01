package note

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrNoteNotFound = errors.New("note not found")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB { return r.db }

func (r *Repository) CreateNote(ctx context.Context, tx *gorm.DB, note *Note) (*Note, error) {
	if err := d(r.db, tx).WithContext(ctx).Create(note).Error; err != nil {
		return nil, err
	}
	return note, nil
}

func (r *Repository) FindNoteByID(ctx context.Context, id uint64) (*Note, error) {
	var note Note
	err := r.db.WithContext(ctx).First(&note, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNoteNotFound
	}
	if err != nil {
		return nil, err
	}
	return &note, nil
}

func (r *Repository) FindNotesByAuthorID(ctx context.Context, authorID uint64, limit, offset int) ([]*Note, int64, error) {
	var notes []*Note
	var total int64

	query := r.db.WithContext(ctx).Where("author_id = ?", authorID)
	if err := query.Model(&Note{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&notes).Error; err != nil {
		return nil, 0, err
	}
	return notes, total, nil
}

func (r *Repository) FindPublicNotesByAuthorID(ctx context.Context, authorID uint64, limit, offset int) ([]*Note, int64, error) {
	var notes []*Note
	var total int64

	query := r.db.WithContext(ctx).Where(
		"author_id = ? AND status = ? AND visibility = ?",
		authorID,
		StatusPublished,
		VisibilityPublic,
	)
	if err := query.Model(&Note{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("published_at DESC, id DESC").Limit(limit).Offset(offset).Find(&notes).Error; err != nil {
		return nil, 0, err
	}
	return notes, total, nil
}

func (r *Repository) FindOrCreateTag(ctx context.Context, tx *gorm.DB, name string) (*Tag, error) {
	var tag Tag
	d := d(r.db, tx)
	err := d.WithContext(ctx).Where("name = ?", name).First(&tag).Error
	if err == nil {
		return &tag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	tag = Tag{Name: name}
	if err := d.WithContext(ctx).Create(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *Repository) CreateNoteTags(ctx context.Context, tx *gorm.DB, noteTags []*NoteTag) error {
	if len(noteTags) == 0 {
		return nil
	}
	return d(r.db, tx).WithContext(ctx).Create(&noteTags).Error
}

func (r *Repository) FindTagNamesByNoteID(ctx context.Context, noteID uint64) ([]string, error) {
	var names []string
	err := r.db.WithContext(ctx).
		Table("tags").
		Select("tags.name").
		Joins("JOIN note_tags ON note_tags.tag_id = tags.id").
		Where("note_tags.note_id = ?", noteID).
		Pluck("tags.name", &names).Error
	return names, err
}

func (r *Repository) FindTagNamesByNoteIDs(ctx context.Context, noteIDs []uint64) (map[uint64][]string, error) {
	if len(noteIDs) == 0 {
		return map[uint64][]string{}, nil
	}

	type row struct {
		NoteID uint64
		Name   string
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("note_tags").
		Select("note_tags.note_id, tags.name").
		Joins("JOIN tags ON tags.id = note_tags.tag_id").
		Where("note_tags.note_id IN ?", uniqueUint64s(noteIDs)).
		Order("note_tags.note_id ASC, tags.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	byNoteID := make(map[uint64][]string, len(noteIDs))
	for _, row := range rows {
		byNoteID[row.NoteID] = append(byNoteID[row.NoteID], row.Name)
	}
	return byNoteID, nil
}

func (r *Repository) CreateNoteImages(ctx context.Context, tx *gorm.DB, images []*NoteImage) error {
	if len(images) == 0 {
		return nil
	}
	return d(r.db, tx).WithContext(ctx).Create(&images).Error
}

func (r *Repository) FindImagesByNoteID(ctx context.Context, noteID uint64) ([]*NoteImage, error) {
	var images []*NoteImage
	err := r.db.WithContext(ctx).Where("note_id = ?", noteID).Order("sort_order").Find(&images).Error
	return images, err
}

func (r *Repository) FindImagesByNoteIDs(ctx context.Context, noteIDs []uint64) (map[uint64][]*NoteImage, error) {
	if len(noteIDs) == 0 {
		return map[uint64][]*NoteImage{}, nil
	}

	var images []*NoteImage
	err := r.db.WithContext(ctx).
		Where("note_id IN ?", uniqueUint64s(noteIDs)).
		Order("note_id ASC, sort_order ASC, id ASC").
		Find(&images).Error
	if err != nil {
		return nil, err
	}

	byNoteID := make(map[uint64][]*NoteImage, len(noteIDs))
	for _, image := range images {
		byNoteID[image.NoteID] = append(byNoteID[image.NoteID], image)
	}
	return byNoteID, nil
}

func (r *Repository) UpdateNoteStatus(ctx context.Context, tx *gorm.DB, noteID uint64, status string, publishedAt *time.Time) error {
	updates := map[string]any{"status": status}
	if publishedAt != nil {
		updates["published_at"] = *publishedAt
	}
	return d(r.db, tx).WithContext(ctx).Model(&Note{}).Where("id = ?", noteID).Updates(updates).Error
}

func (r *Repository) UpdateReviewTaskID(ctx context.Context, tx *gorm.DB, noteID, taskID uint64) error {
	return d(r.db, tx).WithContext(ctx).Model(&Note{}).Where("id = ?", noteID).Update("review_task_id", taskID).Error
}

func (r *Repository) MarkNoteRemoved(ctx context.Context, noteID uint64) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&Note{}).
		Where("id = ? AND status <> ?", noteID, StatusRemoved).
		Update("status", StatusRemoved)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func ExtractStorageKey(url string) string {
	return filepath.Base(url)
}

func NormalizeTagName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func ValidateImageURL(url string) error {
	if !strings.HasPrefix(url, "/uploads/images/") {
		return fmt.Errorf("image url must start with /uploads/images/")
	}
	return nil
}

func d(db, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return db
}

func uniqueUint64s(ids []uint64) []uint64 {
	unique := make([]uint64, 0, len(ids))
	seen := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
