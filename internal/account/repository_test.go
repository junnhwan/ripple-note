package account_test

import (
	"io"
	"log"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ripple-note/internal/account"
)

func TestGormUserRepositoryCreatesAndFindsUserByEmail(t *testing.T) {
	t.Parallel()

	db := newAccountTestDB(t)
	repo := account.NewGormUserRepository(db)

	created, err := repo.Create(t.Context(), &account.User{
		Email:        "User@Example.com",
		PasswordHash: "hash",
		Nickname:     "User",
		Role:         account.RoleUser,
		Status:       account.StatusActive,
	})
	if err != nil {
		t.Fatalf("expected create user to succeed, got error: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected created user id to be set")
	}

	found, err := repo.FindByEmail(t.Context(), "user@example.com")
	if err != nil {
		t.Fatalf("expected find user by normalized email to succeed, got error: %v", err)
	}
	if found.ID != created.ID {
		t.Fatalf("expected found user id %d, got %d", created.ID, found.ID)
	}
}

func TestGormUserRepositoryRejectsDuplicateEmail(t *testing.T) {
	t.Parallel()

	db := newAccountTestDB(t)
	repo := account.NewGormUserRepository(db)

	firstUser := &account.User{
		Email:        "duplicate@example.com",
		PasswordHash: "hash",
		Nickname:     "User",
		Role:         account.RoleUser,
		Status:       account.StatusActive,
	}
	if _, err := repo.Create(t.Context(), firstUser); err != nil {
		t.Fatalf("expected first create to succeed, got error: %v", err)
	}
	secondUser := &account.User{
		Email:        "duplicate@example.com",
		PasswordHash: "hash",
		Nickname:     "Another User",
		Role:         account.RoleUser,
		Status:       account.StatusActive,
	}
	if _, err := repo.Create(t.Context(), secondUser); err == nil {
		t.Fatal("expected duplicate email to fail")
	}
}

func TestGormUserRepositoryFindsUsersByIDs(t *testing.T) {
	t.Parallel()

	db := newAccountTestDB(t)
	repo := account.NewGormUserRepository(db)

	first, err := repo.Create(t.Context(), &account.User{
		Email:        "first@example.com",
		PasswordHash: "hash",
		Nickname:     "First",
		Role:         account.RoleUser,
		Status:       account.StatusActive,
	})
	if err != nil {
		t.Fatalf("create first user: %v", err)
	}
	second, err := repo.Create(t.Context(), &account.User{
		Email:        "second@example.com",
		PasswordHash: "hash",
		Nickname:     "Second",
		Role:         account.RoleUser,
		Status:       account.StatusActive,
	})
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}

	users, err := repo.FindByIDs(t.Context(), []uint64{first.ID, second.ID, first.ID})
	if err != nil {
		t.Fatalf("find users by ids: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 unique users, got %d", len(users))
	}
	if users[first.ID].Nickname != "First" {
		t.Fatalf("expected first nickname to be First, got %q", users[first.ID].Nickname)
	}
	if users[second.ID].Nickname != "Second" {
		t.Fatalf("expected second nickname to be Second, got %q", users[second.ID].Nickname)
	}
}

func newAccountTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.New(log.New(io.Discard, "", 0), logger.Config{}),
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	if err := db.AutoMigrate(&account.User{}); err != nil {
		t.Fatalf("auto migrate users: %v", err)
	}
	return db
}
