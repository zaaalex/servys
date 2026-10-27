package recommender

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
)

const twoGenerations = "Регламент ТО Kia K3 III 2023 - 2026 Седан 1.6 MT (121 л.с.) 15 Под замену Пробег, тыс. км Срок эксплуатации, лет Моторное масло 15 1 Масляный фильтр 15 1 Фильтр салона 15 1 " +
	"Регламент ТО Kia K3 II (China Market) 2019 - 2022 Седан 1.4 AMT (130 л.с.) 15 Под замену Пробег, тыс. км Срок эксплуатации, лет Моторное масло 5 1 Масляный фильтр 5 1 Фильтр салона 15 1 " +
	"1.5 CVT (115 л.с.) 15 Под замену Пробег, тыс. км Срок эксплуатации, лет Моторное масло 15 1 Масляный фильтр 15 1 Фильтр салона 15 1"

func TestScheduleRowsForPicksChinaPower(t *testing.T) {
	facts, label := ScheduleRowsFor(twoGenerations, ScheduleTarget{Year: 2020, PowerHP: 130, Market: "china"})
	km := map[string]int{}
	for _, f := range facts {
		if f.IntervalKM == nil {
			t.Fatalf("no km %+v", f)
		}
		km[f.ComponentCode] = *f.IntervalKM
	}
	if km["engine_oil"] != 5000 || km["engine_oil_filter"] != 5000 || km["cabin_filter"] != 15000 {
		t.Fatalf("wrong section: %v", km)
	}
	if !strings.Contains(label, "China Market") || !strings.Contains(label, "130") {
		t.Fatalf("label %q", label)
	}
	if strings.Contains(label, "121") {
		t.Fatalf("label names the other generation: %q", label)
	}
}

func TestScheduleRowsForSkipsOtherPower(t *testing.T) {
	facts, label := ScheduleRowsFor(twoGenerations, ScheduleTarget{Year: 2020, PowerHP: 115, Market: "china"})
	km := 0
	for _, f := range facts {
		if f.ComponentCode == "engine_oil" && f.IntervalKM != nil {
			km = *f.IntervalKM
		}
	}
	if km != 15000 || !strings.Contains(label, "115") {
		t.Fatalf("km %d label %q", km, label)
	}
}

func TestScheduleRowsForDoesNotUseFirstTable(t *testing.T) {
	facts, _ := ScheduleRowsFor(twoGenerations, ScheduleTarget{Year: 2020, PowerHP: 121})
	if len(facts) != 0 {
		t.Fatalf("2023 table used for a 2020 car: %+v", facts)
	}
}

func TestScheduleRowsForKeepsOnlySharedRowsWithoutPower(t *testing.T) {
	facts, label := ScheduleRowsFor(twoGenerations, ScheduleTarget{Year: 2020, Market: "china"})
	got := map[string]int{}
	for _, f := range facts {
		if f.IntervalKM != nil {
			got[f.ComponentCode] = *f.IntervalKM
		}
	}
	if _, oil := got["engine_oil"]; oil {
		t.Fatalf("different oil rows were merged: %v", got)
	}
	if got["cabin_filter"] != 15000 {
		t.Fatalf("shared cabin row dropped: %v", got)
	}
	if !strings.Contains(label, "Одинаково") {
		t.Fatalf("label %q", label)
	}
}

func TestSearchFailureStillReadsMatchedSection(t *testing.T) {
	p := &KnowledgePipeline{
		Search:     errSearch{},
		Fetch:      textFetch{text: twoGenerations},
		Select:     quietSelect{},
		Extract:    DisabledExtractor{},
		MaxSources: 1,
	}
	s := VehicleSignature{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130, Market: "china"}
	rules, err := p.Rules(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	oil := 0
	for _, r := range rules {
		if r.Code == "engine_oil" {
			oil = r.IntervalKm
			if r.Community == nil || !strings.Contains(r.Community.Note, "China Market") {
				t.Fatalf("note %+v", r.Community)
			}
		}
	}
	if oil != 5000 {
		t.Fatalf("oil %d", oil)
	}
}

func TestReviewDoesNotReplaceDocumentation(t *testing.T) {
	p := &KnowledgePipeline{
		Search:     reviewSearch{},
		Fetch:      splitFetch{},
		Select:     quietSelect{},
		Extract:    DisabledExtractor{},
		MaxSources: 2,
	}
	rules, err := p.Rules(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130, Market: "china"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules {
		if r.Code == "engine_oil" && r.IntervalKm != 5000 {
			t.Fatalf("review replaced the book: %+v", r)
		}
	}
}

type errSearch struct{}

func (errSearch) Search(context.Context, string) ([]SearchResult, error) {
	return nil, errors.New("search down")
}

type textFetch struct{ text string }

func (f textFetch) Fetch(_ context.Context, r SearchResult) (FetchedDocument, error) {
	return FetchedDocument{URL: r.URL, Text: f.text}, nil
}

type quietSelect struct{}

func (quietSelect) Select(context.Context, VehicleSignature, FetchedDocument) ([]DocumentChunk, error) {
	return nil, nil
}

type reviewSearch struct{}

func (reviewSearch) Search(context.Context, string) ([]SearchResult, error) {
	return []SearchResult{{URL: "https://www.drom.ru/reviews/kia/k3/x/", Title: "отзыв владельца масло"}}, nil
}

type splitFetch struct{}

func (splitFetch) Fetch(_ context.Context, r SearchResult) (FetchedDocument, error) {
	if strings.Contains(r.URL, "auto.ru") {
		return FetchedDocument{URL: r.URL, Text: twoGenerations}, nil
	}
	return FetchedDocument{URL: r.URL, Text: "Масло меняю раз в 7 тыс км."}, nil
}

func TestUnmatchedScheduleDoesNotCallModel(t *testing.T) {
	extract := &countExtract{}
	p := &KnowledgePipeline{
		Search:     errSearch{},
		Fetch:      textFetch{text: twoGenerations},
		Select:     quietSelect{},
		Extract:    extract,
		MaxSources: 1,
	}
	rules, err := p.Rules(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 121})
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 || extract.n != 0 {
		t.Fatalf("rules %d model calls %d", len(rules), extract.n)
	}
}

type countExtract struct{ n int }

func (c *countExtract) Extract(context.Context, VehicleSignature, DocumentChunk) (Extraction, error) {
	c.n++
	return Extraction{}, errors.New("model should not run")
}

func TestMarketFromChinaVIN(t *testing.T) {
	v := domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020, EngineCC: 1353, PowerHP: 130, VIN: "ljd3aa293l0051345"}
	s := signatureFrom(v)
	if s.Market != "china" || s.PowerHP != 130 || s.Year != 2020 {
		t.Fatalf("%+v", s)
	}
}
