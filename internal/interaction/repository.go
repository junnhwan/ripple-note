package interaction

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"ripple-note/internal/note"
	"ripple-note/internal/outbox"
)

var ErrNoteNotFound = errors.New("note not found")

// ErrNoteNotAvailable means the note exists but is not in a published+public state.
var ErrNoteNotAvailable = errors.New("note is not available for interaction")

type Repository struct {
	db     *gorm.DB
	outbox OutboxEventCreator
}

type OutboxEventCreator interface {
	CreateEvent(ctx context.Context, tx *gorm.DB, topic, aggregateType string, aggregateID uint64, payload any) error
}

func NewRepository(db *gorm.DB, creators ...OutboxEventCreator) *Repository {
	repo := &Repository{db: db}
	if len(creators) > 0 {
		repo.outbox = creators[0]
	}
	return repo
}

func (r *Repository) DB() *gorm.DB { return r.db }

// NoteAvailable checks that a note exists, is published, and is public.
func (r *Repository) NoteAvailable(ctx context.Context, noteID uint64) error {
	var count int64
	err := r.db.WithContext(ctx).Model(&note.Note{}).
		Where("id = ? AND status = ? AND visibility = ?", noteID, note.StatusPublished, note.VisibilityPublic).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNoteNotAvailable
	}
	return nil
}

// NoteExists checks that a note exists regardless of status (used for comments listing).
func (r *Repository) NoteExists(ctx context.Context, noteID uint64) bool {
	var count int64
	r.db.WithContext(ctx).Model(&note.Note{}).Where("id = ?", noteID).Count(&count)
	return count > 0
}

func (r *Repository) UpsertLike(ctx context.Context, userID, noteID uint64) (bool, error) {
	var created bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing NoteLike
		err := tx.Unscoped().Where("user_id = ? AND note_id = ?", userID, noteID).First(&existing).Error
		if err == nil {
			if existing.DeletedAt.Valid {
				if err := tx.Unscoped().Model(&existing).Update("deleted_at", nil).Error; err != nil {
					return err
				}
				if err := tx.Model(&note.Note{}).Where("id = ?", noteID).
					Update("likes_count", gorm.Expr("likes_count + 1")).Error; err != nil {
					return err
				}
				if err := r.createInteractionEvent(ctx, tx, outbox.TopicInteractionCreated, "note", noteID, map[string]any{
					"note_id": noteID,
					"user_id": userID,
					"action":  "like",
				}); err != nil {
					return err
				}
				created = true
				return nil
			}
			created = false
			return nil // already liked
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		like := &NoteLike{UserID: userID, NoteID: noteID}
		if err := tx.Create(like).Error; err != nil {
			return err
		}
		if err := tx.Model(&note.Note{}).Where("id = ?", noteID).
			Update("likes_count", gorm.Expr("likes_count + 1")).Error; err != nil {
			return err
		}
		if err := r.createInteractionEvent(ctx, tx, outbox.TopicInteractionCreated, "note", noteID, map[string]any{
			"note_id": noteID,
			"user_id": userID,
			"action":  "like",
		}); err != nil {
			return err
		}
		created = true
		return nil
	})
	return created, err
}

func (r *Repository) DeleteLike(ctx context.Context, userID, noteID uint64) (bool, error) {
	var removed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("user_id = ? AND note_id = ?", userID, noteID).Delete(&NoteLike{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			removed = false
			return nil
		}
		removed = true
		if err := tx.Model(&note.Note{}).Where("id = ? AND likes_count > 0", noteID).
			Update("likes_count", gorm.Expr("likes_count - 1")).Error; err != nil {
			return err
		}
		return r.createInteractionEvent(ctx, tx, outbox.TopicInteractionRemoved, "note", noteID, map[string]any{
			"note_id": noteID,
			"user_id": userID,
			"action":  "unlike",
		})
	})
	return removed, err
}

func (r *Repository) UpsertFavorite(ctx context.Context, userID, noteID uint64) (bool, error) {
	var created bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing NoteFavorite
		err := tx.Unscoped().Where("user_id = ? AND note_id = ?", userID, noteID).First(&existing).Error
		if err == nil {
			if existing.DeletedAt.Valid {
				if err := tx.Unscoped().Model(&existing).Update("deleted_at", nil).Error; err != nil {
					return err
				}
				if err := tx.Model(&note.Note{}).Where("id = ?", noteID).
					Update("favorites_count", gorm.Expr("favorites_count + 1")).Error; err != nil {
					return err
				}
				if err := r.createInteractionEvent(ctx, tx, outbox.TopicInteractionCreated, "note", noteID, map[string]any{
					"note_id": noteID,
					"user_id": userID,
					"action":  "favorite",
				}); err != nil {
					return err
				}
				created = true
				return nil
			}
			created = false
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		fav := &NoteFavorite{UserID: userID, NoteID: noteID}
		if err := tx.Create(fav).Error; err != nil {
			return err
		}
		if err := tx.Model(&note.Note{}).Where("id = ?", noteID).
			Update("favorites_count", gorm.Expr("favorites_count + 1")).Error; err != nil {
			return err
		}
		if err := r.createInteractionEvent(ctx, tx, outbox.TopicInteractionCreated, "note", noteID, map[string]any{
			"note_id": noteID,
			"user_id": userID,
			"action":  "favorite",
		}); err != nil {
			return err
		}
		created = true
		return nil
	})
	return created, err
}

