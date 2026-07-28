package userauth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/user"
)

type TokenConfig struct {
	Secret []byte
	Issuer string
	TTL    time.Duration
}

type Claims struct {
	AppID string `json:"app_id"`
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func issueToken(cfg TokenConfig, u user.User) (string, int64, error) {
	if len(cfg.Secret) == 0 {
		return "", 0, fmt.Errorf("jwt secret is required")
	}
	if cfg.TTL <= 0 {
		return "", 0, fmt.Errorf("jwt ttl must be positive")
	}

	now := time.Now().UTC()
	expiresAt := now.Add(cfg.TTL)
	claims := Claims{
		AppID: u.AppID.String(),
		Email: u.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			Issuer:    cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(cfg.Secret)
	if err != nil {
		return "", 0, fmt.Errorf("sign jwt: %w", err)
	}
	return signed, int64(cfg.TTL.Seconds()), nil
}

func ParseToken(cfg TokenConfig, raw string) (Claims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return cfg.Secret, nil
	}, jwt.WithIssuer(cfg.Issuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	if err != nil {
		return Claims{}, err
	}
	if !token.Valid {
		return Claims{}, fmt.Errorf("invalid token")
	}
	return claims, nil
}
