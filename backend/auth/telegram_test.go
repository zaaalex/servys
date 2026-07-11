package auth

import (
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"testing"
)

// makeInitData собирает валидный initData под заданный botToken (тот же алгоритм, что проверяем).
func makeInitData(botToken, userJSON string) string {
	v := url.Values{}
	v.Set("auth_date", "1700000000")
	v.Set("user", userJSON)

	pairs := make([]string, 0, len(v))
	for k := range v {
		pairs = append(pairs, k+"="+v.Get(k))
	}
	sort.Strings(pairs)
	dataCheck := strings.Join(pairs, "\n")

	secret := hmac256([]byte("WebAppData"), []byte(botToken))
	v.Set("hash", hex.EncodeToString(hmac256(secret, []byte(dataCheck))))
	return v.Encode()
}

func TestValidateTelegram(t *testing.T) {
	const bot = "123456:BOTTOKEN"
	initData := makeInitData(bot, `{"id":777,"first_name":"Иван"}`)

	id, err := validateTelegram(initData, bot)
	if err != nil || id != "777" {
		t.Fatalf("валидный initData: id=%q err=%v", id, err)
	}
}

func TestValidateTelegramRejectsWrongToken(t *testing.T) {
	initData := makeInitData("real-bot", `{"id":1}`)
	if _, err := validateTelegram(initData, "fake-bot"); err != ErrTelegramInvalid {
		t.Fatalf("чужой токен бота должен отвергаться: %v", err)
	}
}
