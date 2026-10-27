package messenger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const defaultMaxAPI = "https://platform-api2.max.ru"

// Sender отправляет одно исходящее сообщение. Реализацию даёт NewHTTPSender.
type Sender interface {
	Send(ctx context.Context, msg Outbound) error
}

type httpSender struct {
	cfg    Config
	client *http.Client
}

// NewHTTPSender ходит в Bot API Telegram и platform API MAX.
// Таймаут берётся у client. nil client — http.DefaultClient.
func NewHTTPSender(cfg Config, client *http.Client) Sender {
	if client == nil {
		client = http.DefaultClient
	}
	return &httpSender{cfg: cfg, client: client}
}

func (s *httpSender) Send(ctx context.Context, msg Outbound) error {
	switch msg.Provider {
	case providerTelegram:
		return s.sendTelegram(ctx, msg)
	case providerMax:
		return s.sendMax(ctx, msg)
	default:
		return errors.New("messenger: unknown provider")
	}
}

func (s *httpSender) sendTelegram(ctx context.Context, msg Outbound) error {
	token := s.cfg.TelegramBotToken
	if token == "" {
		return ErrNotConfigured
	}
	payload := map[string]any{
		"chat_id": chatJSON(msg.ChatID),
		"text":    msg.Text,
	}
	if msg.ButtonURL != "" {
		payload["reply_markup"] = map[string]any{
			"inline_keyboard": []any{
				[]any{map[string]string{"text": msg.ButtonText, "url": msg.ButtonURL}},
			},
		}
	}
	endpoint := "https://api.telegram.org/bot" + token + "/sendMessage"
	return s.post(ctx, endpoint, token, "", payload)
}

func (s *httpSender) sendMax(ctx context.Context, msg Outbound) error {
	token := s.cfg.MaxBotToken
	if token == "" {
		return ErrNotConfigured
	}
	base := s.cfg.MaxAPIBase
	if base == "" {
		base = defaultMaxAPI
	}
	base = strings.TrimRight(base, "/")
	endpoint := base + "/messages?user_id=" + url.QueryEscape(msg.ChatID)
	payload := map[string]any{"text": msg.Text}
	if msg.ButtonURL != "" {
		payload["attachments"] = []any{
			map[string]any{
				"type": "inline_keyboard",
				"payload": map[string]any{
					"buttons": []any{
						[]any{map[string]string{
							"type": "link",
							"text": msg.ButtonText,
							"url":  msg.ButtonURL,
						}},
					},
				},
			},
		}
	}
	return s.post(ctx, endpoint, token, token, payload)
}

func chatJSON(id string) any {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return id
	}
	return n
}

func (s *httpSender) post(ctx context.Context, endpoint, redactToken, auth string, payload any) error {
	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return redactSendErr(err, redactToken)
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return redactSendErr(err, redactToken)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("messenger: send failed: status %d", resp.StatusCode)
	}
	return nil
}

func redactSendErr(err error, secret string) error {
	msg := err.Error()
	if secret != "" {
		msg = strings.ReplaceAll(msg, secret, "[redacted]")
		if enc := url.PathEscape(secret); enc != secret {
			msg = strings.ReplaceAll(msg, enc, "[redacted]")
		}
	}
	return errors.New("messenger: send failed: " + msg)
}
