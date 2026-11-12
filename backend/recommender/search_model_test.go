package recommender

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type orderedSearch struct {
	steps      []ResearchStep
	n          int
	order      *[]string
	releaseErr error
}

func (s *orderedSearch) Next(context.Context, VehicleSignature, string) (ResearchStep, error) {
	*s.order = append(*s.order, "search")
	if s.n >= len(s.steps) {
		return ResearchStep{Action: "finish"}, nil
	}
	step := s.steps[s.n]
	s.n++
	return step, nil
}

func (s *orderedSearch) Answer(context.Context, string, string) (string, error) {
	*s.order = append(*s.order, "answer")
	return "45 тыс. км", nil
}

func (s *orderedSearch) Release(context.Context) error {
	*s.order = append(*s.order, "unload")
	return s.releaseErr
}

type orderedText struct {
	order *[]string
	facts []ExtractedFact
}

func (t *orderedText) Extract(context.Context, VehicleSignature, DocumentChunk) (Extraction, error) {
	*t.order = append(*t.order, "text")
	return Extraction{Facts: append([]ExtractedFact(nil), t.facts...)}, nil
}

func TestSearchUnloadsBeforeTextModel(t *testing.T) {
	forum := "https://www.drive2.ru/l/kia-k3/"
	km := 5000
	var order []string
	searcher := &orderedSearch{
		order: &order,
		steps: []ResearchStep{{Action: "search", Query: "регламент kia k3"}, {Action: "open", URL: forum}, {Action: "finish"}},
	}
	text := &orderedText{order: &order, facts: []ExtractedFact{{
		ComponentCode: "engine_oil", Operation: "replace", IntervalKM: &km,
		ScheduleMode: "mileage", UsageMode: "unknown", Evidence: "масло меняют на 5000 км",
	}}}
	pipe := &KnowledgePipeline{
		Search:     &recordingSearch{hits: []SearchResult{{URL: forum, Title: "отзыв"}}},
		Fetch:      &pageFetch{pages: map[string]string{forum: forumPage}},
		Select:     wholeSelect{},
		Extract:    text,
		Reader:     searcher,
		Release:    searcher.Release,
		MaxSources: 3,
		Seek:       true,
	}
	rules, err := pipe.Rules(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130})
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Code != "engine_oil" || rules[0].IntervalKm != 5000 {
		t.Fatalf("%+v", rules)
	}
	joined := strings.Join(order, ",")
	if !strings.Contains(joined, "search") || !strings.Contains(joined, "unload,text") {
		t.Fatalf("order %s", joined)
	}
	unload := strings.Index(joined, "unload")
	read := strings.Index(joined, "text")
	if unload < 0 || read < 0 || unload > read {
		t.Fatalf("text ran before unload: %s", joined)
	}
}

func TestQuestionUsesSearchModel(t *testing.T) {
	var order []string
	searcher := &orderedSearch{order: &order}
	pipe := &KnowledgePipeline{
		Search:  &recordingSearch{hits: []SearchResult{{URL: "https://auto.ru/catalog/cars/kia/k3/reglament-to/", Title: "регламент"}}},
		Fetch:   &pageFetch{pages: map[string]string{"https://auto.ru/catalog/cars/kia/k3/reglament-to/": "Свечи зажигания 45 тыс. км"}},
		Reader:  searcher,
		Extract: &orderedText{order: &order},
	}
	reply, err := pipe.Ask(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130}, "какой интервал у свечей")
	if err != nil {
		t.Fatal(err)
	}
	if reply.Text != "45 тыс. км" || strings.Join(order, ",") != "answer" {
		t.Fatalf("reply %#v order %v", reply, order)
	}
}

func TestTextDoesNotStartWhileSearchStaysLoaded(t *testing.T) {
	forum := "https://www.drive2.ru/l/kia-k3/"
	var order []string
	searcher := &orderedSearch{
		order:      &order,
		releaseErr: errors.New("still loaded"),
		steps:      []ResearchStep{{Action: "search", Query: "регламент kia k3"}, {Action: "open", URL: forum}, {Action: "finish"}},
	}
	pipe := &KnowledgePipeline{
		Search:     &recordingSearch{hits: []SearchResult{{URL: forum, Title: "отзыв"}}},
		Fetch:      &pageFetch{pages: map[string]string{forum: forumPage}},
		Select:     wholeSelect{},
		Extract:    &orderedText{order: &order},
		Reader:     searcher,
		Release:    searcher.Release,
		MaxSources: 3,
		Seek:       true,
	}
	_, err := pipe.Rules(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130})
	if err == nil || !strings.Contains(err.Error(), "still loaded") {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "search,search,search,unload" {
		t.Fatalf("order %v", order)
	}
}
