package recommender

import (
	"regexp"
	"strconv"
	"strings"
)

var kmAmount = regexp.MustCompile(`(?i)(?:раз\s+в|каждые|каждых|через)\s+(\d{1,3}(?:[ \x{00a0}]\d{3})+|\d{1,6})\s*(тыс(?:яч[ауи])?\.?)?\s*км`)

func IntervalQuotes(text string) []ExtractedFact {
	norm := normalizeSpace(text)
	var out []ExtractedFact
	seen := map[string]bool{}
	for _, sentence := range splitSentences(norm) {
		match := kmAmount.FindStringSubmatch(sentence)
		if match == nil {
			continue
		}
		km := parseKM(match[1], match[2])
		if km <= 0 {
			continue
		}
		code, op, ok := quoteComponent(strings.ToLower(sentence))
		if !ok {
			continue
		}
		kmCopy := km
		fact := ExtractedFact{
			ComponentCode: code,
			Operation:     op,
			IntervalKM:    &kmCopy,
			ScheduleMode:  "mileage",
			UsageMode:     "unknown",
			Evidence:      sentence,
		}
		if err := validateFact(fact, norm); err != nil {
			continue
		}
		key := code + "\x00" + op
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, fact)
	}
	return out
}

func splitSentences(text string) []string {
	var out []string
	start := 0
	for i, r := range text {
		if r == '.' || r == '!' || r == '?' || r == ';' {
			sentence := strings.TrimSpace(text[start : i+1])
			if sentence != "" {
				out = append(out, sentence)
			}
			start = i + 1
		}
	}
	if tail := strings.TrimSpace(text[start:]); tail != "" {
		out = append(out, tail)
	}
	return out
}

func parseKM(number, unit string) int {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, number)
	n, err := strconv.Atoi(digits)
	if err != nil || n <= 0 {
		return 0
	}
	if strings.HasPrefix(strings.ToLower(unit), "тыс") {
		n *= 1000
	}
	return n
}

func quoteComponent(s string) (string, string, bool) {
	op := "replace"
	if strings.Contains(s, "провер") || strings.Contains(s, "осмотр") || strings.Contains(s, "inspect") {
		op = "inspect"
	}
	switch {
	case (strings.Contains(s, "вариатор") || strings.Contains(s, "акпп") || strings.Contains(s, "коробк")) && strings.Contains(s, "масл"):
		return "transmission_fluid", op, true
	case strings.Contains(s, "антифриз") || strings.Contains(s, "охлажда"):
		return "engine_coolant", op, true
	case strings.Contains(s, "тормоз") && strings.Contains(s, "жидк"):
		return "brake_fluid", op, true
	case strings.Contains(s, "прокладк") && strings.Contains(s, "пробк"):
		return "oil_drain_plug_gasket", op, true
	case strings.Contains(s, "свеч"):
		return "spark_plugs", op, true
	case strings.Contains(s, "колодк") && strings.Contains(s, "задн"):
		return "brake_pads_rear", op, true
	case strings.Contains(s, "колодк") && strings.Contains(s, "перед"):
		return "brake_pads_front", op, true
	case strings.Contains(s, "фильтр") && strings.Contains(s, "салон"):
		return "cabin_filter", op, true
	case strings.Contains(s, "фильтр") && strings.Contains(s, "возду"):
		return "engine_air_filter", op, true
	case strings.Contains(s, "фильтр") && strings.Contains(s, "топли"):
		return "fuel_filter", op, true
	case strings.Contains(s, "фильтр") && strings.Contains(s, "масл"):
		return "engine_oil_filter", op, true
	case strings.Contains(s, "масл"):
		return "engine_oil", op, true
	default:
		return "", "", false
	}
}
