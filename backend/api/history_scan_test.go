package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/zaaalex/servys/backend/auth"
	"github.com/zaaalex/servys/backend/lane"
	"github.com/zaaalex/servys/backend/recommender"
	"github.com/zaaalex/servys/backend/store"
	"github.com/zaaalex/servys/backend/vin"
)

type holdScan struct {
	mu    sync.Mutex
	calls int
	text  string
}

func (s *holdScan) Read(context.Context, string) (string, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return s.text, nil
}

func (s *holdScan) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestWorkOrderWaitsUntilSearchReleasesTheLane(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	heavy := lane.New(0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go heavy.Start(ctx)

	hold := make(chan struct{})
	in := make(chan struct{})
	go func() {
		_ = heavy.Do(ctx, lane.KindSearch, func(context.Context) error {
			close(in)
			<-hold
			return nil
		})
	}()
	<-in

	scan := &holdScan{text: "Заказ-наряд\nДата 12.03.2024\nПробег: 45 230 км\nЗамена масла двигателя\nЗамена тормозных колодок\nЗамена амортизаторов\n"}
	srv := withTestBot(&Server{
		Store: st, Adv: recommender.NewStubAdvisor(), VIN: vin.NewStub(),
		Auth: auth.New(st, []byte("jwt-secret")), UploadDir: t.TempDir(),
		Lane: heavy, Scan: scan,
	})
	router := srv.Router()
	access := registerAccess(t, router, "scan@x.ru")

	body := []byte(`{"make":"KIA","model":"K3","year":2020,"mileage_km":45230}`)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, bearerReq(http.MethodPost, "/api/v1/vehicles", body, access))
	if w.Code != http.StatusCreated {
		t.Fatalf("vehicle %d %s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(string)

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, err := writer.CreateFormFile("file", "order.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("jpeg-bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := bearerReq(http.MethodPost, "/api/v1/vehicles/"+id+"/history-file", form.Bytes(), access)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		got := httptest.NewRecorder()
		router.ServeHTTP(got, req)
		done <- got
	}()
	select {
	case got := <-done:
		t.Fatalf("снимок прочитался во время поиска: %d %s", got.Code, got.Body.String())
	case <-time.After(50 * time.Millisecond):
	}
	if scan.count() != 0 {
		t.Fatal("paddle вызвался, пока шёл поиск")
	}
	close(hold)
	var got *httptest.ResponseRecorder
	select {
	case got = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("снимок не прочитался после поиска")
	}
	if got.Code != http.StatusCreated {
		t.Fatalf("history-file %d %s", got.Code, got.Body.String())
	}
	var resp struct {
		Suggestions []struct {
			RuleCode string `json:"rule_code"`
			Odometer int    `json:"odometer"`
		} `json:"suggestions"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	found := map[string]int{}
	for _, item := range resp.Suggestions {
		found[item.RuleCode] = item.Odometer
	}
	for _, code := range []string{"engine_oil", "brake_pads_front", "shock_absorbers"} {
		if found[code] != 45230 {
			t.Fatalf("%s: %+v", code, resp.Suggestions)
		}
	}
}
