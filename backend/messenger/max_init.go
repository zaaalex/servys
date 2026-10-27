package messenger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrNotConfigured — токен бота пуст, вход или отправка выключены.
	ErrNotConfigured = errors.New("messenger: not configured")
	// ErrInvalidInitData — initData MAX не прошла проверку. Текст ошибки не содержит само initData.
	ErrInvalidInitData = errors.New("messenger: invalid init data")
)

// ValidateMaxInitData проверяет initData мини-приложения MAX и возвращает user.id.
// Подпись сверяется раньше auth_date и user.
func ValidateMaxInitData(initData, botToken string, now time.Time) (string, error) {
	if botToken == "" {
		return "", ErrNotConfigured
	}
	if strings.TrimSpace(initData) == "" {
		return "", initErr("empty")
	}
	values, err := parseInitPairs(initData)
	if err != nil {
		return "", err
	}
	hash, ok := values["hash"]
	if !ok || hash == "" {
		return "", initErr("hash missing")
	}
	delete(values, "hash")
	if !signaturesMatch(dataCheckString(values), botToken, hash) {
		return "", initErr("hash mismatch")
	}
	sec, ok := parseAuthUnix(values["auth_date"])
	if !ok {
		return "", initErr("auth_date invalid")
	}
	authAt := time.Unix(sec, 0)
	if now.Sub(authAt) > 24*time.Hour {
		return "", initErr("auth_date expired")
	}
	if authAt.Sub(now) > 5*time.Minute {
		return "", initErr("auth_date from the future")
	}
	return userIDFromJSON(values["user"])
}

func initErr(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInitData, reason)
}

func parseInitPairs(initData string) (map[string]string, error) {
	values := make(map[string]string)
	seenRaw := make(map[string]struct{})
	for _, part := range strings.Split(initData, "&") {
		if part == "" || !strings.Contains(part, "=") {
			continue
		}
		rawKey, rawVal, _ := strings.Cut(part, "=")
		if rawKey == "" {
			return nil, initErr("malformed")
		}
		if _, ok := seenRaw[rawKey]; ok {
			return nil, initErr("duplicate key")
		}
		seenRaw[rawKey] = struct{}{}
		key, err := url.QueryUnescape(rawKey)
		if err != nil || key == "" {
			return nil, initErr("malformed")
		}
		val, err := url.QueryUnescape(rawVal)
		if err != nil {
			return nil, initErr("malformed")
		}
		if _, ok := values[key]; ok {
			return nil, initErr("duplicate key")
		}
		values[key] = val
	}
	return values, nil
}

func dataCheckString(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, key := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(values[key])
	}
	return b.String()
}

func signaturesMatch(dataCheck, token, hash string) bool {
	secret := hmacSHA256([]byte("WebAppData"), []byte(token))
	computed := hmacSHA256(secret, []byte(dataCheck))
	got, err := hex.DecodeString(hash)
	if err != nil {
		got = make([]byte, len(computed))
	}
	match := hmac.Equal(computed, got)
	return err == nil && match
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func parseAuthUnix(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func userIDFromJSON(raw string) (string, error) {
	if raw == "" {
		return "", initErr("user invalid")
	}
	var u struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal([]byte(raw), &u); err != nil || len(u.ID) == 0 {
		return "", initErr("user invalid")
	}
	id, ok := jsonIntDecimal(u.ID)
	if !ok || id == "0" {
		return "", initErr("user invalid")
	}
	return id, nil
}
