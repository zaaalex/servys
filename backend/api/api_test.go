package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/zaaalex/servys/backend/auth"
	"github.com/zaaalex/servys/backend/recommender"
	"github.com/zaaalex/servys/backend/store"
	"github.com/zaaalex/servys/backend/vin"
)

const testBotToken = "123456:test-bot"

func newTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return withTestBot(&Server{
		Store: st, Adv: recommender.NewStubAdvisor(), VIN: vin.NewStub(),
		Auth: auth.New(st, []byte("jwt-secret")),
	})
}

func withTestBot(s *Server) *Server {
	s.Auth.BotToken = testBotToken
	return s
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	return newTestServer(t).Router()
}

func TestHealth(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("health code=%d", w.Code)
	}
}

func TestVehiclesRequireAuth(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/vehicles", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("без Bearer ожидали 401, got=%d", w.Code)
	}
}

func TestCreateVehicleAndAlertsFlow(t *testing.T) {
	r := newTestRouter(t)
	access := registerAccess(t, r, "b2c@x.ru")

	body, _ := json.Marshal(map[string]any{
		"make": "KIA", "model": "K3", "year": 2020, "mileage_km": 95000,
		"color": "#112233", "body_type": "suv", "identification_source": "drom",
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/vehicles", body, access))
	if w.Code != http.StatusCreated {
		t.Fatalf("create vehicle code=%d body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("нет id созданного авто")
	}
	if created["color"] != "#112233" || created["body_type"] != "suv" || created["identification_source"] != "drom" {
		t.Fatalf("цвет, кузов и источник не сохранились: %+v", created)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, bearerReq(http.MethodGet, "/api/v1/vehicles/"+id+"/alerts", nil, access))
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

	seBody, _ := json.Marshal(map[string]any{"rule_code": "engine_oil", "odometer": 90000})
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, bearerReq(http.MethodPost, "/api/v1/vehicles/"+id+"/service-events", seBody, access))
	if w3.Code != http.StatusCreated {
		t.Fatalf("service-event code=%d body=%s", w3.Code, w3.Body.String())
	}

	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, bearerReq(http.MethodGet, "/api/v1/vehicles/"+id+"/service-events", nil, access))
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

func TestVehiclesScopedToAccount(t *testing.T) {
	r := newTestRouter(t)
	accA := registerAccess(t, r, "a@x.ru")
	accB := registerAccess(t, r, "b@x.ru")

	body, _ := json.Marshal(map[string]any{"make": "KIA", "model": "K3", "mileage_km": 1000})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/vehicles", body, accA))
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)

	// B не видит авто A
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, bearerReq(http.MethodGet, "/api/v1/vehicles/"+id, nil, accB))
	if w2.Code != http.StatusNotFound {
		t.Fatalf("чужой аккаунт не должен видеть авто, got=%d", w2.Code)
	}
	// B видит пустой гараж
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, bearerReq(http.MethodGet, "/api/v1/vehicles", nil, accB))
	var lst struct {
		Vehicles []map[string]any `json:"vehicles"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &lst)
	if len(lst.Vehicles) != 0 {
		t.Fatalf("гараж B должен быть пуст, got %d", len(lst.Vehicles))
	}
}

func TestServiceEventRequiresRuleCode(t *testing.T) {
	r := newTestRouter(t)
	access := registerAccess(t, r, "c2@x.ru")
	body, _ := json.Marshal(map[string]any{"make": "KIA", "model": "K3", "mileage_km": 1000})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/vehicles", body, access))
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)

	empty, _ := json.Marshal(map[string]any{"odometer": 1000})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, bearerReq(http.MethodPost, "/api/v1/vehicles/"+id+"/service-events", empty, access))
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("без rule_code ожидали 400, got=%d", w2.Code)
	}
}

func TestResolveKnownFixtureVINDoesNotReturn500(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "vin.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	s := withTestBot(&Server{Store: st, Adv: recommender.NewStubAdvisor(), VIN: vin.NewFixture(), Auth: auth.New(st, []byte("jwt-secret"))})
	router := s.Router()
	access := registerAccess(t, router, "vin@x.ru")

	body, _ := json.Marshal(map[string]string{"vin": vin.FixtureVIN})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/vin/resolve", body, access))
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["vin"] != vin.FixtureVIN {
		t.Fatalf("response=%v", response)
	}
}

func registerAccess(t *testing.T, r http.Handler, label string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(label))
	userID := int64(binary.BigEndian.Uint32(sum[:4]) & 0x7fffffff)
	if userID == 0 {
		userID = 1
	}
	body, _ := json.Marshal(map[string]string{"init_data": telegramInitData(testBotToken, userID)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/telegram", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("telegram login code=%d body=%s", w.Code, w.Body.String())
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

func telegramInitData(bot string, userID int64) string {
	user := fmt.Sprintf(`{"id":%d}`, userID)
	pairs := []string{"auth_date=1700000000", "user=" + user}
	sort.Strings(pairs)
	mac := hmac.New(sha256.New, []byte("WebAppData"))
	mac.Write([]byte(bot))
	secret := mac.Sum(nil)
	mac = hmac.New(sha256.New, secret)
	mac.Write([]byte(strings.Join(pairs, "\n")))
	q := url.Values{}
	q.Set("auth_date", "1700000000")
	q.Set("user", user)
	q.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return q.Encode()
}

func bearerReq(method, path string, body []byte, tok string) *http.Request {
	rd := bytes.NewReader(body)
	req := httptest.NewRequest(method, path, rd)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return req
}
