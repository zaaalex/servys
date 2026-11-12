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

func TestInspectReadsTheSavedCheckWithoutSearching(t *testing.T) {
	quiet := &scriptSource{}
	memo := newRemembering(quiet, time.Second, 7*24*time.Hour, "")
	at := time.Date(2026, 11, 7, 13, 12, 20, 0, time.UTC)
	v := domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020, EngineCC: 1353, PowerHP: 130, VIN: "LJD3AA293L0051345"}
	memo.mu.Lock()
	memo.items[vehicleKey(v)] = rememberEntry{At: at, Rules: []domain.Rule{{Code: "engine_oil"}}}
	memo.mu.Unlock()
	every, got, ok := (&Runtime{heavy: memo}).Inspect(v)
	if !ok || !got.Equal(at) || every != 7*24*time.Hour {
		t.Fatalf("every %s at %s ok %v", every, got, ok)
	}
	_, _, ok = (&Runtime{heavy: memo}).Inspect(domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020})
	if ok {
		t.Fatal("другая машина не должна получить чужую дату")
	}
	if doc, full := quiet.counts(); doc != 0 || full != 0 {
		t.Fatalf("inspect called the search: doc %d full %d", doc, full)
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

type seekBook struct{ scriptSource }

func (seekBook) Seeks() bool { return true }

func TestRememberingLetsTheModelSearchOnAColdRead(t *testing.T) {
	book := &seekBook{scriptSource: scriptSource{
		doc:  []domain.Rule{{Code: "engine_oil", Operation: "replace", IntervalKm: 15000}},
		full: []domain.Rule{{Code: "spark_plugs", Operation: "replace", IntervalKm: 45000}},
	}}
	memo := newRemembering(book, time.Second, time.Hour, "")
	v := domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130}
	got, err := memo.Rules(context.Background(), v)
	if err != nil || len(got) != 1 || got[0].Code != "spark_plugs" || got[0].IntervalKm != 45000 {
		t.Fatalf("%v %+v", err, got)
	}
	time.Sleep(40 * time.Millisecond)
	docN, fullN := book.counts()
	if docN != 0 || fullN != 1 {
		t.Fatalf("doc %d full %d", docN, fullN)
	}
}

type deadlineBook struct {
	scriptSource
	has      bool
	deadline time.Time
}

func (deadlineBook) Seeks() bool { return true }

func (d *deadlineBook) Rules(ctx context.Context, s VehicleSignature) ([]domain.Rule, error) {
	d.deadline, d.has = ctx.Deadline()
	return d.scriptSource.Rules(ctx, s)
}

func TestRememberingKeepsTheRequestDeadline(t *testing.T) {
	heavy := lane.New(0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go heavy.Start(ctx)

	book := &deadlineBook{scriptSource: scriptSource{
		full: []domain.Rule{{Code: "engine_oil", Operation: "replace", IntervalKm: 15000}},
	}}
	memo := newRemembering(book, time.Second, time.Hour, "")
	memo.SetLane(heavy)
	req, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	got, err := memo.Rules(req, domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020})
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %+v", err, got)
	}
	if !book.has {
		t.Fatal("work inside the lane had no deadline")
	}
	remain := time.Until(book.deadline)
	if remain <= 0 || remain > 5*time.Second {
		t.Fatalf("deadline left %s", remain)
	}
}

func TestFreshResultSkipsTheSearchQueue(t *testing.T) {
	book := &scriptSource{full: []domain.Rule{{Code: "engine_oil", Operation: "replace", IntervalKm: 15000}}}
	memo := newRemembering(book, time.Second, 7*24*time.Hour, "")
	v := domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020, PowerHP: 130}
	memo.store(vehicleKey(v), book.full)
	heavy := lane.New(time.Hour, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go heavy.Start(ctx)
	held := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = heavy.Do(context.Background(), lane.KindSearch, func(context.Context) error {
			close(held)
			<-release
			return nil
		})
	}()
	select {
	case <-held:
	case <-time.After(time.Second):
		t.Fatal("lane did not take the blocking search")
	}
	memo.SetLane(heavy)
	done := make(chan struct{})
	var got []domain.Rule
	var err error
	go func() {
		got, err = memo.Rules(context.Background(), v)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("saved result waited in the search queue")
	}
	close(release)
	if err != nil || len(got) != 1 || got[0].IntervalKm != 15000 {
		t.Fatalf("%v %+v", err, got)
	}
	if _, fullN := book.counts(); fullN != 0 {
		t.Fatalf("fresh result started a search, full %d", fullN)
	}
}

func TestWeekOldResultStaysAndRefreshesInTheBackground(t *testing.T) {
	gate := make(chan struct{})
	book := &scriptSource{full: []domain.Rule{{Code: "spark_plugs", Operation: "replace", IntervalKm: 45000}}, gate: gate}
	memo := newRemembering(book, time.Second, 7*24*time.Hour, "")
	v := domain.Vehicle{Make: "KIA", Model: "K3", Year: 2020}
	old := []domain.Rule{{Code: "engine_oil", Operation: "replace", IntervalKm: 15000}}
	memo.mu.Lock()
	memo.items[vehicleKey(v)] = rememberEntry{At: time.Now().Add(-8 * 24 * time.Hour), Rules: old}
	memo.mu.Unlock()
	got, err := memo.Rules(context.Background(), v)
	if err != nil || len(got) != 1 || got[0].Code != "engine_oil" {
		t.Fatalf("screen should keep the saved list: %v %+v", err, got)
	}
	deadline := time.Now().Add(time.Second)
	for {
		_, fullN := book.counts()
		if fullN == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("week-old result did not refresh in the background")
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(gate)
}
