package recommender

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/lane"
)

type scriptSource struct {
	mu    sync.Mutex
	doc   []domain.Rule
	full  []domain.Rule
	docN  int
	fullN int
	gate  chan struct{}
}

func (s *scriptSource) ReadDocumentation(context.Context, VehicleSignature) ([]domain.Rule, error) {
	s.mu.Lock()
	s.docN++
	s.mu.Unlock()
	return cloneRules(s.doc), nil
}

func (s *scriptSource) Rules(ctx context.Context, _ VehicleSignature) ([]domain.Rule, error) {
	s.mu.Lock()
	s.fullN++
	s.mu.Unlock()
	if s.gate != nil {
		select {
		case <-s.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return cloneRules(s.full), nil
}

func (s *scriptSource) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.docN, s.fullN
}

func TestRememberingShowsBookWhileForumsRun(t *testing.T) {
	gate := make(chan struct{})
	book := &scriptSource{
		doc:  []domain.Rule{{Code: "engine_oil", Operation: "replace", IntervalKm: 15000}},
		full: []domain.Rule{{Code: "engine_oil", Operation: "replace", IntervalKm: 15000}, {Code: "spark_plugs", Operation: "replace", IntervalKm: 45000}},
		gate: gate,
	}
	memo := newRemembering(book, time.Second, time.Hour, "")
	v := domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130, VIN: "LJD3AA293L0051345"}
	got, err := memo.Rules(context.Background(), v)
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %+v", err, got)
	}
	deadline := time.Now().Add(time.Second)
	for {
		_, fullN := book.counts()
		if fullN == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	again, err := memo.Rules(context.Background(), v)
	if err != nil || len(again) != 1 || again[0].IntervalKm != 15000 {
		t.Fatalf("fresh book %v %+v", err, again)
	}
	docN, fullN := book.counts()
	if docN != 1 || fullN != 1 {
		t.Fatalf("doc %d full %d", docN, fullN)
	}
	close(gate)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		next, err := memo.Rules(context.Background(), v)
		if err == nil && len(next) == 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("forum pass did not update the saved read")
}

func TestRememberingSearchWaitsForOCR(t *testing.T) {
	heavy := lane.New(0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go heavy.Start(ctx)

	hold := make(chan struct{})
	in := make(chan struct{})
	go func() {
		_ = heavy.Do(ctx, lane.KindOCR, func(context.Context) error {
			close(in)
			<-hold
			return nil
		})
	}()
	<-in

	book := &scriptSource{doc: []domain.Rule{{Code: "engine_oil", Operation: "replace", IntervalKm: 15000}}}
	memo := newRemembering(book, time.Second, time.Hour, "")
	memo.SetLane(heavy)
	v := domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130, VIN: "LJD3AA293L0051345"}
	done := make(chan struct{})
	go func() {
		_, _ = memo.Rules(context.Background(), v)
		close(done)
	}()
	time.Sleep(40 * time.Millisecond)
	docN, _ := book.counts()
	if docN != 0 {
		t.Fatal("поиск читал регламент, пока снимок держал очередь")
	}
	close(hold)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("поиск не начался после снимка")
	}
	docN, _ = book.counts()
	if docN != 1 {
		t.Fatalf("doc %d", docN)
	}
}

func TestRememberingReloadsSavedBook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	book := &scriptSource{doc: []domain.Rule{{Code: "engine_oil", Operation: "replace", IntervalKm: 15000}}}
	memo := newRemembering(book, time.Second, time.Hour, path)
	v := domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130, VIN: "LJD3AA293L0051345"}
	if _, err := memo.Rules(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	quiet := &scriptSource{}
	again := newRemembering(quiet, time.Second, time.Hour, path)
	got, err := again.Rules(context.Background(), v)
	if err != nil || len(got) != 1 || got[0].Code != "engine_oil" {
		t.Fatalf("%v %+v", err, got)
	}
	docN, fullN := quiet.counts()
	if docN != 0 || fullN != 0 {
		t.Fatalf("reloaded read called the network: doc %d full %d", docN, fullN)
	}
}
