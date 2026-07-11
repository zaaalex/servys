package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/zaaalex/servys/backend/auth"
	"github.com/zaaalex/servys/backend/domain"
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

// requireAuth проверяет Bearer access-токен и кладёт claims в контекст запроса.
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

// requireAdmin гейтит операторские действия платформы по X-Admin-Token.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.AdminToken == "" {
			writeErr(w, http.StatusServiceUnavailable, "ADMIN_DISABLED", "операторские действия выключены: задайте ADMIN_TOKEN")
			return
		}
		if subtleNe(r.Header.Get("X-Admin-Token"), s.AdminToken) {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", "нужен корректный X-Admin-Token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func subtleNe(a, b string) bool { return len(a) != len(b) || a != b }

func (s *Server) authReady(w http.ResponseWriter) bool {
	if s.Auth == nil {
		writeErr(w, http.StatusServiceUnavailable, "AUTH_DISABLED", "auth выключен: задайте JWT_SECRET")
		return false
	}
	return true
}

type credsReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) authRegister(w http.ResponseWriter, r *http.Request) {
	if !s.authReady(w) {
		return
	}
	var req credsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	tok, err := s.Auth.Register(r.Context(), req.Email, req.Password)
	switch {
	case errors.Is(err, auth.ErrEmailTaken):
		writeErr(w, http.StatusConflict, "EMAIL_TAKEN", "email уже зарегистрирован")
	case errors.Is(err, auth.ErrValidation):
		writeErr(w, http.StatusBadRequest, "VALIDATION", "email обязателен, пароль ≥ 6 символов")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "AUTH_ERROR", err.Error())
	default:
		writeJSON(w, http.StatusCreated, tok)
	}
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	if !s.authReady(w) {
		return
	}
	var req credsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	tok, err := s.Auth.Login(r.Context(), req.Email, req.Password)
	if errors.Is(err, auth.ErrBadCredentials) {
		writeErr(w, http.StatusUnauthorized, "BAD_CREDENTIALS", "неверный email или пароль")
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
	writeJSON(w, http.StatusOK, map[string]any{
		"account_id":     claims.Sub,
		"active_context": map[string]any{"ctx_type": claims.CtxType, "tenant_id": claims.Tenant, "role": claims.Role},
		"contexts":       ctxs,
	})
}

func (s *Server) authSwitch(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFrom(r)
	var req struct {
		CtxType  string `json:"ctx_type"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if req.CtxType != string(domain.TenantB2C) && req.CtxType != string(domain.TenantB2B) {
		writeErr(w, http.StatusBadRequest, "VALIDATION", "ctx_type должен быть b2c или b2b")
		return
	}
	access, err := s.Auth.Switch(r.Context(), claims.Sub, domain.TenantType(req.CtxType), req.TenantID)
	if errors.Is(err, auth.ErrNoMembership) {
		writeErr(w, http.StatusForbidden, "NO_MEMBERSHIP", "нет доступа к этому контексту")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "AUTH_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": access})
}
