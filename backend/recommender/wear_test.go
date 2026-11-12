package recommender

import (
	"strings"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
)

func TestWearCheckAppearsWhenScheduleSkipsPads(t *testing.T) {
	v := domain.Vehicle{ID: "v1", CurrentOdometer: 50000}
	rules := []domain.Rule{{Code: "engine_oil", Title: "Масло", IntervalKm: 15000}}
	alerts := withWearChecks(v, rules, nil, nil)
	if len(alerts) != 1 {
		t.Fatalf("ожидали одну карточку осмотра, получили %d", len(alerts))
	}
	a := alerts[0]
	if a.Type != domain.AlertMaintenanceInspect || a.RuleCode != "brake_pads" || a.DueAtKm != 0 {
		t.Fatalf("карточка %+v", a)
	}
	if a.Title != "Тормозные колодки" || strings.Contains(a.Description, "15000") {
		t.Fatalf("текст не должен подставлять чужой пробег: %+v", a)
	}
}

func TestWearCheckStaysOutWhenPadsHaveAnInterval(t *testing.T) {
	v := domain.Vehicle{ID: "v1"}
	rules := []domain.Rule{
		{Code: "engine_oil", IntervalKm: 15000},
		{Code: "brake_pads_front", Title: "Передние колодки", IntervalKm: 40000},
	}
	if alerts := withWearChecks(v, rules, nil, nil); len(alerts) != 0 {
		t.Fatalf("свой срок колодок не дополняем, получили %+v", alerts)
	}
}

func TestWearCheckStaysOutWithoutASchedule(t *testing.T) {
	v := domain.Vehicle{ID: "v1"}
	if alerts := withWearChecks(v, nil, nil, nil); len(alerts) != 0 {
		t.Fatalf("без регламента осмотр не выдумываем, получили %+v", alerts)
	}
}

func TestWearCheckRemembersTheBook(t *testing.T) {
	v := domain.Vehicle{ID: "v1"}
	rules := []domain.Rule{{Code: "engine_oil", IntervalKm: 15000}}
	history := []domain.ServiceEvent{{RuleCode: "brake_pads_front", Odometer: 45000}}
	alerts := withWearChecks(v, rules, history, nil)
	if len(alerts) != 1 || !strings.Contains(alerts[0].Description, "45000") {
		t.Fatalf("запись книжки не попала в текст: %+v", alerts)
	}
}
