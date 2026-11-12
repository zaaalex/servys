package recommender

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBlockedAddress(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.0.1",
		"169.254.169.254", "100.64.0.1", "255.255.255.255", "0.0.0.0", "0.1.2.3",
		"192.0.2.1", "198.18.0.1", "203.0.113.1", "240.0.0.1",
		"::1", "fe80::1", "fc00::1", "2001:db8::1", "::ffff:10.0.0.1", "::ffff:169.254.169.254",
	}
	for _, raw := range blocked {
		if !blockedAddress(net.ParseIP(raw)) {
			t.Errorf("%s должен быть закрыт", raw)
		}
	}
	for _, raw := range []string{"1.1.1.1", "8.8.8.8", "2001:4860:4860::8888"} {
		if blockedAddress(net.ParseIP(raw)) {
			t.Errorf("%s должен быть открыт", raw)
		}
	}
}

func TestFetchRejectsMetadataAddress(t *testing.T) {
	f := NewDocumentFetcher()
	f.AllowHTTP = true
	_, _, err := f.admit(context.Background(), "http://169.254.169.254/latest/meta-data")
	if !errors.Is(err, ErrUnsafeDocumentURL) {
		t.Fatalf("метаданные облака: %v", err)
	}
	_, err = f.Fetch(context.Background(), SearchResult{URL: "http://127.0.0.1/"})
	if !errors.Is(err, ErrUnsafeDocumentURL) {
		t.Fatalf("loopback: %v", err)
	}
}

func TestFetchPinsAndFollowsOnlyAdmittedRedirect(t *testing.T) {
	secret := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "<p>регламент 15000 км</p>")
	}))
	defer secret.Close()
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, secret.URL, http.StatusFound)
	}))
	defer public.Close()

	local := NewDocumentFetcher()
	local.AllowHTTP = true
	local.AllowPrivate = true
	doc, err := local.Fetch(context.Background(), SearchResult{URL: public.URL, Title: "Kia"})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Text != "регламент 15000 км" || doc.Title != "Kia" {
		t.Fatalf("документ: %+v", doc)
	}

	trap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://user:pass@127.0.0.1/secret", http.StatusFound)
	}))
	defer trap.Close()
	_, err = local.Fetch(context.Background(), SearchResult{URL: trap.URL})
	if !errors.Is(err, ErrUnsafeDocumentURL) {
		t.Fatalf("редирект с паролем в адресе: %v", err)
	}
}

func TestFetchStopsAfterFiveRedirects(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/next", http.StatusFound)
	}))
	defer srv.Close()
	f := NewDocumentFetcher()
	f.AllowHTTP = true
	f.AllowPrivate = true
	_, err := f.Fetch(context.Background(), SearchResult{URL: srv.URL})
	if !errors.Is(err, ErrUnsafeDocumentURL) {
		t.Fatalf("бесконечный редирект: %v", err)
	}
}
