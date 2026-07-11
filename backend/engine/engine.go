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
// Полный чек-лист: эмитим статус по КАЖДОМУ применимому правилу, включая «в норме»
// (Type=MAINTENANCE_OK, severity low) — чтобы витрина показывала все запчасти, а не только
// требующие внимания.
//
// Эффективный интервал: регламентный (IntervalKm), иначе отзывной (Community.RealIntervalKm) —
// «народные» данные подсвечиваются как alert, когда официального интервала нет. Ни того, ни
// другого — правило по-км не считаем (напр. интервал только по времени).
//
// MVP-упрощение: baseline последнего выполнения = 0 (отсчёт «от нового»), поэтому
// next_due_km = эффективный интервал. Когда появятся service_events (ADR-001 §5.6),
// baseline берётся из последнего подтверждённого ТО.
func BuildAlerts(v domain.Vehicle, rules []domain.Rule) []domain.Alert {
	var alerts []domain.Alert
	for _, r := range rules {
		nextDue := effectiveInterval(r)
		if nextDue <= 0 {
			continue // ни регламента, ни отзывного интервала — по-км не считаем
		}
		status := EvaluateByOdometer(v.CurrentOdometer, nextDue, r.LeadKm, OverdueMarginKm)
		alertType := status
		if status == "OK" {
			alertType = domain.AlertMaintenanceOK // полный чек-лист: показываем и «в норме»
		}
		alerts = append(alerts, domain.Alert{
			ID:          fmt.Sprintf("%s:%s", v.ID, r.Code),
			VehicleID:   v.ID,
			RuleCode:    r.Code,
			Type:        alertType,
			Severity:    severityFor(status),
			Title:       r.Title,
			Description: fmt.Sprintf("Ориентир: %d км, текущий пробег: %d км (источник: %s)", nextDue, v.CurrentOdometer, r.Source),
			DueAtKm:     nextDue,
			Category:    r.Category,
			Community:   r.Community,
		})
	}
	return alerts
}

// effectiveInterval — регламентный интервал, иначе отзывной (community), иначе 0.
func effectiveInterval(r domain.Rule) int {
	if r.IntervalKm > 0 {
		return r.IntervalKm
	}
	if r.Community != nil && r.Community.RealIntervalKm > 0 {
		return r.Community.RealIntervalKm
	}
	return 0
}
