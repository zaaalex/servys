package recommender

import (
	"context"
	"errors"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

var ErrUnsafeDocumentURL = errors.New("unsafe document URL")

type HTTPDocumentFetcher struct {
	Client                  *http.Client
	MaxBodyBytes            int64
	AllowHTTP, AllowPrivate bool
}

func NewDocumentFetcher() *HTTPDocumentFetcher { return &HTTPDocumentFetcher{MaxBodyBytes: 4 << 20} }
func (f *HTTPDocumentFetcher) Fetch(ctx context.Context, s SearchResult) (FetchedDocument, error) {
	u, err := url.Parse(s.URL)
	if err != nil || u.Hostname() == "" || u.User != nil {
		return FetchedDocument{}, ErrUnsafeDocumentURL
	}
	if u.Scheme != "https" && !(f.AllowHTTP && u.Scheme == "http") {
		return FetchedDocument{}, ErrUnsafeDocumentURL
	}
	if !f.AllowPrivate {
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, u.Hostname())
		if e != nil {
			return FetchedDocument{}, ErrUnsafeDocumentURL
		}
		for _, ip := range ips {
			if ip.IP.IsPrivate() || ip.IP.IsLoopback() {
				return FetchedDocument{}, ErrUnsafeDocumentURL
			}
		}
	}
	c := f.Client
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		return FetchedDocument{}, err
	}
	defer resp.Body.Close()
	limit := f.MaxBodyBytes
	if limit <= 0 {
		limit = 4 << 20
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(b)) > limit {
		return FetchedDocument{}, errors.New("document read failed")
	}
	return FetchedDocument{URL: s.URL, Title: s.Title, Text: NormalizeHTML(string(b)), ContentType: resp.Header.Get("Content-Type")}, nil
}

var dropHTML = regexp.MustCompile(`(?is)<(?:script|style|head)[^>]*>.*?</(?:script|style|head)>`)
var tagHTML = regexp.MustCompile(`(?s)<[^>]*>`)

func NormalizeHTML(s string) string {
	return normalizeSpace(html.UnescapeString(tagHTML.ReplaceAllString(dropHTML.ReplaceAllString(s, " "), " ")))
}
