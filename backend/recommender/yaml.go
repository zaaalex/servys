package recommender

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/zaaalex/servys/backend/domain"
)

type YAMLRecommender struct{ variants []yamlVariant }

type yamlVariant struct {
	make, model      string
	from, to, cc, hp int
	rules            []domain.Rule
}

func NewYAML(path string) (*YAMLRecommender, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
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
			case "category":
				r.Category = val
			case "community_km":
				ensureCommunity(r).RealIntervalKm = n
			case "community_note":
				ensureCommunity(r).Note = unquote(val)
			case "community_source":
				ensureCommunity(r).Source = unquote(val)
			case "community_reports":
				ensureCommunity(r).Reports = n
			}
		}
	}
	if err = s.Err(); err != nil {
		return nil, err
	}
	for vi := range y.variants {
		for ri := range y.variants[vi].rules {
			rr := &y.variants[vi].rules[ri]
			if rr.Title == "" {
				rr.Title = TitleFor(rr.Code)
			}
			if rr.Category == "" {
				rr.Category = CategoryFor(rr.Code)
			}
		}
	}
	return y, nil
}

func ensureCommunity(r *domain.Rule) *domain.CommunityNote {
	if r.Community == nil {
		r.Community = &domain.CommunityNote{}
	}
	return r.Community
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

func (y *YAMLRecommender) Rules(_ context.Context, v domain.Vehicle) ([]domain.Rule, error) {
	for _, x := range y.variants {
		if !matchVariant(x, v) {
			continue
		}
		out := make([]domain.Rule, 0, len(x.rules))
		for _, r := range x.rules {
			if r.Source == "demo" || r.Source == "typical" || r.Source == "stub" {
				continue
			}
			if r.Community != nil && (r.Community.Source == "demo" || r.Community.Source == "") {
				r.Community = nil
			}
			out = append(out, r)
		}
		if len(out) == 0 {
			continue
		}
		return out, nil
	}
	return nil, nil
}

func matchVariant(x yamlVariant, v domain.Vehicle) bool {
	if !strings.EqualFold(x.make, v.Make) || !strings.EqualFold(x.model, v.Model) {
		return false
	}
	if (x.from != 0 && v.Year < x.from) || (x.to != 0 && v.Year > x.to) {
		return false
	}
	if (x.cc != 0 && v.EngineCC != x.cc) || (x.hp != 0 && v.PowerHP != x.hp) {
		return false
	}
	return true
}
