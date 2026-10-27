// Package messenger — вебхуки Telegram и MAX и решение, отвечать ли на /start.
// Handle* в сеть не ходят: отправку делает Sender.
package messenger

import (
	"bytes"
	"crypto/hmac"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	startText        = "Это servys. Здесь ваша машина, история обслуживания и что пора сделать."
	startButtonText  = "Открыть гараж"
	providerTelegram = "telegram"
	providerMax      = "max"
)

// Config — токены и секреты провайдеров. Пустой токен выключает его вебхук (404).
type Config struct {
	TelegramBotToken      string
	TelegramWebhookSecret string
	MaxBotToken           string
	MaxWebhookSecret      string
	MaxAPIBase            string // пусто значит https://platform-api2.max.ru
	MiniAppURL            string
}

// Outbound — одно исходящее сообщение. Его отправляет Sender, не Handle*.
type Outbound struct {
	Provider   string // "telegram" или "max"
	ChatID     string
	Text       string
	ButtonText string
	ButtonURL  string
}

// Outcome — статус HTTP для вебхука и сообщение, если на апдейт надо ответить.
type Outcome struct {
	Status int
	Send   *Outbound // nil, если отправлять нечего
}

// HandleTelegram разбирает апдейт Bot API. Секрет — заголовок X-Telegram-Bot-Api-Secret-Token.
func HandleTelegram(cfg Config, secretHeader string, body []byte) Outcome {
	if cfg.TelegramBotToken == "" {
		return Outcome{Status: http.StatusNotFound}
	}
	if !secretMatch(cfg.TelegramWebhookSecret, secretHeader) {
		return Outcome{Status: http.StatusForbidden}
	}
	obj, ok := decodeObject(body)
	if !ok {
		return Outcome{Status: http.StatusBadRequest}
	}
	raw := obj["message"]
	if len(raw) == 0 || string(raw) == "null" {
		return Outcome{Status: http.StatusOK}
	}
	var msg struct {
		Text string `json:"text"`
		Chat struct {
			ID json.RawMessage `json:"id"`
		} `json:"chat"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil || !isTelegramStart(msg.Text) {
		return Outcome{Status: http.StatusOK}
	}
	chatID, ok := jsonIntDecimal(msg.Chat.ID)
	if !ok {
		return Outcome{Status: http.StatusOK}
	}
	return startOutcome(providerTelegram, chatID, cfg)
}

// HandleMax разбирает апдейт MAX. Секрет — заголовок X-Max-Bot-Api-Secret.
func HandleMax(cfg Config, secretHeader string, body []byte) Outcome {
	if cfg.MaxBotToken == "" {
		return Outcome{Status: http.StatusNotFound}
	}
	if !secretMatch(cfg.MaxWebhookSecret, secretHeader) {
		return Outcome{Status: http.StatusForbidden}
	}
	obj, ok := decodeObject(body)
	if !ok {
		return Outcome{Status: http.StatusBadRequest}
	}
	kind, _ := jsonString(obj["update_type"])
	switch kind {
	case "bot_started":
		chatID, ok := objectUserID(obj["user"])
		if !ok {
			return Outcome{Status: http.StatusOK}
		}
		return startOutcome(providerMax, chatID, cfg)
	case "message_created":
		text, chatID, ok := maxMessage(obj["message"])
		if !ok || chatID == "" || !isMaxStartText(text) {
			return Outcome{Status: http.StatusOK}
		}
		return startOutcome(providerMax, chatID, cfg)
	default:
		return Outcome{Status: http.StatusOK}
	}
}

// secretMatch — пустой секрет не совпадает даже с пустым заголовком.
func secretMatch(want, got string) bool {
	if want == "" || got == "" {
		return false
	}
	return hmac.Equal([]byte(want), []byte(got))
}

func startOutcome(provider, chatID string, cfg Config) Outcome {
	msg := &Outbound{Provider: provider, ChatID: chatID, Text: startText}
	if cfg.MiniAppURL != "" {
		msg.ButtonText = startButtonText
		msg.ButtonURL = cfg.MiniAppURL
	}
	return Outcome{Status: http.StatusOK, Send: msg}
}

func isTelegramStart(text string) bool {
	text = strings.TrimSpace(text)
	return text == "/start" || strings.HasPrefix(text, "/start ") || strings.HasPrefix(text, "/start@")
}

func isMaxStartText(text string) bool {
	text = strings.TrimSpace(text)
	return text == "/start" || strings.HasPrefix(text, "/start ")
}

func decodeObject(body []byte) (map[string]json.RawMessage, bool) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return nil, false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, false
	}
	return obj, true
}

func jsonString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

func jsonIntDecimal(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] == '"' {
		return "", false
	}
	n, err := json.Number(raw).Int64()
	if err != nil {
		return "", false
	}
	return strconv.FormatInt(n, 10), true
}

func objectUserID(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var user struct {
		UserID json.RawMessage `json:"user_id"`
	}
	if err := json.Unmarshal(raw, &user); err != nil {
		return "", false
	}
	return jsonIntDecimal(user.UserID)
}

func maxMessage(raw json.RawMessage) (text, userID string, ok bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", "", false
	}
	var msg struct {
		Sender struct {
			UserID json.RawMessage `json:"user_id"`
		} `json:"sender"`
		Text string `json:"text"`
		Body *struct {
			Text *string `json:"text"`
		} `json:"body"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return "", "", false
	}
	if msg.Body != nil && msg.Body.Text != nil {
		text = *msg.Body.Text
	} else {
		text = msg.Text
	}
	id, idOK := jsonIntDecimal(msg.Sender.UserID)
	if !idOK {
		return text, "", true
	}
	return text, id, true
}