func (r *Repository) DeleteFavorite(ctx context.Context, userID, noteID uint64) (bool, error) {
	var removed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("user_id = ? AND note_id = ?", userID, noteID).Delete(&NoteFavorite{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			removed = false
			return nil
		}
		removed = true
		if err := tx.Model(&note.Note{}).Where("id = ? AND favorites_count > 0", noteID).
			Update("favorites_count", gorm.Expr("favorites_count - 1")).Error; err != nil {
			return err
		}
		return r.createInteractionEvent(ctx, tx, outbox.TopicInteractionRemoved, "note", noteID, map[string]any{
			"note_id": noteID,
			"user_id": userID,
			"action":  "unfavorite",
		})
	})
	return removed, err
}

func (r *Repository) CreateComment(ctx context.Context, comment *Comment) (*Comment, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comment).Error; err != nil {
			return err
		}
		if err := tx.Model(&note.Note{}).Where("id = ?", comment.NoteID).
			Update("comments_count", gorm.Expr("comments_count + 1")).Error; err != nil {
			return err
		}
		return r.createInteractionEvent(ctx, tx, outbox.TopicInteractionCreated, "comment", comment.ID, map[string]any{
			"note_id":    comment.NoteID,
			"user_id":    comment.AuthorID,
			"comment_id": comment.ID,
			"action":     "comment",
		})
	})
	if err != nil {
		return nil, err
	}
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
	var created bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing Follow
		err := tx.Where("follower_id = ? AND followee_id = ?", followerID, followeeID).First(&existing).Error
		if err == nil {
			created = false
			return nil // already following
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(&Follow{FollowerID: followerID, FolloweeID: followeeID}).Error; err != nil {
			return err
		}
		created = true
		return r.createInteractionEvent(ctx, tx, outbox.TopicInteractionCreated, "follow", followeeID, map[string]any{
			"user_id":     followerID,
			"follower_id": followerID,
			"followee_id": followeeID,
			"action":      "follow",
		})
	})
	return created, err
}

func (r *Repository) DeleteFollow(ctx context.Context, followerID, followeeID uint64) (bool, error) {
	var removed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("follower_id = ? AND followee_id = ?", followerID, followeeID).Delete(&Follow{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			removed = false
			return nil
		}
		removed = true
		return r.createInteractionEvent(ctx, tx, outbox.TopicInteractionRemoved, "follow", followeeID, map[string]any{
			"user_id":     followerID,
			"follower_id": followerID,
			"followee_id": followeeID,
			"action":      "unfollow",
		})
	})
	return removed, err
}

func (r *Repository) FollowingIDs(ctx context.Context, userID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Model(&Follow{}).Where("follower_id = ?", userID).Pluck("followee_id", &ids).Error
	return ids, err
}

// HasLiked checks if a user has liked a specific note.
func (r *Repository) HasLiked(ctx context.Context, userID, noteID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&NoteLike{}).Where("user_id = ? AND note_id = ?", userID, noteID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) LikedNoteIDs(ctx context.Context, userID uint64, noteIDs []uint64) (map[uint64]bool, error) {
	if userID == 0 || len(noteIDs) == 0 {
		return map[uint64]bool{}, nil
	}

	var ids []uint64
	err := r.db.WithContext(ctx).
		Model(&NoteLike{}).
		Where("user_id = ? AND note_id IN ?", userID, uniqueUint64s(noteIDs)).
		Pluck("note_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return uint64Set(ids), nil
}

// HasFavorited checks if a user has favorited a specific note.
func (r *Repository) HasFavorited(ctx context.Context, userID, noteID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&NoteFavorite{}).Where("user_id = ? AND note_id = ?", userID, noteID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) FavoritedNoteIDs(ctx context.Context, userID uint64, noteIDs []uint64) (map[uint64]bool, error) {
	if userID == 0 || len(noteIDs) == 0 {
		return map[uint64]bool{}, nil
	}

	var ids []uint64
	err := r.db.WithContext(ctx).
		Model(&NoteFavorite{}).
		Where("user_id = ? AND note_id IN ?", userID, uniqueUint64s(noteIDs)).
		Pluck("note_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return uint64Set(ids), nil
}

// IsFollowing checks if a user is following another user.
func (r *Repository) IsFollowing(ctx context.Context, followerID, followeeID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Follow{}).Where("follower_id = ? AND followee_id = ?", followerID, followeeID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) FollowingAuthorIDs(ctx context.Context, followerID uint64, authorIDs []uint64) (map[uint64]bool, error) {
	if followerID == 0 || len(authorIDs) == 0 {
		return map[uint64]bool{}, nil
	}

	var ids []uint64
	err := r.db.WithContext(ctx).
		Model(&Follow{}).
		Where("follower_id = ? AND followee_id IN ?", followerID, uniqueUint64s(authorIDs)).
		Pluck("followee_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return uint64Set(ids), nil
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

func uint64Set(ids []uint64) map[uint64]bool {
	set := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func (r *Repository) createInteractionEvent(ctx context.Context, tx *gorm.DB, topic, aggregateType string, aggregateID uint64, payload any) error {
	if r.outbox == nil {
		return nil
	}
	return r.outbox.CreateEvent(ctx, tx, topic, aggregateType, aggregateID, payload)
}
