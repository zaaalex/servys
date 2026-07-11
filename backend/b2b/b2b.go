// Package b2b — оркестрация b2b-сценария удержания: скан автопарка СТО из CRM →
// выявление подошедшего ТО (через рекомендательный слой) → ретеншн-дела в CRM (идемпотентно).
// Владелец: Dev 1. Порты реализуют bitrix (Fleet/Retention) и store (Dedupe); Advisor — Dev 3.
package b2b

import (
	"context"
	"fmt"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/recommender"
)

// FleetSource — автопарк клиентов тенанта (реализует bitrix через CRM).
type FleetSource interface {
	Fleet(ctx context.Context, sc domain.ServiceCenter) ([]domain.ClientCar, error)
}

// Retention — создание ретеншн-действия (реализует bitrix: дело на контакте в CRM).
type Retention interface {
	// Одно ретеншн-действие на клиента: все подошедшие позиции ТО — в одном деле.
	Push(ctx context.Context, sc domain.ServiceCenter, cc domain.ClientCar, alerts []domain.Alert) (remoteID string, err error)
}

// DedupeStore — защита от повторного создания (реализует store).
type DedupeStore interface {
	AlreadyPushed(ctx context.Context, tenantID, dedupeKey string) (bool, error)
	RecordPush(ctx context.Context, tenantID, dedupeKey, remoteID string) error
}

// Service — b2b-оркестратор.
type Service struct {
	Fleet     FleetSource
	Advisor   recommender.Advisor // тот же движок рекомендаций, что и в b2c
	Retention Retention
	Dedupe    DedupeStore
}

// Report — итог скана.
type Report struct {
	Cars    int      `json:"cars"`
	Due     int      `json:"due_items"`
	Pushed  int      `json:"pushed"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

// ScanAndPush: автопарк СТО → alerts по каждому авто → на «подошедшие» создаём ретеншн-дело
// в CRM (идемпотентно). Ошибки по отдельному авто/действию не роняют весь скан.
func (s *Service) ScanAndPush(ctx context.Context, sc domain.ServiceCenter) (Report, error) {
	cars, err := s.Fleet.Fleet(ctx, sc)
	if err != nil {
		return Report{}, fmt.Errorf("fleet: %w", err)
	}
	rep := Report{Cars: len(cars)}
	for _, cc := range cars {
		alerts, err := s.Advisor.Alerts(ctx, cc.AsVehicle(), nil)
		if err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("advisor %s %s: %v", cc.Make, cc.Model, err))
			continue
		}
		// собираем все подошедшие позиции клиента → одно дело на клиента (не по каждой позиции)
		var worthy []domain.Alert
		for _, a := range alerts {
			if retentionWorthy(a) {
				worthy = append(worthy, a)
			}
		}
		if len(worthy) == 0 {
			continue
		}
		rep.Due += len(worthy)
		key := dedupeKey(cc)
		done, err := s.Dedupe.AlreadyPushed(ctx, sc.ID, key)
		if err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("dedupe %s: %v", key, err))
			continue
		}
		if done {
			rep.Skipped++
			continue
		}
		remoteID, err := s.Retention.Push(ctx, sc, cc, worthy)
		if err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("push %s: %v", key, err))
			continue
		}
		if err := s.Dedupe.RecordPush(ctx, sc.ID, key, remoteID); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("record %s: %v", key, err))
		}
		rep.Pushed++
	}
	return rep, nil
}

// retentionWorthy — статусы, на которые стоит инициировать контакт с клиентом.
func retentionWorthy(a domain.Alert) bool {
	switch a.Type {
	case domain.AlertMaintenanceSoon, domain.AlertMaintenanceDue, domain.AlertMaintenanceOverdue:
		return true
	default:
		return false
	}
}

func dedupeKey(cc domain.ClientCar) string {
	return fmt.Sprintf("%d|%s %s", cc.CRMContactID, cc.Make, cc.Model)
}
