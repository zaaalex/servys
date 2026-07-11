// Package recommender — рекомендационный слой (порт + стаб).
// Владелец боевой реализации: Dev 3 (YAML-правила + LLM/Claude).
package recommender

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/engine"
)

// Recommender — источник знаний: правила регламента для конкретного авто.
// Пустой результат => REGULATION_NOT_FOUND (не выдумываем интервалы, ADR-001 §5.5).
// Движок напоминаний (пакет engine) превращает правила + пробег в Alert.
type Recommender interface {
	Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error)
}

type YAMLRecommender struct{ variants []yamlVariant }
type yamlVariant struct {
	make, model      string
	from, to, cc, hp int
	rules            []domain.Rule
}

func NewYAML(path string) (*YAMLRecommender, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	y := &YAMLRecommender{}
	var v *yamlVariant
	var r *domain.Rule
	section := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(strings.SplitN(s.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		if line == "- match:" {
			y.variants = append(y.variants, yamlVariant{})
			v = &y.variants[len(y.variants)-1]
			section = "match"
			continue
		}
		if line == "rules:" {
			section = "rules"
			continue
		}
		if strings.HasPrefix(line, "- code:") {
			v.rules = append(v.rules, domain.Rule{Code: strings.TrimSpace(strings.SplitN(line, ":", 2)[1])})
			r = &v.rules[len(v.rules)-1]
			continue
		}
		p := strings.SplitN(line, ":", 2)
		if len(p) < 2 {
			continue
		}
		k, val := p[0], strings.TrimSpace(p[1])
		n, _ := strconv.Atoi(val)
		if section == "match" && v != nil {
			switch k {
			case "make":
				v.make = val
			case "model":
				v.model = val
			case "year_from":
				v.from = n
			case "year_to":
				v.to = n
			case "engine_displacement_cc":
				v.cc = n
			case "power_hp":
				v.hp = n
			}
		}
		if r != nil {
			switch k {
			case "title":
				r.Title = val
			case "operation":
				r.Operation = val
			case "interval_km":
				r.IntervalKm = n
			case "interval_months":
				r.IntervalMonths = n
			case "lead_km":
				r.LeadKm = n
			case "source":
				r.Source = val
			case "verified":
				r.Verified = val == "true"
			}
		}
	}
	if e = s.Err(); e != nil {
		return nil, e
	}
	return y, nil
}
func (y *YAMLRecommender) Rules(_ context.Context, v domain.Vehicle) ([]domain.Rule, error) {
	for _, x := range y.variants {
		if strings.EqualFold(x.make, v.Make) && strings.EqualFold(x.model, v.Model) && (x.from == 0 || v.Year >= x.from) && (x.to == 0 || v.Year <= x.to) && (x.cc == 0 || v.EngineCC == x.cc) && (x.hp == 0 || v.PowerHP == x.hp) {
			return append([]domain.Rule(nil), x.rules...), nil
		}
	}
	return nil, nil
}

type advisor struct{ rec Recommender }

func NewAdvisor(r Recommender) Advisor { return &advisor{r} }
func (a *advisor) Alerts(ctx context.Context, v domain.Vehicle, h []domain.ServiceEvent) ([]domain.Alert, error) {
	rules, e := a.rec.Rules(ctx, v)
	if e != nil {
		return nil, e
	}
	_ = h
	return engine.BuildAlerts(v, rules), nil
}

type FallbackRecommender struct{ Primary, Secondary Recommender }

func (f FallbackRecommender) Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error) {
	r, e := f.Primary.Rules(ctx, v)
	if e != nil || len(r) > 0 || f.Secondary == nil {
		return r, e
	}
	return f.Secondary.Rules(ctx, v)
}

var _ = fmt.Sprintf

// Stub — временная заглушка Dev 1, чтобы бэкенд компилировался и демонстрировался.
// Dev 3 заменит на YAML+LLM. Возвращает базовый демо-набор правил.
type Stub struct{}

func NewStub() *Stub { return &Stub{} }

func (Stub) Rules(_ context.Context, _ domain.Vehicle) ([]domain.Rule, error) {
	return []domain.Rule{
		{Code: "engine_oil", Title: "Моторное масло", Operation: "replace", IntervalKm: 10000, IntervalMonths: 12, LeadKm: 500, Verified: false, Source: "stub"},
		{Code: "brakes", Title: "Тормозная система", Operation: "inspect", IntervalKm: 30000, LeadKm: 1000, Verified: false, Source: "stub"},
		{Code: "timing_belt", Title: "Ремень ГРМ", Operation: "replace", IntervalKm: 90000, LeadKm: 2000, Verified: false, Source: "stub"},
	}, nil
}

// Advisor — ЕДИНЫЙ шов между платформой (Dev 1) и рекомендательным слоем (Dev 3).
// `api` (Dev 1) зовёт только это: по авто → готовые alerts. Внутренности (правила, движок,
// LLM, источники) — целиком за Dev 3. Пусто правил => Dev 3 отдаёт alert REGULATION_NOT_FOUND.
type Advisor interface {
	// history — подтверждённые ТО (baseline для расчёта следующего срока), может быть пустым.
	Alerts(ctx context.Context, v domain.Vehicle, history []domain.ServiceEvent) ([]domain.Alert, error)
}

// StubAdvisor — заглушка Dev 1 для компиляции/демо: правила из Stub + движок BuildAlerts.
// Dev 3 заменит на боевую реализацию (YAML/LLM + свой engine). См. backend/recommender/README.md.
type StubAdvisor struct{ rec Recommender }

func NewStubAdvisor() *StubAdvisor { return &StubAdvisor{rec: NewStub()} }

func (a *StubAdvisor) Alerts(ctx context.Context, v domain.Vehicle, _ []domain.ServiceEvent) ([]domain.Alert, error) {
	rules, err := a.rec.Rules(ctx, v)
	if err != nil {
		return nil, err
	}
	// history пока не используется (baseline=0); Dev 3 задействует её для next-due от последней замены.
	return engine.BuildAlerts(v, rules), nil
}
