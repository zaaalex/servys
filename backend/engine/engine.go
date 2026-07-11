// Package engine — движок напоминаний: из правил + пробега считает alerts.
// Владелец: Dev 1. Порог overdue — параметр MVP (ADR-001 §5.6), не универсальное правило.
package engine

import (
	"fmt"

	"github.com/zaaalex/servys/backend/domain"
)

// OverdueMarginKm — насколько выше срока считаем ТО просроченным (MVP-параметр).
const OverdueMarginKm = 1000

// EvaluateByOdometer возвращает статус ТО по текущему пробегу (ADR-001 §5.6).
// "OK" => alert не нужен; иначе одна из domain.AlertMaintenance* констант.
func EvaluateByOdometer(currentKm, nextDueKm, leadKm, overdueMarginKm int) string {
	switch {
	case currentKm >= nextDueKm+overdueMarginKm:
		return domain.AlertMaintenanceOverdue
	case currentKm >= nextDueKm:
		return domain.AlertMaintenanceDue
	case currentKm >= nextDueKm-leadKm:
		return domain.AlertMaintenanceSoon
	default:
		return "OK"
	}
}

// severityFor маппит статус ТО в severity для UI.
func severityFor(status string) domain.Severity {
	switch status {
	case domain.AlertMaintenanceOverdue:
		return domain.SeverityHigh
	case domain.AlertMaintenanceDue:
		return domain.SeverityMedium
	default:
		return domain.SeverityLow
	}
}

// BuildAlerts считает alerts по правилам для авто.
//
// MVP-упрощение: baseline последнего выполнения = 0 (отсчёт «от нового»),
// поэтому next_due_km = interval_km. Когда появятся service_events (ADR-001 §5.6),
// baseline берётся из последнего подтверждённого ТО, а без него — MAINTENANCE_HISTORY_REQUIRED.
func BuildAlerts(v domain.Vehicle, rules []domain.Rule) []domain.Alert {
	var alerts []domain.Alert
	for _, r := range rules {
		if r.IntervalKm <= 0 {
			continue // по времени — не считаем в MVP-скелете
		}
		nextDue := r.IntervalKm
		status := EvaluateByOdometer(v.CurrentOdometer, nextDue, r.LeadKm, OverdueMarginKm)
		if status == "OK" {
			continue
		}
		alerts = append(alerts, domain.Alert{
			ID:          fmt.Sprintf("%s:%s", v.ID, r.Code),
			VehicleID:   v.ID,
			RuleCode:    r.Code,
			Type:        status,
			Severity:    severityFor(status),
			Title:       r.Title,
			Description: fmt.Sprintf("Ориентир: %d км, текущий пробег: %d км (источник: %s)", nextDue, v.CurrentOdometer, r.Source),
			DueAtKm:     nextDue,
		})
	}
	return alerts
}
