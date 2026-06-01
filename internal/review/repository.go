package review

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"ripple-note/internal/note"
)

var ErrTaskNotFound = errors.New("review task not found")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB { return r.db }

func (r *Repository) CreateTask(ctx context.Context, tx *gorm.DB, task *ReviewTask) (*ReviewTask, error) {
	if err := d(r.db, tx).WithContext(ctx).Create(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}

func (r *Repository) FindByID(ctx context.Context, tx *gorm.DB, id uint64) (*ReviewTask, error) {
	var task ReviewTask
	err := d(r.db, tx).WithContext(ctx).First(&task, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *Repository) List(ctx context.Context, status string, limit, offset int) ([]*ReviewTask, int64, error) {
	var tasks []*ReviewTask
	var total int64

	query := r.db.WithContext(ctx)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Model(&ReviewTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

func (r *Repository) SearchNotes(ctx context.Context, status, keyword string, limit, offset int) ([]*note.Note, int64, error) {
	var notes []*note.Note
	var total int64

	query := r.db.WithContext(ctx).Model(&note.Note{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(title) LIKE ? OR LOWER(body) LIKE ?", like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&notes).Error; err != nil {
		return nil, 0, err
	}
	return notes, total, nil
}

func (r *Repository) UpdateTask(ctx context.Context, tx *gorm.DB, task *ReviewTask) error {
	return d(r.db, tx).WithContext(ctx).Save(task).Error
}

func (r *Repository) CreateEvent(ctx context.Context, tx *gorm.DB, event *ReviewTaskEvent) error {
	return d(r.db, tx).WithContext(ctx).Create(event).Error
}

func d(db, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return db
}
