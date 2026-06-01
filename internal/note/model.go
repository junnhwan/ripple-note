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
	ID             uint64         `gorm:"primaryKey;autoIncrement;index:idx_notes_latest,priority:4,sort:desc;index:idx_notes_hot,priority:4,sort:desc;index:idx_notes_author_latest,priority:5,sort:desc"`
	AuthorID       uint64         `gorm:"not null;index;index:idx_notes_author_latest,priority:3"`
	Title          string         `gorm:"size:120;not null"`
	Body           string         `gorm:"type:text;not null"`
	Status         string         `gorm:"size:32;not null;index;index:idx_notes_latest,priority:1;index:idx_notes_hot,priority:1;index:idx_notes_author_latest,priority:1"`
	Visibility     string         `gorm:"size:32;not null;index:idx_notes_latest,priority:2;index:idx_notes_hot,priority:2;index:idx_notes_author_latest,priority:2"`
	ReviewTaskID   *uint64        `gorm:"index"`
	PublishedAt    *time.Time     `gorm:"index;index:idx_notes_latest,priority:3,sort:desc;index:idx_notes_author_latest,priority:4,sort:desc"`
	LikesCount     uint64         `gorm:"not null;default:0"`
	FavoritesCount uint64         `gorm:"not null;default:0"`
	CommentsCount  uint64         `gorm:"not null;default:0"`
	HotScore       float64        `gorm:"not null;default:0;index:idx_notes_hot,priority:3,sort:desc"`
	CreatedAt      time.Time      `gorm:"not null"`
	UpdatedAt      time.Time      `gorm:"not null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (Note) TableName() string { return "notes" }

type NoteImage struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	NoteID     uint64    `gorm:"not null;index;index:idx_note_images_note_order,priority:1"`
	URL        string    `gorm:"size:512;not null"`
	StorageKey string    `gorm:"size:512;not null"`
	SortOrder  int       `gorm:"not null;default:0;index:idx_note_images_note_order,priority:2"`
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
	NoteID uint64 `gorm:"primaryKey;index:idx_note_tags_note;index:idx_note_tags_tag_note,priority:2"`
	TagID  uint64 `gorm:"primaryKey;index:idx_note_tags_tag_note,priority:1"`
}

func (NoteTag) TableName() string { return "note_tags" }
