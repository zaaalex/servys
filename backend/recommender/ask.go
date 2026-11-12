package recommender

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/lane"
)

// ErrAskOff — модель или поиск не включены, вопрос задать нельзя.
var ErrAskOff = errors.New("вопрос к модели выключен")

type Citation struct {
	Title string
	URL   string
}

type Reply struct {
	Text    string
	Sources []Citation
}

// pageAnswerer отвечает на вопрос только по уже открытому тексту.
type pageAnswerer interface {
	Answer(context.Context, string, string) (string, error)
}

func (p *KnowledgePipeline) answerer() (pageAnswerer, bool) {
	if p != nil && p.Reader != nil {
		if answerer, ok := p.Reader.(pageAnswerer); ok {
			return answerer, true
		}
	}
	if p == nil || p.Extract == nil {
		return nil, false
	}
	answerer, ok := p.Extract.(pageAnswerer)
	return answerer, ok
}

// Ask ищет страницы по вопросу владельца и просит модель ответить по ним.
// Поиск один. Адрес берётся только из его ответа.
func (p *KnowledgePipeline) Ask(ctx context.Context, s VehicleSignature, question string) (Reply, error) {
	if p == nil || p.Search == nil || p.Fetch == nil {
		return Reply{}, ErrAskOff
	}
	answerer, ok := p.answerer()
	if !ok {
		return Reply{}, ErrAskOff
	}
	question = normalizeSpace(question)
	if question == "" {
		return Reply{}, errors.New("пустой вопрос")
	}
	hits, err := p.Search.Search(ctx, askQuery(s, question))
	if err != nil {
		return Reply{}, err
	}
	hits = RankAndDedupeSources(s.Model, hits)
	n := p.MaxSources
	if n <= 0 || n > 3 {
		n = 3
	}
	if len(hits) > n {
		hits = hits[:n]
	}
	if len(hits) == 0 {
		return Reply{Text: "По этому вопросу страницы не нашлись."}, nil
	}
	var pages []string
	var sources []Citation
	for i, hit := range hits {
		doc, ferr := p.Fetch.Fetch(ctx, hit)
		if ferr != nil || strings.TrimSpace(doc.Text) == "" {
			continue
		}
		title := doc.Title
		if title == "" {
			title = hit.Title
		}
		sources = append(sources, Citation{Title: title, URL: doc.URL})
		pages = append(pages, fmt.Sprintf("[%d] %s\n%s\n%s", i+1, title, doc.URL, pageWindow(question, doc.Text, 900)))
	}
	if len(pages) == 0 {
		return Reply{}, errors.New("страницы поиска не открылись")
	}
	brief := askBrief(s, question)
	text, err := answerer.Answer(ctx, brief, strings.Join(pages, "\n\n"))
	if err != nil {
		return Reply{}, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		text = "Страницы на вопрос не ответили."
	}
	return Reply{Text: text, Sources: sources}, nil
}

func askBrief(s VehicleSignature, question string) string {
	car := normalizeSpace(fmt.Sprintf("%s %s %d года", s.Make, s.Model, s.Year))
	if s.PowerHP > 0 {
		car += fmt.Sprintf(", %d л.с.", s.PowerHP)
	}
	return car + ". Если на странице несколько комплектаций, бери блок этого года и этой мощности. Вопрос: " + question
}

func pageWindow(question, text string, n int) string {
	text = normalizeSpace(text)
	if n <= 0 || utf8.RuneCountInString(text) <= n {
		return text
	}
	lower := strings.ToLower(text)
	at := -1
	for _, word := range strings.Fields(strings.ToLower(question)) {
		word = strings.Trim(word, ".,?!;:\"'«»()[]")
		if utf8.RuneCountInString(word) < 4 || askSkip[word] {
			continue
		}
		stem := word
		if utf8.RuneCountInString(stem) > 4 {
			stem = string([]rune(stem)[:4])
		}
		if i := strings.Index(lower, stem); i >= 0 && (at < 0 || i < at) {
			at = i
		}
	}
	if at < 0 {
		return clipRunes(text, n)
	}
	runes := []rune(text)
	hit := utf8.RuneCountInString(text[:at])
	start := hit - n/3
	if start < 0 {
		start = 0
	}
	if start > len(runes) {
		start = 0
	}
	return clipRunes(string(runes[start:]), n)
}

var askSkip = map[string]bool{
	"какой": true, "какая": true, "какие": true, "какое": true, "каких": true,
	"интервал": true, "замена": true, "замены": true, "менять": true, "поменять": true,
	"нужно": true, "когда": true, "сколько": true, "пробег": true, "срок": true,
	"машина": true, "машины": true, "этой": true, "этот": true, "этого": true,
}

func askQuery(s VehicleSignature, question string) string {
	q := normalizeSpace(strings.TrimSpace(s.Make + " " + s.Model + " " + fmt.Sprint(s.Year) + " " + question))
	return clipRunes(q, 240)
}

func clipRunes(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for k := range s {
		if i == n {
			return s[:k]
		}
		i++
	}
	return s
}

// Ask ставит вопрос в очередь поиска запчастей.
// Разбор документов в это время не идёт: модели включаются по очереди.
func (rt *Runtime) Ask(ctx context.Context, v domain.Vehicle, question string) (Reply, error) {
	if rt == nil || rt.heavy == nil {
		return Reply{}, ErrAskOff
	}
	pipe, ok := rt.heavy.pipe.(*KnowledgePipeline)
	if !ok || pipe == nil {
		return Reply{}, ErrAskOff
	}
	run := func(ctx context.Context) (Reply, error) {
		return pipe.Ask(ctx, signatureFrom(v), question)
	}
	rt.heavy.mu.Lock()
	heavy := rt.heavy.lane
	rt.heavy.mu.Unlock()
	if heavy == nil {
		return run(ctx)
	}
	wait, stop := detachDeadline(ctx)
	defer stop()
	var reply Reply
	var err error
	werr := heavy.Do(wait, lane.KindParts, func(work context.Context) error {
		var cancel context.CancelFunc
		work, cancel = context.WithTimeout(work, 60*time.Minute)
		defer cancel()
		reply, err = run(work)
		return nil
	})
	if werr != nil {
		return Reply{}, werr
	}
	return reply, err
}
