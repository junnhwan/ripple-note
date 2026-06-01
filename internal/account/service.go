package account

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"ripple-note/internal/auth"
)

var (
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInvalidInput           = errors.New("invalid input")
	ErrUserDisabled           = errors.New("user disabled")
)

type RegisterInput struct {
	Email    string
	Password string
	Nickname string
}

type LoginInput struct {
	Email    string
	Password string
}

type UpdateProfileInput struct {
	Nickname  *string
	AvatarURL *string
	Bio       *string
}

type Service struct {
	users  UserRepository
	hashes auth.PasswordHasher
	tokens *auth.JWTManager
}

func NewService(users UserRepository, hashes auth.PasswordHasher, tokens *auth.JWTManager) *Service {
	return &Service{
		users:  users,
		hashes: hashes,
		tokens: tokens,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (UserDTO, error) {
	email, err := validateEmail(input.Email)
	if err != nil {
		return UserDTO{}, err
	}
	if err := validatePassword(input.Password); err != nil {
		return UserDTO{}, err
	}

	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return UserDTO{}, ErrEmailAlreadyRegistered
	} else if !errors.Is(err, ErrUserNotFound) {
		return UserDTO{}, err
	}

	passwordHash, err := s.hashes.Hash(input.Password)
	if err != nil {
		return UserDTO{}, fmt.Errorf("hash password: %w", err)
	}

	nickname := strings.TrimSpace(input.Nickname)
	if nickname == "" {
		nickname = strings.Split(email, "@")[0]
	}

	user, err := s.users.Create(ctx, &User{
		Email:        email,
		PasswordHash: passwordHash,
		Nickname:     nickname,
		Role:         RoleUser,
		Status:       StatusActive,
	})
	if err != nil {
		return UserDTO{}, err
	}

	return ToUserDTO(user), nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (SessionDTO, error) {
	email, err := validateEmail(input.Email)
	if err != nil {
		return SessionDTO{}, ErrInvalidCredentials
	}

	user, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		return SessionDTO{}, ErrInvalidCredentials
	}
	if err != nil {
		return SessionDTO{}, err
	}
	if user.Status != StatusActive {
		return SessionDTO{}, ErrUserDisabled
	}
	if err := s.hashes.Compare(user.PasswordHash, input.Password); err != nil {
		return SessionDTO{}, ErrInvalidCredentials
	}

	token, err := s.tokens.Issue(auth.UserClaims{
		UserID: user.ID,
		Role:   user.Role,
	})
	if err != nil {
		return SessionDTO{}, err
	}

	return SessionDTO{
		Token: token,
		User:  ToUserDTO(user),
	}, nil
}

func (s *Service) CurrentUser(ctx context.Context, userID uint64) (UserDTO, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return UserDTO{}, err
	}
	if user.Status != StatusActive {
		return UserDTO{}, ErrUserDisabled
	}
	return ToUserDTO(user), nil
}

func (s *Service) PublicProfile(ctx context.Context, userID uint64) (PublicUserDTO, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return PublicUserDTO{}, err
	}
	if user.Status != StatusActive {
		return PublicUserDTO{}, ErrUserNotFound
	}
	return ToPublicUserDTO(user), nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID uint64, input UpdateProfileInput) (UserDTO, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return UserDTO{}, err
	}
	if user.Status != StatusActive {
		return UserDTO{}, ErrUserDisabled
	}

	nickname := user.Nickname
	if input.Nickname != nil {
		nickname = strings.TrimSpace(*input.Nickname)
		if nickname == "" {
			return UserDTO{}, fmt.Errorf("%w: nickname is required", ErrInvalidInput)
		}
		if len([]rune(nickname)) > 64 {
			return UserDTO{}, fmt.Errorf("%w: nickname must be at most 64 characters", ErrInvalidInput)
		}
	}

	avatarURL := user.AvatarURL
	if input.AvatarURL != nil {
		avatarURL = strings.TrimSpace(*input.AvatarURL)
		if len(avatarURL) > 512 {
			return UserDTO{}, fmt.Errorf("%w: avatar url must be at most 512 characters", ErrInvalidInput)
		}
	}

	bio := user.Bio
	if input.Bio != nil {
		bio = strings.TrimSpace(*input.Bio)
		if len([]rune(bio)) > 512 {
			return UserDTO{}, fmt.Errorf("%w: bio must be at most 512 characters", ErrInvalidInput)
		}
	}

	updated, err := s.users.UpdateProfile(ctx, userID, nickname, avatarURL, bio)
	if err != nil {
		return UserDTO{}, err
	}
	if updated.Status != StatusActive {
		return UserDTO{}, ErrUserDisabled
	}
	return ToUserDTO(updated), nil
}

func validateEmail(value string) (string, error) {
	email := NormalizeEmail(value)
	if email == "" {
		return "", fmt.Errorf("%w: email is required", ErrInvalidInput)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "", fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	return email, nil
}

func validatePassword(value string) error {
	if len(value) < 6 {
		return fmt.Errorf("%w: password must be at least 6 characters", ErrInvalidInput)
	}
	return nil
}
