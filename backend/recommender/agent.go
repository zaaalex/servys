package recommender

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

// ResearchStep — один ход модели. Адрес в Open должен быть из уже показанных ссылок.
type ResearchStep struct {
	Action string `json:"action"`
	Query  string `json:"query"`
	URL    string `json:"url"`
}

// WebResearcher выбирает следующий поиск или страницу. Сеть открывает код.
type WebResearcher interface {
	Next(context.Context, VehicleSignature, string) (ResearchStep, error)
}

func (p *KnowledgePipeline) research(ctx context.Context, s VehicleSignature) ([]domain.Rule, error) {
	researcher := p.researcher()
	if researcher == nil {
		return p.fixedRules(ctx, s)
	}
	m := map[string]domain.Rule{}
	quoted := map[string]bool{}
	links := map[string]SearchResult{}
	asked := map[string]bool{}
	var notes []string
	opened := map[string]bool{}
	var held []DocumentChunk
	repeats := 0
	searchCtx := ctx
	if p.Reader != nil {
		if deadline, ok := ctx.Deadline(); ok {
			until := deadline.Add(-10 * time.Minute)
			if time.Now().Before(until) {
				var cancel context.CancelFunc
				searchCtx, cancel = context.WithDeadline(ctx, until)
				defer cancel()
			}
		}
	}
	work := searchCtx
	if p.Reader != nil {
		work = context.WithValue(searchCtx, holdChunksKey{}, &held)
	}
	limit := p.MaxSources
	if limit <= 0 {
		limit = 3
	}
	for step := 0; step < 8; step++ {
		if ctx.Err() != nil || work.Err() != nil {
			break
		}
		callCtx, cancel, ok := modelCallContext(work, len(m) == 0)
		if !ok {
			log.Printf("knowledge: model search left too little time, fixed queries")
			break
		}
		act, err := researcher.Next(callCtx, s, researchBrief(s, links, notes))
		cancel()
		if err != nil {
			log.Printf("knowledge: model search: %v", err)
			break
		}
		switch strings.ToLower(strings.TrimSpace(act.Action)) {
		case "search":
			q := normalizeSpace(act.Query)
			if q == "" {
				notes = append(notes, "пустой запрос пропущен")
				continue
			}
			if asked[q] {
				repeats++
				notes = append(notes, "запрос уже был, открой ссылку из списка")
				if repeats >= 2 && len(m) == 0 {
					log.Printf("knowledge: model repeated a query, fixed queries")
					step = 8
				}
				continue
			}
			asked[q] = true
			hits, err := p.Search.Search(ctx, q)
			if err != nil {
				log.Printf("knowledge: search %q: %v", q, err)
				notes = append(notes, "поиск не удался: "+q)
				continue
			}
			hits = RankAndDedupeSources(s.Model, hits)
			if len(hits) > 5 {
				hits = hits[:5]
			}
			for _, hit := range hits {
				links[strings.TrimRight(hit.URL, "/")] = hit
			}
			notes = append(notes, fmt.Sprintf("поиск %q, ссылок %d", q, len(hits)))
			log.Printf("knowledge: model search %q -> %d", q, len(hits))
		case "open":
			key := strings.TrimRight(strings.TrimSpace(act.URL), "/")
			if opened[key] {
				notes = append(notes, "страница уже открыта")
				continue
			}
			if len(opened) >= limit {
				notes = append(notes, "лимит страниц")
				continue
			}
			hit, known := links[key]
			if !known {
				notes = append(notes, "адрес не из поиска и не открыт")
				log.Printf("knowledge: model open refused %s", act.URL)
				continue
			}
			openCtx, release, ok := modelCallContext(work, len(m) == 0)
			if !ok {
				log.Printf("knowledge: model search left too little time, fixed queries")
				step = 8
				continue
			}
			opened[key] = true
			summary := p.absorb(openCtx, s, hit, m, quoted)
			release()
			notes = append(notes, hit.URL+" — "+summary)
		case "finish", "":
			notes = append(notes, "модель закончила поиск")
			step = 8
		default:
			notes = append(notes, "неизвестный шаг")
		}
	}
	if p.Reader != nil && p.Release != nil {
		if err := p.Release(ctx); err != nil {
			log.Printf("knowledge: unload search model: %v", err)
			return nil, err
		}
	}
	if len(held) > 0 {
		p.keepFacts(ctx, s, held, m, quoted)
	}
	if len(m) == 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Printf("knowledge: model search found nothing, fixed queries")
		return p.fixedRules(ctx, s)
	}
	out := make([]domain.Rule, 0, len(m))
	for _, rule := range m {
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func researchBrief(s VehicleSignature, links map[string]SearchResult, notes []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Машина: %s %s %d", s.Make, s.Model, s.Year)
	if s.PowerHP > 0 {
		fmt.Fprintf(&b, ", %d л.с.", s.PowerHP)
	}
	if s.Market != "" {
		fmt.Fprintf(&b, ", рынок %s", s.Market)
	}
	b.WriteString("\nСначала найди регламент ТО именно этой машины. Затем отзывы владельцев на форумах про замены и интервалы.\n")
	b.WriteString("Ответ — один JSON: {\"action\":\"search\",\"query\":\"...\"} или {\"action\":\"open\",\"url\":\"...\"} или {\"action\":\"finish\"}.\n")
	b.WriteString("Адрес бери только из списка. Не выдумывай ссылку и не открывай одну страницу дважды.\n")
	b.WriteString("Если в списке уже есть регламент, открой его. Тот же запрос второй раз не отправляй.\n")
	if len(notes) > 0 {
		b.WriteString("Уже сделано:\n")
		start := 0
		if len(notes) > 8 {
			start = len(notes) - 8
		}
		for _, note := range notes[start:] {
			b.WriteString("- " + note + "\n")
		}
	}
	if len(links) > 0 {
		b.WriteString("Ссылки:\n")
		urls := make([]string, 0, len(links))
		for u := range links {
			urls = append(urls, u)
		}
		sort.Slice(urls, func(i, j int) bool {
			hi, hj := links[urls[i]], links[urls[j]]
			if hi.Score != hj.Score {
				return hi.Score > hj.Score
			}
			return urls[i] < urls[j]
		})
		if len(urls) > 8 {
			urls = urls[:8]
		}
		for _, u := range urls {
			hit := links[u]
			b.WriteString("- " + hit.URL + " | " + normalizeSpace(hit.Title) + " | " + normalizeSpace(hit.Snippet) + "\n")
		}
	}
	return b.String()
}

func (p *KnowledgePipeline) researcher() WebResearcher {
	if p != nil && p.Reader != nil {
		return p.Reader
	}
	if p == nil || p.Extract == nil {
		return nil
	}
	researcher, _ := p.Extract.(WebResearcher)
	return researcher
}

// fallbackReserve — запас на готовые запросы, пока модель ещё ничего не нашла.
// И чтение страницы, и следующий ход модели останавливаются раньше срока запроса.
var fallbackReserve = 40 * time.Second

func modelCallContext(ctx context.Context, leaveFallback bool) (context.Context, context.CancelFunc, bool) {
	noop := func() {}
	if !leaveFallback {
		return ctx, noop, true
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return ctx, noop, true
	}
	until := deadline.Add(-fallbackReserve)
	if !time.Now().Before(until) {
		return nil, nil, false
	}
	call, cancel := context.WithDeadline(ctx, until)
	return call, cancel, true
}
