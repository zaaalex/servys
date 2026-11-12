// Package recommender превращает авто в правила и сроки обслуживания.
package recommender

import (
	"context"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/engine"
)

type Recommender interface {
	Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error)
}

type Advisor interface {
	Alerts(ctx context.Context, v domain.Vehicle, history []domain.ServiceEvent) ([]domain.Alert, error)
}

type advisor struct{ rec Recommender }

func NewAdvisor(r Recommender) Advisor { return &advisor{r} }

func (a *advisor) Alerts(ctx context.Context, v domain.Vehicle, h []domain.ServiceEvent) ([]domain.Alert, error) {
	rules, err := a.rec.Rules(ctx, v)
	if err != nil {
		return nil, err
	}
	return withWearChecks(v, rules, h, engine.BuildAlerts(v, rules, h)), nil
}
