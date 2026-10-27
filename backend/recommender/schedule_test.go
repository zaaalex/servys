package recommender

import (
	"strings"
	"testing"
)

func TestScheduleRowsFromReglamentTable(t *testing.T) {
	text := "Под замену Пробег, тыс. км Срок эксплуатации, лет Антифриз 45 3 Воздушный фильтр 45 3 Масло в АКПП 90 6 Масляный фильтр 15 1 Моторное масло 15 1 Прокладка сливной пробки 15 1 Свечи зажигания 45 3 Топливный фильтр 45 3 Фильтр салона 15 1"
	facts := ScheduleRows(text)
	got := map[string]int{}
	for _, f := range facts {
		if f.IntervalKM == nil {
			t.Fatalf("no km %+v", f)
		}
		got[f.ComponentCode] = *f.IntervalKM
	}
	want := map[string]int{
		"engine_coolant":     45000,
		"engine_air_filter":  45000,
		"transmission_fluid": 90000,
		"engine_oil_filter":  15000,
		"engine_oil":         15000,
		"spark_plugs":        45000,
		"fuel_filter":        45000,
		"cabin_filter":       15000,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for code, km := range want {
		if got[code] != km {
			t.Fatalf("%s: %d want %d", code, got[code], km)
		}
	}
	for _, f := range facts {
		if f.ComponentCode == "engine_oil" && (f.IntervalMonths == nil || *f.IntervalMonths != 12) {
			t.Fatalf("oil months %+v", f)
		}
	}
}

func TestScheduleRowsIgnoreOwnerReview(t *testing.T) {
	text := "Масло меняю раз в 5 тыс км. За 130 т.км масло, фильтра, колодки."
	if rows := ScheduleRows(text); len(rows) != 0 {
		t.Fatalf("review produced %d rows", len(rows))
	}
}

func TestNormalizeHTMLKeepsScheduleTable(t *testing.T) {
	raw := `<head><script>var x=1</script></head><header><nav>menu</nav></header><table><tr><td>Под замену</td><td>Пробег, тыс. км</td></tr><tr><td>Моторное масло</td><td>15</td><td>1</td></tr></table>`
	got := NormalizeHTML(raw)
	if !strings.Contains(got, "Моторное масло") || !strings.Contains(got, "Под замену") {
		t.Fatalf("table dropped: %q", got)
	}
	if strings.Contains(got, "var x") {
		t.Fatalf("script kept: %q", got)
	}
	facts := ScheduleRows(got)
	if len(facts) != 1 || facts[0].ComponentCode != "engine_oil" || facts[0].IntervalKM == nil || *facts[0].IntervalKM != 15000 {
		t.Fatalf("%+v", facts)
	}
}

func TestPreferSchedulePutsCatalogFirst(t *testing.T) {
	got := preferSchedule(VehicleSignature{Make: "KIA", Model: "K3"}, []SearchResult{
		{URL: "https://www.drom.ru/reviews/kia/k3/5kopeek/"},
		{URL: "https://auto.ru/catalog/cars/kia/k3/reglament-to/"},
	})
	if len(got) != 2 || got[0].URL != "https://auto.ru/catalog/cars/kia/k3/reglament-to/" {
		t.Fatalf("%+v", got)
	}
}

func TestScheduleNote(t *testing.T) {
	if got := scheduleNote(15000, 12); got != "По регламенту: 15 тыс. км или 1 год" {
		t.Fatalf("%q", got)
	}
}
