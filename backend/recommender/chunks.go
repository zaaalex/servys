package recommender

import (
	"context"
	"regexp"
	"strings"
)

type DeterministicChunkSelector struct{ MaxRunes, OverlapRunes, MaxChunks int }

var numberPattern = regexp.MustCompile(`\d[\d\s,.]*`)
var unitTerms = []string{" km", "км", "kilomet", "mile", " mi", "месяц", "месяцев", "month", "year", "год", "лет", "公里", "个月", "年"}
var operationTerms = []string{"replace", "change", "inspect", "check", "замен", "проверк", "осмотр", "更换", "检查", "每隔", "every", "кажды"}
var componentTerms = []string{"engine oil", "motor oil", "масло двигателя", "моторное масло", "机油", "oil filter", "масляный фильтр", "机油滤清器", "air filter", "воздушный фильтр", "空气滤清器", "cabin filter", "салонный фильтр", "brake fluid", "тормозная жидкость", "transmission fluid", "трансмиссионное масло", "spark plug", "свеч", "火花塞", "coolant", "антифриз", "охлаждающая жидкость", "timing belt", "timing chain", "грм", "fuel filter", "топливный фильтр"}

func (s DeterministicChunkSelector) Select(_ context.Context, _ VehicleSignature, doc FetchedDocument) ([]DocumentChunk, error) {
	max := s.MaxRunes
	if max <= 0 {
		max = 4000
	}
	overlap := s.OverlapRunes
	if overlap < 0 || overlap >= max {
		overlap = 400
	}
	limit := s.MaxChunks
	if limit <= 0 {
		limit = 8
	}
	runes := []rune(doc.Text)
	var candidates []DocumentChunk
	for start := 0; start < len(runes); {
		end := start + max
		if end > len(runes) {
			end = len(runes)
		}
		text := normalizeSpace(string(runes[start:end]))
		lower := strings.ToLower(text)
		if numberPattern.MatchString(lower) && hasTerm(lower, unitTerms) && hasTerm(lower, operationTerms) && hasTerm(lower, componentTerms) {
			candidates = append(candidates, DocumentChunk{SourceID: doc.SourceID, URL: doc.URL, Title: doc.Title, Text: text})
			if len(candidates) == limit {
				break
			}
		}
		if end == len(runes) {
			break
		}
		start = end - overlap
	}
	seen := map[string]bool{}
	out := candidates[:0]
	for _, candidate := range candidates {
		if !seen[candidate.Text] {
			seen[candidate.Text] = true
			out = append(out, candidate)
		}
	}
	return out, nil
}

func hasTerm(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}
