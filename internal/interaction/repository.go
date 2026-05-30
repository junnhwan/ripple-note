package interaction

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"ripple-note/internal/note"
)

var ErrNoteNotFound = errors.New("note not found")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB { return r.db }

func (r *Repository) UpsertLike(ctx context.Context, userID, noteID uint64) (bool, error) {
	var existing NoteLike
	err := r.db.WithContext(ctx).Unscoped().Where("user_id = ? AND note_id = ?", userID, noteID).First(&existing).Error
	if err == nil {
		if existing.DeletedAt.Valid {
			if err := r.db.WithContext(ctx).Unscoped().Model(&existing).Update("deleted_at", nil).Error; err != nil {
				return false, err
			}
			if err := r.db.WithContext(ctx).Model(&note.Note{}).Where("id = ?", noteID).
				Update("likes_count", gorm.Expr("likes_count + 1")).Error; err != nil {
				return false, err
			}
			return true, nil
		}
		return false, nil // already liked
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}

	like := &NoteLike{UserID: userID, NoteID: noteID}
	if err := r.db.WithContext(ctx).Create(like).Error; err != nil {
		return false, err
	}
	if err := r.db.WithContext(ctx).Model(&note.Note{}).Where("id = ?", noteID).
		Update("likes_count", gorm.Expr("likes_count + 1")).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) DeleteLike(ctx context.Context, userID, noteID uint64) (bool, error) {
	result := r.db.WithContext(ctx).Where("user_id = ? AND note_id = ?", userID, noteID).Delete(&NoteLike{})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	if err := r.db.WithContext(ctx).Model(&note.Note{}).Where("id = ? AND likes_count > 0", noteID).
		Update("likes_count", gorm.Expr("likes_count - 1")).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) UpsertFavorite(ctx context.Context, userID, noteID uint64) (bool, error) {
	var existing NoteFavorite
	err := r.db.WithContext(ctx).Unscoped().Where("user_id = ? AND note_id = ?", userID, noteID).First(&existing).Error
	if err == nil {
		if existing.DeletedAt.Valid {
			if err := r.db.WithContext(ctx).Unscoped().Model(&existing).Update("deleted_at", nil).Error; err != nil {
				return false, err
			}
			if err := r.db.WithContext(ctx).Model(&note.Note{}).Where("id = ?", noteID).
				Update("favorites_count", gorm.Expr("favorites_count + 1")).Error; err != nil {
				return false, err
			}
			return true, nil
		}
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}

	fav := &NoteFavorite{UserID: userID, NoteID: noteID}
	if err := r.db.WithContext(ctx).Create(fav).Error; err != nil {
		return false, err
	}
	if err := r.db.WithContext(ctx).Model(&note.Note{}).Where("id = ?", noteID).
		Update("favorites_count", gorm.Expr("favorites_count + 1")).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) DeleteFavorite(ctx context.Context, userID, noteID uint64) (bool, error) {
	result := r.db.WithContext(ctx).Where("user_id = ? AND note_id = ?", userID, noteID).Delete(&NoteFavorite{})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	if err := r.db.WithContext(ctx).Model(&note.Note{}).Where("id = ? AND favorites_count > 0", noteID).
		Update("favorites_count", gorm.Expr("favorites_count - 1")).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) CreateComment(ctx context.Context, comment *Comment) (*Comment, error) {
	if err := r.db.WithContext(ctx).Create(comment).Error; err != nil {
		return nil, err
	}
	_ = r.db.WithContext(ctx).Model(&note.Note{}).Where("id = ?", comment.NoteID).
		Update("comments_count", gorm.Expr("comments_count + 1")).Error
	return comment, nil
}

func (r *Repository) ListComments(ctx context.Context, noteID uint64, limit, offset int) ([]*Comment, int64, error) {
	var comments []*Comment
	var total int64
	query := r.db.WithContext(ctx).Where("note_id = ? AND status = ?", noteID, CommentStatusVisible)
	if err := query.Model(&Comment{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id ASC").Limit(limit).Offset(offset).Find(&comments).Error; err != nil {
		return nil, 0, err
	}
	return comments, total, nil
}

func (r *Repository) UpsertFollow(ctx context.Context, followerID, followeeID uint64) (bool, error) {
	var existing Follow
	err := r.db.WithContext(ctx).Where("follower_id = ? AND followee_id = ?", followerID, followeeID).First(&existing).Error
	if err == nil {
		return false, nil // already following
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if err := r.db.WithContext(ctx).Create(&Follow{FollowerID: followerID, FolloweeID: followeeID}).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) DeleteFollow(ctx context.Context, followerID, followeeID uint64) (bool, error) {
	result := r.db.WithContext(ctx).Where("follower_id = ? AND followee_id = ?", followerID, followeeID).Delete(&Follow{})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *Repository) FollowingIDs(ctx context.Context, userID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Model(&Follow{}).Where("follower_id = ?", userID).Pluck("followee_id", &ids).Error
	return ids, err
}

func (r *Repository) NoteExists(ctx context.Context, noteID uint64) bool {
	var count int64
	r.db.WithContext(ctx).Model(&note.Note{}).Where("id = ?", noteID).Count(&count)
	return count > 0
}
