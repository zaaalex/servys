// Package sink — исходящий порт доставки напоминаний (b2b, отложено).
// Порт определяет Dev 1; реализацию Bitrix (tasks.task.add) добавит Dev 3 на этапе b2b.
// В b2c не задействован: напоминания живут в приложении.
package sink

import (
	"context"

	"github.com/zaaalex/servys/backend/domain"
)

// Reminder — единица доставки во внешний канал.
type Reminder struct {
	Tenant  domain.Tenant
	Vehicle domain.Vehicle
	Alert   domain.Alert
}

// Sink — куда толкаем напоминание (Bitrix/календарь/CRM — на этапе b2b).
type Sink interface {
	Deliver(ctx context.Context, r Reminder) error
}

// Noop — пустая реализация для b2c-MVP (ничего не отправляет).
type Noop struct{}

func NewNoop() *Noop { return &Noop{} }

func (Noop) Deliver(_ context.Context, _ Reminder) error { return nil }
