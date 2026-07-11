package vin

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/zaaalex/servys/backend/domain"
)

const FixtureVIN = "LJD3AA293L0051345"

type Fixture struct{}

func NewFixture() *Fixture { return &Fixture{} }

func (Fixture) Resolve(_ context.Context, raw string) (domain.Vehicle, error) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if !validVIN.MatchString(value) {
		return domain.Vehicle{}, ErrInvalidVIN
	}
	if value != FixtureVIN {
		return domain.Vehicle{}, ErrNotFound
	}
	return domain.Vehicle{VIN: value, Make: "KIA", Model: "K3", Year: 2020, EngineCC: 1353, PowerHP: 130, IdentificationSource: "fixture"}, nil
}

func NewFromEnv(client *http.Client) (VINProvider, error) { return NewFromLookupEnv(os.Getenv, client) }

func NewFromLookupEnv(getenv func(string) string, client *http.Client) (VINProvider, error) {
	switch mode := getenv("VIN_MODE"); mode {
	case "", "fixture":
		return NewFixture(), nil
	case "live":
		var provider VINProvider
		if baseURL := getenv("VIN_BASE_URL"); baseURL != "" {
			provider = NewDromWithBaseURL(client, baseURL)
		} else {
			provider = NewDrom(client)
		}
		// Drom → NHTSA: страница Drom стала лендингом покупки отчёта (без данных),
		// vPIC добирает то, что может, по WMI.
		var nhtsa VINProvider
		if baseURL := getenv("NHTSA_BASE_URL"); baseURL != "" {
			nhtsa = NewNHTSAWithBaseURL(client, baseURL)
		} else {
			nhtsa = NewNHTSA(client)
		}
		provider = FallbackProvider{Primary: provider, Fallback: nhtsa}
		if getenv("VIN_FIXTURE_FALLBACK") == "1" {
			provider = FallbackProvider{Primary: provider, Fallback: NewFixture()}
		}
		return provider, nil
	default:
		return nil, fmt.Errorf("invalid VIN_MODE %q (want live or fixture)", mode)
	}
}

type FallbackProvider struct{ Primary, Fallback VINProvider }

func (p FallbackProvider) Resolve(ctx context.Context, value string) (domain.Vehicle, error) {
	vehicle, err := p.Primary.Resolve(ctx, value)
	if err == nil {
		return vehicle, nil
	}
	if p.Fallback != nil {
		if fixture, fixtureErr := p.Fallback.Resolve(ctx, value); fixtureErr == nil {
			return fixture, nil
		}
	}
	return domain.Vehicle{}, err
}
