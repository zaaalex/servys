package recommender

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

const forumPage = "Владелец пишет масло меняют на 5000 км. Тормозную жидкость меняют раз в 24 месяца."

func TestModelSearchOpensOnlyFoundPages(t *testing.T) {
	const query = "Kia K3 2020 регламент ТО именно этой машины"
	schedule := "https://auto.ru/catalog/cars/kia/k3/reglament-to/"
	forum := "https://www.drive2.ru/l/kia-k3/"
	search := &recordingSearch{hits: []SearchResult{
		{URL: schedule, Title: "Kia K3 регламент ТО", Snippet: "интервалы замены"},
		{URL: forum, Title: "Kia K3 отзыв владельца на форуме", Snippet: "бортовой журнал"},
	}}
	fetch := &pageFetch{pages: map[string]string{
		schedule: k3Page,
		forum:    forumPage,
	}}
	km := 5000
	months := 24
	model := &scriptResearch{steps: []ResearchStep{
		{Action: "search", Query: query},
		{Action: "open", URL: schedule},
		{Action: "open", URL: "https://evil.example/secret"},
		{Action: "open", URL: forum},
		{Action: "finish"},
	}, facts: []ExtractedFact{
		{ComponentCode: "engine_oil", Operation: "replace", IntervalKM: &km, ScheduleMode: "mileage", UsageMode: "unknown", Evidence: "масло меняют на 5000 км"},
		{ComponentCode: "brake_fluid", Operation: "replace", IntervalMonths: &months, ScheduleMode: "time", UsageMode: "unknown", Evidence: "Тормозную жидкость меняют раз в 24 месяца"},
	}}
	pipe := &KnowledgePipeline{Search: search, Fetch: fetch, Select: wholeSelect{}, Extract: model, MaxSources: 3, Seek: true}
	rules, err := pipe.Rules(context.Background(), VehicleSignature{
		Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130, Market: "china",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domainRuleView{}
	for _, rule := range rules {
		got[rule.Code] = domainRuleView{km: rule.IntervalKm, months: rule.IntervalMonths, source: rule.Source}
	}
	oil := got["engine_oil"]
	if oil.km != 15000 || !strings.Contains(oil.source, "reglament-to") {
		t.Fatalf("schedule oil was replaced: %+v", got)
	}
	fluid := got["brake_fluid"]
	if fluid.months != 24 || !strings.Contains(fluid.source, "drive2.ru") {
		t.Fatalf("forum review was not kept: %+v", got)
	}
	if strings.Contains(fetch.urls(), "evil.example") {
		t.Fatalf("opened an address the search did not return: %s", fetch.urls())
	}
	queries := search.queries()
	if len(queries) != 1 || queries[0] != query {
		t.Fatalf("search queries: %q", queries)
	}
}

func TestModelSearchFallsBackToFixedQueries(t *testing.T) {
	schedule := "https://auto.ru/catalog/cars/kia/k3/reglament-to/"
	fetch := &pageFetch{pages: map[string]string{schedule: k3Page}}
	s := VehicleSignature{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130, Market: "china"}
	t.Run("model finishes without pages", func(t *testing.T) {
		search := &recordingSearch{}
		pipe := &KnowledgePipeline{Search: search, Fetch: fetch, Select: quietSelect{}, Extract: &scriptResearch{steps: []ResearchStep{{Action: "finish"}}}, Seek: true}
		rules, err := pipe.Rules(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		if intervalsOf(rules)["engine_oil"] != 15000 {
			t.Fatalf("%+v", rules)
		}
		if len(search.queries()) != len(PlanQueries(s)) {
			t.Fatalf("fixed queries did not run: %q", search.queries())
		}
	})
	t.Run("model errors", func(t *testing.T) {
		search := &recordingSearch{}
		pipe := &KnowledgePipeline{Search: search, Fetch: fetch, Select: quietSelect{}, Extract: failResearch{}, Seek: true}
		rules, err := pipe.Rules(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		if intervalsOf(rules)["engine_oil"] != 15000 {
			t.Fatalf("%+v", rules)
		}
	})
	t.Run("extractor cannot choose a query", func(t *testing.T) {
		search := &recordingSearch{}
		pipe := &KnowledgePipeline{Search: search, Fetch: fetch, Select: quietSelect{}, Extract: DisabledExtractor{}, Seek: true}
		rules, err := pipe.Rules(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		if intervalsOf(rules)["engine_oil"] != 15000 {
			t.Fatalf("%+v", rules)
		}
	})
	t.Run("almost no time left", func(t *testing.T) {
		search := &recordingSearch{}
		model := &scriptResearch{steps: []ResearchStep{{Action: "search", Query: "should not run"}}}
		pipe := &KnowledgePipeline{Search: search, Fetch: fetch, Select: quietSelect{}, Extract: model, Seek: true}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		rules, err := pipe.Rules(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		if intervalsOf(rules)["engine_oil"] != 15000 {
			t.Fatalf("%+v", rules)
		}
		if model.n != 0 {
			t.Fatalf("model was called with %d steps left on the clock", model.n)
		}
	})
}

func TestRepeatedModelQueryFallsBack(t *testing.T) {
	const query = "Kia K3 2020 регламент"
	schedule := "https://auto.ru/catalog/cars/kia/k3/reglament-to/"
	forum := "https://www.drive2.ru/l/kia-k3/"
	search := &recordingSearch{hits: []SearchResult{
		{URL: schedule, Title: "Kia K3 регламент ТО"},
		{URL: forum, Title: "Kia K3 отзыв владельца"},
	}}
	fetch := &pageFetch{pages: map[string]string{schedule: k3Page, forum: forumPage}}
	model := &scriptResearch{steps: []ResearchStep{
		{Action: "search", Query: query},
		{Action: "search", Query: query},
		{Action: "search", Query: query},
		{Action: "open", URL: forum},
	}}
	pipe := &KnowledgePipeline{Search: search, Fetch: fetch, Select: quietSelect{}, Extract: model, MaxSources: 1, Seek: true}
	rules, err := pipe.Rules(context.Background(), VehicleSignature{
		Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130, Market: "china",
	})
	if err != nil {
		t.Fatal(err)
	}
	if intervalsOf(rules)["engine_oil"] != 15000 {
		t.Fatalf("%+v", rules)
	}
	sent := 0
	for _, q := range search.queries() {
		if q == query {
			sent++
		}
	}
	if sent != 1 {
		t.Fatalf("repeated query was sent %d times: %q", sent, search.queries())
	}
	if strings.Contains(fetch.urls(), "drive2.ru") {
		t.Fatalf("forum was opened after the repeated query: %s", fetch.urls())
	}
}

func TestOpenLeavesTimeForTheFixedSchedule(t *testing.T) {
	prev := fallbackReserve
	fallbackReserve = 200 * time.Millisecond
	t.Cleanup(func() { fallbackReserve = prev })

	schedule := "https://auto.ru/catalog/cars/kia/k3/reglament-to/"
	forum := "https://www.drive2.ru/l/kia-k3/"
	search := &recordingSearch{hits: []SearchResult{{URL: forum, Title: "Kia K3 отзыв владельца", Snippet: "форум"}}}
	fetch := &pageFetch{pages: map[string]string{schedule: k3Page, forum: forumPage}}
	model := &scriptResearch{
		blockExtract: true,
		steps: []ResearchStep{
			{Action: "search", Query: "Kia K3 отзывы"},
			{Action: "open", URL: forum},
			{Action: "finish"},
		},
	}
	pipe := &KnowledgePipeline{Search: search, Fetch: fetch, Select: wholeSelect{}, Extract: model, MaxSources: 1, Seek: true}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rules, err := pipe.Rules(ctx, VehicleSignature{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130, Market: "china"})
	if err != nil {
		t.Fatal(err)
	}
	if intervalsOf(rules)["engine_oil"] != 15000 {
		t.Fatalf("%+v", rules)
	}
	if !strings.Contains(fetch.urls(), "reglament-to") {
		t.Fatalf("fixed schedule was not opened: %s", fetch.urls())
	}
}

func TestResearchBriefListsTheScheduleFirst(t *testing.T) {
	links := map[string]SearchResult{
		"https://www.drive2.ru/l/kia-k3":                   {URL: "https://www.drive2.ru/l/kia-k3/", Title: "отзыв", Score: 4},
		"https://auto.ru/catalog/cars/kia/k3/reglament-to": {URL: "https://auto.ru/catalog/cars/kia/k3/reglament-to/", Title: "регламент", Score: 40},
	}
	brief := researchBrief(VehicleSignature{Make: "KIA", Model: "K3", Year: 2020}, links, nil)
	reg := strings.Index(brief, "reglament-to")
	forum := strings.Index(brief, "drive2")
	if reg < 0 || forum < 0 || reg > forum {
		t.Fatalf("%s", brief)
	}
}

func TestResearchBriefNamesTheCar(t *testing.T) {
	brief := researchBrief(VehicleSignature{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130, Market: "china"}, nil, nil)
	if !strings.Contains(brief, "KIA K3 2020") || !strings.Contains(brief, "регламент") || !strings.Contains(brief, "форум") {
		t.Fatalf("%s", brief)
	}
}

type domainRuleView struct {
	km, months int
	source     string
}

type recordingSearch struct {
	mu   sync.Mutex
	got  []string
	hits []SearchResult
}

func (s *recordingSearch) Search(_ context.Context, q string) ([]SearchResult, error) {
	s.mu.Lock()
	s.got = append(s.got, q)
	s.mu.Unlock()
	return append([]SearchResult(nil), s.hits...), nil
}

func (s *recordingSearch) queries() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.got...)
}

type wholeSelect struct{}

func (wholeSelect) Select(_ context.Context, _ VehicleSignature, doc FetchedDocument) ([]DocumentChunk, error) {
	return []DocumentChunk{{URL: doc.URL, Text: doc.Text}}, nil
}

type scriptResearch struct {
	steps        []ResearchStep
	facts        []ExtractedFact
	n            int
	blockExtract bool
}

func (s *scriptResearch) Next(context.Context, VehicleSignature, string) (ResearchStep, error) {
	if s.n >= len(s.steps) {
		return ResearchStep{Action: "finish"}, nil
	}
	step := s.steps[s.n]
	s.n++
	return step, nil
}

func (s *scriptResearch) Extract(ctx context.Context, _ VehicleSignature, _ DocumentChunk) (Extraction, error) {
	if s.blockExtract {
		<-ctx.Done()
		return Extraction{}, ctx.Err()
	}
	return Extraction{Facts: append([]ExtractedFact(nil), s.facts...)}, nil
}

type failResearch struct{}

func (failResearch) Next(context.Context, VehicleSignature, string) (ResearchStep, error) {
	return ResearchStep{}, errors.New("model down")
}

func (failResearch) Extract(context.Context, VehicleSignature, DocumentChunk) (Extraction, error) {
	return Extraction{}, nil
}
