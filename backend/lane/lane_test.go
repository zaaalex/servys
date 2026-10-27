package lane

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestOCRRunsBeforeQueuedSearch(t *testing.T) {
	l := New(5*time.Millisecond, 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var order []string
	l.Later(KindSearch, func(context.Context) {
		mu.Lock()
		order = append(order, "search")
		mu.Unlock()
	})
	l.Later(KindOCR, func(context.Context) {
		mu.Lock()
		order = append(order, "ocr")
		mu.Unlock()
	})
	go l.Start(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(order)
		mu.Unlock()
		if n == 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 || order[0] != "ocr" || order[1] != "search" {
		t.Fatalf("order %v", order)
	}
}

func TestSearchAndOCRDoNotOverlap(t *testing.T) {
	l := New(15*time.Millisecond, 15*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go l.Start(ctx)

	hold := make(chan struct{})
	searchIn := make(chan struct{})
	var mu sync.Mutex
	running, max := 0, 0
	bump := func(delta int) {
		mu.Lock()
		running += delta
		if running > max {
			max = running
		}
		mu.Unlock()
	}
	go func() {
		_ = l.Do(ctx, KindSearch, func(context.Context) error {
			close(searchIn)
			bump(1)
			<-hold
			bump(-1)
			return nil
		})
	}()
	<-searchIn

	ocrDone := make(chan struct{})
	go func() {
		_ = l.Do(ctx, KindOCR, func(context.Context) error {
			bump(1)
			bump(-1)
			return nil
		})
		close(ocrDone)
	}()
	select {
	case <-ocrDone:
		t.Fatal("ocr started while search still held the lane")
	case <-time.After(40 * time.Millisecond):
	}
	close(hold)
	select {
	case <-ocrDone:
	case <-time.After(2 * time.Second):
		t.Fatal("ocr did not start after search released the lane")
	}
	mu.Lock()
	defer mu.Unlock()
	if max != 1 {
		t.Fatalf("max concurrent jobs %d", max)
	}
}

func TestGapAfterSearchBeforeNextOCR(t *testing.T) {
	gap := 120 * time.Millisecond
	l := New(0, gap)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go l.Start(ctx)

	var searchEnded time.Time
	err := l.Do(ctx, KindSearch, func(context.Context) error {
		searchEnded = time.Now()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var ocrStarted time.Time
	err = l.Do(ctx, KindOCR, func(context.Context) error {
		ocrStarted = time.Now()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if ocrStarted.Sub(searchEnded) < gap {
		t.Fatalf("ocr started %s after search, gap is %s", ocrStarted.Sub(searchEnded), gap)
	}
}

func TestQueuedOCRDoesNotWaitForSearchGap(t *testing.T) {
	l := New(0, 250*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go l.Start(ctx)

	var firstEnded time.Time
	if err := l.Do(ctx, KindOCR, func(context.Context) error {
		firstEnded = time.Now()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var secondStarted time.Time
	if err := l.Do(ctx, KindOCR, func(context.Context) error {
		secondStarted = time.Now()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if wait := secondStarted.Sub(firstEnded); wait > 80*time.Millisecond {
		t.Fatalf("second ocr waited %s", wait)
	}
}

func TestIdleLaneRunsALaterJob(t *testing.T) {
	l := New(0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go l.Start(ctx)
	time.Sleep(30 * time.Millisecond)

	done := make(chan struct{})
	l.Later(KindOCR, func(context.Context) { close(done) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("job queued into an idle lane did not run")
	}
}

func TestGapsFromEnv(t *testing.T) {
	ocr, search, err := GapsFromEnv(func(string) string { return "" })
	if err != nil || ocr != 3*time.Second || search != 35*time.Second {
		t.Fatalf("defaults ocr=%s search=%s err=%v", ocr, search, err)
	}
	ocr, search, err = GapsFromEnv(func(key string) string {
		if key == "HEAVY_GAP" {
			return "10s"
		}
		if key == "HEAVY_GAP_OCR" {
			return "1s"
		}
		return ""
	})
	if err != nil || ocr != time.Second || search != 10*time.Second {
		t.Fatalf("override ocr=%s search=%s err=%v", ocr, search, err)
	}
}
