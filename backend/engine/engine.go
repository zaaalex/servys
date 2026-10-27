// Package engine считает срок обслуживания по пробегу и правилам.
package engine

import (
	"fmt"

	"github.com/zaaalex/servys/backend/domain"
)

const OverdueMarginKm = 1000

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

func BuildAlerts(v domain.Vehicle, rules []domain.Rule, history []domain.ServiceEvent) []domain.Alert {
	baseline := lastServiceByRule(history)
	var alerts []domain.Alert
	for _, r := range rules {
		interval := effectiveInterval(r)
		if interval <= 0 {
			continue
		}
		nextDue := baseline[r.Code] + interval
		status := EvaluateByOdometer(v.CurrentOdometer, nextDue, r.LeadKm, OverdueMarginKm)
		alertType := status
		if status == "OK" {
			alertType = domain.AlertMaintenanceOK
		}
		desc := fmt.Sprintf("Ориентир: %d км, текущий пробег: %d км", nextDue, v.CurrentOdometer)
		if r.Source != "" && r.Source != "demo" && r.Source != "typical" && r.Source != "stub" {
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
			Basis:       basisFor(r.Source),
		})
	}
	return alerts
}

func lastServiceByRule(history []domain.ServiceEvent) map[string]int {
	m := make(map[string]int, len(history))
	for _, e := range history {
		if odo, ok := m[e.RuleCode]; !ok || e.Odometer > odo {
			m[e.RuleCode] = e.Odometer
		}
	}
	return m
}

func basisFor(source string) string {
	switch source {
	case "typical":
		return "typical"
	case "demo", "", "stub":
		return "profile"
	default:
		return "found"
	}
}

func effectiveInterval(r domain.Rule) int {
	if r.IntervalKm > 0 {
		return r.IntervalKm
	}
	if r.Community != nil && r.Community.RealIntervalKm > 0 {
		return r.Community.RealIntervalKm
	}
	return 0
}
