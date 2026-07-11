// Package recommender — рекомендационный слой (порт + стаб).
// Владелец боевой реализации: Dev 3 (YAML-правила + LLM/Claude).
package recommender

import (
	"context"

	"github.com/zaaalex/servys/backend/domain"
)

// Recommender — источник знаний: правила регламента для конкретного авто.
// Пустой результат => REGULATION_NOT_FOUND (не выдумываем интервалы, ADR-001 §5.5).
// Движок напоминаний (пакет engine) превращает правила + пробег в Alert.
type Recommender interface {
	Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error)
}

// Stub — временная заглушка Dev 1, чтобы бэкенд компилировался и демонстрировался.
// Dev 3 заменит на YAML+LLM. Возвращает базовый демо-набор правил.
type Stub struct{}

func NewStub() *Stub { return &Stub{} }

func (Stub) Rules(_ context.Context, _ domain.Vehicle) ([]domain.Rule, error) {
	return []domain.Rule{
		{Code: "engine_oil", Title: "Моторное масло", Operation: "replace", IntervalKm: 10000, IntervalMonths: 12, LeadKm: 500, Verified: false, Source: "stub"},
		{Code: "brakes", Title: "Тормозная система", Operation: "inspect", IntervalKm: 30000, LeadKm: 1000, Verified: false, Source: "stub"},
		{Code: "timing_belt", Title: "Ремень ГРМ", Operation: "replace", IntervalKm: 90000, LeadKm: 2000, Verified: false, Source: "stub"},
	}, nil
}
