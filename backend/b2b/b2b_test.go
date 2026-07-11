package b2b

import (
	"context"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
)

type fakeFleet struct{ cars []domain.ClientCar }

func (f fakeFleet) Fleet(context.Context, domain.ServiceCenter) ([]domain.ClientCar, error) {
	return f.cars, nil
}

type fakeAdvisor struct{ alerts []domain.Alert }

func (f fakeAdvisor) Alerts(context.Context, domain.Vehicle, []domain.ServiceEvent) ([]domain.Alert, error) {
	return f.alerts, nil
}

type fakeRetention struct{ calls int }

func (f *fakeRetention) Push(context.Context, domain.ServiceCenter, domain.ClientCar, domain.Alert) (string, error) {
	f.calls++
	return "remote-id", nil
}

type memDedupe struct{ seen map[string]bool }

func newMemDedupe() *memDedupe { return &memDedupe{seen: map[string]bool{}} }
func (m *memDedupe) AlreadyPushed(_ context.Context, t, k string) (bool, error) {
	return m.seen[t+"|"+k], nil
}
func (m *memDedupe) RecordPush(_ context.Context, t, k, _ string) error {
	m.seen[t+"|"+k] = true
	return nil
}

func TestScanAndPushAndIdempotency(t *testing.T) {
	sc := domain.ServiceCenter{ID: "sc1"}
	cars := []domain.ClientCar{
		{CRMContactID: 10, Make: "KIA", Model: "K3", MileageKm: 95000},
		{CRMContactID: 11, Make: "Toyota", Model: "Camry", MileageKm: 95000},
	}
	alerts := []domain.Alert{
		{RuleCode: "engine_oil", Type: domain.AlertMaintenanceOverdue, DueAtKm: 10000, Title: "Масло"},
		{RuleCode: "trivia", Type: "SOMETHING_ELSE", DueAtKm: 0}, // не ретеншн-достойно
	}
	svc := &Service{
		Fleet:     fakeFleet{cars: cars},
		Advisor:   fakeAdvisor{alerts: alerts},
		Retention: &fakeRetention{},
		Dedupe:    newMemDedupe(),
	}

	rep, err := svc.ScanAndPush(context.Background(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Cars != 2 || rep.Due != 2 || rep.Pushed != 2 || rep.Skipped != 0 {
		t.Fatalf("первый скан: %+v (ожидали cars=2 due=2 pushed=2 skipped=0)", rep)
	}

	// повторный скан — всё уже создано → только skipped
	rep2, _ := svc.ScanAndPush(context.Background(), sc)
	if rep2.Pushed != 0 || rep2.Skipped != 2 {
		t.Fatalf("второй скан: %+v (ожидали pushed=0 skipped=2)", rep2)
	}
}
