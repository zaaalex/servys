package recommender

import (
	"context"
	"fmt"
	"github.com/zaaalex/servys/backend/domain"
	"log"
	"sort"
	"strings"
)

type FetchedDocument struct{ SourceID, URL, Title, Text, ContentType string }
type DocumentFetcher interface {
	Fetch(context.Context, SearchResult) (FetchedDocument, error)
}
type ChunkSelector interface {
	Select(context.Context, VehicleSignature, FetchedDocument) ([]DocumentChunk, error)
}
type KnowledgePipeline struct {
	Search     SearchProvider
	Fetch      DocumentFetcher
	Select     ChunkSelector
	Extract    KnowledgeExtractor
	MaxSources int
}
type PipelineRecommender struct{ Pipeline *KnowledgePipeline }

func (r PipelineRecommender) Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error) {
	return r.Pipeline.Rules(ctx, VehicleSignature{Make: v.Make, Model: v.Model, Year: v.Year, Powertrain: fmt.Sprintf("%dcc %dhp", v.EngineCC, v.PowerHP)})
}
func (p *KnowledgePipeline) Rules(ctx context.Context, s VehicleSignature) ([]domain.Rule, error) {
	if p.Search == nil || p.Fetch == nil || p.Select == nil || p.Extract == nil {
		return nil, fmt.Errorf("knowledge pipeline is not fully configured")
	}
	var found []SearchResult
	for _, q := range PlanQueries(s) {
		r, e := p.Search.Search(ctx, q)
		if e != nil {
			return nil, e
		}
		found = append(found, r...)
	}
	found = RankAndDedupeSources(found)
	n := p.MaxSources
	if n <= 0 || n > len(found) {
		n = len(found)
	}
	m := map[string]domain.Rule{}
	for _, src := range found[:n] {
		doc, e := p.Fetch.Fetch(ctx, src)
		if e != nil {
			log.Printf("knowledge: fetch %s: %v", src.URL, e)
			continue
		}
		chunks, e := p.Select.Select(ctx, s, doc)
		if e != nil {
			log.Printf("knowledge: select %s: %v", src.URL, e)
			continue
		}
		if len(chunks) == 0 {
			log.Printf("knowledge: no maintenance chunks in %s", src.URL)
		}
		for _, c := range chunks {
			x, e := p.Extract.Extract(ctx, s, c)
			if e != nil {
				log.Printf("knowledge: extract %s: %v", c.URL, e)
				continue
			}
			// Маленькие локальные модели иногда корректно извлекают один интервал,
			// но галлюцинируют второй. Отбрасываем только заведомо невозможное поле,
			// а не весь факт с точной evidence-цитатой.
			for i := range x.Facts {
				f := &x.Facts[i]
				if f.IntervalMonths != nil && (*f.IntervalMonths < 1 || *f.IntervalMonths > 240) {
					f.IntervalMonths = nil
				}
				if bounds, ok := components[f.ComponentCode]; f.IntervalKM != nil && ok && (*f.IntervalKM < bounds[0] || *f.IntervalKM > bounds[1]) {
					f.IntervalKM = nil
				}
			}
			a, rejected := ValidateExtraction(x, c)
			for _, reason := range rejected {
				log.Printf("knowledge: reject %s: %v", c.URL, reason)
			}
			for _, f := range a.Facts {
				r := domain.Rule{Code: f.ComponentCode, Title: strings.ReplaceAll(f.ComponentCode, "_", " "), Operation: f.Operation, Verified: true, Source: c.URL}
				if f.IntervalKM != nil {
					r.IntervalKm = *f.IntervalKM
				}
				if f.IntervalMonths != nil {
					r.IntervalMonths = *f.IntervalMonths
				}
				m[r.Code+"\x00"+r.Operation] = r
			}
		}
	}
	out := []domain.Rule{}
	for _, r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}
