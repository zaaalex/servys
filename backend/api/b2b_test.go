package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/zaaalex/servys/backend/auth"
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

func (fakeRetention) Push(context.Context, domain.ServiceCenter, domain.ClientCar, []domain.Alert) (string, error) {
	return "rid", nil
}

func newFullServer(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "full.db"))
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
	s := &Server{
		Store: st, Adv: recommender.NewStubAdvisor(), VIN: vin.NewStub(),
		B2B: svc, Auth: auth.New(st, []byte("jwt-secret")), AdminToken: "admintok",
	}
	return s.Router()
}

func registerAccess(t *testing.T, r http.Handler, email string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"email": email, "password": "secret1"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("register code=%d body=%s", w.Code, w.Body.String())
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

func bearerReq(method, path string, body []byte, tok string) *http.Request {
	var rd *bytes.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return req
}

func TestB2BFlowWithAuth(t *testing.T) {
	r := newFullServer(t)
	access := registerAccess(t, r, "sto@x.ru")

	// connect (владелец создаётся автоматически)
	body, _ := json.Marshal(map[string]any{"name": "СТО-1", "webhook": "https://acme.bitrix24.ru/rest/1/tok/", "responsible_id": 5})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/b2b/service-centers", body, access))
	if w.Code != http.StatusCreated {
		t.Fatalf("connect code=%d body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)

	// list — только свои
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, bearerReq(http.MethodGet, "/api/v1/b2b/service-centers", nil, access))
	if w2.Code != http.StatusOK {
		t.Fatalf("list code=%d", w2.Code)
	}
	var lst struct {
		ServiceCenters []map[string]any `json:"service_centers"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &lst)
	if len(lst.ServiceCenters) != 1 {
		t.Fatalf("ожидали 1 свой СТО, got %d", len(lst.ServiceCenters))
	}

	// scan своего СТО (membership owner есть) → есть ретеншн-дела
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, bearerReq(http.MethodPost, "/api/v1/b2b/service-centers/"+id+"/scan", nil, access))
	if w3.Code != http.StatusOK {
		t.Fatalf("scan code=%d body=%s", w3.Code, w3.Body.String())
	}
	var rep b2b.Report
	_ = json.Unmarshal(w3.Body.Bytes(), &rep)
	if rep.Cars != 1 || rep.Pushed == 0 {
		t.Fatalf("scan report: %+v", rep)
	}

	// scan-all — операторское, по X-Admin-Token
	req := httptest.NewRequest(http.MethodPost, "/api/v1/b2b/scan-all", nil)
	req.Header.Set("X-Admin-Token", "admintok")
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req)
	if w4.Code != http.StatusOK {
		t.Fatalf("scan-all code=%d", w4.Code)
	}
}

func TestB2BRequiresAuth(t *testing.T) {
	r := newFullServer(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/b2b/service-centers", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("без токена ожидали 401, got %d", w.Code)
	}
}

func TestScanAllRequiresAdmin(t *testing.T) {
	r := newFullServer(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/b2b/scan-all", nil)) // без X-Admin-Token
	if w.Code != http.StatusForbidden {
		t.Fatalf("без admin-токена ожидали 403, got %d", w.Code)
	}
}

func TestCrossTenantScanForbidden(t *testing.T) {
	r := newFullServer(t)
	// СТО-1 создаёт владелец A
	accA := registerAccess(t, r, "a@x.ru")
	body, _ := json.Marshal(map[string]any{"name": "A", "webhook": "https://a.bitrix24.ru/rest/1/tok/"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/b2b/service-centers", body, accA))
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	idA, _ := created["id"].(string)

	// Юзер B пытается сканить чужой СТО → 403
	accB := registerAccess(t, r, "b@x.ru")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, bearerReq(http.MethodPost, "/api/v1/b2b/service-centers/"+idA+"/scan", nil, accB))
	if w2.Code != http.StatusForbidden {
		t.Fatalf("чужой СТО: ожидали 403, got %d", w2.Code)
	}
}
