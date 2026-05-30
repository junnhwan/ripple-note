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
