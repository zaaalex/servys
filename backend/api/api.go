// Package api — HTTP-слой (Dev 1): chi-роутер, хендлеры, CORS.
// Реализует контракт §4.A спеки (модель vehicles/alerts).
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/zaaalex/servys/backend/b2b"
	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/recommender"
	"github.com/zaaalex/servys/backend/store"
	"github.com/zaaalex/servys/backend/vin"
)

type Server struct {
	Store *store.Store
	Adv   recommender.Advisor // шов с рекомендательным слоем (Dev 3)
	VIN   vin.VINProvider
	B2B   *b2b.Service // b2b-оркестратор; nil => b2b выключен (нет APP_SECRET_KEY)
}

// Router собирает все маршруты и middleware.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Get("/api/v1/health", s.health)
	r.Get("/api/v1/me", s.me)
	r.Post("/api/v1/vin/resolve", s.resolveVIN)
	r.Route("/api/v1/vehicles", func(r chi.Router) {
		r.Get("/", s.listVehicles)
		r.Post("/", s.createVehicle)
		r.Get("/{id}", s.getVehicle)
		r.Patch("/{id}/odometer", s.patchOdometer)
		r.Post("/{id}/service-events", s.createServiceEvent)
		r.Get("/{id}/service-events", s.listServiceEvents)
		r.Get("/{id}/alerts", s.getAlerts)
	})
	r.Route("/api/v1/b2b", func(r chi.Router) {
		r.Post("/scan-all", s.scanAllServiceCenters)
		r.Route("/service-centers", func(r chi.Router) {
			r.Post("/", s.connectServiceCenter)
			r.Get("/", s.listServiceCenters)
			r.Post("/{id}/scan", s.scanServiceCenter)
		})
	})
	return r
}

// --- middleware ---

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Client-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientID достаёт browser-token; пусто => 401.
func clientID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.Header.Get("X-Client-ID")
	if id == "" {
		writeErr(w, http.StatusUnauthorized, "MISSING_CLIENT_ID", "нужен заголовок X-Client-ID")
		return "", false
	}
	return id, true
}

// --- handlers ---

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	key, ok := clientID(w, r)
	if !ok {
		return
	}
	u, err := s.Store.EnsureUser(r.Context(), key)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": u.ID, "tenant_type": u.TenantType})
}

type vehicleReq struct {
	VIN      string `json:"vin"`
	Make     string `json:"make"`
	Model    string `json:"model"`
	Year     int    `json:"year"`
	EngineCC int    `json:"engine_cc"`
	PowerHP  int    `json:"power_hp"`
	Mileage  int    `json:"mileage_km"`
}

func (s *Server) createVehicle(w http.ResponseWriter, r *http.Request) {
	key, ok := clientID(w, r)
	if !ok {
		return
	}
	u, err := s.Store.EnsureUser(r.Context(), key)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	var req vehicleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if req.Make == "" || req.Model == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION", "make и model обязательны")
		return
	}
	src := "manual"
	if req.VIN != "" {
		src = "vin"
	}
	v, err := s.Store.AddVehicle(r.Context(), domain.Vehicle{
		UserID: u.ID, VIN: req.VIN, Make: req.Make, Model: req.Model, Year: req.Year,
		EngineCC: req.EngineCC, PowerHP: req.PowerHP, IdentificationSource: src, CurrentOdometer: req.Mileage,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, vehicleJSON(v))
}

func (s *Server) listVehicles(w http.ResponseWriter, r *http.Request) {
	key, ok := clientID(w, r)
	if !ok {
		return
	}
	u, err := s.Store.EnsureUser(r.Context(), key)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	list, err := s.Store.ListVehicles(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, v := range list {
		out = append(out, vehicleJSON(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"vehicles": out})
}

func (s *Server) getVehicle(w http.ResponseWriter, r *http.Request) {
	v, ok := s.loadVehicle(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, vehicleJSON(v))
}

type odometerReq struct {
	MileageKm int `json:"mileage_km"`
}

func (s *Server) patchOdometer(w http.ResponseWriter, r *http.Request) {
	key, ok := clientID(w, r)
	if !ok {
		return
	}
	u, err := s.Store.EnsureUser(r.Context(), key)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	var req odometerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	v, err := s.Store.UpdateOdometer(r.Context(), u.ID, chi.URLParam(r, "id"), req.MileageKm)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "авто не найдено")
	case errors.Is(err, store.ErrOdometerDecrease):
		writeErr(w, http.StatusConflict, "ODOMETER_DECREASE_NOT_ALLOWED", "пробег нельзя уменьшать")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
	default:
		writeJSON(w, http.StatusOK, vehicleJSON(v))
	}
}

