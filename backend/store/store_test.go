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

func TestEnsureUserIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u1, err := s.EnsureUser(ctx, "client-a")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := s.EnsureUser(ctx, "client-a")
	if err != nil {
		t.Fatal(err)
	}
	if u1.ID != u2.ID {
		t.Fatalf("один client-key => один юзер, получили %s и %s", u1.ID, u2.ID)
	}
}

func TestVehicleScopedToUser(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ua, _ := s.EnsureUser(ctx, "a")
	ub, _ := s.EnsureUser(ctx, "b")
	v, err := s.AddVehicle(ctx, domain.Vehicle{UserID: ua.ID, Make: "KIA", Model: "K3", CurrentOdometer: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetVehicle(ctx, ub.ID, v.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужой юзер не должен видеть авто, err=%v", err)
	}
	got, err := s.GetVehicle(ctx, ua.ID, v.ID)
	if err != nil || got.Make != "KIA" {
		t.Fatalf("владелец должен видеть авто, got=%+v err=%v", got, err)
	}
}

func TestUpdateOdometerRejectsDecrease(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, _ := s.EnsureUser(ctx, "a")
	v, _ := s.AddVehicle(ctx, domain.Vehicle{UserID: u.ID, Make: "KIA", Model: "K3", CurrentOdometer: 50000})

	if _, err := s.UpdateOdometer(ctx, u.ID, v.ID, 40000); !errors.Is(err, ErrOdometerDecrease) {
		t.Fatalf("уменьшение пробега должно отклоняться, err=%v", err)
	}
	up, err := s.UpdateOdometer(ctx, u.ID, v.ID, 60000)
	if err != nil || up.CurrentOdometer != 60000 {
		t.Fatalf("рост пробега ок, got=%+v err=%v", up, err)
	}
}
