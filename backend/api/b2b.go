package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/zaaalex/servys/backend/b2b"
	"github.com/zaaalex/servys/backend/bitrix"
	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/store"
)

// b2bReady проверяет, включён ли b2b (нужен APP_SECRET_KEY для шифрования вебхуков).
func (s *Server) b2bReady(w http.ResponseWriter) bool {
	if s.B2B == nil {
		writeErr(w, http.StatusServiceUnavailable, "B2B_DISABLED", "b2b выключен: задайте APP_SECRET_KEY")
		return false
	}
	return true
}

type connectSCReq struct {
	Name          string `json:"name"`
	Webhook       string `json:"webhook"`
	ResponsibleID int    `json:"responsible_id"`
}

// connectServiceCenter подключает СТО по входящему вебхуку (без OAuth). Вебхук валидируется и шифруется.
func (s *Server) connectServiceCenter(w http.ResponseWriter, r *http.Request) {
	if !s.b2bReady(w) {
		return
	}
	var req connectSCReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if req.Name == "" || req.Webhook == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION", "name и webhook обязательны")
		return
	}
	if _, err := bitrix.NewClient(req.Webhook); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_WEBHOOK", err.Error())
		return
	}
	claims, _ := claimsFrom(r)
	sc, err := s.Store.AddServiceCenter(r.Context(), domain.ServiceCenter{
		Name: req.Name, BitrixWebhook: req.Webhook, ResponsibleID: req.ResponsibleID,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	// подключивший становится владельцем СТО (b2b-membership)
	if _, err := s.Store.AddMembership(r.Context(), domain.Membership{
		AccountID: claims.Sub, CtxType: domain.TenantB2B, TenantID: sc.ID, Role: domain.RoleOwner,
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": sc.ID, "name": sc.Name, "responsible_id": sc.ResponsibleID})
}

func (s *Server) listServiceCenters(w http.ResponseWriter, r *http.Request) {
	if !s.b2bReady(w) {
		return
	}
	claims, _ := claimsFrom(r)
	list, err := s.Store.ServiceCentersForAccount(r.Context(), claims.Sub)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, sc := range list {
		out = append(out, map[string]any{"id": sc.ID, "name": sc.Name, "responsible_id": sc.ResponsibleID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"service_centers": out})
}

// scanAllServiceCenters — разовый скан всех подключённых СТО (то же, что делает шедулер по расписанию).
func (s *Server) scanAllServiceCenters(w http.ResponseWriter, r *http.Request) {
	if !s.b2bReady(w) {
		return
	}
	sum := b2b.ScanAll(r.Context(), s.B2B, s.Store)
	writeJSON(w, http.StatusOK, sum)
}

// scanServiceCenter запускает скан автопарка СТО и создаёт ретеншн-дела в CRM (идемпотентно).
func (s *Server) scanServiceCenter(w http.ResponseWriter, r *http.Request) {
	if !s.b2bReady(w) {
		return
	}
	claims, _ := claimsFrom(r)
	id := chi.URLParam(r, "id")
	// per-СТО доступ: аккаунт должен состоять в этом СТО
	if _, found, err := s.Store.FindMembership(r.Context(), claims.Sub, domain.TenantB2B, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	} else if !found {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "нет доступа к этому СТО")
		return
	}
	sc, err := s.Store.GetServiceCenter(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "СТО не найдено")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	rep, err := s.B2B.ScanAndPush(r.Context(), sc)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "SCAN_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rep)
}
