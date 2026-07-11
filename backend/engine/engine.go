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
// baseline (ADR-001 §5.6): пробег последнего подтверждённого ТО по этому компоненту из history;
// next_due_km = baseline + эффективный интервал. Нет истории по компоненту => baseline=0 (отсчёт
// «от нового»), поведение как раньше. Так «Отметить выполненным» реально отодвигает срок.
func BuildAlerts(v domain.Vehicle, rules []domain.Rule, history []domain.ServiceEvent) []domain.Alert {
	baseline := lastServiceByRule(history)
	var alerts []domain.Alert
	for _, r := range rules {
		interval := effectiveInterval(r)
		if interval <= 0 {
			continue // ни регламента, ни отзывного интервала — по-км не считаем
		}
		nextDue := baseline[r.Code] + interval // от последнего ТО, иначе от нового (baseline 0)
		status := EvaluateByOdometer(v.CurrentOdometer, nextDue, r.LeadKm, OverdueMarginKm)
		alertType := status
		if status == "OK" {
			alertType = domain.AlertMaintenanceOK // полный чек-лист: показываем и «в норме»
		}
		desc := fmt.Sprintf("Ориентир: %d км, текущий пробег: %d км", nextDue, v.CurrentOdometer)
		if r.Source != "" && r.Source != "demo" {
			desc += fmt.Sprintf(" (источник: %s)", r.Source)
		}
		alerts = append(alerts, domain.Alert{
			ID:          fmt.Sprintf("%s:%s", v.ID, r.Code),
			VehicleID:   v.ID,
			RuleCode:    r.Code,
			Type:        alertType,
			Severity:    severityFor(status),
			Title:       r.Title,
			Description: desc,
			DueAtKm:     nextDue,
			Category:    r.Category,
			Community:   r.Community,
		})
	}
	return alerts
}

// lastServiceByRule — по истории ТО: rule_code → максимальный пробег подтверждённого выполнения.
// Отсутствие компонента в мапе трактуется вызывающим как baseline 0 (никогда не обслуживали).
func lastServiceByRule(history []domain.ServiceEvent) map[string]int {
	m := make(map[string]int, len(history))
	for _, e := range history {
		if odo, ok := m[e.RuleCode]; !ok || e.Odometer > odo {
			m[e.RuleCode] = e.Odometer
		}
	}
	return m
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
