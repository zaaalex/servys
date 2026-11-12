package recommender

import (
	"context"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

type budgeted struct {
	Inner  Recommender
	Budget time.Duration
}

func (b budgeted) Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error) {
	if b.Inner == nil {
		return nil, nil
	}
	if b.Budget > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.Budget)
		defer cancel()
	}
	return b.Inner.Rules(ctx, v)
}
