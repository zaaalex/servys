package recommender

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/lane"
)

type Runtime struct {
	Recommender Recommender
	Advisor     Advisor
	Extractor   KnowledgeExtractor
	heavy       *remembering
}

func (rt *Runtime) SetHeavyLane(l *lane.Lane) {
	if rt != nil && rt.heavy != nil {
		rt.heavy.SetLane(l)
	}
}

func NewFromEnv() (*Runtime, error) { return NewFromLookupEnv(os.Getenv) }

func NewFromLookupEnv(getenv func(string) string) (*Runtime, error) {
	var extractor KnowledgeExtractor
	var err error
	switch mode := getenv("LLM_MODE"); mode {
	case "", "disabled":
		extractor = DisabledExtractor{}
	case "fixture":
		extractor = FixtureExtractor{Extraction: Extraction{Facts: []ExtractedFact{}}}
	case "live":
		timeout := 120 * time.Second
		if raw := getenv("OLLAMA_TIMEOUT"); raw != "" {
			timeout, err = time.ParseDuration(raw)
			if err != nil || timeout <= 0 {
				return nil, fmt.Errorf("invalid OLLAMA_TIMEOUT %q", raw)
			}
		}
		ollama := NewOllamaExtractor(getenv("OLLAMA_BASE_URL"), getenv("OLLAMA_MODEL"), timeout)
		if raw := getenv("OLLAMA_CONTEXT_LENGTH"); raw != "" {
			ollama.ContextLength, err = strconv.Atoi(raw)
			if err != nil || ollama.ContextLength <= 0 {
				return nil, fmt.Errorf("invalid OLLAMA_CONTEXT_LENGTH %q", raw)
			}
		}
		if raw := getenv("OLLAMA_MAX_OUTPUT_TOKENS"); raw != "" {
			ollama.MaxOutputTokens, err = strconv.Atoi(raw)
			if err != nil || ollama.MaxOutputTokens <= 0 {
				return nil, fmt.Errorf("invalid OLLAMA_MAX_OUTPUT_TOKENS %q", raw)
			}
		}
		if raw := getenv("OLLAMA_KEEP_ALIVE"); raw != "" {
			ollama.KeepAlive, err = time.ParseDuration(raw)
			if err != nil || ollama.KeepAlive < 0 {
				return nil, fmt.Errorf("invalid OLLAMA_KEEP_ALIVE %q", raw)
			}
		}
		extractor = ollama
	default:
		return nil, fmt.Errorf("invalid LLM_MODE %q (want live, fixture, or disabled)", mode)
	}
	// Интервалы только из найденных страниц.
	var rec Recommender = none{}
	var heavy *remembering
	if getenv("LLM_MODE") == "live" {
		searchURL := getenv("SEARXNG_BASE_URL")
		if searchURL == "" {
			searchURL = "http://127.0.0.1:8081"
		}
		maxSources := 5
		if raw := getenv("KNOWLEDGE_MAX_SOURCES"); raw != "" {
			maxSources, err = strconv.Atoi(raw)
			if err != nil || maxSources <= 0 {
				return nil, fmt.Errorf("invalid KNOWLEDGE_MAX_SOURCES %q", raw)
			}
		}
		pipeline := &KnowledgePipeline{
			Search:     NewSearXNG(searchURL, nil),
			Fetch:      NewDocumentFetcher(),
			Select:     DeterministicChunkSelector{},
			Extract:    extractor,
			MaxSources: maxSources,
		}
		budget := 90 * time.Second
		if raw := getenv("RECO_LLM_BUDGET"); raw != "" {
			budget, err = time.ParseDuration(raw)
			if err != nil || budget <= 0 {
				return nil, fmt.Errorf("invalid RECO_LLM_BUDGET %q", raw)
			}
		}
		refresh := 6 * time.Hour
		if raw := getenv("RECO_REFRESH"); raw != "" {
			refresh, err = time.ParseDuration(raw)
			if err != nil || refresh <= 0 {
				return nil, fmt.Errorf("invalid RECO_REFRESH %q", raw)
			}
		}
		memo := newRemembering(pipeline, budget, refresh, getenv("RECO_CACHE_FILE"))
		heavy = memo
		rec = budgeted{
			Inner:  memo,
			Budget: budget,
		}
	}
	return &Runtime{Recommender: rec, Advisor: NewAdvisor(rec), Extractor: extractor, heavy: heavy}, nil
}

type none struct{}

func (none) Rules(context.Context, domain.Vehicle) ([]domain.Rule, error) {
	return nil, nil
}
