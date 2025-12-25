package jwtx

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/lildannita/qtiva-cloud/internal/commonx"
)

var (
	// Возвращается, когда токен невалидный или подпись не совпала
	ErrInvalidToken = errors.New("invalid token")
	// Возвращается, когда токен просрочен
	ErrExpiredToken = errors.New("expired token")
)

type Config struct {
	Secret []byte
	TTL    time.Duration
}

type Claims struct {
	jwt.RegisteredClaims
	ClientID string `json:"client_id"`
	Role     string `json:"role"`
}

func LoadConfigFromEnv() (Config, error) {
	secret, err := commonx.RequireString("QTIVA_JWT_HMAC_SECRET")
	if err != nil {
		return Config{}, err
	}
	ttl, err := commonx.RequireDuration("QTIVA_JWT_TTL")
	if err != nil {
		return Config{}, err
	}
	if len(secret) < 32 {
		return Config{}, fmt.Errorf("QTIVA_JWT_HMAC_SECRET слишком короткий, нужно минимум 32 символа")
	}
	if ttl <= 0 {
		return Config{}, fmt.Errorf("QTIVA_JWT_TTL должен быть > 0")
	}
	return Config{
		Secret: []byte(secret),
		TTL:    ttl,
	}, nil
}

func Issue(cfg Config, userID, clientID, role string, now time.Time) (token string, expiresAt time.Time, err error) {
	expiresAt = now.Add(cfg.TTL)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		ClientID: clientID,
		Role:     role,
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := t.SignedString(cfg.Secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("не удалось подписать токен: %w", err)
	}
	return s, expiresAt, nil
}

func Parse(cfg Config, tokenStr string, now time.Time) (*Claims, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)

	var claims Claims
	tok, err := parser.ParseWithClaims(tokenStr, &claims, func(token *jwt.Token) (any, error) {
		return cfg.Secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !tok.Valid {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}
