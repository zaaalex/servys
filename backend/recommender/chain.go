package recommender

import (
	"context"
	"log"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

type Chain struct{ Steps []Recommender }

func (c Chain) Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error) {
	var last error
	for _, step := range c.Steps {
		if step == nil {
			continue
		}
		rules, err := step.Rules(ctx, v)
		if err != nil {
			last = err
			log.Printf("recommender: шаг пропущен (%s): %v", v.Make+" "+v.Model, err)
			continue
		}
		if len(rules) > 0 {
			return rules, nil
		}
	}
	return nil, last
}

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

type limited struct {
	Inner  Recommender
	Budget time.Duration
}

func (l limited) Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error) {
	if l.Inner == nil {
		return nil, nil
	}
	if l.Budget > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, l.Budget)
		defer cancel()
	}
	rules, err := l.Inner.Rules(ctx, v)
	if err != nil {
		log.Printf("recommender: поиск пропущен (%s): %v", v.Make+" "+v.Model, err)
		return nil, nil
	}
	return rules, nil
}
