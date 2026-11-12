package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/zaaalex/servys/backend/auth"
)

type ctxKey int

const claimsKey ctxKey = iota

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):])
	}
	return ""
}

func claimsFrom(r *http.Request) (auth.Claims, bool) {
	c, ok := r.Context().Value(claimsKey).(auth.Claims)
	return c, ok
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Auth == nil {
			writeErr(w, http.StatusServiceUnavailable, "AUTH_DISABLED", "auth выключен: задайте JWT_SECRET")
			return
		}
		tok := bearer(r)
		if tok == "" {
			writeErr(w, http.StatusUnauthorized, "NO_TOKEN", "нужен заголовок Authorization: Bearer <token>")
			return
		}
		claims, err := s.Auth.Verify(tok)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	})
}

func (s *Server) authReady(w http.ResponseWriter) bool {
	if s.Auth == nil {
		writeErr(w, http.StatusServiceUnavailable, "AUTH_DISABLED", "auth выключен: задайте JWT_SECRET")
		return false
	}
	return true
}

func (s *Server) authDev(w http.ResponseWriter, r *http.Request) {
	if !s.authReady(w) {
		return
	}
	var req struct {
		AsAdmin bool `json:"as_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	tok, err := s.Auth.LoginDev(r.Context(), req.AsAdmin)
	if errors.Is(err, auth.ErrProviderDisabled) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "AUTH_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tok)
}

func (s *Server) authTelegram(w http.ResponseWriter, r *http.Request) {
	if !s.authReady(w) {
		return
	}
	var req struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	tok, err := s.Auth.LoginTelegram(r.Context(), req.InitData)
	switch {
	case errors.Is(err, auth.ErrProviderDisabled):
		writeErr(w, http.StatusServiceUnavailable, "TELEGRAM_DISABLED", "вход через Telegram не настроен")
	case errors.Is(err, auth.ErrTelegramInvalid):
		writeErr(w, http.StatusUnauthorized, "TELEGRAM_INVALID", "не удалось проверить данные Telegram")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "AUTH_ERROR", err.Error())
	default:
		writeJSON(w, http.StatusOK, tok)
	}
}

func (s *Server) authMax(w http.ResponseWriter, r *http.Request) {
	if !s.authReady(w) {
		return
	}
	var req struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	tok, err := s.Auth.LoginMax(r.Context(), req.InitData)
	switch {
	case errors.Is(err, auth.ErrProviderDisabled):
		writeErr(w, http.StatusServiceUnavailable, "MAX_DISABLED", "вход через MAX не настроен")
	case errors.Is(err, auth.ErrMaxInvalid):
		writeErr(w, http.StatusUnauthorized, "MAX_INVALID", "не удалось проверить данные MAX")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "AUTH_ERROR", err.Error())
	default:
		writeJSON(w, http.StatusOK, tok)
	}
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) authRefresh(w http.ResponseWriter, r *http.Request) {
	if !s.authReady(w) {
		return
	}
	var req refreshReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	tok, err := s.Auth.Refresh(r.Context(), req.RefreshToken)
	if errors.Is(err, auth.ErrInvalidRefresh) {
		writeErr(w, http.StatusUnauthorized, "INVALID_REFRESH", "refresh недействителен")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "AUTH_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tok)
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	if !s.authReady(w) {
		return
	}
	var req refreshReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	_ = s.Auth.Logout(r.Context(), req.RefreshToken)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFrom(r)
	list, err := s.Auth.Memberships(r.Context(), claims.Sub)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "AUTH_ERROR", err.Error())
		return
	}
	ctxs := make([]map[string]any, 0, len(list))
	for _, m := range list {
		ctxs = append(ctxs, map[string]any{"ctx_type": m.CtxType, "tenant_id": m.TenantID, "role": m.Role})
	}
	body := map[string]any{
		"account_id":     claims.Sub,
		"active_context": map[string]any{"ctx_type": claims.CtxType, "tenant_id": claims.Tenant, "role": claims.Role},
		"contexts":       ctxs,
		"is_admin":       claims.Role == auth.RoleAdmin,
	}
	if person, found, err := s.Store.TelegramAccountByID(r.Context(), claims.Sub); err != nil {
		writeErr(w, http.StatusInternalServerError, "AUTH_ERROR", err.Error())
		return
	} else if found {
		body["telegram_id"] = person.TelegramID
		body["name"] = person.Name
		body["username"] = person.Username
	}
	writeJSON(w, http.StatusOK, body)
}
