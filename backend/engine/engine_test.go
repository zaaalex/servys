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

func TestBuildAlertsSkipsOK(t *testing.T) {
	v := domain.Vehicle{ID: "v1", CurrentOdometer: 1000}
	rules := []domain.Rule{
		{Code: "engine_oil", Title: "Масло", IntervalKm: 10000, LeadKm: 500}, // 1000 < 9500 => OK, без alert
	}
	alerts := BuildAlerts(v, rules)
	if len(alerts) != 0 {
		t.Fatalf("для OK-правила alert не создаём, получили %d", len(alerts))
	}
}

func TestBuildAlertsEmitsDue(t *testing.T) {
	v := domain.Vehicle{ID: "v1", CurrentOdometer: 10500}
	rules := []domain.Rule{
		{Code: "engine_oil", Title: "Масло", IntervalKm: 10000, LeadKm: 500}, // due (10000..10999)
	}
	alerts := BuildAlerts(v, rules)
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
