// Package lane выполняет тяжёлую работу по одной задаче.
// Снимки и поиск регламента стоят в разных очередях и не идут одновременно.
// Пока очереди пусты, рабочий спит: PaddleOCR и поиск не запущены.
package lane

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Kind string

const (
	KindOCR    Kind = "ocr"
	KindSearch Kind = "search"
	// KindParts — вопрос про запчасть. Эта модель не включается вместе с разбором документов.
	KindParts Kind = "parts"
)

type job struct {
	kind Kind
	ctx  context.Context
	fn   func(context.Context) error
	errc chan error
}

// Lane — один рабочий. Снимок, разбор документов и поиск запчастей не идут вместе.
// OCR берётся раньше моделей. Модели занимают рабочего в порядке прихода.
// Перед сменой модели выдерживается пауза, чтобы предыдущая отпустила память.
// Повтор той же модели паузу не ждёт.
type Lane struct {
	afterOCR    time.Duration
	afterSearch time.Duration

	mu     sync.Mutex
	ocr    []*job
	models []*job
	wake   chan struct{}
	once   sync.Once
}

func New(afterOCR, afterSearch time.Duration) *Lane {
	return &Lane{
		afterOCR:    afterOCR,
		afterSearch: afterSearch,
		wake:        make(chan struct{}, 1),
	}
}

// GapsFromEnv читает паузы. HEAVY_GAP задаёт обе, отдельные переменные её перекрывают.
// Пауза после поиска по умолчанию длиннее удержания модели Ollama.
func GapsFromEnv(getenv func(string) string) (time.Duration, time.Duration, error) {
	ocr := 3 * time.Second
	search := 35 * time.Second
	if raw := getenv("HEAVY_GAP"); raw != "" {
		gap, err := parseGap("HEAVY_GAP", raw)
		if err != nil {
			return 0, 0, err
		}
		ocr, search = gap, gap
	}
	if raw := getenv("HEAVY_GAP_OCR"); raw != "" {
		gap, err := parseGap("HEAVY_GAP_OCR", raw)
		if err != nil {
			return 0, 0, err
		}
		ocr = gap
	}
	if raw := getenv("HEAVY_GAP_SEARCH"); raw != "" {
		gap, err := parseGap("HEAVY_GAP_SEARCH", raw)
		if err != nil {
			return 0, 0, err
		}
		search = gap
	}
	return ocr, search, nil
}

func parseGap(name, raw string) (time.Duration, error) {
	gap, err := time.ParseDuration(raw)
	if err != nil || gap < 0 {
		return 0, fmt.Errorf("invalid %s %q", name, raw)
	}
	return gap, nil
}

// Do ставит задачу и ждёт её окончания либо отмены ctx.
func (l *Lane) Do(ctx context.Context, kind Kind, fn func(context.Context) error) error {
	if l == nil {
		return fn(ctx)
	}
	errc := make(chan error, 1)
	l.enqueue(&job{kind: kind, ctx: ctx, fn: fn, errc: errc})
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Later ставит задачу и сразу возвращается. Так фоновый поиск не блокирует ответ.
func (l *Lane) Later(kind Kind, fn func(context.Context)) {
	if l == nil {
		go fn(context.Background())
		return
	}
	l.enqueue(&job{
		kind: kind,
		ctx:  context.Background(),
		fn: func(ctx context.Context) error {
			fn(ctx)
			return nil
		},
	})
}

func (l *Lane) enqueue(j *job) {
	l.mu.Lock()
	switch j.kind {
	case KindOCR:
		l.ocr = append(l.ocr, j)
	default:
		l.models = append(l.models, j)
	}
	l.mu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *Lane) pop() *job {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.ocr) > 0 {
		return take(&l.ocr)
	}
	if len(l.models) > 0 {
		return take(&l.models)
	}
	return nil
}

func take(q *[]*job) *job {
	j := (*q)[0]
	*q = (*q)[1:]
	return j
}

// Start работает, пока жив ctx. Повторный вызов ничего не делает.
func (l *Lane) Start(ctx context.Context) {
	if l == nil {
		return
	}
	l.once.Do(func() { l.loop(ctx) })
}

func (l *Lane) loop(ctx context.Context) {
	var lastKind Kind
	var lastAt time.Time
	for {
		j, ok := l.next(ctx)
		if !ok {
			return
		}
		if !lastAt.IsZero() && !l.wait(ctx, lastAt, l.gapBefore(lastKind, j.kind)) {
			l.finish(j, ctx.Err())
			return
		}
		err := l.run(ctx, j)
		l.finish(j, err)
		if j.ctx.Err() == nil {
			lastKind = j.kind
			lastAt = time.Now()
		}
	}
}

func (l *Lane) gapAfter(kind Kind) time.Duration {
	if kind == KindOCR {
		return l.afterOCR
	}
	return l.afterSearch
}

// gapBefore — пауза перед следующей задачей. Та же модель остаётся включённой.
// Другая модель или снимок ждут, пока предыдущая отпустит память.
func (l *Lane) gapBefore(prev, next Kind) time.Duration {
	if prev == "" || (prev == next && prev != KindOCR) {
		return 0
	}
	return l.gapAfter(prev)
}

func (l *Lane) next(ctx context.Context) (*job, bool) {
	for {
		if j := l.pop(); j != nil {
			return j, true
		}
		select {
		case <-ctx.Done():
			return nil, false
		case <-l.wake:
		}
	}
}

func (l *Lane) wait(ctx context.Context, finished time.Time, gap time.Duration) bool {
	remain := gap - time.Since(finished)
	if remain <= 0 {
		return true
	}
	timer := time.NewTimer(remain)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (l *Lane) run(laneCtx context.Context, j *job) (err error) {
	if j.ctx.Err() != nil {
		return j.ctx.Err()
	}
	ctx, cancel := context.WithCancel(laneCtx)
	defer cancel()
	stop := make(chan struct{})
	go func() {
		select {
		case <-j.ctx.Done():
			cancel()
		case <-stop:
		}
	}()
	defer close(stop)
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("heavy: %v", rec)
		}
	}()
	log.Printf("heavy: %s", j.kind)
	return j.fn(ctx)
}

func (l *Lane) finish(j *job, err error) {
	if j.errc == nil {
		return
	}
	select {
	case j.errc <- err:
	default:
	}
}
