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
	return r.Pipeline.Rules(ctx, signatureFrom(v))
}

func (p *KnowledgePipeline) ReadDocumentation(ctx context.Context, s VehicleSignature) ([]domain.Rule, error) {
	if p == nil || p.Fetch == nil {
		return nil, nil
	}
	page := catalogScheduleURL(s)
	if page == "" {
		return nil, nil
	}
	doc, err := p.Fetch.Fetch(ctx, SearchResult{URL: page, Title: "Регламент ТО"})
	if err != nil {
		return nil, err
	}
	rules, _ := rulesFromSchedule(doc.Text, doc.URL, s)
	return rules, nil
}

func (p *KnowledgePipeline) Rules(ctx context.Context, s VehicleSignature) ([]domain.Rule, error) {
	if p.Search == nil || p.Fetch == nil || p.Select == nil || p.Extract == nil {
		return nil, fmt.Errorf("knowledge pipeline is not fully configured")
	}
	var found []SearchResult
	var searchErr error
	for _, q := range PlanQueries(s) {
		r, e := p.Search.Search(ctx, q)
		if e != nil {
			searchErr = e
			log.Printf("knowledge: search: %v", e)
			continue
		}
		found = append(found, r...)
	}
	found = preferSchedule(s, RankAndDedupeSources(s.Model, found))
	if len(found) == 0 {
		return nil, searchErr
	}
	n := p.MaxSources
	if n <= 0 || n > len(found) {
		n = len(found)
	}
	m := map[string]domain.Rule{}
	quoted := map[string]bool{}
	scheduleRows := 0
	for _, src := range found[:n] {
		doc, e := p.Fetch.Fetch(ctx, src)
		if e != nil {
			log.Printf("knowledge: fetch %s: %v", src.URL, e)
			continue
		}
		rows, missed := rulesFromSchedule(doc.Text, doc.URL, s)
		if missed {
			log.Printf("knowledge: no matching modification in %s", doc.URL)
		}
		for _, rule := range rows {
			key := rule.Code + "\x00" + rule.Operation
			if _, ok := m[key]; ok {
				continue
			}
			m[key] = rule
			quoted[key] = true
			scheduleRows++
			log.Printf("knowledge: schedule %s %s %d", doc.URL, rule.Code, rule.IntervalKm)
		}
		for _, f := range IntervalQuotes(doc.Text) {
			key := f.ComponentCode + "\x00" + f.Operation
			if quoted[key] {
				continue
			}
			if _, ok := m[key]; ok {
				continue
			}
			m[key] = ruleFromFact(f, doc.URL)
			quoted[key] = true
			km := 0
			if f.IntervalKM != nil {
				km = *f.IntervalKM
			}
			log.Printf("knowledge: quote %s %s %d", doc.URL, f.ComponentCode, km)
		}
		if scheduleRows >= 3 || missed {
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
				key := f.ComponentCode + "\x00" + f.Operation
				if quoted[key] {
					continue
				}
				m[key] = ruleFromFact(f, c.URL)
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

func rulesFromSchedule(text, page string, s VehicleSignature) ([]domain.Rule, bool) {
	target := ScheduleTarget{Year: s.Year, PowerHP: s.PowerHP, Market: s.Market}
	facts, label := ScheduleRowsFor(text, target)
	missed := targeted(target) && strings.Contains(normalizeSpace(text), "Регламент ТО") && len(facts) == 0
	if label != "" {
		log.Printf("knowledge: schedule section %s", label)
	}
	out := make([]domain.Rule, 0, len(facts))
	for _, f := range facts {
		rule := ruleFromFact(f, page)
		months := 0
		if f.IntervalMonths != nil {
			months = *f.IntervalMonths
		}
		if rule.Community != nil && f.IntervalKM != nil {
			note := scheduleNote(*f.IntervalKM, months)
			if label != "" {
				note = label + ". " + note
			}
			rule.Community.Note = note
		}
		out = append(out, rule)
	}
	return out, missed
}

func ruleFromFact(f ExtractedFact, page string) domain.Rule {
	r := domain.Rule{
		Code:      f.ComponentCode,
		Title:     TitleFor(f.ComponentCode),
		Operation: f.Operation,
		Verified:  true,
		Source:    page,
		Category:  CategoryFor(f.ComponentCode),
		Community: &domain.CommunityNote{Note: f.Evidence, Source: page},
	}
	if r.Title == "" {
		r.Title = strings.ReplaceAll(f.ComponentCode, "_", " ")
	}
	if f.IntervalKM != nil {
		r.IntervalKm = *f.IntervalKM
	}
	if f.IntervalMonths != nil {
		r.IntervalMonths = *f.IntervalMonths
	}
	return r
}
