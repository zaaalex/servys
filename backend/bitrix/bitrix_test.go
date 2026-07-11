package bitrix

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/sink"
)

func TestValidateWebhook(t *testing.T) {
	ok := []string{
		"https://acme.bitrix24.ru/rest/1/abc123def/",
		"https://acme.bitrix24.ru/rest/12/tok",
	}
	bad := []string{
		"http://acme.bitrix24.ru/rest/1/tok/",      // не https
		"https://acme.bitrix24.ru/webhook",         // не /rest/
		"https://1.2.3.4/rest/1/tok/",              // IP
		"https://acme.bitrix24.ru/rest/1/tok/?a=1", // query
		"https://u:p@acme.bitrix24.ru/rest/1/tok/", // userinfo
		"https://localhost/rest/1/tok/",            // localhost
	}
	for _, u := range ok {
		if err := validateWebhook(u); err != nil {
			t.Errorf("valid %q => err %v", u, err)
		}
	}
	for _, u := range bad {
		if err := validateWebhook(u); err == nil {
			t.Errorf("invalid %q принят как валидный", u)
		}
	}
}

// fakeBitrix — тестовый REST-эндпоинт Bitrix.
func fakeBitrix(t *testing.T, body string, code int) (*Client, *string) {
	t.Helper()
	var gotPath string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	// в обход NewClient (он запрещает IP-хост тестового сервера) — проверяем транспорт отдельно
	c := &Client{base: srv.URL + "/rest/1/tok/", http: srv.Client()}
	return c, &gotPath
}

func TestSinkDeliverSendsTask(t *testing.T) {
	c, gotPath := fakeBitrix(t, `{"result":{"task":{"id":"777"}}}`, http.StatusOK)
	s := &Sink{client: c, responsibleID: 5}

	err := s.Deliver(context.Background(), sink.Reminder{
		Vehicle: domain.Vehicle{Make: "KIA", Model: "K3", CurrentOdometer: 95000},
		Alert:   domain.Alert{Title: "Моторное масло", Description: "пора менять", DueAtKm: 90000},
	})
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if !strings.HasSuffix(*gotPath, "/tasks.task.add") {
		t.Fatalf("ожидали вызов tasks.task.add, path=%s", *gotPath)
	}
}

func TestClientAddTaskError(t *testing.T) {
	c, _ := fakeBitrix(t, `{"error":"NOT_FOUND","error_description":"нет прав"}`, http.StatusBadRequest)
	if _, err := c.AddTask(context.Background(), map[string]any{"TITLE": "x"}); err == nil {
		t.Fatal("ожидали ошибку от Bitrix")
	}
}

func TestRedactHidesToken(t *testing.T) {
	got := redact("dial https://p/rest/1/SECRETTOK/tasks.task.add failed", "https://p/rest/1/SECRETTOK/")
	if strings.Contains(got, "SECRETTOK") {
		t.Fatalf("токен не замаскирован: %s", got)
	}
}

// TestLive_SendTask — реальный прогон на портале. Запуск:
//
//	BITRIX_WEBHOOK_URL='https://<портал>/rest/<id>/<token>/' go test ./bitrix/ -run Live -v
func TestLive_SendTask(t *testing.T) {
	webhook := os.Getenv("BITRIX_WEBHOOK_URL")
	if webhook == "" {
		t.Skip("установи BITRIX_WEBHOOK_URL для прогона на реальном портале")
	}
	s, err := NewSink(webhook, 0)
	if err != nil {
		t.Fatalf("NewSink: %v", err)
	}
	err = s.Deliver(context.Background(), sink.Reminder{
		Vehicle: domain.Vehicle{Make: "servys", Model: "тест", CurrentOdometer: 95000},
		Alert:   domain.Alert{Title: "тестовое уведомление", Description: "проверка коннектора servys", DueAtKm: 100000},
	})
	if err != nil {
		t.Fatalf("live Deliver: %v", err)
	}
	t.Log("задача создана — проверь портал Bitrix24")
}
