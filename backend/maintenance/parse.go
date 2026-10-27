// Package maintenance читает свободный текст сервисной книжки и предлагает
// подтверждённые работы. Срок следующего обслуживания здесь не считается.
package maintenance

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/zaaalex/servys/backend/recommender"
)

// Suggestion — одна работа, которую можно записать в историю после подтверждения.
type Suggestion struct {
	RuleCode    string
	Title       string
	Odometer    int
	PerformedAt time.Time // нулевое время, если даты в строке не было
	Source      string
}

type phrase struct {
	text string
	code string
}

var (
	// \b в RE2 — только латиница. После «км» допускаем конец строки или не-букву.
	kmPattern     = regexp.MustCompile(`(?i)(\d{1,3}(?:[ \x{00A0}]\d{3})+|\d{3,7})\s*(?:км|km)(?:\P{L}|$)`)
	thousand      = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*тыс\.?(?:\s*(?:км|km)(?:\P{L}|$))?`)
	probegPattern = regexp.MustCompile(`(?i)пробег(?:\s*,\s*(?:км|km))?\s*[:\-]?\s*(\d{1,3}(?:[ \x{00A0}]\d{3})+|\d{3,7})`)
	dateDMY       = regexp.MustCompile(`\b(\d{1,2})[./](\d{1,2})[./](\d{2,4})\b`)
	dateISO       = regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`)
)

// Parse достаёт из текста работы с пробегом. Нераспознанные строки не становятся фактами.
func Parse(text string, now time.Time) []Suggestion {
	return parseLines(text, now, phrases())
}

// ParseDocument читает заказ-наряд: пробег в шапке, работы отдельными строками.
// Если в тексте несколько разных пробегов, работы без своего пробега не угадываются.
func ParseDocument(text string, now time.Time) []Suggestion {
	direct := Parse(text, now)
	if len(direct) > 0 {
		return direct
	}
	kms := distinctOdometers(text)
	if len(kms) != 1 {
		return nil
	}
	when, hasDate := singleDate(text, now)
	list := phrases()
	var out []Suggestion
	seen := map[string]struct{}{}
	for _, part := range partsOf(text) {
		for _, code := range codesIn(part, list) {
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			item := Suggestion{
				RuleCode: code,
				Title:    recommender.TitleFor(code),
				Odometer: kms[0],
				Source:   part,
			}
			if hasDate {
				item.PerformedAt = when
			}
			out = append(out, item)
		}
	}
	return out
}

func parseLines(text string, now time.Time, list []phrase) []Suggestion {
	var out []Suggestion
	seen := map[string]struct{}{}
	for _, part := range partsOf(text) {
		km, ok := odometerOf(part)
		if !ok {
			continue
		}
		when, hasDate := dateOf(part, now)
		for _, code := range codesIn(part, list) {
			key := code + "|" + strconv.Itoa(km)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			item := Suggestion{
				RuleCode: code,
				Title:    recommender.TitleFor(code),
				Odometer: km,
				Source:   part,
			}
			if hasDate {
				item.PerformedAt = when
			}
			out = append(out, item)
		}
	}
	return out
}

