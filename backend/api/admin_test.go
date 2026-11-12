package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminAddsTelegramUser(t *testing.T) {
	s := newTestServer(t)
	s.Auth.DevLogin = true
	r := s.Router()

	plain := registerAccess(t, r, "guest")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodGet, "/api/v1/admin/users", nil, plain))
	if w.Code != http.StatusForbidden {
		t.Fatalf("чужой список: %d %s", w.Code, w.Body.String())
	}

	admin := devAdminAccess(t, r)
	body, _ := json.Marshal(map[string]string{"telegram_id": "424242", "name": "Лена"})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/admin/users", body, admin))
	if w.Code != http.StatusCreated {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	var created struct {
		AccountID  string `json:"account_id"`
		TelegramID string `json:"telegram_id"`
		Name       string `json:"name"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.AccountID == "" || created.TelegramID != "424242" || created.Name != "Лена" {
		t.Fatalf("created %+v", created)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/admin/users", body, admin))
	if w.Code != http.StatusOK {
		t.Fatalf("repeat %d %s", w.Code, w.Body.String())
	}
	var again struct {
		AccountID string `json:"account_id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &again)
	if again.AccountID != created.AccountID {
		t.Fatalf("повтор завёл другой аккаунт: %s vs %s", again.AccountID, created.AccountID)
	}

	loginBody, _ := json.Marshal(map[string]string{"init_data": telegramInitData(testBotToken, 424242)})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/telegram", bytes.NewReader(loginBody)))
	if w.Code != http.StatusOK {
		t.Fatalf("login %d %s", w.Code, w.Body.String())
	}
	var tok struct {
		Access string `json:"access_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &tok)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodGet, "/api/v1/auth/me", nil, tok.Access))
	var me struct {
		AccountID  string `json:"account_id"`
		TelegramID string `json:"telegram_id"`
		IsAdmin    bool   `json:"is_admin"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &me)
	if me.AccountID != created.AccountID || me.TelegramID != "424242" || me.IsAdmin {
		t.Fatalf("свой аккаунт: %+v", me)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/admin/users", []byte(`{"telegram_id":"abc"}`), admin))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad id %d", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodGet, "/api/v1/admin/overview", nil, admin))
	if w.Code != http.StatusOK {
		t.Fatalf("overview %d %s", w.Code, w.Body.String())
	}
	var pulse struct {
		Users          int `json:"users"`
		CheckEveryDays int `json:"check_every_days"`
		Days           []struct {
			Date string `json:"date"`
		} `json:"days"`
		Inspections []struct {
			Make      string `json:"make"`
			CheckedAt string `json:"checked_at"`
			NextAt    string `json:"next_at"`
		} `json:"inspections"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &pulse)
	if pulse.Users < 1 || len(pulse.Days) != 7 || pulse.CheckEveryDays != 7 || pulse.Inspections == nil {
		t.Fatalf("pulse %+v", pulse)
	}
}

func TestAdminListsTheRunningModels(t *testing.T) {
	t.Setenv("OLLAMA_SEARCH_MODEL", "bonsai-2:27b")
	t.Setenv("OLLAMA_MODEL", "qwen3.5:9b")
	s := newTestServer(t)
	s.Auth.DevLogin = true
	r := s.Router()

	plain := registerAccess(t, r, "guest")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodGet, "/api/v1/admin/models", nil, plain))
	if w.Code != http.StatusForbidden {
		t.Fatalf("чужие модели: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodGet, "/api/v1/admin/models", nil, devAdminAccess(t, r)))
	if w.Code != http.StatusOK {
		t.Fatalf("models %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Models []struct {
			Role string `json:"role"`
			Name string `json:"name"`
		} `json:"models"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	got := map[string]string{}
	for _, model := range body.Models {
		got[model.Role] = model.Name
	}
	if got["search"] != "bonsai-2:27b" || got["text"] != "qwen3.5:9b" || got["book"] != "PaddleOCR" || len(body.Models) != 3 {
		t.Fatalf("models %+v", body.Models)
	}
}

func devAdminAccess(t *testing.T, r http.Handler) string {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/dev", bytes.NewReader([]byte(`{"as_admin":true}`))))
	if w.Code != http.StatusOK {
		t.Fatalf("dev admin %d %s", w.Code, w.Body.String())
	}
	var tok struct {
		Access string `json:"access_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &tok)
	return tok.Access
}
