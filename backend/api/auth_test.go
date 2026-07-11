package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMeRequiresToken(t *testing.T) {
	r := newFullServer(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("без токена /me → 401, got %d", w.Code)
	}
}

func TestAuthRegisterLoginMe(t *testing.T) {
	r := newFullServer(t)

	// register → login
	body, _ := json.Marshal(map[string]any{"email": "ivan@x.ru", "password": "secret1"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("register code=%d", w.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body)))
	if w2.Code != http.StatusOK {
		t.Fatalf("login code=%d body=%s", w2.Code, w2.Body.String())
	}
	var tok struct {
		Access string `json:"access_token"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &tok)

	// me → есть личный b2c-контекст
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, bearerReq(http.MethodGet, "/api/v1/auth/me", nil, tok.Access))
	if w3.Code != http.StatusOK {
		t.Fatalf("me code=%d", w3.Code)
	}
	var me struct {
		AccountID     string           `json:"account_id"`
		ActiveContext map[string]any   `json:"active_context"`
		Contexts      []map[string]any `json:"contexts"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &me)
	if me.AccountID == "" || me.ActiveContext["ctx_type"] != "b2c" || len(me.Contexts) != 1 {
		t.Fatalf("me: %+v", me)
	}
}

func TestAuthSwitchNoMembershipForbidden(t *testing.T) {
	r := newFullServer(t)
	access := registerAccess(t, r, "ivan@x.ru")
	body, _ := json.Marshal(map[string]any{"ctx_type": "b2b", "tenant_id": "sc-not-mine"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/auth/switch", body, access))
	if w.Code != http.StatusForbidden {
		t.Fatalf("switch без membership → 403, got %d body=%s", w.Code, w.Body.String())
	}
}
