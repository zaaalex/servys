package vin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

type NHTSA struct {
	client  *http.Client
	baseURL string
}

func NewNHTSA(c *http.Client) *NHTSA {
	return NewNHTSAWithBaseURL(c, "https://vpic.nhtsa.dot.gov/api/vehicles/DecodeVinValues/")
}

func NewNHTSAWithBaseURL(c *http.Client, u string) *NHTSA {
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	return &NHTSA{client: c, baseURL: strings.TrimRight(u, "/") + "/"}
}

func (n *NHTSA) Resolve(ctx context.Context, raw string) (domain.Vehicle, error) {
	vin := strings.ToUpper(strings.TrimSpace(raw))
	if !validVIN.MatchString(vin) {
		return domain.Vehicle{}, ErrInvalidVIN
	}
	endpoint := n.baseURL + url.PathEscape(vin) + "?format=json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.Vehicle{}, ErrProviderUnavailable
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return domain.Vehicle{}, ErrProviderUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return domain.Vehicle{}, ErrProviderUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return domain.Vehicle{}, ErrProviderUnavailable
	}
	return parseNHTSA(body, vin)
}

func parseNHTSA(body []byte, vin string) (domain.Vehicle, error) {
	var payload struct {
		Results []struct {
			Make           string `json:"Make"`
			Model          string `json:"Model"`
			ModelYear      string `json:"ModelYear"`
			DisplacementCC string `json:"DisplacementCC"`
			EngineHP       string `json:"EngineHP"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || len(payload.Results) == 0 {
		return domain.Vehicle{}, ErrProviderUnavailable
	}
	row := payload.Results[0]
	make := strings.TrimSpace(row.Make)
	model := strings.TrimSpace(row.Model)
	year := integer(row.ModelYear)
	if make == "" || model == "" || year == 0 {
		return domain.Vehicle{}, ErrNotFound
	}
	return domain.Vehicle{
		VIN:                  vin,
		Make:                 make,
		Model:                model,
		Year:                 year,
		EngineCC:             displacementCC(row.DisplacementCC),
		PowerHP:              integer(row.EngineHP),
		IdentificationSource: domain.SourceNHTSA,
	}, nil
}

func displacementCC(raw string) int {
	cc := integer(raw)
	if cc > 0 && cc < 20 {
		return cc * 1000
	}
	return cc
}
