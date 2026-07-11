package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/zaaalex/servys/backend/b2b"
	"github.com/zaaalex/servys/backend/crypto"
	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/recommender"
	"github.com/zaaalex/servys/backend/store"
	"github.com/zaaalex/servys/backend/vin"
)

type fakeFleet struct{ cars []domain.ClientCar }

func (f fakeFleet) Fleet(context.Context, domain.ServiceCenter) ([]domain.ClientCar, error) {
	return f.cars, nil
}

type fakeRetention struct{}

func (fakeRetention) Push(context.Context, domain.ServiceCenter, domain.ClientCar, domain.Alert) (string, error) {
	return "rid", nil
}

func newB2BRouter(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "b2b.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	c, _ := crypto.New("test-key")
	st.SetCipher(c)
	svc := &b2b.Service{
		Fleet:     fakeFleet{cars: []domain.ClientCar{{CRMContactID: 10, Make: "KIA", Model: "K3", MileageKm: 95000}}},
		Advisor:   recommender.NewStubAdvisor(),
		Retention: fakeRetention{},
		Dedupe:    st,
	}
	s := &Server{Store: st, Adv: recommender.NewStubAdvisor(), VIN: vin.NewStub(), B2B: svc}
	return s.Router()
}

func TestB2BConnectListScan(t *testing.T) {
	r := newB2BRouter(t)

	// connect
	body, _ := json.Marshal(map[string]any{"name": "СТО-1", "webhook": "https://acme.bitrix24.ru/rest/1/tok/", "responsible_id": 5})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/b2b/service-centers", bytes.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("connect code=%d body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("нет id СТО")
	}

	// list
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/v1/b2b/service-centers", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("list code=%d", w2.Code)
	}

	// scan → на демо-правилах (95000 км) будут ретеншн-дела
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest(http.MethodPost, "/api/v1/b2b/service-centers/"+id+"/scan", nil))
	if w3.Code != http.StatusOK {
		t.Fatalf("scan code=%d body=%s", w3.Code, w3.Body.String())
	}
	var rep b2b.Report
	_ = json.Unmarshal(w3.Body.Bytes(), &rep)
	if rep.Cars != 1 || rep.Pushed == 0 {
		t.Fatalf("scan report: %+v (ожидали cars=1, pushed>0)", rep)
	}
}

func TestB2BDisabledWhenNoCipher(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "b2b2.db"))
	t.Cleanup(func() { _ = st.Close() })
	s := &Server{Store: st, Adv: recommender.NewStubAdvisor(), VIN: vin.NewStub()} // B2B == nil
	r := s.Router()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/b2b/service-centers", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("без B2B ожидали 503, got %d", w.Code)
	}
}
