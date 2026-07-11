package recommender

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Runtime struct {
	Recommender Recommender
	Advisor     Advisor
	Extractor   KnowledgeExtractor
}

func NewFromEnv() (*Runtime, error) { return NewFromLookupEnv(os.Getenv) }

func NewFromLookupEnv(getenv func(string) string) (*Runtime, error) {
	path := getenv("MAINTENANCE_RULES_PATH")
	if path == "" {
		path = "data/maintenance_rules.yaml"
	}
	rules, err := NewYAML(path)
	if err != nil {
		return nil, fmt.Errorf("load maintenance rules: %w", err)
	}
	var extractor KnowledgeExtractor
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
	var rec Recommender = rules
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
		pipeline := &KnowledgePipeline{Search: NewSearXNG(searchURL, nil), Fetch: NewDocumentFetcher(), Select: DeterministicChunkSelector{}, Extract: extractor, MaxSources: maxSources}
		// Бюджет времени на LLM-путь: неизвестное авто не должно держать /alerts на 120с.
		budget := 15 * time.Second
		if raw := getenv("RECO_LLM_BUDGET"); raw != "" {
			budget, err = time.ParseDuration(raw)
			if err != nil || budget <= 0 {
				return nil, fmt.Errorf("invalid RECO_LLM_BUDGET %q", raw)
			}
		}
		rec = FallbackRecommender{Primary: rules, Secondary: PipelineRecommender{Pipeline: pipeline}, SecondaryBudget: budget}
	}
	return &Runtime{Recommender: rec, Advisor: NewAdvisor(rec), Extractor: extractor}, nil
}
