package bitrix

import (
	"context"
	"fmt"
	"strings"

	"github.com/zaaalex/servys/backend/sink"
)

// Sink доставляет напоминание в Bitrix24 задачей (tasks.task.add) через вебхук.
// Реализует sink.Sink. Этап b2b (в b2c не подключается).
type Sink struct {
	client        *Client
	responsibleID int // на кого вешаем задачу (владелец вебхука по умолчанию)
}

var _ sink.Sink = (*Sink)(nil)

// NewSink создаёт Bitrix-синк на входящем вебхуке. responsibleID<=0 => 1.
func NewSink(webhook string, responsibleID int) (*Sink, error) {
	c, err := NewClient(webhook)
	if err != nil {
		return nil, err
	}
	if responsibleID <= 0 {
		responsibleID = 1
	}
	return &Sink{client: c, responsibleID: responsibleID}, nil
}

func (s *Sink) Deliver(ctx context.Context, r sink.Reminder) error {
	_, err := s.client.AddTask(ctx, map[string]any{
		"TITLE":          taskTitle(r),
		"DESCRIPTION":    taskDescription(r),
		"RESPONSIBLE_ID": s.responsibleID,
	})
	return err
}

func taskTitle(r sink.Reminder) string {
	car := strings.TrimSpace(r.Vehicle.Make + " " + r.Vehicle.Model)
	if car == "" {
		car = "Автомобиль"
	}
	return fmt.Sprintf("%s: %s", car, r.Alert.Title)
}

func taskDescription(r sink.Reminder) string {
	return fmt.Sprintf("%s\nТекущий пробег: %d км. Ориентир: %d км.",
		r.Alert.Description, r.Vehicle.CurrentOdometer, r.Alert.DueAtKm)
}
