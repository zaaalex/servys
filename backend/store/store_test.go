package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func accountID(t *testing.T, s *Store) string {
	t.Helper()
	a, err := s.CreateAccount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return a.ID
}

func TestVehicleScopedToAccount(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	owner := accountID(t, s)
	other := accountID(t, s)
	v, err := s.AddVehicle(ctx, domain.Vehicle{AccountID: owner, Make: "KIA", Model: "K3", CurrentOdometer: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetVehicle(ctx, other, v.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужой аккаунт не должен видеть авто, err=%v", err)
	}
	got, err := s.GetVehicle(ctx, owner, v.ID)
	if err != nil || got.Make != "KIA" {
		t.Fatalf("владелец должен видеть авто, got=%+v err=%v", got, err)
	}
}

func TestUpdateOdometerRejectsDecrease(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	owner := accountID(t, s)
	v, _ := s.AddVehicle(ctx, domain.Vehicle{AccountID: owner, Make: "KIA", Model: "K3", CurrentOdometer: 50000})

	if _, err := s.UpdateOdometer(ctx, owner, v.ID, 40000); !errors.Is(err, ErrOdometerDecrease) {
		t.Fatalf("уменьшение пробега должно отклоняться, err=%v", err)
	}
	up, err := s.UpdateOdometer(ctx, owner, v.ID, 60000)
	if err != nil || up.CurrentOdometer != 60000 {
		t.Fatalf("рост пробега ок, got=%+v err=%v", up, err)
	}
}

func TestServiceEventsAddListAndScope(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	owner := accountID(t, s)
	other := accountID(t, s)
	v, _ := s.AddVehicle(ctx, domain.Vehicle{AccountID: owner, Make: "KIA", Model: "K3", CurrentOdometer: 60000})

	if _, err := s.AddServiceEvent(ctx, other, v.ID, domain.ServiceEvent{RuleCode: "engine_oil", Odometer: 60000}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужой аккаунт не должен писать в чужое авто, err=%v", err)
	}
	ev, err := s.AddServiceEvent(ctx, owner, v.ID, domain.ServiceEvent{RuleCode: "engine_oil", Odometer: 60000})
	if err != nil || ev.ID == "" || ev.PerformedAt.IsZero() {
		t.Fatalf("владелец должен добавить событие, ev=%+v err=%v", ev, err)
	}
	list, err := s.ListServiceEvents(ctx, owner, v.ID)
	if err != nil || len(list) != 1 || list[0].RuleCode != "engine_oil" {
		t.Fatalf("список должен содержать 1 событие, list=%+v err=%v", list, err)
	}
}

func TestRenameOwnerColumn(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.db.Exec(`ALTER TABLE vehicles RENAME COLUMN account_id TO user_id`); err != nil {
		t.Fatal(err)
	}
	if err := s.renameOwnerColumn(); err != nil {
		t.Fatal(err)
	}
	owner := accountID(t, s)
	if _, err := s.AddVehicle(ctx, domain.Vehicle{AccountID: owner, Make: "KIA", Model: "K3", CurrentOdometer: 1}); err != nil {
		t.Fatal(err)
	}
}
