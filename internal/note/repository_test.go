package note_test

import (
	"io"
	"log"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ripple-note/internal/note"
)

func TestRepositoryFindsTagNamesByNoteIDs(t *testing.T) {
	t.Parallel()

	db := newNoteTestDB(t)
	repo := note.NewRepository(db)
	now := time.Now()

	first := createNote(t, db, 1, "first", now)
	second := createNote(t, db, 1, "second", now)
	goTag := createTag(t, db, "go")
	redisTag := createTag(t, db, "redis")
	dockerTag := createTag(t, db, "docker")

	if err := db.Create([]*note.NoteTag{
		{NoteID: first.ID, TagID: goTag.ID},
		{NoteID: first.ID, TagID: redisTag.ID},
		{NoteID: second.ID, TagID: dockerTag.ID},
	}).Error; err != nil {
		t.Fatalf("create note tags: %v", err)
	}

	tags, err := repo.FindTagNamesByNoteIDs(t.Context(), []uint64{first.ID, second.ID})
	if err != nil {
		t.Fatalf("find tag names by note ids: %v", err)
	}
	assertStringSet(t, tags[first.ID], []string{"go", "redis"})
	assertStringSet(t, tags[second.ID], []string{"docker"})
}

func TestRepositoryFindsImagesByNoteIDs(t *testing.T) {
	t.Parallel()

	db := newNoteTestDB(t)
	repo := note.NewRepository(db)
	now := time.Now()

	first := createNote(t, db, 1, "first", now)
	second := createNote(t, db, 1, "second", now)

	if err := db.Create([]*note.NoteImage{
		{NoteID: first.ID, URL: "/uploads/images/first-b.png", StorageKey: "first-b", SortOrder: 2},
		{NoteID: first.ID, URL: "/uploads/images/first-a.png", StorageKey: "first-a", SortOrder: 1},
		{NoteID: second.ID, URL: "/uploads/images/second.png", StorageKey: "second", SortOrder: 1},
	}).Error; err != nil {
		t.Fatalf("create note images: %v", err)
	}

	images, err := repo.FindImagesByNoteIDs(t.Context(), []uint64{first.ID, second.ID})
	if err != nil {
		t.Fatalf("find images by note ids: %v", err)
	}
	if got := imageURLs(images[first.ID]); len(got) != 2 || got[0] != "/uploads/images/first-a.png" || got[1] != "/uploads/images/first-b.png" {
		t.Fatalf("expected first images in sort order, got %#v", got)
	}
	if got := imageURLs(images[second.ID]); len(got) != 1 || got[0] != "/uploads/images/second.png" {
		t.Fatalf("expected second image, got %#v", got)
	}
}

func newNoteTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.New(log.New(io.Discard, "", 0), logger.Config{}),
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	if err := db.AutoMigrate(&note.Note{}, &note.NoteImage{}, &note.Tag{}, &note.NoteTag{}); err != nil {
		t.Fatalf("auto migrate note tables: %v", err)
	}
	return db
}

func createNote(t *testing.T, db *gorm.DB, authorID uint64, title string, now time.Time) *note.Note {
	t.Helper()

	n := &note.Note{
		AuthorID:    authorID,
		Title:       title,
		Body:        title + " body",
		Status:      note.StatusPublished,
		Visibility:  note.VisibilityPublic,
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(n).Error; err != nil {
		t.Fatalf("create note %q: %v", title, err)
	}
	return n
}

func createTag(t *testing.T, db *gorm.DB, name string) note.Tag {
	t.Helper()

	tag := note.Tag{Name: name, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatalf("create tag %q: %v", name, err)
	}
	return tag
}

func imageURLs(images []*note.NoteImage) []string {
	urls := make([]string, 0, len(images))
	for _, image := range images {
		urls = append(urls, image.URL)
	}
	return urls
}

func assertStringSet(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	seen := make(map[string]bool, len(got))
	for _, value := range got {
		seen[value] = true
	}
	for _, value := range want {
		if !seen[value] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}
