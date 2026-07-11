package b2b

import (
	"context"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
)

type fakeLister struct {
	list []domain.ServiceCenter
	full map[string]domain.ServiceCenter
}

func (f fakeLister) ListServiceCenters(context.Context) ([]domain.ServiceCenter, error) {
	return f.list, nil
}
func (f fakeLister) GetServiceCenter(_ context.Context, id string) (domain.ServiceCenter, error) {
	return f.full[id], nil
}

func TestScanAllAcrossCenters(t *testing.T) {
	lister := fakeLister{
		list: []domain.ServiceCenter{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}},
		full: map[string]domain.ServiceCenter{
			"a": {ID: "a", Name: "A"},
			"b": {ID: "b", Name: "B"},
		},
	}
	svc := &Service{
		Fleet:     fakeFleet{cars: []domain.ClientCar{{CRMContactID: 1, Make: "KIA", Model: "K3", MileageKm: 95000}}},
		Advisor:   fakeAdvisor{alerts: []domain.Alert{{RuleCode: "oil", Type: domain.AlertMaintenanceOverdue, DueAtKm: 10000}}},
		Retention: &fakeRetention{},
		Dedupe:    newMemDedupe(),
	}

	sum := ScanAll(context.Background(), svc, lister)
	if sum.Centers != 2 || sum.Pushed != 2 {
		t.Fatalf("ScanAll: %+v (ожидали centers=2, pushed=2)", sum)
	}

	// повторный прогон — всё уже создано → 0 pushed, 2 skipped (по 1 на СТО)
	sum2 := ScanAll(context.Background(), svc, lister)
	if sum2.Pushed != 0 || sum2.Skipped != 2 {
		t.Fatalf("повторный ScanAll: %+v (ожидали pushed=0, skipped=2)", sum2)
	}
}
