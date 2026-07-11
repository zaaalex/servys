// Package vin — VINProvider (порт + стаб).
// Владелец боевой реализации: Dev 3 (адаптер Drom, best-effort).
package vin

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

// Типизированные ошибки провайдера (ADR-001 §5.4). При любой — фронт открывает ручную форму.
var (
	ErrInvalidVIN           = errors.New("INVALID_VIN")
	ErrNotFound             = errors.New("NOT_FOUND")
	ErrProviderUnavailable  = errors.New("PROVIDER_UNAVAILABLE")
	ErrProviderBlocked      = errors.New("PROVIDER_BLOCKED")
	ErrPageStructureChanged = errors.New("PAGE_STRUCTURE_CHANGED")
	ErrIncompleteResult     = errors.New("INCOMPLETE_RESULT")
	ErrVINMismatch          = errors.New("VIN_MISMATCH")
)

// VINProvider — VIN -> характеристики авто.
type VINProvider interface {
	Resolve(ctx context.Context, vin string) (domain.Vehicle, error)
}

var validVIN = regexp.MustCompile(`^[A-HJ-NPR-Z0-9]{17}$`)

type Drom struct {
	client  *http.Client
	baseURL string
}

func NewDrom(c *http.Client) *Drom { return NewDromWithBaseURL(c, "https://vin.drom.ru/report/") }
func NewDromWithBaseURL(c *http.Client, u string) *Drom {
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	return &Drom{c, strings.TrimRight(u, "/") + "/"}
}
func (d *Drom) Resolve(ctx context.Context, raw string) (domain.Vehicle, error) {
	vin := strings.ToUpper(strings.TrimSpace(raw))
	if !validVIN.MatchString(vin) {
		return domain.Vehicle{}, ErrInvalidVIN
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+url.PathEscape(vin)+"/", nil)
	resp, err := d.client.Do(req)
	if err != nil {
		return domain.Vehicle{}, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return domain.Vehicle{}, ErrNotFound
	}
	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return domain.Vehicle{}, ErrProviderBlocked
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return domain.Vehicle{}, ErrProviderUnavailable
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return parseDrom(b, vin)
}

var tags = regexp.MustCompile(`(?s)<[^>]*>`)
var spaces = regexp.MustCompile(`[\s\p{Zs}]+`)
var number = regexp.MustCompile(`\d[\d\s.,]*`)

func visibleText(b []byte) string {
	s := strings.NewReplacer("</td>", "\n", "</th>", "\n", "</div>", "\n", "</li>", "\n", "<br>", "\n", "<br/>", "\n", "<br />", "\n").Replace(string(b))
	s = html.UnescapeString(tags.ReplaceAllString(s, " "))
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(spaces.ReplaceAllString(lines[i], " "))
	}
	return strings.Join(lines, "\n")
}
func field(t string, labels ...string) string {
	lines := strings.Split(t, "\n")
	for lineNo, line := range lines {
		for _, label := range labels {
			match := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(label)).FindStringIndex(line)
			if match == nil {
				continue
			}
			value := strings.TrimLeft(line[match[1]:], " :—–-\t")
			if value != "" {
				return value
			}
			for next := lineNo + 1; next < len(lines); next++ {
				if value = strings.TrimSpace(lines[next]); value != "" {
					return value
				}
			}
		}
	}
	return ""
}
func integer(s string) int {
	m := number.FindString(s)
	m = strings.NewReplacer(" ", "", ",", ".").Replace(m)
	f, _ := strconv.ParseFloat(m, 64)
	return int(f + .5)
}
func parseDrom(b []byte, vin string) (domain.Vehicle, error) {
	t := visibleText(b)
	low := strings.ToLower(t)
	if strings.Contains(low, "captcha") || strings.Contains(low, "не робот") {
		return domain.Vehicle{}, ErrProviderBlocked
	}
	name := field(t, "Автомобиль", "Марка, модель", "Марка и модель")
	year := integer(field(t, "Год выпуска", "Год"))
	p := strings.Fields(name)
	if len(p) < 2 || year == 0 {
		return domain.Vehicle{}, ErrIncompleteResult
	}
	cc := integer(field(t, "Объем двигателя", "Объём двигателя"))
	if cc > 0 && cc < 20 {
		cc *= 1000
	}
	return domain.Vehicle{VIN: vin, Make: p[0], Model: strings.Join(p[1:], " "), Year: year, EngineCC: cc, PowerHP: integer(field(t, "Мощность")), IdentificationSource: "drom"}, nil
}

// Stub — заглушка Dev 1: всегда «недоступно», чтобы основной путь был ручной ввод.
// Dev 3 заменит на адаптер Drom.
type Stub struct{}

func NewStub() *Stub { return &Stub{} }

func (Stub) Resolve(_ context.Context, _ string) (domain.Vehicle, error) {
	return domain.Vehicle{}, ErrProviderUnavailable
}
