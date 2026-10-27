package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zaaalex/servys/backend/messenger"
)

func jsonUnmarshal(t *testing.T, raw []byte, dst any) error {
	t.Helper()
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatal(err)
	}
	return nil
}

type captureSender struct{ got []messenger.Outbound }

func (c *captureSender) Send(_ context.Context, msg messenger.Outbound) error {
	c.got = append(c.got, msg)
	return nil
}

func TestTelegramWebhookRequiresConfiguredBot(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telegram/webhook", strings.NewReader(`{}`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestTelegramStartSendsGarageButton(t *testing.T) {
	st := newTestServer(t)
	sent := &captureSender{}
	st.Messenger = messenger.Config{
		TelegramBotToken:      "token",
		TelegramWebhookSecret: "secret",
		MiniAppURL:            "https://app.example/garage",
	}
	st.Send = sent
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telegram/webhook", strings.NewReader(`{"message":{"text":"/start","chat":{"id":42}}}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret")
	st.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if len(sent.got) != 1 || sent.got[0].ChatID != "42" || sent.got[0].ButtonURL != "https://app.example/garage" {
		t.Fatalf("%+v", sent.got)
	}
}

func TestHistoryTextFindsOilChange(t *testing.T) {
	r := newTestRouter(t)
	access := registerAccess(t, r, "hist@x.ru")
	body := []byte(`{"make":"KIA","model":"K3","year":2020,"mileage_km":50000}`)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/vehicles", body, access))
	if w.Code != http.StatusCreated {
		t.Fatalf("vehicle code=%d body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = jsonUnmarshal(t, w.Body.Bytes(), &created)
	id, _ := created["id"].(string)

	w = httptest.NewRecorder()
	note := []byte(`{"text":"12.03.2024 замена масла, пробег 45 000 км"}`)
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/vehicles/"+id+"/history", note, access))
	if w.Code != http.StatusCreated {
		t.Fatalf("history code=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Suggestions []struct {
			RuleCode string `json:"rule_code"`
			Odometer int    `json:"odometer"`
		} `json:"suggestions"`
	}
	_ = jsonUnmarshal(t, w.Body.Bytes(), &resp)
	if len(resp.Suggestions) != 1 || resp.Suggestions[0].RuleCode != "engine_oil" || resp.Suggestions[0].Odometer != 45000 {
		t.Fatalf("%+v", resp.Suggestions)
	}
}
