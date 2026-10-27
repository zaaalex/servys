package recommender

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
)

func TestYAMLSkipsPreparedSources(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	body := []byte(`
version: 1
variants:
  - match:
      make: KIA
      model: K3
      year_from: 2020
      year_to: 2020
      engine_displacement_cc: 1353
      power_hp: 130
    rules:
      - code: engine_oil
        operation: replace
        interval_km: 10000
        source: demo
      - code: spark_plugs
        operation: replace
        interval_km: 40000
        source: https://example.com/kia-k3-review
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	y, err := NewYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := y.Rules(context.Background(), domain.Vehicle{
		Make: "KIA", Model: "K3", Year: 2020, EngineCC: 1353, PowerHP: 130,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Code != "spark_plugs" || rules[0].Source != "https://example.com/kia-k3-review" {
		t.Fatalf("ждали одну цитату со страницы, получили %+v", rules)
	}
}

func TestDisabledModeReturnsNoRules(t *testing.T) {
	rt, err := NewFromLookupEnv(func(string) string { return "disabled" })
	if err != nil {
		t.Fatal(err)
	}
	rules, err := rt.Recommender.Rules(context.Background(), domain.Vehicle{
		Make: "KIA", Model: "K3", Year: 2020, EngineCC: 1353, PowerHP: 130, CurrentOdometer: 83971,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Fatalf("без поиска правил быть не должно, получили %+v", rules)
	}
}

func TestLiveModeIsReviewPipeline(t *testing.T) {
	rt, err := NewFromLookupEnv(func(k string) string {
		switch k {
		case "LLM_MODE":
			return "live"
		case "SEARXNG_BASE_URL":
			return "http://127.0.0.1:9"
		case "OLLAMA_BASE_URL":
			return "http://127.0.0.1:9"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	step, ok := rt.Recommender.(budgeted)
	if !ok {
		t.Fatalf("ожидали поиск с бюджетом, получили %T", rt.Recommender)
	}
	memo, ok := step.Inner.(*remembering)
	if !ok || memo.pipe == nil {
		t.Fatalf("ожидали фоновый разбор документации, получили %T", step.Inner)
	}
}
