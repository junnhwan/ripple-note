package note

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

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
