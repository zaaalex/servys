package recommender

import (
	"context"
	"fmt"
	"github.com/zaaalex/servys/backend/domain"
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
			continue
		}
		chunks, e := p.Select.Select(ctx, s, doc)
		if e != nil {
			continue
		}
		for _, c := range chunks {
			x, e := p.Extract.Extract(ctx, s, c)
			if e != nil {
				continue
			}
			a, _ := ValidateExtraction(x, c)
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
