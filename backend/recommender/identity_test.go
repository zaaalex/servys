package recommender

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
)

// Две таблицы одного года и таблица другого года. Числа нарочно разные,
// чтобы чтение чужого поколения было видно по пробегу.
const camryPage = "Регламент ТО Toyota Camry 2018 - 2021 Седан 2.5 AT (181 л.с.) 10 Под замену Пробег, тыс. км Срок эксплуатации, лет Моторное масло 10 1 Топливный фильтр 40 3 Фильтр салона 15 1 " +
	"2.0 AT (150 л.с.) 10 Под замену Пробег, тыс. км Срок эксплуатации, лет Моторное масло 10 1 Топливный фильтр 30 3 Фильтр салона 15 1 " +
	"Регламент ТО Toyota Camry 2011 - 2014 Седан 2.5 AT (181 л.с.) 10 Под замену Пробег, тыс. км Срок эксплуатации, лет Моторное масло 7 1 Топливный фильтр 40 3 Фильтр салона 20 1"

const k3Page = "Регламент ТО Kia K3 II (China Market) 2019 - 2022 Седан 1.4 AMT (130 л.с.) 15 Под замену Пробег, тыс. км Срок эксплуатации, лет Моторное масло 15 1 Масляный фильтр 15 1 Фильтр салона 15 1"

func TestScheduleReaderMatchesYearOnAnyMake(t *testing.T) {
	t.Run("power picks the entered year", func(t *testing.T) {
		facts, _ := ScheduleRowsFor(camryPage, ScheduleTarget{Year: 2019, PowerHP: 181})
		got := intervals(t, facts)
		if got["engine_oil"] != 10000 || got["fuel_filter"] != 40000 || got["cabin_filter"] != 15000 {
			t.Fatalf("2019 181 hp: %v", got)
		}
		if got["engine_oil"] == 7000 || got["cabin_filter"] == 20000 {
			t.Fatalf("older table leaked: %v", got)
		}
	})
	t.Run("unknown power keeps only shared rows", func(t *testing.T) {
		facts, _ := ScheduleRowsFor(camryPage, ScheduleTarget{Year: 2019})
		got := intervals(t, facts)
		if got["engine_oil"] != 10000 || got["cabin_filter"] != 15000 {
			t.Fatalf("shared rows: %v", got)
		}
		if _, ok := got["fuel_filter"]; ok {
			t.Fatalf("fuel filter differs across modifications: %v", got)
		}
		if got["cabin_filter"] == 20000 || got["engine_oil"] == 7000 {
			t.Fatalf("other year leaked: %v", got)
		}
	})
	t.Run("other year is a different table", func(t *testing.T) {
		facts, _ := ScheduleRowsFor(camryPage, ScheduleTarget{Year: 2012, PowerHP: 181})
		got := intervals(t, facts)
		if got["engine_oil"] != 7000 || got["cabin_filter"] != 20000 {
			t.Fatalf("2012: %v", got)
		}
		if got["engine_oil"] == 10000 {
			t.Fatalf("2018 table used for 2012: %v", got)
		}
	})
}

func TestCatalogAndQueriesFollowTheEnteredCar(t *testing.T) {
	s := VehicleSignature{Make: "Toyota", Model: "Camry", Year: 2019, PowerHP: 181}
	page := catalogScheduleURL(s)
	if page != "https://auto.ru/catalog/cars/toyota/camry/reglament-to/" {
		t.Fatalf("address %s", page)
	}
	if strings.Contains(page, "/kia/") {
		t.Fatalf("address is a fixed Kia path: %s", page)
	}
	queries := PlanQueries(s)
	if len(queries) == 0 {
		t.Fatal("no queries")
	}
	for _, q := range queries {
		if !strings.Contains(q, "Toyota Camry 2019") {
			t.Fatalf("query does not name the car: %q", q)
		}
	}
	if !strings.Contains(queries[0], "регламент") {
		t.Fatalf("reviews are ahead of the maintenance page: %q", queries[0])
	}
	if strings.Contains(queries[0], "отзывы") {
		t.Fatalf("first query is a review search: %q", queries[0])
	}
	found := preferSchedule(s, []SearchResult{{URL: "https://www.drom.ru/reviews/toyota/camry/1/"}})
	if len(found) < 2 || found[0].URL != page {
		t.Fatalf("catalog is not first: %+v", found)
	}
}

func TestManualAndVINBothReadTheSameLookup(t *testing.T) {
	fetch := &pageFetch{pages: map[string]string{
		"https://auto.ru/catalog/cars/toyota/camry/reglament-to/": camryPage,
		"https://auto.ru/catalog/cars/kia/k3/reglament-to/":       k3Page,
	}}
	pipe := &KnowledgePipeline{
		Search:     emptySearch{},
		Fetch:      fetch,
		Select:     quietSelect{},
		Extract:    DisabledExtractor{},
		MaxSources: 1,
	}
	memo := newRemembering(pipe, 0, 0, "")
	manual := domain.Vehicle{Make: "Toyota", Model: "Camry", Year: 2019, PowerHP: 181}
	vinCar := domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020, EngineCC: 1353, PowerHP: 130, VIN: "LJD3AA293L0051345"}

	manualRules, err := memo.Rules(context.Background(), manual)
	if err != nil {
		t.Fatal(err)
	}
	vinRules, err := memo.Rules(context.Background(), vinCar)
	if err != nil {
		t.Fatal(err)
	}
	if intervalsOf(manualRules)["engine_oil"] != 10000 {
		t.Fatalf("manual car: %+v", manualRules)
	}
	if intervalsOf(vinRules)["engine_oil"] != 15000 {
		t.Fatalf("vin car: %+v", vinRules)
	}
	for _, rule := range append(manualRules, vinRules...) {
		if rule.Title == "" || rule.IntervalKm <= 0 || rule.Source == "" {
			t.Fatalf("part without a name or interval: %+v", rule)
		}
	}
	seen := fetch.urls()
	if !strings.Contains(seen, "toyota/camry") || !strings.Contains(seen, "kia/k3") {
		t.Fatalf("pages read: %v", seen)
	}
}

func intervals(t *testing.T, facts []ExtractedFact) map[string]int {
	t.Helper()
	got := map[string]int{}
	for _, f := range facts {
		if f.IntervalKM == nil {
			t.Fatalf("no interval %+v", f)
		}
		got[f.ComponentCode] = *f.IntervalKM
	}
	return got
}

func intervalsOf(rules []domain.Rule) map[string]int {
	got := map[string]int{}
	for _, rule := range rules {
		got[rule.Code] = rule.IntervalKm
	}
	return got
}

type pageFetch struct {
	mu    sync.Mutex
	pages map[string]string
	got   []string
}

func (f *pageFetch) Fetch(_ context.Context, r SearchResult) (FetchedDocument, error) {
	f.mu.Lock()
	f.got = append(f.got, r.URL)
	f.mu.Unlock()
	text, ok := f.pages[r.URL]
	if !ok {
		return FetchedDocument{}, errNotPage
	}
	return FetchedDocument{URL: r.URL, Text: text}, nil
}

func (f *pageFetch) urls() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(append([]string(nil), f.got...), " ")
}

var errNotPage = errString("page was not requested for this car")

type errString string

func (e errString) Error() string { return string(e) }

type emptySearch struct{}

func (emptySearch) Search(context.Context, string) ([]SearchResult, error) { return nil, nil }
