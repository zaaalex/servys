package recommender

import "testing"

func TestDecode1251Review(t *testing.T) {
	raw := []byte{0xCC, 0xE0, 0xF1, 0xEB, 0xEE, ' ', 0xEC, 0xE5, 0xED, 0xFF, 0xFE}
	got := decodePage(raw, "text/html; charset=windows-1251")
	if got != "Масло меняю" {
		t.Fatalf("cp1251: %q", got)
	}
}

func TestIntervalQuoteFromOwnerSentence(t *testing.T) {
	text := "Масло меняю раз в 5 тыс км. За 130 т.км масло, фильтра, колодки. На пробеге 105000км, только масла."
	facts := IntervalQuotes(text)
	if len(facts) != 1 {
		t.Fatalf("facts %d", len(facts))
	}
	f := facts[0]
	if f.ComponentCode != "engine_oil" || f.IntervalKM == nil || *f.IntervalKM != 5000 {
		t.Fatalf("fact %+v", f)
	}
	if f.Evidence != "Масло меняю раз в 5 тыс км." {
		t.Fatalf("evidence %q", f.Evidence)
	}
}

func TestRankPrefersReviewOfThisModel(t *testing.T) {
	ranked := RankAndDedupeSources("K3", []SearchResult{
		{URL: "https://dealer.example/kia-maintenance-schedule", Title: "Kia maintenance schedule replace interval", Score: 3},
		{URL: "https://www.drom.ru/reviews/kia/k3/5kopeek/", Title: "Киа К3 отзывы владельцев", Score: 1},
	})
	if len(ranked) != 2 || ranked[0].URL != "https://www.drom.ru/reviews/kia/k3/5kopeek/" {
		t.Fatalf("order %+v", ranked)
	}
}
