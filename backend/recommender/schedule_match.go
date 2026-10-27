package recommender

import (
	"regexp"
	"strconv"
	"strings"
)

// ScheduleTarget — чем страница регламента выбирается под конкретную машину.
// Год, мощность и рынок берутся из карточки и VIN, не из первой таблицы на странице.
type ScheduleTarget struct {
	Year    int
	PowerHP int
	Market  string
}

type schedBlock struct {
	heading  string
	from, to int
	china    bool
	mods     []modHit
}

type modHit struct {
	name  string
	power int
	table string
}

var (
	yearSpanRe = regexp.MustCompile(`(19\d{2}|20\d{2})\s*[-–]\s*(19\d{2}|20\d{2})`)
	modRe      = regexp.MustCompile(`(?i)(\d\.\d[^()]{0,40})\((\d+)\s*л\.\s*с\.\)`)
)

// ScheduleRowsFor читает таблицу «Под замену» того поколения и той мощности,
// которые совпали с машиной. Без года, мощности и рынка остаётся прежнее чтение
// всей страницы: его используют тесты и машины, у которых нечего сопоставлять.
func ScheduleRowsFor(text string, t ScheduleTarget) ([]ExtractedFact, string) {
	norm := normalizeSpace(text)
	if !targeted(t) {
		return ScheduleRows(norm), ""
	}
	if !strings.Contains(norm, "Регламент ТО") {
		return ScheduleRows(norm), ""
	}
	hits := filterBlocks(parseBlocks(norm), t)
	if len(hits) == 0 {
		return nil, ""
	}
	if t.PowerHP > 0 {
		var matched []struct {
			block schedBlock
			mod   modHit
		}
		for _, b := range hits {
			for _, m := range b.mods {
				if m.power == t.PowerHP {
					matched = append(matched, struct {
						block schedBlock
						mod   modHit
					}{b, m})
				}
			}
		}
		if len(matched) != 1 {
			return nil, ""
		}
		facts := ScheduleRows(matched[0].mod.table)
		if len(facts) == 0 {
			return nil, ""
		}
		return facts, sectionLabel(matched[0].block.heading, matched[0].mod.name)
	}
	var first []ExtractedFact
	var agreed map[string]ExtractedFact
	seenHeading := map[string]bool{}
	var headings []string
	for _, b := range hits {
		if len(b.mods) == 0 {
			continue
		}
		if b.heading != "" && !seenHeading[b.heading] {
			seenHeading[b.heading] = true
			headings = append(headings, b.heading)
		}
		for _, m := range b.mods {
			rows := ScheduleRows(m.table)
			if len(rows) == 0 {
				continue
			}
			if agreed == nil {
				first = rows
				agreed = indexFacts(rows)
				continue
			}
			other := indexFacts(rows)
			for key, fact := range agreed {
				got, ok := other[key]
				if !ok || !sameOne(fact, got) {
					delete(agreed, key)
				}
			}
		}
	}
	if len(agreed) == 0 {
		return nil, ""
	}
	out := make([]ExtractedFact, 0, len(agreed))
	for _, fact := range first {
		if _, ok := agreed[fact.ComponentCode+"\x00"+fact.Operation]; ok {
			out = append(out, fact)
		}
	}
	label := strings.Join(headings, "; ")
	if label != "" {
		label += ". Одинаково у модификаций этих лет"
	}
	return out, label
}

func targeted(t ScheduleTarget) bool {
	return t.Year > 0 || t.PowerHP > 0 || t.Market != ""
}

func marketFromVIN(vin string) string {
	vin = strings.ToUpper(strings.TrimSpace(vin))
	if len(vin) < 3 {
		return ""
	}
	// LJD — Dongfeng Yueda Kia, китайский рынок.
	if vin[:3] == "LJD" {
		return "china"
	}
	return ""
}

