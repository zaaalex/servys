package recommender

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type VehicleSignature struct {
	Make, Model, Market, Powertrain string
	Year                            int
}
type DocumentChunk struct{ SourceID, URL, Title, Text string }
type ExtractedFact struct {
	ComponentCode  string `json:"componentCode"`
	Operation      string `json:"operation"`
	IntervalKM     *int   `json:"intervalKm"`
	IntervalMonths *int   `json:"intervalMonths"`
	ScheduleMode   string `json:"scheduleMode"`
	UsageMode      string `json:"usageMode"`
	Evidence       string `json:"evidence"`
}
type Extraction struct {
	Facts []ExtractedFact `json:"facts"`
}
type KnowledgeExtractor interface {
	Extract(context.Context, VehicleSignature, DocumentChunk) (Extraction, error)
}

var ErrExtractionDisabled = errors.New("knowledge extraction disabled")

type DisabledExtractor struct{}

func (DisabledExtractor) Extract(context.Context, VehicleSignature, DocumentChunk) (Extraction, error) {
	return Extraction{}, ErrExtractionDisabled
}

type FixtureExtractor struct {
	Extraction Extraction
	Err        error
}

func (f FixtureExtractor) Extract(context.Context, VehicleSignature, DocumentChunk) (Extraction, error) {
	return f.Extraction, f.Err
}

var components = map[string][2]int{"engine_oil": {1000, 50000}, "engine_oil_filter": {1000, 50000}, "engine_air_filter": {1000, 150000}, "cabin_filter": {1000, 150000}, "spark_plugs": {5000, 250000}, "brake_fluid": {1000, 150000}, "engine_coolant": {5000, 300000}, "transmission_fluid": {5000, 300000}, "timing_drive": {10000, 300000}, "fuel_filter": {1000, 200000}}
var operations = map[string]bool{"replace": true, "inspect": true, "measure": true, "adjust": true, "diagnose": true}
var schedules = map[string]bool{"mileage": true, "time": true, "whichever_first": true, "unspecified": true}
var usages = map[string]bool{"normal": true, "severe": true, "unknown": true}

func ValidateExtraction(x Extraction, c DocumentChunk) (Extraction, []error) {
	a := Extraction{Facts: []ExtractedFact{}}
	var bad []error
	t := normalizeSpace(c.Text)
	for i, f := range x.Facts {
		if err := validateFact(f, t); err != nil {
			bad = append(bad, fmt.Errorf("fact %d: %w", i, err))
		} else {
			a.Facts = append(a.Facts, f)
		}
	}
	return a, bad
}
func validateFact(f ExtractedFact, text string) error {
	r, ok := components[f.ComponentCode]
	if !ok {
		return errors.New("unknown component")
	}
	if !operations[f.Operation] || !schedules[f.ScheduleMode] || !usages[f.UsageMode] {
		return errors.New("invalid enum")
	}
	if f.IntervalKM == nil && f.IntervalMonths == nil {
		return errors.New("fact has no interval")
	}
	if f.IntervalKM != nil && (*f.IntervalKM < r[0] || *f.IntervalKM > r[1]) {
		return errors.New("intervalKm out of range")
	}
	if f.IntervalMonths != nil && (*f.IntervalMonths < 1 || *f.IntervalMonths > 240) {
		return errors.New("intervalMonths out of range")
	}
	ev := normalizeSpace(f.Evidence)
	if ev == "" || !strings.Contains(text, ev) {
		return errors.New("evidence is not an exact normalized substring")
	}
	low := strings.ToLower(ev)
	if f.Operation == "replace" && !containsAny(low, "replace", "change", "замен", "更换") {
		return errors.New("replace unsupported")
	}
	if f.Operation == "inspect" && !containsAny(low, "inspect", "check", "проверк", "осмотр", "检查") {
		return errors.New("inspect unsupported")
	}
	return nil
}
func normalizeSpace(s string) string { return strings.Join(strings.Fields(s), " ") }
func containsAny(s string, xs ...string) bool {
	for _, x := range xs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}
