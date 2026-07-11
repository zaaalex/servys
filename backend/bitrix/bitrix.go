// Package bitrix — коннектор к Bitrix24 через ВХОДЯЩИЙ вебхук (без OAuth).
// Владелец: Dev 1 (интеграция за портом sink.Sink). Используется на этапе b2b;
// в b2c-сборке не подключается (там sink.Noop).
package bitrix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ErrInvalidWebhook — вебхук не прошёл проверки безопасности (ADR-001 §10.1).
var ErrInvalidWebhook = errors.New("invalid bitrix webhook url")

var webhookPath = regexp.MustCompile(`^/rest/\d+/[A-Za-z0-9]+/?$`)

// validateWebhook проверяет URL входящего вебхука: https, структура /rest/{id}/{token}/,
// без userinfo/query/fragment, host — не IP и не localhost (защита от SSRF).
func validateWebhook(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("%w: не парсится", ErrInvalidWebhook)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: требуется https", ErrInvalidWebhook)
	}
	if u.User != nil {
		return fmt.Errorf("%w: userinfo запрещён", ErrInvalidWebhook)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("%w: query/fragment запрещены", ErrInvalidWebhook)
	}
	if !webhookPath.MatchString(u.Path) {
		return fmt.Errorf("%w: путь должен быть /rest/{user_id}/{token}/", ErrInvalidWebhook)
	}
	host := u.Hostname()
	if host == "" || host == "localhost" {
		return fmt.Errorf("%w: недопустимый host", ErrInvalidWebhook)
	}
	if net.ParseIP(host) != nil {
		return fmt.Errorf("%w: IP-адрес запрещён", ErrInvalidWebhook)
	}
	return nil
}

// Client вызывает методы Bitrix REST через вебхук.
type Client struct {
	base string // https://portal/rest/{id}/{token}/
	http *http.Client
}

// NewClient валидирует вебхук и создаёт клиент с таймаутом.
func NewClient(webhook string) (*Client, error) {
	if err := validateWebhook(webhook); err != nil {
		return nil, err
	}
	base := strings.TrimRight(strings.TrimSpace(webhook), "/") + "/"
	return &Client{base: base, http: &http.Client{Timeout: 5 * time.Second}}, nil
}

type apiResponse struct {
	Result           json.RawMessage `json:"result"`
	Error            string          `json:"error"`
	ErrorDescription string          `json:"error_description"`
}

// call POST-ит метод REST с JSON-параметрами и разбирает ответ Bitrix.
func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if params == nil {
		params = map[string]any{}
	}
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+method, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitrix %s: %s", method, redact(err.Error(), c.base))
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // не читаем гигантские тела
	var ar apiResponse
	if err := json.Unmarshal(raw, &ar); err != nil {
		return nil, fmt.Errorf("bitrix %s: неожиданный ответ (http %d)", method, resp.StatusCode)
	}
	if ar.Error != "" {
		return nil, fmt.Errorf("bitrix %s: %s (%s)", method, ar.Error, ar.ErrorDescription)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("bitrix %s: http %d", method, resp.StatusCode)
	}
	return ar.Result, nil
}

// Profile проверяет валидность вебхука/прав (Bitrix метод profile).
func (c *Client) Profile(ctx context.Context) error {
	_, err := c.call(ctx, "profile", nil)
	return err
}

// AddTask создаёт задачу (tasks.task.add) и возвращает её id.
func (c *Client) AddTask(ctx context.Context, fields map[string]any) (int64, error) {
	raw, err := c.call(ctx, "tasks.task.add", map[string]any{"fields": fields})
	if err != nil {
		return 0, err
	}
	var r struct {
		Task struct {
			ID json.Number `json:"id"`
		} `json:"task"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return 0, nil // задача создана, id не распарсили — не критично
	}
	id, _ := r.Task.ID.Int64()
	return id, nil
}

// redact вырезает токен вебхука из строки (для логов/ошибок).
func redact(s, base string) string {
	if tok := tokenOf(base); tok != "" {
		return strings.ReplaceAll(s, tok, "***")
	}
	return s
}

func tokenOf(base string) string {
	parts := strings.Split(strings.Trim(base, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
