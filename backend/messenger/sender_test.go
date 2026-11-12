package messenger

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHTTPSender(t *testing.T) {
	const tgToken = "123456:TELEGRAMTOKEN"
	const maxToken = "max-raw-token-XYZ"
	tests := []struct {
		name     string
		provider string
		chatID   string
		button   bool
		base     string // empty keeps the default MAX host; telegram ignores it
		status   int
		wantPath string
		wantAuth string
		wantHost string
	}{
		{name: "telegram button", provider: "telegram", chatID: "42", button: true, status: 200, wantPath: "/bot" + tgToken + "/sendMessage", wantHost: "api.telegram.org"},
		{name: "telegram text only", provider: "telegram", chatID: "room", button: false, status: 200, wantPath: "/bot" + tgToken + "/sendMessage", wantHost: "api.telegram.org"},
		{name: "telegram negative chat", provider: "telegram", chatID: "-100", button: false, status: 200, wantPath: "/bot" + tgToken + "/sendMessage", wantHost: "api.telegram.org"},
		{name: "max button", provider: "max", chatID: "77", button: true, status: 200, wantPath: "/messages", wantAuth: maxToken},
		{name: "max text only", provider: "max", chatID: "77", button: false, status: 200, wantPath: "/messages", wantAuth: maxToken},
		{name: "max default host", provider: "max", chatID: "77", button: false, status: 200, base: "default", wantPath: "/messages", wantAuth: maxToken, wantHost: "platform-api2.max.ru"},
		{name: "telegram status", provider: "telegram", chatID: "42", button: true, status: 418, wantPath: "/bot" + tgToken + "/sendMessage", wantHost: "api.telegram.org"},
		{name: "max status", provider: "max", chatID: "77", button: true, status: 502, wantPath: "/messages", wantAuth: maxToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var orig string
			var gotBody, gotAuth, gotPath, gotQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				gotBody = string(b)
				gotAuth = r.Header.Get("Authorization")
				gotPath = r.URL.Path
				gotQuery = r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, `{"description":"`+tgToken+" "+maxToken+` leaked-body"}`)
			}))
			defer srv.Close()

			cfg := Config{TelegramBotToken: tgToken, MaxBotToken: maxToken, MaxAPIBase: srv.URL + "/"}
			if tt.base == "default" {
				cfg.MaxAPIBase = ""
			}
			client := &http.Client{Transport: rewriteTransport{base: srv.URL, orig: &orig}}
			msg := Outbound{Provider: tt.provider, ChatID: tt.chatID, Text: "гараж готов"}
			if tt.button {
				msg.ButtonText = "Открыть гараж"
				msg.ButtonURL = "https://garage.example/app"
			}
			err := NewHTTPSender(cfg, client).Send(context.Background(), msg)
			if tt.status >= 200 && tt.status < 300 {
				if err != nil {
					t.Fatalf("send: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "status "+strconv.Itoa(tt.status)) {
					t.Fatalf("status error: %v", err)
				}
				if strings.Contains(err.Error(), "leaked-body") || strings.Contains(err.Error(), tgToken) || strings.Contains(err.Error(), maxToken) {
					t.Fatalf("error echoes body or token: %v", err)
				}
			}
			if gotPath != tt.wantPath {
				t.Fatalf("path %s, want %s", gotPath, tt.wantPath)
			}
			if gotAuth != tt.wantAuth {
				t.Fatalf("auth %q, want %q", gotAuth, tt.wantAuth)
			}
			if strings.Contains(gotBody, tgToken) || strings.Contains(gotBody, maxToken) {
				t.Fatalf("token leaked into json: %s", gotBody)
			}
			if tt.provider == "telegram" && tt.button && !strings.Contains(gotBody, "web_app") {
				t.Fatalf("telegram button must be a mini app: %s", gotBody)
			}
			if tt.provider != "telegram" && strings.Contains(gotBody, "web_app") {
				t.Fatalf("web_app button: %s", gotBody)
			}
			if tt.wantHost != "" && !strings.Contains(orig, "https://"+tt.wantHost) {
				t.Fatalf("original url %s, want host %s", orig, tt.wantHost)
			}
			assertPayload(t, tt.provider, tt.chatID, tt.button, gotBody, gotQuery)
		})
	}
}

