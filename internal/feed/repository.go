package feed

import (
	"context"

	"gorm.io/gorm"

	"ripple-note/internal/note"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListLatest(ctx context.Context, cursor Cursor, limit int) ([]*note.Note, error) {
	query := r.db.WithContext(ctx).
		Where("status = ? AND visibility = ? AND published_at IS NOT NULL", note.StatusPublished, note.VisibilityPublic).
		Order("published_at DESC, id DESC")

	query = applyLatestCursor(query, cursor)
	query = query.Limit(limit + 1) // fetch one extra to determine has_more

	var notes []*note.Note
	if err := query.Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *Repository) ListHot(ctx context.Context, cursor Cursor, limit int) ([]*note.Note, error) {
	query := r.db.WithContext(ctx).
		Where("status = ? AND visibility = ? AND published_at IS NOT NULL", note.StatusPublished, note.VisibilityPublic).
		Order("hot_score DESC, id DESC")

	query = applyHotCursor(query, cursor)
	query = query.Limit(limit + 1)

	var notes []*note.Note
	if err := query.Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *Repository) ListByAuthorIDs(ctx context.Context, authorIDs []uint64, cursor Cursor, limit int) ([]*note.Note, error) {
	if len(authorIDs) == 0 {
		return nil, nil
	}
	query := r.db.WithContext(ctx).
		Where("status = ? AND visibility = ? AND published_at IS NOT NULL AND author_id IN ?", note.StatusPublished, note.VisibilityPublic, authorIDs).
		Order("published_at DESC, id DESC")

	query = applyLatestCursor(query, cursor)
	query = query.Limit(limit + 1)

	var notes []*note.Note
	if err := query.Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *Repository) ListByTagID(ctx context.Context, tagID uint64, cursor Cursor, limit int) ([]*note.Note, error) {
	query := r.db.WithContext(ctx).
		Joins("JOIN note_tags ON note_tags.note_id = notes.id").
		Where("notes.status = ? AND notes.visibility = ? AND notes.published_at IS NOT NULL AND note_tags.tag_id = ?", note.StatusPublished, note.VisibilityPublic, tagID).
		Order("notes.published_at DESC, notes.id DESC")

	query = applyLatestCursor(query, cursor)
	query = query.Limit(limit + 1)

	var notes []*note.Note
	if err := query.Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *Repository) FindTagByName(ctx context.Context, name string) (*note.Tag, error) {
	var tag note.Tag
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&tag).Error
	if err != nil {
		return nil, err
	}
	return &tag, nil
}
