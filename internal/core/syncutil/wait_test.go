package syncutil

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// VALIDATES: WaitGroupWait returns nil when WaitGroup completes before context.
// PREVENTS: regression if select logic changes.
func TestWaitGroupWait_Completes(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := WaitGroupWait(ctx, &wg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// VALIDATES: WaitGroupWait returns context error when context expires first.
// PREVENTS: blocking forever if WaitGroup never completes.
func TestWaitGroupWait_ContextCanceled(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1) // never Done — will block

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := WaitGroupWait(ctx, &wg)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// VALIDATES: WaitGroupWait returns DeadlineExceeded on timeout.
// PREVENTS: wrong error type on timeout vs cancel.
func TestWaitGroupWait_ContextTimeout(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1) // never Done

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := WaitGroupWait(ctx, &wg)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

// VALIDATES: WaitGroupWait handles zero-count WaitGroup (already done).
// PREVENTS: edge case where WaitGroup.Wait() returns immediately.
func TestWaitGroupWait_AlreadyDone(t *testing.T) {
	var wg sync.WaitGroup
	// no Add — already at zero

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := WaitGroupWait(ctx, &wg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