func (s *Server) getAlerts(w http.ResponseWriter, r *http.Request) {
	v, ok := s.loadVehicle(w, r)
	if !ok {
		return
	}
	history, err := s.Store.ListServiceEvents(r.Context(), v.UserID, v.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	alerts, err := s.Adv.Alerts(r.Context(), v, history)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "ADVISOR_ERROR", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(alerts))
	for _, a := range alerts {
		out = append(out, alertJSON(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"vehicle": vehicleJSON(v),
		"alerts":  out,
	})
}

type serviceEventReq struct {
	RuleCode string `json:"rule_code"`
	Odometer int    `json:"odometer"`
}

func (s *Server) createServiceEvent(w http.ResponseWriter, r *http.Request) {
	v, ok := s.loadVehicle(w, r)
	if !ok {
		return
	}
	var req serviceEventReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	if req.RuleCode == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION", "rule_code обязателен")
		return
	}
	ev, err := s.Store.AddServiceEvent(r.Context(), v.UserID, v.ID,
		domain.ServiceEvent{RuleCode: req.RuleCode, Odometer: req.Odometer})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, serviceEventJSON(ev))
}

func (s *Server) listServiceEvents(w http.ResponseWriter, r *http.Request) {
	v, ok := s.loadVehicle(w, r)
	if !ok {
		return
	}
	events, err := s.Store.ListServiceEvents(r.Context(), v.UserID, v.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		out = append(out, serviceEventJSON(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"service_events": out})
}

func serviceEventJSON(e domain.ServiceEvent) map[string]any {
	return map[string]any{
		"id": e.ID, "rule_code": e.RuleCode, "odometer": e.Odometer, "performed_at": e.PerformedAt,
	}
}

type vinReq struct {
	VIN string `json:"vin"`
}

func (s *Server) resolveVIN(w http.ResponseWriter, r *http.Request) {
	var req vinReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	v, err := s.VIN.Resolve(r.Context(), req.VIN)
	if err != nil {
		// best-effort: любая ошибка => фронт открывает ручную форму
		writeErr(w, http.StatusFailedDependency, err.Error(), "автоопределение недоступно, заполните вручную")
		return
	}
	writeJSON(w, http.StatusOK, vehicleJSON(v))
}

// --- helpers ---

func (s *Server) loadVehicle(w http.ResponseWriter, r *http.Request) (domain.Vehicle, bool) {
	key, ok := clientID(w, r)
	if !ok {
		return domain.Vehicle{}, false
	}
	u, err := s.Store.EnsureUser(r.Context(), key)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return domain.Vehicle{}, false
	}
	v, err := s.Store.GetVehicle(r.Context(), u.ID, chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "авто не найдено")
		return domain.Vehicle{}, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return domain.Vehicle{}, false
	}
	return v, true
}

func vehicleJSON(v domain.Vehicle) map[string]any {
	return map[string]any{
		"id": v.ID, "vin": v.VIN, "make": v.Make, "model": v.Model, "year": v.Year,
		"engine_cc": v.EngineCC, "power_hp": v.PowerHP,
		"identification_source": v.IdentificationSource,
		"mileage_km":            v.CurrentOdometer,
		"odometer_updated_at":   v.OdometerUpdatedAt,
	}
}

func alertJSON(a domain.Alert) map[string]any {
	return map[string]any{
		"id": a.ID, "rule_code": a.RuleCode, "type": a.Type,
		"severity": a.Severity, "title": a.Title, "description": a.Description, "due_at_km": a.DueAtKm,
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]any{"error": map[string]string{"code": errCode, "message": msg}})
}
