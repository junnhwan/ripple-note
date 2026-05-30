package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const defaultJWTIssuer = "ripple-note"

var ErrInvalidToken = errors.New("invalid token")

type JWTConfig struct {
	Secret string
	Issuer string
	TTL    time.Duration
}

type UserClaims struct {
	UserID uint64
	Role   string
}

type JWTManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

type jwtClaims struct {
	UserID uint64 `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func NewJWTManager(cfg JWTConfig) *JWTManager {
	issuer := cfg.Issuer
	if issuer == "" {
		issuer = defaultJWTIssuer
	}
	ttl := cfg.TTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}

	return &JWTManager{
		secret: []byte(cfg.Secret),
		issuer: issuer,
		ttl:    ttl,
	}
}

func (m *JWTManager) Issue(claims UserClaims) (string, error) {
	if len(m.secret) == 0 {
		return "", errors.New("jwt secret is required")
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		UserID: claims.UserID,
		Role:   claims.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("%d", claims.UserID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	})

	return token.SignedString(m.secret)
}

func (m *JWTManager) Parse(tokenString string) (UserClaims, error) {
	if len(m.secret) == 0 {
		return UserClaims{}, errors.New("jwt secret is required")
	}

	parsed, err := jwt.ParseWithClaims(tokenString, &jwtClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer))
	if err != nil {
		return UserClaims{}, ErrInvalidToken
	}

	claims, ok := parsed.Claims.(*jwtClaims)
	if !ok || !parsed.Valid || claims.UserID == 0 {
		return UserClaims{}, ErrInvalidToken
	}

	return UserClaims{
		UserID: claims.UserID,
		Role:   claims.Role,
	}, nil
}
