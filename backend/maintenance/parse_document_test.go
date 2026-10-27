package maintenance

import (
	"testing"
	"time"
)

func TestParseDocumentAttachesWorksToSingleMileage(t *testing.T) {
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	text := "" +
		"Заказ-наряд №15\n" +
		"Дата 12.03.2024\n" +
		"Пробег: 45 230 км\n" +
		"1. Замена масла двигателя\n" +
		"2. Замена тормозных колодок\n" +
		"3. Замена амортизаторов\n"
	got := ParseDocument(text, now)
	byCode := map[string]Suggestion{}
	for _, item := range got {
		byCode[item.RuleCode] = item
	}
	for _, code := range []string{"engine_oil", "brake_pads_front", "shock_absorbers"} {
		item, ok := byCode[code]
		if !ok || item.Odometer != 45230 || item.PerformedAt.Format("2006-01-02") != "2024-03-12" {
			t.Fatalf("%s: %+v", code, item)
		}
	}
}

func TestParseDocumentDoesNotGuessAcrossMileages(t *testing.T) {
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	got := ParseDocument("Пробег 10 000 км\nПробег 20 000 км\nзамена масла\nзамена колодок", now)
	if len(got) != 0 {
		t.Fatalf("разные пробеги нельзя склеить: %+v", got)
	}
}

func TestParseDocumentKeepsLineParser(t *testing.T) {
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	text := "12.03.2024 замена масла и масляного фильтра, пробег 45 000 км\nсвечи 60 тыс км\n"
	if len(ParseDocument(text, now)) != len(Parse(text, now)) {
		t.Fatal("строки с пробегом должны читаться как раньше")
	}
}
