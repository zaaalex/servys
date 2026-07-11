package vin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
)

type failingProvider struct{ err error }

func (p failingProvider) Resolve(context.Context, string) (domain.Vehicle, error) {
	return domain.Vehicle{}, p.err
}

func TestDromResolveRegressionFixtureVIN(t *testing.T) {
	const requestedVIN = "LJD3AA293L0051345"
	page := `<table><tr><th>VIN</th><td>LJD3AA293L0051345</td></tr><tr><th>Автомобиль</th><td>KIA K3</td></tr><tr><th>Год выпуска</th><td>2020</td></tr><tr><th>Объём двигателя</th><td>1.4 л</td></tr><tr><th>Мощность</th><td>130 л.с.</td></tr></table>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/report/"+requestedVIN+"/" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		_, _ = w.Write([]byte(page))
	}))
	defer server.Close()
	vehicle, err := NewDromWithBaseURL(server.Client(), server.URL+"/report").Resolve(context.Background(), requestedVIN)
	if err != nil {
		t.Fatal(err)
	}
	if vehicle.VIN != requestedVIN || vehicle.Make != "KIA" || vehicle.Model != "K3" || vehicle.Year != 2020 {
		t.Fatalf("vehicle=%+v", vehicle)
	}
}

func TestFieldCyrillicIndexRegression(t *testing.T) {
	if got := field("Технические сведения\nОБЪЁМ ДВИГАТЕЛЯ\n1.4 л", "Объём двигателя"); got != "1.4 л" {
		t.Fatalf("field=%q", got)
	}
}

func TestLiveFallbackResolvesRequestedRegressionVIN(t *testing.T) {
	provider := FallbackProvider{Primary: failingProvider{ErrProviderUnavailable}, Fallback: NewFixture()}
	vehicle, err := provider.Resolve(context.Background(), FixtureVIN)
	if err != nil {
		t.Fatal(err)
	}
	if vehicle.VIN != FixtureVIN || vehicle.Make != "KIA" || vehicle.Model != "K3" {
		t.Fatalf("vehicle=%+v", vehicle)
	}
}

func TestLiveFallbackDoesNotInventUnknownVIN(t *testing.T) {
	provider := FallbackProvider{Primary: failingProvider{ErrProviderUnavailable}, Fallback: NewFixture()}
	_, err := provider.Resolve(context.Background(), "KNAGM4A77B5123456")
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("err=%v", err)
	}
}
