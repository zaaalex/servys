package recommender

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/lane"
)

type askSearch struct {
	q    string
	hits []SearchResult
	err  error
}

func (s *askSearch) Search(_ context.Context, q string) ([]SearchResult, error) {
	s.q = q
	if s.err != nil {
		return nil, s.err
	}
	return s.hits, nil
}

type askFetch struct {
	docs map[string]FetchedDocument
	fail map[string]bool
}

func (f *askFetch) Fetch(_ context.Context, hit SearchResult) (FetchedDocument, error) {
	if f.fail[hit.URL] {
		return FetchedDocument{}, errors.New("не открылась")
	}
	doc, ok := f.docs[hit.URL]
	if !ok {
		return FetchedDocument{}, errors.New("нет страницы")
	}
	return doc, nil
}

type askModel struct {
	called bool
	seen   string
	pages  string
	text   string
	err    error
}

func (m *askModel) Extract(context.Context, VehicleSignature, DocumentChunk) (Extraction, error) {
	return Extraction{}, nil
}

func (m *askModel) Answer(_ context.Context, question, pages string) (string, error) {
	m.called = true
	m.seen = question
	m.pages = pages
	return m.text, m.err
}

func TestAskUsesOneSearchAndOpenedPages(t *testing.T) {
	search := &askSearch{hits: []SearchResult{
		{URL: "https://auto.ru/k3-plugs", Title: "K3 свечи", Snippet: "регламент"},
		{URL: "https://example.com/closed", Title: "K3 закрыто", Snippet: "регламент"},
	}}
	fetch := &askFetch{
		docs: map[string]FetchedDocument{
			"https://auto.ru/k3-plugs": {URL: "https://auto.ru/k3-plugs", Title: "Свечи K3", Text: "Свечи меняют каждые 45 000 км."},
		},
		fail: map[string]bool{"https://example.com/closed": true},
	}
	model := &askModel{text: "Каждые 45 000 км."}
	pipe := &KnowledgePipeline{Search: search, Fetch: fetch, Extract: model, MaxSources: 3}

	reply, err := pipe.Ask(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020}, "какой интервал у свечей")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(search.q, "KIA") || !strings.Contains(search.q, "K3") || !strings.Contains(search.q, "2020") || !strings.Contains(search.q, "свечей") {
		t.Fatalf("запрос поиска: %q", search.q)
	}
	if !model.called || !strings.Contains(model.pages, "Свечи меняют каждые 45 000 км.") || !strings.Contains(model.seen, "какой интервал у свечей") || !strings.Contains(model.seen, "KIA K3 2020") {
		t.Fatalf("модель увидела %q / %q", model.seen, model.pages)
	}
	if reply.Text != "Каждые 45 000 км." || len(reply.Sources) != 1 || reply.Sources[0].URL != "https://auto.ru/k3-plugs" {
		t.Fatalf("ответ: %+v", reply)
	}
}

func TestAskSearchErrorSkipsModel(t *testing.T) {
	model := &askModel{text: "нельзя"}
	pipe := &KnowledgePipeline{
		Search:  &askSearch{err: errors.New("поиск отзывов недоступен")},
		Fetch:   &askFetch{},
		Extract: model,
	}
	_, err := pipe.Ask(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020}, "свечи")
	if err == nil || model.called {
		t.Fatalf("err=%v called=%v", err, model.called)
	}
}

func TestAskEmptyHitsDoesNotCallModel(t *testing.T) {
	model := &askModel{text: "нельзя"}
	pipe := &KnowledgePipeline{Search: &askSearch{}, Fetch: &askFetch{}, Extract: model}
	reply, err := pipe.Ask(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020}, "свечи")
	if err != nil || model.called || reply.Text != "По этому вопросу страницы не нашлись." {
		t.Fatalf("reply=%+v err=%v called=%v", reply, err, model.called)
	}
}

