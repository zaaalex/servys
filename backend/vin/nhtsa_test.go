package vin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
)

func TestNHTSAReadsDecodeVinValues(t *testing.T) {
	const vin = "LJD3AA293L0051345"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/DecodeVinValues/"+vin {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.URL.Query().Get("format") != "json" {
			t.Fatalf("format=%s", r.URL.Query().Get("format"))
		}
		_, _ = w.Write([]byte(`{"Results":[{"Make":"KIA","Model":"K3","ModelYear":"2020","DisplacementCC":"1353","EngineHP":"130"}]}`))
	}))
	defer srv.Close()

	got, err := NewNHTSAWithBaseURL(srv.Client(), srv.URL+"/DecodeVinValues").Resolve(context.Background(), vin)
	if err != nil {
		t.Fatal(err)
	}
	if got.Make != "KIA" || got.Model != "K3" || got.Year != 2020 || got.EngineCC != 1353 || got.PowerHP != 130 || got.IdentificationSource != domain.SourceNHTSA {
		t.Fatalf("%+v", got)
	}
}

func TestNHTSAEmptyMakeIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Results":[{"Make":"","Model":"","ModelYear":""}]}`))
	}))
	defer srv.Close()
	_, err := NewNHTSAWithBaseURL(srv.Client(), srv.URL+"/DecodeVinValues").Resolve(context.Background(), "LJD3AA293L0051345")
	if err != ErrNotFound {
		t.Fatalf("err=%v", err)
	}
}
