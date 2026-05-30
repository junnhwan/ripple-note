package interaction

import (
	"time"

	"gorm.io/gorm"
)

type NoteLike struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement"`
	UserID    uint64         `gorm:"not null;uniqueIndex:idx_user_note_like"`
	NoteID    uint64         `gorm:"not null;uniqueIndex:idx_user_note_like"`
	CreatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"uniqueIndex:idx_user_note_like"`
}

func (NoteLike) TableName() string { return "note_likes" }

type NoteFavorite struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement"`
	UserID    uint64         `gorm:"not null;uniqueIndex:idx_user_note_fav"`
	NoteID    uint64         `gorm:"not null;uniqueIndex:idx_user_note_fav"`
	CreatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"uniqueIndex:idx_user_note_fav"`
}

func (NoteFavorite) TableName() string { return "note_favorites" }

type Comment struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement"`
	NoteID    uint64         `gorm:"not null;index"`
	AuthorID  uint64         `gorm:"not null;index"`
	ParentID  *uint64        `gorm:"index"`
	Body      string         `gorm:"size:1000;not null"`
	Status    string         `gorm:"size:32;not null;default:visible"`
	CreatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (Comment) TableName() string { return "comments" }

const (
	CommentStatusVisible = "visible"
	CommentStatusRemoved = "removed"
)

type Follow struct {
	FollowerID uint64    `gorm:"not null;uniqueIndex:idx_follow"`
	FolloweeID uint64    `gorm:"not null;uniqueIndex:idx_follow"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (Follow) TableName() string { return "follows" }
