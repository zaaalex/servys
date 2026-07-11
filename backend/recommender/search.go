package recommender

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type SearchResult struct {
	URL, Title, Snippet string
	Score               float64
}
type SearchProvider interface {
	Search(context.Context, string) ([]SearchResult, error)
}
type SearXNG struct {
	BaseURL string
	Client  *http.Client
}

func NewSearXNG(u string, c *http.Client) *SearXNG {
	if c == nil {
		c = http.DefaultClient
	}
	return &SearXNG{strings.TrimRight(u, "/"), c}
}
func (s *SearXNG) Search(ctx context.Context, q string) ([]SearchResult, error) {
	if strings.TrimSpace(q) == "" {
		return nil, errors.New("empty search query")
	}
	u, _ := url.Parse(s.BaseURL + "/search")
	v := u.Query()
	v.Set("q", q)
	v.Set("format", "json")
	u.RawQuery = v.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("searxng status %d", resp.StatusCode)
	}
	var b struct {
		Results []struct {
			URL, Title, Content string
			Score               float64
		} `json:"results"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return nil, err
	}
	out := []SearchResult{}
	for _, r := range b.Results {
		out = append(out, SearchResult{r.URL, r.Title, r.Content, r.Score})
	}
	return out, nil
}
func PlanQueries(v VehicleSignature) []string {
	id := normalizeSpace(fmt.Sprintf("%s %s %d", v.Make, v.Model, v.Year))
	if id == "" {
		return nil
	}
	return []string{id + " maintenance schedule replace interval", id + " owners manual maintenance schedule pdf", id + " регламент ТО интервалы замены"}
}
func RankAndDedupeSources(in []SearchResult) []SearchResult {
	m := map[string]SearchResult{}
	for _, r := range in {
		u, err := url.Parse(r.URL)
		if err != nil || u.Host == "" {
			continue
		}
		u.Fragment = ""
		r.URL = u.String()
		text := strings.ToLower(r.Title + " " + r.Snippet + " " + u.Path)
		for _, term := range []string{"maintenance", "schedule", "service interval", "owner manual", "replace", "регламент", "интервал", "замен"} {
			if strings.Contains(text, term) {
				r.Score += 3
			}
		}
		for _, term := range []string{"specification", "features", "for sale", "facebook.com", "instagram.com"} {
			if strings.Contains(text, term) {
				r.Score -= 10
			}
		}
		if old, ok := m[r.URL]; !ok || r.Score > old.Score {
			m[r.URL] = r
		}
	}
	out := []SearchResult{}
	for _, r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}