func TestAskBlankModelText(t *testing.T) {
	search := &askSearch{hits: []SearchResult{{URL: "https://auto.ru/k3", Title: "K3", Snippet: "регламент"}}}
	fetch := &askFetch{docs: map[string]FetchedDocument{
		"https://auto.ru/k3": {URL: "https://auto.ru/k3", Title: "K3", Text: "На странице нет свечей."},
	}}
	pipe := &KnowledgePipeline{Search: search, Fetch: fetch, Extract: &askModel{}}
	reply, err := pipe.Ask(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020}, "свечи")
	if err != nil || reply.Text != "Страницы на вопрос не ответили." || len(reply.Sources) != 1 {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}
}

func TestAskAllPagesClosed(t *testing.T) {
	search := &askSearch{hits: []SearchResult{{URL: "https://auto.ru/k3", Title: "K3", Snippet: "регламент"}}}
	model := &askModel{text: "нельзя"}
	pipe := &KnowledgePipeline{
		Search:  search,
		Fetch:   &askFetch{fail: map[string]bool{"https://auto.ru/k3": true}},
		Extract: model,
	}
	_, err := pipe.Ask(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020}, "свечи")
	if err == nil || !strings.Contains(err.Error(), "не открылись") || model.called {
		t.Fatalf("err=%v called=%v", err, model.called)
	}
}

func TestAskOffWithoutAnswerer(t *testing.T) {
	search := &askSearch{}
	pipe := &KnowledgePipeline{Search: search, Fetch: &askFetch{}, Extract: DisabledExtractor{}}
	_, err := pipe.Ask(context.Background(), VehicleSignature{Make: "KIA", Model: "K3", Year: 2020}, "свечи")
	if !errors.Is(err, ErrAskOff) || search.q != "" {
		t.Fatalf("err=%v q=%q", err, search.q)
	}
}

func TestRuntimeAskWithoutLane(t *testing.T) {
	search := &askSearch{hits: []SearchResult{{URL: "https://auto.ru/k3", Title: "K3", Snippet: "регламент"}}}
	fetch := &askFetch{docs: map[string]FetchedDocument{
		"https://auto.ru/k3": {URL: "https://auto.ru/k3", Title: "K3", Text: "Масло меняют каждые 15 000 км."},
	}}
	pipe := &KnowledgePipeline{Search: search, Fetch: fetch, Extract: &askModel{text: "15 000 км."}}
	rt := &Runtime{heavy: &remembering{pipe: pipe}}
	reply, err := rt.Ask(context.Background(), domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020}, "масло")
	if err != nil || reply.Text != "15 000 км." {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}
}

func TestPageWindowKeepsTheMatchingRow(t *testing.T) {
	text := "интервал обслуживания " + strings.Repeat("меню ", 300) + "Под замену Пробег, тыс. км Срок эксплуатации, лет Свечи зажигания 45 3 " + strings.Repeat("хвост ", 300)
	got := pageWindow("какой интервал у свечей", text, 900)
	if !strings.Contains(got, "Свечи зажигания 45 3") || !strings.Contains(got, "тыс. км") {
		t.Fatalf("%s", got)
	}
}

func TestAskWaitsBehindDocumentAnalysis(t *testing.T) {
	heavy := lane.New(0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go heavy.Start(ctx)

	hold := make(chan struct{})
	started := make(chan struct{})
	go func() {
		_ = heavy.Do(ctx, lane.KindSearch, func(context.Context) error {
			close(started)
			<-hold
			return nil
		})
	}()
	<-started

	search := &askSearch{}
	pipe := &KnowledgePipeline{Search: search, Fetch: &askFetch{}, Extract: &askModel{}}
	rt := &Runtime{heavy: &remembering{pipe: pipe, lane: heavy}}
	done := make(chan struct{})
	go func() {
		_, _ = rt.Ask(context.Background(), domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020}, "свечи")
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("вопрос пошёл, пока разбор документов держит модель")
	case <-time.After(40 * time.Millisecond):
	}
	if search.q != "" {
		t.Fatal("поиск запчастей начался во время разбора")
	}
	close(hold)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("вопрос не начался после разбора")
	}
}

func TestRuntimeAskOff(t *testing.T) {
	var rt Runtime
	_, err := rt.Ask(context.Background(), domain.Vehicle{Make: "KIA"}, "масло")
	if !errors.Is(err, ErrAskOff) {
		t.Fatal(err)
	}
}
