package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrTelegramInvalid = errors.New("auth: невалидные данные Telegram")

func hmac256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// TelegramProfile — человек из подписанного initData.
type TelegramProfile struct {
	ID       string
	Name     string
	Username string
}

// validateTelegram проверяет initData Telegram Mini App (HMAC по алгоритму Telegram)
// и возвращает человека. secret = HMAC_SHA256("WebAppData", bot_token).
func validateTelegram(initData, botToken string, now time.Time) (TelegramProfile, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return TelegramProfile{}, ErrTelegramInvalid
	}
	hash := values.Get("hash")
	if hash == "" {
		return TelegramProfile{}, ErrTelegramInvalid
	}
	values.Del("hash")

	pairs := make([]string, 0, len(values))
	for k := range values {
		pairs = append(pairs, k+"="+values.Get(k))
	}
	sort.Strings(pairs)
	dataCheck := strings.Join(pairs, "\n")

	secret := hmac256([]byte("WebAppData"), []byte(botToken))
	computed := hex.EncodeToString(hmac256(secret, []byte(dataCheck)))
	if !hmac.Equal([]byte(computed), []byte(hash)) {
		return TelegramProfile{}, ErrTelegramInvalid
	}
	sec, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return TelegramProfile{}, ErrTelegramInvalid
	}
	authAt := time.Unix(sec, 0)
	if now.Sub(authAt) > 24*time.Hour || authAt.Sub(now) > 5*time.Minute {
		return TelegramProfile{}, ErrTelegramInvalid
	}

	var u struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
	}
	if err := json.Unmarshal([]byte(values.Get("user")), &u); err != nil || u.ID == 0 {
		return TelegramProfile{}, ErrTelegramInvalid
	}
	return TelegramProfile{
		ID:       strconv.FormatInt(u.ID, 10),
		Name:     clipRunes(strings.TrimSpace(u.FirstName+" "+u.LastName), 80),
		Username: clipRunes(strings.TrimPrefix(strings.TrimSpace(u.Username), "@"), 32),
	}, nil
}

func clipRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for k := range s {
		if i == n {
			return s[:k]
		}
		i++
	}
	return s
}
