package maintenance

import (
	"testing"
	"time"
)

func TestParseServiceBook(t *testing.T) {
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	text := "" +
		"12.03.2024 замена масла и масляного фильтра, пробег 45 000 км\n" +
		"свечи 60 тыс км\n" +
		"просто осмотр без цифр\n" +
		"передние колодки 30.01.2025 на 72000 км; задние колодки на 72000 км\n"
	got := Parse(text, now)
	if len(got) != 5 {
		t.Fatalf("хотели 5 работ, получили %d: %#v", len(got), got)
	}
	byCode := map[string]Suggestion{}
	for _, s := range got {
		byCode[s.RuleCode] = s
	}
	oil := byCode["engine_oil"]
	if oil.Odometer != 45000 || oil.PerformedAt.Format("2006-01-02") != "2024-03-12" {
		t.Fatalf("масло: %+v", oil)
	}
	if byCode["engine_oil_filter"].Odometer != 45000 {
		t.Fatalf("фильтр не отделился от масла: %+v", byCode["engine_oil_filter"])
	}
	if byCode["spark_plugs"].Odometer != 60000 || !byCode["spark_plugs"].PerformedAt.IsZero() {
		t.Fatalf("свечи: %+v", byCode["spark_plugs"])
	}
	if byCode["brake_pads_front"].Odometer != 72000 || byCode["brake_pads_rear"].Odometer != 72000 {
		t.Fatalf("колодки: %+v %+v", byCode["brake_pads_front"], byCode["brake_pads_rear"])
	}
}

func TestParseIgnoresFutureDate(t *testing.T) {
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	got := Parse("01.01.2030 масло 10000 км", now)
	if len(got) != 1 || !got[0].PerformedAt.IsZero() {
		t.Fatalf("дата из будущего не должна записываться: %+v", got)
	}
}
