package engine

import (
	"testing"

	"github.com/zaaalex/servys/backend/domain"
)

// Пороги ADR-001 §5.6:
//   current < next_due - lead        => OK
//   current >= next_due - lead       => MAINTENANCE_SOON
//   current >= next_due              => MAINTENANCE_DUE
//   current >= next_due + overdue    => MAINTENANCE_OVERDUE
func TestEvaluateByOdometer(t *testing.T) {
	const (
		nextDue = 10000
		lead    = 500
		overdue = 1000
	)
	cases := []struct {
		name    string
		current int
		want    string
	}{
		{"далеко до ТО", 5000, "OK"},
		{"граница OK/SOON снизу", 9499, "OK"},
		{"начало окна SOON", 9500, domain.AlertMaintenanceSoon},
		{"в окне SOON", 9999, domain.AlertMaintenanceSoon},
		{"наступило DUE", 10000, domain.AlertMaintenanceDue},
		{"ещё DUE до порога overdue", 10999, domain.AlertMaintenanceDue},
		{"перешло в OVERDUE", 11000, domain.AlertMaintenanceOverdue},
		{"глубоко OVERDUE", 20000, domain.AlertMaintenanceOverdue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EvaluateByOdometer(c.current, nextDue, lead, overdue)
			if got != c.want {
				t.Fatalf("EvaluateByOdometer(%d, %d, %d, %d) = %q, ожидалось %q",
					c.current, nextDue, lead, overdue, got, c.want)
			}
		})
	}
}

func TestBuildAlertsEmitsOK(t *testing.T) {
	// Полный чек-лист: «в норме» теперь эмитится как MAINTENANCE_OK (severity low).
	v := domain.Vehicle{ID: "v1", CurrentOdometer: 1000}
	rules := []domain.Rule{
		{Code: "engine_oil", Title: "Масло", IntervalKm: 10000, LeadKm: 500}, // 1000 < 9500 => OK
	}
	alerts := BuildAlerts(v, rules, nil)
	if len(alerts) != 1 {
		t.Fatalf("для OK-правила ожидали 1 alert (чек-лист), получили %d", len(alerts))
	}
	if alerts[0].Type != domain.AlertMaintenanceOK {
		t.Fatalf("ожидали тип %q, получили %q", domain.AlertMaintenanceOK, alerts[0].Type)
	}
	if alerts[0].Severity != domain.SeverityLow {
		t.Fatalf("OK-alert должен быть severity low, получили %q", alerts[0].Severity)
	}
}

func TestBuildAlertsUsesCommunityInterval(t *testing.T) {
	// Правило без регламента, но с отзывным интервалом — статус считается по community.
	v := domain.Vehicle{ID: "v1", CurrentOdometer: 80000}
	rules := []domain.Rule{
		{Code: "engine_mounts", Title: "Подушки двигателя", IntervalKm: 0,
			Community: &domain.CommunityNote{RealIntervalKm: 90000}},
	}
	alerts := BuildAlerts(v, rules, nil)
	if len(alerts) != 1 {
		t.Fatalf("ожидали 1 alert по отзывному интервалу, получили %d", len(alerts))
	}
	if alerts[0].DueAtKm != 90000 {
		t.Fatalf("DueAtKm должен браться из community (90000), получили %d", alerts[0].DueAtKm)
	}
	if alerts[0].Community == nil {
		t.Fatalf("community должен прокидываться в alert")
	}
}

func TestBuildAlertsSkipsWithoutAnyInterval(t *testing.T) {
	// Ни регламента, ни отзывного интервала — по-км не считаем.
	v := domain.Vehicle{ID: "v1", CurrentOdometer: 80000}
	rules := []domain.Rule{{Code: "x", IntervalKm: 0}}
	if alerts := BuildAlerts(v, rules, nil); len(alerts) != 0 {
		t.Fatalf("без интервала alert не создаём, получили %d", len(alerts))
	}
}

func TestBuildAlertsEmitsDue(t *testing.T) {
	v := domain.Vehicle{ID: "v1", CurrentOdometer: 10500}
	rules := []domain.Rule{
		{Code: "engine_oil", Title: "Масло", IntervalKm: 10000, LeadKm: 500}, // due (10000..10999)
	}
	alerts := BuildAlerts(v, rules, nil)
	if len(alerts) != 1 {
		t.Fatalf("ожидали 1 alert, получили %d", len(alerts))
	}
	if alerts[0].Type != domain.AlertMaintenanceDue {
		t.Fatalf("ожидали тип %q, получили %q", domain.AlertMaintenanceDue, alerts[0].Type)
	}
	if alerts[0].VehicleID != "v1" || alerts[0].RuleCode != "engine_oil" {
		t.Fatalf("alert должен ссылаться на авто и правило, получили %+v", alerts[0])
	}
}

func TestBuildAlertsHistoryShiftsDue(t *testing.T) {
	// baseline из истории: свежее ТО на 80 000 км отодвигает срок → next_due = 80000+10000 = 90000,
	// при пробеге 80 000 (lead 500) это OK. Без истории то же правило было бы OVERDUE.
	v := domain.Vehicle{ID: "v1", CurrentOdometer: 80000}
	rules := []domain.Rule{{Code: "engine_oil", Title: "Масло", IntervalKm: 10000, LeadKm: 500}}

	overdue := BuildAlerts(v, rules, nil)
	if overdue[0].Type != domain.AlertMaintenanceOverdue {
		t.Fatalf("без истории ожидали OVERDUE, получили %q", overdue[0].Type)
	}

	// две записи по компоненту — берётся максимальный пробег (свежее ТО).
	history := []domain.ServiceEvent{
		{RuleCode: "engine_oil", Odometer: 40000},
		{RuleCode: "engine_oil", Odometer: 80000},
	}
	done := BuildAlerts(v, rules, history)
	if done[0].DueAtKm != 90000 {
		t.Fatalf("DueAtKm должен быть baseline(80000)+интервал(10000)=90000, получили %d", done[0].DueAtKm)
	}
	if done[0].Type != domain.AlertMaintenanceOK {
		t.Fatalf("после ТО на 80 000 км ожидали OK, получили %q", done[0].Type)
	}
}
