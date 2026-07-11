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
)

var ErrTelegramInvalid = errors.New("auth: невалидные данные Telegram")

func hmac256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// validateTelegram проверяет initData Telegram Mini App (HMAC по алгоритму Telegram)
// и возвращает id пользователя. secret = HMAC_SHA256("WebAppData", bot_token).
func validateTelegram(initData, botToken string) (string, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return "", ErrTelegramInvalid
	}
	hash := values.Get("hash")
	if hash == "" {
		return "", ErrTelegramInvalid
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
		return "", ErrTelegramInvalid
	}

	var u struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(values.Get("user")), &u); err != nil || u.ID == 0 {
		return "", ErrTelegramInvalid
	}
	return strconv.FormatInt(u.ID, 10), nil
}
