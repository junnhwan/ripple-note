package account

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository interface {
	Create(ctx context.Context, user *User) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id uint64) (*User, error)
	UpdateProfile(ctx context.Context, id uint64, nickname, avatarURL, bio string) (*User, error)
}

type GormUserRepository struct {
	db *gorm.DB
}

func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) Create(ctx context.Context, user *User) (*User, error) {
	user.Email = NormalizeEmail(user.Email)
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (r *GormUserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("email = ?", NormalizeEmail(email)).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) FindByID(ctx context.Context, id uint64) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) UpdateProfile(ctx context.Context, id uint64, nickname, avatarURL, bio string) (*User, error) {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Updates(map[string]any{
		"nickname":   nickname,
		"avatar_url": avatarURL,
		"bio":        bio,
	})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrUserNotFound
	}
	return r.FindByID(ctx, id)
}

func (r *GormUserRepository) FindByIDs(ctx context.Context, ids []uint64) (map[uint64]*User, error) {
	if len(ids) == 0 {
		return map[uint64]*User{}, nil
	}

	uniqueIDs := make([]uint64, 0, len(ids))
	seen := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		return map[uint64]*User{}, nil
	}

	var users []*User
	if err := r.db.WithContext(ctx).Where("id IN ?", uniqueIDs).Find(&users).Error; err != nil {
		return nil, err
	}

	byID := make(map[uint64]*User, len(users))
	for _, user := range users {
		byID[user.ID] = user
	}
	return byID, nil
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
