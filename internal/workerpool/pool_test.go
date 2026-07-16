package workerpool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMapOrderPreserved(t *testing.T) {
	items := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	got := Map(context.Background(), items, 4, func(_ context.Context, n int) int {
		return n * n
	})
	if len(got) != len(items) {
		t.Fatalf("Map() returned %d results, want %d", len(got), len(items))
	}
	for i, v := range got {
		if v != i*i {
			t.Errorf("Map()[%d] = %d, want %d", i, v, i*i)
		}
	}
}

func TestMapEmptyInput(t *testing.T) {
	got := Map(context.Background(), []int{}, 4, func(_ context.Context, n int) int {
		t.Error("fn should not be called for empty input")
		return n
	})
	if len(got) != 0 {
		t.Errorf("Map() = %v, want empty", got)
	}
}

func TestMapMaxInflightCap(t *testing.T) {
	const workers = 3
	items := make([]int, 30)

	var inflight, maxInflight int64
	var start sync.WaitGroup
	start.Add(1)

	got := Map(context.Background(), items, workers, func(_ context.Context, _ int) int {
		cur := atomic.AddInt64(&inflight, 1)
		for {
			old := atomic.LoadInt64(&maxInflight)
			if cur <= old || atomic.CompareAndSwapInt64(&maxInflight, old, cur) {
				break
			}
		}
		// Hold briefly so multiple workers overlap.
		time.Sleep(2 * time.Millisecond)
		atomic.AddInt64(&inflight, -1)
		return 0
	})

	if len(got) != len(items) {
		t.Fatalf("Map() returned %d results, want %d", len(got), len(items))
	}
	if maxInflight > workers {
		t.Errorf("max concurrent = %d, want <= %d", maxInflight, workers)
	}
	if maxInflight < 2 {
		t.Errorf("max concurrent = %d, expected genuine parallelism (>= 2)", maxInflight)
	}
}

func TestMapWorkersClampedToOne(t *testing.T) {
	items := []int{1, 2, 3}
	got := Map(context.Background(), items, 0, func(_ context.Context, n int) int {
		return n + 1
	})
	want := []int{2, 3, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Map()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestMapContextCancellationStopsScheduling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}

	var processed atomic.Int64
	got := Map(ctx, items, 2, func(_ context.Context, n int) int {
		// Cancel partway through; subsequent items should not be scheduled.
		if processed.Add(1) == 5 {
			cancel()
		}
		time.Sleep(time.Millisecond)
		return n
	})

	if len(got) != len(items) {
		t.Fatalf("Map() returned %d results, want %d (slots preserved)", len(got), len(items))
	}
	if p := processed.Load(); p >= int64(len(items)) {
		t.Errorf("processed %d items, expected cancellation to stop scheduling before all %d", p, len(items))
	}
}

func TestDefaultInRange(t *testing.T) {
	d := Default()
	if d < 1 || d > 8 {
		t.Errorf("Default() = %d, want between 1 and 8", d)
	}
}
