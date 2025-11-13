package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNextOccurrenceBeforeTarget(t *testing.T) {
	loc := time.UTC
	d, err := NewDaily(8, 0, loc)
	if err != nil {
		t.Fatalf("NewDaily error: %v", err)
	}

	now := time.Date(2023, 3, 10, 7, 30, 0, 0, loc)
	next := d.nextOccurrence(now)

	expected := time.Date(2023, 3, 10, 8, 0, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected next occurrence %v, got %v", expected, next)
	}
}

func TestNextOccurrenceAfterTarget(t *testing.T) {
	loc := time.UTC
	d, err := NewDaily(8, 0, loc)
	if err != nil {
		t.Fatalf("NewDaily error: %v", err)
	}

	now := time.Date(2023, 3, 10, 9, 0, 0, 0, loc)
	next := d.nextOccurrence(now)

	expected := time.Date(2023, 3, 11, 8, 0, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected next occurrence %v, got %v", expected, next)
	}
}

func TestDailyStartTriggersCallback(t *testing.T) {
	loc := time.UTC
	d, err := NewDaily(8, 0, loc)
	if err != nil {
		t.Fatalf("NewDaily error: %v", err)
	}

	now := time.Date(2023, 3, 10, 7, 59, 59, 0, loc)
	var mu sync.Mutex
	callCount := 0

	d.now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		return now
	}

	waitCalls := 0
	d.wait = func(ctx context.Context, delay time.Duration) error {
		mu.Lock()
		waitCalls++
		mu.Unlock()
		if delay != time.Second {
			t.Fatalf("expected delay 1s, got %v", delay)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	var once sync.Once

	d.Start(ctx, func(runCtx context.Context) {
		once.Do(func() {
			close(done)
			cancel()
		})
	})

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("callback not invoked")
	}

	mu.Lock()
	defer mu.Unlock()
	if callCount == 0 {
		t.Fatal("now() was not invoked")
	}
	if waitCalls == 0 {
		t.Fatal("wait function was not invoked")
	}
}
