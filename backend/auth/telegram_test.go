package auth

import (
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
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

	got, err := validateTelegram(initData, bot, time.Unix(1700000000, 0))
	if err != nil || got.ID != "777" || got.Name != "Иван" {
		t.Fatalf("валидный initData: %+v err=%v", got, err)
	}
}

func TestValidateTelegramRejectsStaleAuthDate(t *testing.T) {
	initData := makeInitData("real-bot", `{"id":1}`)
	if _, err := validateTelegram(initData, "real-bot", time.Unix(1700000000, 0).Add(25*time.Hour)); err != ErrTelegramInvalid {
		t.Fatalf("старый auth_date должен отвергаться: %v", err)
	}
}

func TestValidateTelegramRejectsWrongToken(t *testing.T) {
	initData := makeInitData("real-bot", `{"id":1}`)
	if _, err := validateTelegram(initData, "fake-bot", time.Unix(1700000000, 0)); err != ErrTelegramInvalid {
		t.Fatalf("чужой токен бота должен отвергаться: %v", err)
	}
}
