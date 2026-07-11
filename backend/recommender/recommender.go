// Package recommender — рекомендационный слой (порт + стаб).
// Владелец боевой реализации: Dev 3 (YAML-правила + LLM/Claude).
package recommender

import (
	"context"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/engine"
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

// Advisor — ЕДИНЫЙ шов между платформой (Dev 1) и рекомендательным слоем (Dev 3).
// `api` (Dev 1) зовёт только это: по авто → готовые alerts. Внутренности (правила, движок,
// LLM, источники) — целиком за Dev 3. Пусто правил => Dev 3 отдаёт alert REGULATION_NOT_FOUND.
type Advisor interface {
	Alerts(ctx context.Context, v domain.Vehicle) ([]domain.Alert, error)
}

// StubAdvisor — заглушка Dev 1 для компиляции/демо: правила из Stub + движок BuildAlerts.
// Dev 3 заменит на боевую реализацию (YAML/LLM + свой engine). См. backend/recommender/README.md.
type StubAdvisor struct{ rec Recommender }

func NewStubAdvisor() *StubAdvisor { return &StubAdvisor{rec: NewStub()} }

func (a *StubAdvisor) Alerts(ctx context.Context, v domain.Vehicle) ([]domain.Alert, error) {
	rules, err := a.rec.Rules(ctx, v)
	if err != nil {
		return nil, err
	}
	return engine.BuildAlerts(v, rules), nil
}
