package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMeRequiresToken(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("без токена /me → 401, got %d", w.Code)
	}
}

func TestDevLoginHiddenUnlessEnabled(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/dev", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("dev login code=%d", w.Code)
	}
}

func TestDevLoginOpensSameGarage(t *testing.T) {
	s := newTestServer(t)
	s.Auth.DevLogin = true
	r := s.Router()
	access := devAccess(t, r)
	again := devAccess(t, r)
	me := func(tok string) string {
		t.Helper()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, bearerReq(http.MethodGet, "/api/v1/auth/me", nil, tok))
		if w.Code != http.StatusOK {
			t.Fatalf("me code=%d", w.Code)
		}
		var body struct {
			AccountID string `json:"account_id"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return body.AccountID
	}
	if me(access) == "" || me(access) != me(again) {
		t.Fatal("локальный вход должен открывать один и тот же гараж")
	}
}

func devAccess(t *testing.T, r http.Handler) string {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/dev", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("dev login code=%d body=%s", w.Code, w.Body.String())
	}
	var tok struct {
		Access string `json:"access_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &tok)
	if tok.Access == "" {
		t.Fatal("нет access-токена")
	}
	return tok.Access
}

func TestPasswordLoginRemoved(t *testing.T) {
	r := newTestRouter(t)
	body := []byte(`{"email":"ivan@x.ru","password":"secret1"}`)
	for _, path := range []string{"/api/v1/auth/register", "/api/v1/auth/login"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s code=%d", path, w.Code)
		}
	}
}

func TestAuthTelegramMe(t *testing.T) {
	r := newTestRouter(t)
	access := registerAccess(t, r, "ivan")

	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, bearerReq(http.MethodGet, "/api/v1/auth/me", nil, access))
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
