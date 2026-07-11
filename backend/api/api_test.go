package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/zaaalex/servys/backend/recommender"
	"github.com/zaaalex/servys/backend/store"
	"github.com/zaaalex/servys/backend/vin"
)

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	s := &Server{Store: st, Adv: recommender.NewStubAdvisor(), VIN: vin.NewStub()}
	return s.Router()
}

func TestHealth(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("health code=%d", w.Code)
	}
}

func TestVehiclesRequireClientID(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/vehicles", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("без X-Client-ID ожидали 401, got=%d", w.Code)
	}
}

func TestCreateVehicleAndAlertsFlow(t *testing.T) {
	r := newTestRouter(t)

	body, _ := json.Marshal(map[string]any{"make": "KIA", "model": "K3", "year": 2020, "mileage_km": 95000})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vehicles", bytes.NewReader(body))
	req.Header.Set("X-Client-ID", "client-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create vehicle code=%d body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("нет id созданного авто")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/vehicles/"+id+"/alerts", nil)
	req2.Header.Set("X-Client-ID", "client-1")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("alerts code=%d body=%s", w2.Code, w2.Body.String())
	}
	var resp struct {
		Alerts []map[string]any `json:"alerts"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp)
	if len(resp.Alerts) == 0 {
		t.Fatalf("при 95000 км ожидали alerts, got 0; body=%s", w2.Body.String())
	}

	// service-event по этому авто
	seBody, _ := json.Marshal(map[string]any{"rule_code": "engine_oil", "odometer": 90000})
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/vehicles/"+id+"/service-events", bytes.NewReader(seBody))
	req3.Header.Set("X-Client-ID", "client-1")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusCreated {
		t.Fatalf("service-event code=%d body=%s", w3.Code, w3.Body.String())
	}

	// журнал ТО должен содержать созданное событие
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/vehicles/"+id+"/service-events", nil)
	req4.Header.Set("X-Client-ID", "client-1")
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("list service-events code=%d", w4.Code)
	}
	var seResp struct {
		ServiceEvents []map[string]any `json:"service_events"`
	}
	_ = json.Unmarshal(w4.Body.Bytes(), &seResp)
	if len(seResp.ServiceEvents) != 1 {
		t.Fatalf("ожидали 1 событие в журнале, got %d", len(seResp.ServiceEvents))
	}
}

func TestServiceEventRequiresRuleCode(t *testing.T) {
	r := newTestRouter(t)
	body, _ := json.Marshal(map[string]any{"make": "KIA", "model": "K3", "mileage_km": 1000})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vehicles", bytes.NewReader(body))
	req.Header.Set("X-Client-ID", "c2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)

	empty, _ := json.Marshal(map[string]any{"odometer": 1000})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/vehicles/"+id+"/service-events", bytes.NewReader(empty))
	req2.Header.Set("X-Client-ID", "c2")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("без rule_code ожидали 400, got=%d", w2.Code)
	}
}
