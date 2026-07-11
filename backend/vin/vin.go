// Package vin — VINProvider (порт + стаб).
// Владелец боевой реализации: Dev 3 (адаптер Drom, best-effort).
package vin

import (
	"context"
	"errors"

	"github.com/zaaalex/servys/backend/domain"
)

// Типизированные ошибки провайдера (ADR-001 §5.4). При любой — фронт открывает ручную форму.
var (
	ErrInvalidVIN          = errors.New("INVALID_VIN")
	ErrNotFound            = errors.New("NOT_FOUND")
	ErrProviderUnavailable = errors.New("PROVIDER_UNAVAILABLE")
)

// VINProvider — VIN -> характеристики авто.
type VINProvider interface {
	Resolve(ctx context.Context, vin string) (domain.Vehicle, error)
}

// Stub — заглушка Dev 1: всегда «недоступно», чтобы основной путь был ручной ввод.
// Dev 3 заменит на адаптер Drom.
type Stub struct{}

func NewStub() *Stub { return &Stub{} }

func (Stub) Resolve(_ context.Context, _ string) (domain.Vehicle, error) {
	return domain.Vehicle{}, ErrProviderUnavailable
}
