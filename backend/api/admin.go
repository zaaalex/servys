package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zaaalex/servys/backend/auth"
	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/store"
)

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFrom(r)
		if !ok || claims.Role != auth.RoleAdmin {
			writeErr(w, http.StatusForbidden, "NOT_ADMIN", "этот раздел только для администратора")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type adminUserJSON struct {
	AccountID  string `json:"account_id"`
	TelegramID string `json:"telegram_id"`
	Name       string `json:"name"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	CreatedAt  string `json:"created_at"`
}

func adminUserFrom(p store.TelegramAccount) adminUserJSON {
	return adminUserJSON{
		AccountID: p.AccountID, TelegramID: p.TelegramID, Name: p.Name,
		Username: p.Username, Role: p.Role, CreatedAt: p.CreatedAt,
	}
}

func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	users, cars, days, err := s.Store.AdminPulse(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "ADMIN_ERROR", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(days))
	var usersToday, carsToday int
	for i, d := range days {
		out = append(out, map[string]any{"date": d.Date, "users": d.Users, "cars": d.Cars})
		if i == len(days)-1 {
			usersToday, carsToday = d.Users, d.Cars
		}
	}
	inspections, every, checksToday, err := s.inspectionBoard(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "ADMIN_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": users, "cars": cars, "days": out,
		"users_today": usersToday, "cars_today": carsToday,
		"checks_today": checksToday, "check_every_days": everyDays(every),
		"inspections": inspections,
	})
}

func everyDays(every time.Duration) int {
	days := int(every / (24 * time.Hour))
	if days < 1 {
		return 1
	}
	return days
}

func (s *Server) inspectionBoard(ctx context.Context) ([]map[string]any, time.Duration, int, error) {
	list, err := s.Store.ListAllVehicles(ctx)
	if err != nil {
		return nil, 0, 0, err
	}
	every := 7 * 24 * time.Hour
	today := time.Now().UTC().Format("2006-01-02")
	out := make([]map[string]any, 0, len(list))
	checksToday := 0
	for _, v := range list {
		at, found := time.Time{}, false
		if s.Checks != nil {
			every, at, found = s.Checks.Inspect(v)
		}
		out = append(out, inspectionJSON(v, every, at, found))
		if found && at.UTC().Format("2006-01-02") == today {
			checksToday++
		}
	}
	return out, every, checksToday, nil
}

func inspectionJSON(v domain.Vehicle, every time.Duration, at time.Time, found bool) map[string]any {
	item := map[string]any{
		"vehicle_id": v.ID,
		"make":       v.Make,
		"model":      v.Model,
		"year":       v.Year,
		"vin":        v.VIN,
		"mileage_km": v.CurrentOdometer,
		"checked_at": "",
		"next_at":    "",
	}
	if found {
		item["checked_at"] = at.UTC().Format(time.RFC3339)
		item["next_at"] = at.UTC().Add(every).UTC().Format(time.RFC3339)
	}
	return item
}

func (s *Server) adminModels(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(s.SearchModel)
	if search == "" {
		search = "bonsai-2:27b"
	}
	text := strings.TrimSpace(s.TextModel)
	if text == "" {
		text = "qwen3.5:9b"
	}
	models := []map[string]string{
		{"role": "search", "name": search},
		{"role": "text", "name": text},
	}
	if s.Scan != nil {
		models = append(models, map[string]string{"role": "book", "name": "PaddleOCR"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (s *Server) listAdminUsers(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListTelegramAccounts(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "ADMIN_ERROR", err.Error())
		return
	}
	out := make([]adminUserJSON, 0, len(list))
	for _, p := range list {
		out = append(out, adminUserFrom(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (s *Server) createAdminUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TelegramID string `json:"telegram_id"`
		Name       string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	id, ok := normalizeTelegramID(req.TelegramID)
	if !ok {
		writeErr(w, http.StatusBadRequest, "BAD_TELEGRAM_ID", "нужен числовой номер Telegram")
		return
	}
	name := strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(name) > 80 {
		writeErr(w, http.StatusBadRequest, "BAD_NAME", "имя длиннее 80 знаков")
		return
	}
	person, created, err := s.Store.EnsureTelegramAccount(r.Context(), id, name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "ADMIN_ERROR", err.Error())
		return
	}
	code := http.StatusOK
	if created {
		code = http.StatusCreated
	}
	writeJSON(w, code, adminUserFrom(person))
}

func normalizeTelegramID(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 18 {
		return "", false
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return "", false
	}
	return strconv.FormatInt(n, 10), true
}
