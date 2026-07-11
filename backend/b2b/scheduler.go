package b2b

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

// CenterLister — доступ ко всем подключённым СТО (реализует store).
type CenterLister interface {
	ListServiceCenters(ctx context.Context) ([]domain.ServiceCenter, error)
	GetServiceCenter(ctx context.Context, id string) (domain.ServiceCenter, error)
}

// Summary — агрегат по скану всех СТО.
type Summary struct {
	Centers int      `json:"centers"`
	Due     int      `json:"due_items"`
	Pushed  int      `json:"pushed"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

// ScanAll прогоняет ScanAndPush по всем СТО. Сбой одного СТО не роняет остальных.
func ScanAll(ctx context.Context, svc *Service, lister CenterLister) Summary {
	var sum Summary
	list, err := lister.ListServiceCenters(ctx)
	if err != nil {
		sum.Errors = append(sum.Errors, fmt.Sprintf("list: %v", err))
		return sum
	}
	for _, c := range list {
		full, err := lister.GetServiceCenter(ctx, c.ID)
		if err != nil {
			sum.Errors = append(sum.Errors, fmt.Sprintf("get %s: %v", c.ID, err))
			continue
		}
		rep, err := svc.ScanAndPush(ctx, full)
		if err != nil {
			sum.Errors = append(sum.Errors, fmt.Sprintf("scan %s: %v", full.Name, err))
			continue
		}
		sum.Centers++
		sum.Due += rep.Due
		sum.Pushed += rep.Pushed
		sum.Skipped += rep.Skipped
		sum.Errors = append(sum.Errors, rep.Errors...)
	}
	return sum
}

// Scheduler периодически сканирует все СТО.
type Scheduler struct {
	Svc      *Service
	Lister   CenterLister
	Interval time.Duration
	Logf     func(format string, args ...any) // nil => log.Printf
}

func (s *Scheduler) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// Run крутит скан с интервалом Interval, пока не отменят ctx (Run — тонкая обёртка над ScanAll).
func (s *Scheduler) Run(ctx context.Context) {
	s.logf("b2b шедулер запущен (интервал %s)", s.Interval)
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logf("b2b шедулер остановлен")
			return
		case <-t.C:
			sum := ScanAll(ctx, s.Svc, s.Lister)
			s.logf("b2b тик: СТО=%d, создано дел=%d, пропущено=%d", sum.Centers, sum.Pushed, sum.Skipped)
		}
	}
}
