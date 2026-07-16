// Package workerpool provides a small bounded-concurrency helper for running a
// function over a slice of items. It lives under internal/ because it is CLI
// plumbing, not part of gitm's importable API.
package workerpool

import (
	"context"
	"runtime"
	"sync"
)

// Default returns a sensible worker count for gitm's workloads. The work is
// subprocess- and IO-bound (git invocations, SSH, disk), so it caps at 8 to
// bound fork/SSH-agent/disk pressure rather than scaling with every core.
func Default() int {
	n := runtime.NumCPU()
	if n > 8 {
		return 8
	}
	if n < 1 {
		return 1
	}
	return n
}

// Map applies fn to every item concurrently using at most `workers` goroutines
// and returns the results in the same order as items. fn receives the context
// so it can honor cancellation; Map itself does not abort in-flight work, but it
// stops scheduling new items once ctx is done (their result slots keep the zero
// value of R).
//
// workers < 1 is treated as 1. An empty items slice returns an empty slice.
func Map[T, R any](ctx context.Context, items []T, workers int, fn func(context.Context, T) R) []R {
	results := make([]R, len(items))
	if len(items) == 0 {
		return results
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(items) {
		workers = len(items)
	}

	type job struct {
		index int
		item  T
	}

	jobs := make(chan job)
	var wg sync.WaitGroup
	wg.Add(workers)

	for range workers {
		go func() {
			defer wg.Done()
			for j := range jobs {
				results[j.index] = fn(ctx, j.item)
			}
		}()
	}

	// Feed jobs, stopping early if the context is cancelled. Slots for
	// unscheduled items keep the zero value of R.
	for i, item := range items {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return results
		case jobs <- job{index: i, item: item}:
		}
	}
	close(jobs)
	wg.Wait()
	return results
}
