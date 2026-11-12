package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/recommender"
)

type stubAsker struct {
	reply recommender.Reply
	err   error
	got   string
}

func (s *stubAsker) Ask(_ context.Context, _ domain.Vehicle, question string) (recommender.Reply, error) {
	s.got = question
	return s.reply, s.err
}

func TestAskVehicleReturnsSources(t *testing.T) {
	s := newTestServer(t)
	stub := &stubAsker{reply: recommender.Reply{
		Text:    "15 000 км",
		Sources: []recommender.Citation{{Title: "Регламент", URL: "https://auto.ru/k3"}},
	}}
	s.Ask = stub
	r := s.Router()
	access := registerAccess(t, r, "ask@x.ru")
	body, _ := json.Marshal(map[string]any{"make": "KIA", "model": "K3", "year": 2020, "mileage_km": 1000})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/vehicles", body, access))
	if w.Code != http.StatusCreated {
		t.Fatalf("create code=%d body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)

	askBody, _ := json.Marshal(map[string]any{"question": "какой интервал у масла"})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, bearerReq(http.MethodPost, "/api/v1/vehicles/"+id+"/ask", askBody, access))
	if w2.Code != http.StatusOK {
		t.Fatalf("ask code=%d body=%s", w2.Code, w2.Body.String())
	}
	var resp struct {
		Answer  string `json:"answer"`
		Sources []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"sources"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp)
	if resp.Answer != "15 000 км" || len(resp.Sources) != 1 || resp.Sources[0].URL != "https://auto.ru/k3" {
		t.Fatalf("resp=%+v", resp)
	}
	if stub.got != "какой интервал у масла" {
		t.Fatalf("вопрос: %q", stub.got)
	}
}

func TestAskVehicleRejectsEmptyQuestion(t *testing.T) {
	s := newTestServer(t)
	s.Ask = &stubAsker{}
	r := s.Router()
	access := registerAccess(t, r, "ask-empty@x.ru")
	body, _ := json.Marshal(map[string]any{"make": "KIA", "model": "K3", "year": 2020, "mileage_km": 1000})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/vehicles", body, access))
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, bearerReq(http.MethodPost, "/api/v1/vehicles/"+id+"/ask", []byte(`{"question":"  "}`), access))
	if w2.Code != http.StatusBadRequest || !strings.Contains(w2.Body.String(), "BAD_QUESTION") {
		t.Fatalf("code=%d body=%s", w2.Code, w2.Body.String())
	}
}

func TestAskVehicleOff(t *testing.T) {
	s := newTestServer(t)
	r := s.Router()
	access := registerAccess(t, r, "ask-off@x.ru")
	body, _ := json.Marshal(map[string]any{"make": "KIA", "model": "K3", "year": 2020, "mileage_km": 1000})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/vehicles", body, access))
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, bearerReq(http.MethodPost, "/api/v1/vehicles/"+id+"/ask", []byte(`{"question":"свечи"}`), access))
	if w2.Code != http.StatusBadGateway || !strings.Contains(w2.Body.String(), "ADVISOR_ERROR") {
		t.Fatalf("code=%d body=%s", w2.Code, w2.Body.String())
	}
}
