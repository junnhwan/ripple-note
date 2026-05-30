package note

import (
	"time"

	"gorm.io/gorm"
)

const (
	StatusPendingReview = "pending_review"
	StatusPublished     = "published"
	StatusRejected      = "rejected"
	StatusRemoved       = "removed"

	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

type Note struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement"`
	AuthorID       uint64         `gorm:"not null;index"`
	Title          string         `gorm:"size:120;not null"`
	Body           string         `gorm:"type:text;not null"`
	Status         string         `gorm:"size:32;not null;index"`
	Visibility     string         `gorm:"size:32;not null"`
	ReviewTaskID   *uint64        `gorm:"index"`
	PublishedAt    *time.Time     `gorm:"index"`
	LikesCount     uint64         `gorm:"not null;default:0"`
	FavoritesCount uint64         `gorm:"not null;default:0"`
	CommentsCount  uint64         `gorm:"not null;default:0"`
	HotScore       float64        `gorm:"not null;default:0"`
	CreatedAt      time.Time      `gorm:"not null"`
	UpdatedAt      time.Time      `gorm:"not null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (Note) TableName() string { return "notes" }

type NoteImage struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	NoteID     uint64    `gorm:"not null;index"`
	URL        string    `gorm:"size:512;not null"`
	StorageKey string    `gorm:"size:512;not null"`
	SortOrder  int       `gorm:"not null;default:0"`
	Width      *int      `gorm:""`
	Height     *int      `gorm:""`
	CreatedAt  time.Time `gorm:"not null"`
}

func (NoteImage) TableName() string { return "note_images" }

type Tag struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	Name       string    `gorm:"size:64;not null;uniqueIndex"`
	NotesCount uint64    `gorm:"not null;default:0"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (Tag) TableName() string { return "tags" }

type NoteTag struct {
	NoteID uint64 `gorm:"primaryKey"`
	TagID  uint64 `gorm:"primaryKey"`
}

func (NoteTag) TableName() string { return "note_tags" }