func parseBlocks(norm string) []schedBlock {
	const mark = "Регламент ТО "
	parts := strings.Split(norm, mark)
	var out []schedBlock
	for _, part := range parts[1:] {
		block := mark + part
		heading := blockHeading(part)
		b := schedBlock{heading: heading, mods: findMods(block)}
		if m := yearSpanRe.FindStringSubmatch(heading); len(m) == 3 {
			b.from, _ = strconv.Atoi(m[1])
			b.to, _ = strconv.Atoi(m[2])
		}
		low := strings.ToLower(heading)
		b.china = strings.Contains(low, "china") || strings.Contains(low, "китай")
		out = append(out, b)
	}
	return out
}

func blockHeading(afterMark string) string {
	end := len(afterMark)
	for _, mark := range []string{" Ещё", " Седан", " Хэтчбек", " Все характеристики", " Модификация"} {
		if i := strings.Index(afterMark, mark); i > 0 && i < end {
			end = i
		}
	}
	if m := modRe.FindStringIndex(afterMark); m != nil && m[0] < end {
		end = m[0]
	}
	return strings.TrimSpace(afterMark[:end])
}

func findMods(block string) []modHit {
	idxs := modRe.FindAllStringSubmatchIndex(block, -1)
	out := make([]modHit, 0, len(idxs))
	for i, idx := range idxs {
		if len(idx) < 6 {
			continue
		}
		power, err := strconv.Atoi(block[idx[4]:idx[5]])
		if err != nil {
			continue
		}
		end := len(block)
		if i+1 < len(idxs) {
			end = idxs[i+1][0]
		}
		region := block[idx[1]:end]
		table := ""
		if at := strings.Index(strings.ToLower(region), "под замену"); at >= 0 {
			table = region[at:]
		}
		out = append(out, modHit{
			name:  strings.TrimSpace(block[idx[0]:idx[1]]),
			power: power,
			table: table,
		})
	}
	return out
}

func filterBlocks(blocks []schedBlock, t ScheduleTarget) []schedBlock {
	var hits []schedBlock
	for _, b := range blocks {
		if t.Year > 0 && (b.from == 0 || t.Year < b.from || t.Year > b.to) {
			continue
		}
		hits = append(hits, b)
	}
	if t.Market != "china" {
		return hits
	}
	var china []schedBlock
	for _, b := range hits {
		if b.china {
			china = append(china, b)
		}
	}
	if len(china) == 0 {
		return hits
	}
	return china
}

func sectionLabel(heading, mod string) string {
	heading = strings.TrimSpace(heading)
	mod = strings.TrimSpace(mod)
	switch {
	case heading != "" && mod != "":
		return heading + ", " + mod
	case heading != "":
		return heading
	default:
		return mod
	}
}

func indexFacts(rows []ExtractedFact) map[string]ExtractedFact {
	out := make(map[string]ExtractedFact, len(rows))
	for _, fact := range rows {
		out[fact.ComponentCode+"\x00"+fact.Operation] = fact
	}
	return out
}

func sameOne(a, b ExtractedFact) bool {
	left := factKey([]ExtractedFact{a})
	right := factKey([]ExtractedFact{b})
	key := a.ComponentCode + "\x00" + a.Operation
	return left[key] == right[key]
}

func sameFacts(a, b []ExtractedFact) bool {
	if len(a) != len(b) {
		return false
	}
	left := factKey(a)
	right := factKey(b)
	if len(left) != len(right) {
		return false
	}
	for k, v := range left {
		if right[k] != v {
			return false
		}
	}
	return true
}

func factKey(facts []ExtractedFact) map[string]string {
	out := make(map[string]string, len(facts))
	for _, f := range facts {
		km, months := 0, 0
		if f.IntervalKM != nil {
			km = *f.IntervalKM
		}
		if f.IntervalMonths != nil {
			months = *f.IntervalMonths
		}
		out[f.ComponentCode+"\x00"+f.Operation] = strconv.Itoa(km) + ":" + strconv.Itoa(months)
	}
	return out
}
