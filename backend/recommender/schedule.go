package recommender

import (
	"fmt"
	"strconv"
	"strings"
)

// Фразы, по которым на странице регламента узнаётся узел.
// Срок берётся из числа рядом, не из каталога.
var schedulePhrases = []string{
	"передние тормозные колодки",
	"задние тормозные колодки",
	"тормозные колодки передние",
	"тормозные колодки задние",
	"охлаждающая жидкость",
	"тормозная жидкость",
	"трансмиссионное масло",
	"масло в вариаторе",
	"масло вариатора",
	"масло в двигателе",
	"масло в коробке",
	"свечи зажигания",
	"свеча зажигания",
	"масляный фильтр",
	"фильтр масляный",
	"воздушный фильтр",
	"фильтр воздушный",
	"салонный фильтр",
	"топливный фильтр",
	"фильтр топливный",
	"фильтр салона",
	"прокладка сливной пробки",
	"моторное масло",
	"масло двигателя",
	"масло в акпп",
	"масло акпп",
	"антифриз",
}

// ScheduleRows читает таблицу «Под замену / Пробег, тыс. км».
// Число в такой колонке — тысячи километров: 15 означает 15 000.
func ScheduleRows(text string) []ExtractedFact {
	norm := normalizeSpace(text)
	low := strings.ToLower(norm)
	header := strings.Index(low, "под замену")
	if header < 0 {
		return nil
	}
	head := low[header:]
	if len(head) > 80 {
		head = head[:80]
	}
	if !strings.Contains(head, "тыс") {
		return nil
	}
	region := norm[header:]
	regionLow := low[header:]
	var out []ExtractedFact
	seen := map[string]bool{}
	for i := 0; i < len(regionLow); {
		code, km, years, end, ok := scheduleAt(region, regionLow, i)
		if !ok {
			i++
			continue
		}
		if km < 1000 {
			km *= 1000
		}
		kmCopy := km
		fact := ExtractedFact{
			ComponentCode: code,
			Operation:     "replace",
			IntervalKM:    &kmCopy,
			ScheduleMode:  "mileage",
			UsageMode:     "unknown",
			Evidence:      strings.TrimSpace(region[i:end]),
		}
		if years > 0 {
			months := years * 12
			fact.IntervalMonths = &months
			fact.ScheduleMode = "whichever_first"
		}
		if err := validateFact(fact, norm); err != nil {
			i = end
			continue
		}
		key := code + "\x00" + fact.Operation
		if !seen[key] {
			seen[key] = true
			out = append(out, fact)
		}
		i = end
	}
	return out
}

func scheduleAt(region, low string, i int) (code string, km, years, end int, ok bool) {
	if i > 0 && low[i-1] != ' ' {
		return "", 0, 0, 0, false
	}
	for _, phrase := range schedulePhrases {
		if !strings.HasPrefix(low[i:], phrase) {
			continue
		}
		afterPhrase := i + len(phrase)
		if afterPhrase >= len(region) || region[afterPhrase] != ' ' {
			continue
		}
		rest := strings.TrimLeft(region[afterPhrase:], " ")
		n1, tail, good := leadingInt(rest)
		if !good {
			continue
		}
		mapped, _, yes := quoteComponent(phrase)
		if !yes {
			continue
		}
		end = len(region) - len(tail)
		n2 := 0
		if tail != "" && tail[0] == ' ' {
			yearTail := strings.TrimLeft(tail, " ")
			if n, more, yes := leadingInt(yearTail); yes && n <= 20 && len(strconv.Itoa(n)) <= 2 {
				if more == "" || more[0] == ' ' {
					n2 = n
					end = len(region) - len(more)
				}
			}
		}
		return mapped, n1, n2, end, true
	}
	return "", 0, 0, 0, false
}

func leadingInt(s string) (int, string, bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i > 6 {
		return 0, s, false
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil || n <= 0 {
		return 0, s, false
	}
	return n, s[i:], true
}

func scheduleNote(km, months int) string {
	if km <= 0 {
		return ""
	}
	kmText := fmt.Sprintf("%d км", km)
	if km%1000 == 0 {
		kmText = fmt.Sprintf("%d тыс. км", km/1000)
	}
	if months > 0 && months%12 == 0 {
		return "По регламенту: " + kmText + " или " + yearsLabel(months/12)
	}
	return "По регламенту: " + kmText
}

func yearsLabel(y int) string {
	n := y % 100
	if n < 11 || n > 14 {
		switch y % 10 {
		case 1:
			return fmt.Sprintf("%d год", y)
		case 2, 3, 4:
			return fmt.Sprintf("%d года", y)
		}
	}
	return fmt.Sprintf("%d лет", y)
}

func isSchedulePage(url string) bool {
	u := strings.ToLower(url)
	return strings.Contains(u, "reglament") || strings.Contains(u, "regulations-to")
}
