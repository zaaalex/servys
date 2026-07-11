// Package auth — единый вход и авторизация: аккаунт, точки входа (identities),
// контексты (memberships b2c/b2b), JWT access + серверный refresh, переключение контекста.
// Владелец: Dev 1. Логически отдельный модуль (можно вынести в свой сервис позже — JWT это позволяет).
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("auth: невалидный токен")
	ErrExpired      = errors.New("auth: токен истёк")
)

// Claims — полезная нагрузка access-токена. Несёт активный контекст (b2c|b2b + тенант + роль).
type Claims struct {
	Sub     string `json:"sub"`            // account id
	CtxType string `json:"ctx"`            // "b2c" | "b2b"
	Tenant  string `json:"tnt,omitempty"`  // id СТО для b2b
	Role    string `json:"role,omitempty"` // роль в b2b
	Iat     int64  `json:"iat"`
	Exp     int64  `json:"exp"`
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func hmacSig(body string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(body))
	return b64(h.Sum(nil))
}

// signJWT выпускает HS256-токен.
func signJWT(c Claims, secret []byte) string {
	header := b64([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(c)
	body := header + "." + b64(payload)
	return body + "." + hmacSig(body, secret)
}

// parseJWT проверяет подпись и срок, возвращает claims.
func parseJWT(token string, secret []byte, now time.Time) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	body := parts[0] + "." + parts[1]
	expected := hmacSig(body, secret)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(parts[2])) != 1 {
		return Claims{}, ErrInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if now.Unix() >= c.Exp {
		return Claims{}, ErrExpired
	}
	return c, nil
}