func partsOf(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		for _, part := range strings.Split(line, ";") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func phrases() []phrase {
	var list []phrase
	for code, spec := range recommender.Catalog {
		list = append(list, phrase{text: strings.ToLower(spec.TitleRU), code: code})
	}
	extra := []phrase{
		{"масло", "engine_oil"},
		{"моторное масло", "engine_oil"},
		{"моторного масла", "engine_oil"},
		{"масло двигателя", "engine_oil"},
		{"замена масла", "engine_oil"},
		{"замену масла", "engine_oil"},
		{"масло двс", "engine_oil"},
		{"масляный фильтр", "engine_oil_filter"},
		{"масляного фильтра", "engine_oil_filter"},
		{"фильтр масла", "engine_oil_filter"},
		{"воздушный фильтр", "engine_air_filter"},
		{"воздушного фильтра", "engine_air_filter"},
		{"салонный фильтр", "cabin_filter"},
		{"салонного фильтра", "cabin_filter"},
		{"топливный фильтр", "fuel_filter"},
		{"топливного фильтра", "fuel_filter"},
		{"свечи зажигания", "spark_plugs"},
		{"свечей зажигания", "spark_plugs"},
		{"свечи", "spark_plugs"},
		{"свечей", "spark_plugs"},
		{"тормозная жидкость", "brake_fluid"},
		{"тормозуха", "brake_fluid"},
		{"передние колодки", "brake_pads_front"},
		{"передние тормозные колодки", "brake_pads_front"},
		{"задние колодки", "brake_pads_rear"},
		{"задние тормозные колодки", "brake_pads_rear"},
		{"передних колодок", "brake_pads_front"},
		{"задних колодок", "brake_pads_rear"},
		{"тормозных колодок", "brake_pads_front"},
		{"колодки", "brake_pads_front"},
		{"колодок", "brake_pads_front"},
		{"амортизаторов", "shock_absorbers"},
		{"амортизатора", "shock_absorbers"},
		{"амортизатор", "shock_absorbers"},
		{"ремень грм", "timing_belt"},
		{"грм", "timing_belt"},
		{"антифриз", "engine_coolant"},
		{"охлаждайка", "engine_coolant"},
		{"масло акпп", "transmission_fluid"},
		{"масло в коробке", "transmission_fluid"},
		{"аккумулятор", "battery"},
		{"акб", "battery"},
	}
	list = append(list, extra...)
	sort.Slice(list, func(i, j int) bool {
		if len([]rune(list[i].text)) == len([]rune(list[j].text)) {
			return list[i].text < list[j].text
		}
		return len([]rune(list[i].text)) > len([]rune(list[j].text))
	})
	return list
}

func codesIn(line string, phrases []phrase) []string {
	low := strings.ToLower(line)
	runes := []rune(low)
	taken := make([]bool, len(runes))
	var codes []string
	have := map[string]struct{}{}
	for _, p := range phrases {
		needle := []rune(p.text)
		if len(needle) == 0 {
			continue
		}
		for i := 0; i+len(needle) <= len(runes); i++ {
			if string(runes[i:i+len(needle)]) != p.text {
				continue
			}
			if !boundary(runes, i, i+len(needle)) {
				continue
			}
			overlap := false
			for j := i; j < i+len(needle); j++ {
				if taken[j] {
					overlap = true
					break
				}
			}
			if overlap {
				continue
			}
			for j := i; j < i+len(needle); j++ {
				taken[j] = true
			}
			if _, ok := have[p.code]; !ok {
				have[p.code] = struct{}{}
				codes = append(codes, p.code)
			}
			break
		}
	}
	return codes
}

func boundary(runes []rune, from, to int) bool {
	if from > 0 && isWord(runes[from-1]) {
		return false
	}
	if to < len(runes) && isWord(runes[to]) {
		return false
	}
	return true
}

func isWord(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func distinctOdometers(text string) []int {
	seen := map[int]struct{}{}
	var out []int
	add := func(n int) {
		if n <= 0 || n > 2_000_000 {
			return
		}
		if _, ok := seen[n]; ok {
			return
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	for _, m := range thousand.FindAllStringSubmatch(text, -1) {
		if n, ok := thousandKm(m[1]); ok {
			add(n)
		}
	}
	for _, m := range kmPattern.FindAllStringSubmatch(text, -1) {
		if n, ok := plainKm(m[1]); ok {
			add(n)
		}
	}
	for _, m := range probegPattern.FindAllStringSubmatch(text, -1) {
		if n, ok := plainKm(m[1]); ok {
			add(n)
		}
	}
	return out
}

func singleDate(text string, now time.Time) (time.Time, bool) {
	seen := map[string]time.Time{}
	consider := func(t time.Time, ok bool) {
		if ok {
			seen[t.Format("2006-01-02")] = t
		}
	}
	for _, m := range dateISO.FindAllStringSubmatch(text, -1) {
		consider(buildDate(m[1], m[2], m[3], now))
	}
	for _, m := range dateDMY.FindAllStringSubmatch(text, -1) {
		year := m[3]
		if len(year) == 2 {
			year = "20" + year
		}
		consider(buildDate(year, m[2], m[1], now))
	}
	if len(seen) != 1 {
		return time.Time{}, false
	}
	for _, t := range seen {
		return t, true
	}
	return time.Time{}, false
}

func odometerOf(line string) (int, bool) {
	if m := thousand.FindStringSubmatch(line); m != nil {
		if km, ok := thousandKm(m[1]); ok {
			return km, true
		}
	}
	m := kmPattern.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	return plainKm(m[1])
}

func thousandKm(raw string) (int, bool) {
	raw = strings.ReplaceAll(raw, ",", ".")
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	km := int(f*1000 + 0.5)
	if km <= 0 || km > 2_000_000 {
		return 0, false
	}
	return km, true
}

func plainKm(raw string) (int, bool) {
	raw = strings.ReplaceAll(raw, " ", "")
	raw = strings.ReplaceAll(raw, "\u00a0", "")
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 || n > 2_000_000 {
		return 0, false
	}
	return n, true
}

func dateOf(line string, now time.Time) (time.Time, bool) {
	if m := dateISO.FindStringSubmatch(line); m != nil {
		return buildDate(m[1], m[2], m[3], now)
	}
	if m := dateDMY.FindStringSubmatch(line); m != nil {
		year := m[3]
		if len(year) == 2 {
			year = "20" + year
		}
		return buildDate(year, m[2], m[1], now)
	}
	return time.Time{}, false
}

func buildDate(year, month, day string, now time.Time) (time.Time, bool) {
	y, errY := strconv.Atoi(year)
	mo, errM := strconv.Atoi(month)
	d, errD := strconv.Atoi(day)
	if errY != nil || errM != nil || errD != nil {
		return time.Time{}, false
	}
	t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	if t.Year() != y || int(t.Month()) != mo || t.Day() != d {
		return time.Time{}, false
	}
	if t.After(now.Add(24 * time.Hour)) {
		return time.Time{}, false
	}
	return t, true
}
