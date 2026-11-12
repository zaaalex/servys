package recommender

import (
	"context"
	"fmt"
	"log"
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

// StartSavedRefresh раз в сутки смотрит гараж. Разбор младше недели не трогает.
// Более старый остаётся на экране, а новый считается в фоне.
func (rt *Runtime) StartSavedRefresh(ctx context.Context, every time.Duration, list func(context.Context) ([]domain.Vehicle, error)) {
	if rt == nil || rt.heavy == nil || list == nil {
		return
	}
	if every <= 0 {
		every = 24 * time.Hour
	}
	go func() {
		run := func() {
			check, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			cars, err := list(check)
			if err != nil {
				log.Printf("knowledge: weekly refresh: %v", err)
				return
			}
			rt.heavy.refreshSaved(cars)
		}
		run()
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				run()
			}
		}
	}()
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
		timeout := 70 * time.Minute
		if raw := getenv("OLLAMA_TIMEOUT"); raw != "" {
			timeout, err = time.ParseDuration(raw)
			if err != nil || timeout <= 0 {
				return nil, fmt.Errorf("invalid OLLAMA_TIMEOUT %q", raw)
			}
		}
		textModel := NewOllamaExtractor(getenv("OLLAMA_BASE_URL"), getenv("OLLAMA_MODEL"), timeout)
		if err = applyOllamaOptions(textModel, getenv); err != nil {
			return nil, err
		}
		extractor = textModel
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
			Seek:       true,
		}
		if text, ok := extractor.(*OllamaExtractor); ok {
			searchName := getenv("OLLAMA_SEARCH_MODEL")
			if searchName == "" {
				searchName = "bonsai-2:27b"
			}
			if searchName != text.Model {
				searchModel := NewOllamaExtractor(text.BaseURL, searchName, text.Client.Timeout)
				searchModel.ContextLength = text.ContextLength
				searchModel.MaxOutputTokens = text.MaxOutputTokens
				searchModel.KeepAlive = text.KeepAlive
				pipeline.Reader = searchModel
				pipeline.Release = searchModel.Release
			}
		}
		budget := 70 * time.Minute
		if raw := getenv("RECO_LLM_BUDGET"); raw != "" {
			budget, err = time.ParseDuration(raw)
			if err != nil || budget <= 0 {
				return nil, fmt.Errorf("invalid RECO_LLM_BUDGET %q", raw)
			}
		}
		refresh := 7 * 24 * time.Hour
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

func applyOllamaOptions(model *OllamaExtractor, getenv func(string) string) error {
	if raw := getenv("OLLAMA_CONTEXT_LENGTH"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid OLLAMA_CONTEXT_LENGTH %q", raw)
		}
		model.ContextLength = n
	}
	if raw := getenv("OLLAMA_MAX_OUTPUT_TOKENS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n == 0 || n < -1 {
			return fmt.Errorf("invalid OLLAMA_MAX_OUTPUT_TOKENS %q", raw)
		}
		model.MaxOutputTokens = n
	}
	if raw := getenv("OLLAMA_KEEP_ALIVE"); raw != "" {
		keep, err := time.ParseDuration(raw)
		if err != nil || keep < 0 {
			return fmt.Errorf("invalid OLLAMA_KEEP_ALIVE %q", raw)
		}
		model.KeepAlive = keep
	}
	return nil
}
