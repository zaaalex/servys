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
}
