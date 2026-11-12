package recommender

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractorAllowsAFullReplacementList(t *testing.T) {
	ex := NewOllamaExtractor("", "", 0)
	if ex.Model != "qwen3.5:9b" {
		t.Fatalf("model %s", ex.Model)
	}
	if ex.MaxOutputTokens != -1 {
		t.Fatalf("output limit %d", ex.MaxOutputTokens)
	}
	var predict int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("request: %v", err)
		}
		if body["think"] != false {
			t.Errorf("think %v", body["think"])
		}
		opts, _ := body["options"].(map[string]any)
		predict = int(opts["num_predict"].(float64))
		content := `{"facts":[` + closedFact("spark_plugs", 45000, "свечи меняют каждые 45000 км") + `,` +
			closedFact("timing_belt", 90000, "ремень ГРМ меняют каждые 90000 км") +
			`,{"componentCode":"brake_fluid","operation":"replace","intervalKm":`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message":     map[string]string{"content": content},
			"done_reason": "length",
		})
	}))
	defer srv.Close()
	ex.BaseURL = srv.URL
	got, err := ex.Extract(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020}, DocumentChunk{Text: "ремень"})
	if err != nil {
		t.Fatal(err)
	}
	if predict != -1 {
		t.Fatalf("num_predict %d", predict)
	}
	if len(got.Facts) != 2 || got.Facts[0].ComponentCode != "spark_plugs" || got.Facts[1].ComponentCode != "timing_belt" {
		t.Fatalf("facts %+v", got.Facts)
	}
	if got.Facts[1].IntervalKM == nil || *got.Facts[1].IntervalKM != 90000 || got.Facts[1].Evidence == "" || got.Facts[1].Operation != "replace" {
		t.Fatalf("belt fact %+v", got.Facts[1])
	}
}

func TestCompleteModelReplyKeepsEveryFact(t *testing.T) {
	content := `{"facts":[` + closedFact("engine_oil", 15000, "масло меняют каждые 15000 км") + `]}`
	got, err := parseModelFacts(content)
	if err != nil || len(got.Facts) != 1 || got.Facts[0].ScheduleMode != "mileage" || got.Facts[0].UsageMode != "unknown" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestEmptyModelReplyIsAnError(t *testing.T) {
	if _, err := parseModelFacts("  "); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatal(err)
	}
}

func TestReleaseWaitsUntilSearchModelIsGone(t *testing.T) {
	var keep any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			keep = body["keep_alive"]
			w.WriteHeader(http.StatusOK)
		case "/api/ps":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ex := NewOllamaExtractor(srv.URL, "bonsai-2:27b", time.Second)
	ex.UnloadWait = 200 * time.Millisecond
	if err := ex.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	if keep != float64(0) {
		t.Fatalf("keep_alive %#v", keep)
	}
}

func TestReleaseRefusesWhileModelStaysResident(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate":
			w.WriteHeader(http.StatusOK)
		case "/api/ps":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{{"name": "bonsai-2:27b"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ex := NewOllamaExtractor(srv.URL, "bonsai-2:27b", time.Second)
	ex.UnloadWait = 40 * time.Millisecond
	err := ex.Release(context.Background())
	if err == nil || !strings.Contains(err.Error(), "still loaded") {
		t.Fatal(err)
	}
}

func closedFact(code string, km int, evidence string) string {
	fact := ExtractedFact{
		ComponentCode: code,
		Operation:     "replace",
		IntervalKM:    &km,
		ScheduleMode:  "mileage",
		UsageMode:     "unknown",
		Evidence:      evidence,
	}
	b, err := json.Marshal(fact)
	if err != nil {
		panic(err)
	}
	return string(b)
}
