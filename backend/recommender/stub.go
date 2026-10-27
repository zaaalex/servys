package recommender

import (
	"context"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/engine"
)

type Stub struct{}

func NewStub() *Stub { return &Stub{} }

func (Stub) Rules(_ context.Context, _ domain.Vehicle) ([]domain.Rule, error) {
	return []domain.Rule{
		{Code: "engine_oil", Title: "Моторное масло", Operation: "replace", IntervalKm: 10000, IntervalMonths: 12, LeadKm: 500, Verified: false, Source: "stub"},
		{Code: "brakes", Title: "Тормозная система", Operation: "inspect", IntervalKm: 30000, LeadKm: 1000, Verified: false, Source: "stub"},
		{Code: "timing_belt", Title: "Ремень ГРМ", Operation: "replace", IntervalKm: 90000, LeadKm: 2000, Verified: false, Source: "stub"},
	}, nil
}

type StubAdvisor struct{ rec Recommender }

func NewStubAdvisor() *StubAdvisor { return &StubAdvisor{rec: NewStub()} }

func (a *StubAdvisor) Alerts(ctx context.Context, v domain.Vehicle, history []domain.ServiceEvent) ([]domain.Alert, error) {
	rules, err := a.rec.Rules(ctx, v)
	if err != nil {
		return nil, err
	}
	return engine.BuildAlerts(v, rules, history), nil
}