func TestHTTPSenderSkipsCallWithoutToken(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	sender := NewHTTPSender(Config{MaxAPIBase: srv.URL}, srv.Client())
	err := sender.Send(context.Background(), Outbound{Provider: "max", ChatID: "1", Text: "x"})
	if !errors.Is(err, ErrNotConfigured) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
	err = sender.Send(context.Background(), Outbound{Provider: "telegram", ChatID: "1", Text: "x"})
	if !errors.Is(err, ErrNotConfigured) || called {
		t.Fatalf("telegram err=%v called=%v", err, called)
	}
	err = sender.Send(context.Background(), Outbound{Provider: "other", Text: "x"})
	if err == nil || called {
		t.Fatalf("unknown provider err=%v called=%v", err, called)
	}
}

func TestHTTPSenderUsesClientTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	client := srv.Client()
	client.Timeout = 30 * time.Millisecond
	err := NewHTTPSender(Config{MaxBotToken: "max-raw-token-XYZ", MaxAPIBase: srv.URL}, client).
		Send(context.Background(), Outbound{Provider: "max", ChatID: "1", Text: "x"})
	if err == nil || strings.Contains(err.Error(), "max-raw-token-XYZ") {
		t.Fatalf("timeout error: %v", err)
	}
}

func assertPayload(t *testing.T, provider, chatID string, button bool, body, rawQuery string) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("json: %v body %s", err, body)
	}
	if payload["text"] != "гараж готов" {
		t.Fatalf("text %#v", payload["text"])
	}
	if provider == "telegram" {
		rawChat, ok := payload["chat_id"]
		if !ok {
			t.Fatal("missing chat_id")
		}
		encoded, _ := json.Marshal(rawChat)
		if _, err := json.Number(chatID).Int64(); err == nil {
			if string(encoded) != chatID {
				t.Fatalf("chat_id %s, want number %s", encoded, chatID)
			}
		} else if rawChat != chatID {
			t.Fatalf("chat_id %#v", rawChat)
		}
		_, hasMarkup := payload["reply_markup"]
		if hasMarkup != button {
			t.Fatalf("reply_markup present=%v want %v body %s", hasMarkup, button, body)
		}
		if button {
			markup := payload["reply_markup"].(map[string]any)
			rows := markup["inline_keyboard"].([]any)
			btn := rows[0].([]any)[0].(map[string]any)
			webApp, _ := btn["web_app"].(map[string]any)
			if btn["text"] != "Открыть гараж" || webApp["url"] != "https://garage.example/app" || btn["url"] != nil {
				t.Fatalf("button %#v", btn)
			}
		}
		return
	}
	if rawQuery != "user_id="+chatID {
		t.Fatalf("query %s", rawQuery)
	}
	_, hasAttachments := payload["attachments"]
	if hasAttachments != button {
		t.Fatalf("attachments present=%v body %s", hasAttachments, body)
	}
	if !button {
		return
	}
	atts := payload["attachments"].([]any)
	att := atts[0].(map[string]any)
	if att["type"] != "inline_keyboard" {
		t.Fatalf("attachment %#v", att)
	}
	pl := att["payload"].(map[string]any)
	rows := pl["buttons"].([]any)
	btn := rows[0].([]any)[0].(map[string]any)
	if btn["type"] != "link" || btn["text"] != "Открыть гараж" || btn["url"] != "https://garage.example/app" {
		t.Fatalf("max button %#v", btn)
	}
}

type rewriteTransport struct {
	base  string
	orig  *string
	inner http.RoundTripper
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.orig != nil {
		*t.orig = req.URL.String()
	}
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(t.base, "http://")
	clone.Host = clone.URL.Host
	inner := t.inner
	if inner == nil {
		inner = http.DefaultTransport
	}
	return inner.RoundTrip(clone)
}
