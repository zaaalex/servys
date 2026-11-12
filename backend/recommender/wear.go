package recommender

import (
	"fmt"

	"github.com/zaaalex/servys/backend/domain"
)

// withWearChecks добавляет осмотр колодок, если найденный регламент задаёт
// другие сроки, но не называет колодки. Пробег замены не выдумывается.
func withWearChecks(v domain.Vehicle, rules []domain.Rule, history []domain.ServiceEvent, alerts []domain.Alert) []domain.Alert {
	if !scheduleFound(rules) || padsNamed(rules) {
		return alerts
	}
	desc := "В найденном регламенте нет пробега для замены. На каждом ТО смотрят толщину накладки и меняют её, когда осталось около 2 мм."
	if km := lastPadKm(history); km > 0 {
		desc += fmt.Sprintf(" Последняя запись в книжке: %d км.", km)
	}
	return append(alerts, domain.Alert{
		ID:          fmt.Sprintf("%s:brake_pads", v.ID),
		VehicleID:   v.ID,
		RuleCode:    "brake_pads",
		Type:        domain.AlertMaintenanceInspect,
		Severity:    domain.SeverityLow,
		Title:       "Тормозные колодки",
		Description: desc,
		Category:    domain.CategoryPrimary,
		Basis:       "found",
	})
}

func scheduleFound(rules []domain.Rule) bool {
	for _, r := range rules {
		if r.IntervalKm > 0 {
			return true
		}
		if r.Community != nil && r.Community.RealIntervalKm > 0 {
			return true
		}
	}
	return false
}

func padsNamed(rules []domain.Rule) bool {
	for _, r := range rules {
		switch r.Code {
		case "brake_pads", "brake_pads_front", "brake_pads_rear":
			return true
		}
	}
	return false
}

func lastPadKm(history []domain.ServiceEvent) int {
	km := 0
	for _, e := range history {
		switch e.RuleCode {
		case "brake_pads", "brake_pads_front", "brake_pads_rear":
			if e.Odometer > km {
				km = e.Odometer
			}
		}
	}
	return km
}
