package interaction_test

import (
	"io"
	"log"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ripple-note/internal/interaction"
)

func TestRepositoryFindsBatchViewerState(t *testing.T) {
	t.Parallel()

	db := newInteractionTestDB(t)
	repo := interaction.NewRepository(db)
	now := time.Now()

	if err := db.Create([]*interaction.NoteLike{
		{UserID: 1, NoteID: 10, CreatedAt: now},
		{UserID: 1, NoteID: 20, CreatedAt: now},
		{UserID: 2, NoteID: 30, CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create likes: %v", err)
	}
	if err := db.Create([]*interaction.NoteFavorite{
		{UserID: 1, NoteID: 20, CreatedAt: now},
		{UserID: 2, NoteID: 10, CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create favorites: %v", err)
	}
	if err := db.Create([]*interaction.Follow{
		{FollowerID: 1, FolloweeID: 100, CreatedAt: now},
		{FollowerID: 1, FolloweeID: 200, CreatedAt: now},
		{FollowerID: 2, FolloweeID: 300, CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create follows: %v", err)
	}

	liked, err := repo.LikedNoteIDs(t.Context(), 1, []uint64{10, 20, 30})
	if err != nil {
		t.Fatalf("liked note ids: %v", err)
	}
	if !liked[10] || !liked[20] || liked[30] {
		t.Fatalf("unexpected liked map: %#v", liked)
	}

	favorited, err := repo.FavoritedNoteIDs(t.Context(), 1, []uint64{10, 20, 30})
	if err != nil {
		t.Fatalf("favorited note ids: %v", err)
	}
	if favorited[10] || !favorited[20] || favorited[30] {
		t.Fatalf("unexpected favorited map: %#v", favorited)
	}

	following, err := repo.FollowingAuthorIDs(t.Context(), 1, []uint64{100, 200, 300})
	if err != nil {
		t.Fatalf("following author ids: %v", err)
	}
	if !following[100] || !following[200] || following[300] {
		t.Fatalf("unexpected following map: %#v", following)
	}
}

func newInteractionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.New(log.New(io.Discard, "", 0), logger.Config{}),
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	if err := db.AutoMigrate(&interaction.NoteLike{}, &interaction.NoteFavorite{}, &interaction.Follow{}); err != nil {
		t.Fatalf("auto migrate interaction tables: %v", err)
	}
	return db
}
