package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var ErrPasswordMismatch = errors.New("password mismatch")

type PasswordHasher interface {
	Hash(rawPassword string) (string, error)
	Compare(passwordHash string, rawPassword string) error
}

type BcryptPasswordHasher struct {
	cost int
}

func NewBcryptPasswordHasher() BcryptPasswordHasher {
	return BcryptPasswordHasher{cost: bcrypt.DefaultCost}
}

func (h BcryptPasswordHasher) Hash(rawPassword string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), h.cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (h BcryptPasswordHasher) Compare(passwordHash string, rawPassword string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(rawPassword)); err != nil {
		return ErrPasswordMismatch
	}
	return nil
}
