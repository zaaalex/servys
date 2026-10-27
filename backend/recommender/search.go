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
		return nil, fmt.Errorf("поиск отзывов недоступен")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("поиск отзывов недоступен (%d)", resp.StatusCode)
	}
	var b struct {
		Results []struct {
			URL, Title, Content string
			Score               float64
		} `json:"results"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return nil, fmt.Errorf("поиск отзывов недоступен")
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
	return []string{
		id + " регламент ТО интервалы замены auto.ru",
		id + " регламент ТО carsbag autospot",
		id + " руководство по эксплуатации периодичность обслуживания",
		id + " отзывы владельцев замена интервал км",
	}
}
func preferSchedule(s VehicleSignature, found []SearchResult) []SearchResult {
	out := make([]SearchResult, 0, len(found)+1)
	seen := map[string]bool{}
	add := func(r SearchResult) {
		key := strings.TrimRight(r.URL, "/")
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, r)
	}
	if u := catalogScheduleURL(s); u != "" {
		add(SearchResult{URL: u, Title: "Регламент ТО"})
	}
	for _, r := range found {
		add(r)
	}
	return out
}

func catalogScheduleURL(s VehicleSignature) string {
	make := slugToken(s.Make)
	model := slugToken(s.Model)
	if make == "" || model == "" {
		return ""
	}
	return "https://auto.ru/catalog/cars/" + make + "/" + model + "/reglament-to/"
}

func slugToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

func RankAndDedupeSources(focus string, in []SearchResult) []SearchResult {
	m := map[string]SearchResult{}
	foldedFocus := foldLatin(focus)
	for _, r := range in {
		u, err := url.Parse(r.URL)
		if err != nil || u.Host == "" {
			continue
		}
		u.Fragment = ""
		r.URL = u.String()
		text := strings.ToLower(r.Title + " " + r.Snippet + " " + u.Host + u.Path)
		if foldedFocus != "" && containsToken(foldLatin(text), foldedFocus) {
			r.Score += 12
		}
		for _, term := range []string{"отзыв", "форум", "бортов", "владел"} {
			if strings.Contains(text, term) {
				r.Score += 4
			}
		}
		host := strings.ToLower(u.Host)
		if strings.Contains(host, "drom.ru") || strings.Contains(host, "drive2.ru") || strings.Contains(host, "auto.ru") || strings.Contains(host, "oil-club.ru") || strings.Contains(host, "carsbag.ru") || strings.Contains(host, "autospot.ru") || strings.Contains(host, "ownersmanual.kia.com") {
			r.Score += 6
		}
		if isSchedulePage(u.Path) && foldedFocus != "" && containsToken(foldLatin(text), foldedFocus) {
			r.Score += 30
		}
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

func foldLatin(s string) string {
	return strings.NewReplacer(
		"а", "a", "е", "e", "о", "o", "р", "p", "с", "c", "х", "x", "у", "y",
		"к", "k", "м", "m", "т", "t", "в", "b", "н", "h", "і", "i",
	).Replace(strings.ToLower(s))
}

func containsToken(text, token string) bool {
	if token == "" {
		return false
	}
	for i := 0; i+len(token) <= len(text); {
		j := strings.Index(text[i:], token)
		if j < 0 {
			return false
		}
		j += i
		before := j == 0 || !isTokenByte(text[j-1])
		after := j+len(token) == len(text) || !isTokenByte(text[j+len(token)])
		if before && after {
			return true
		}
		i = j + 1
	}
	return false
}

func isTokenByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b >= 0x80
}
