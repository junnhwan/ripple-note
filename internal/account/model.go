package account

import (
	"time"

	"gorm.io/gorm"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"

	StatusActive   = "active"
	StatusDisabled = "disabled"
)

type User struct {
	ID           uint64         `gorm:"primaryKey;autoIncrement"`
	Email        string         `gorm:"size:191;not null;uniqueIndex"`
	PasswordHash string         `gorm:"size:255;not null"`
	Nickname     string         `gorm:"size:64;not null"`
	AvatarURL    string         `gorm:"size:512"`
	Bio          string         `gorm:"size:512"`
	Role         string         `gorm:"size:32;not null;index"`
	Status       string         `gorm:"size:32;not null;index"`
	CreatedAt    time.Time      `gorm:"not null"`
	UpdatedAt    time.Time      `gorm:"not null"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (User) TableName() string {
	return "users"
}
