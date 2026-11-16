// Design: docs/labs/pppoe-interop.md -- signal-safe scratch control I/O.
package testvpppppoe

import (
	"context"
	"errors"
	"syscall"
	"testing"
)

// TestControlIORetriesInterruption keeps the successful result after a signal.
func TestControlIORetriesInterruption(t *testing.T) {
	calls := 0
	length, err := controlIO(context.Background(), func() (int, error) {
		calls++
		if calls == 1 {
			return 0, syscall.EINTR
		}
		return 7, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if length != 7 {
		t.Fatalf("successful length = %d, want 7", length)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

// TestControlIOStopsInterruptedCancellation prevents a signal retry escaping the run budget.
func TestControlIOStopsInterruptedCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	_, err := controlIO(ctx, func() (int, error) {
		calls++
		cancel()
		return 0, syscall.EINTR
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want canceled", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

// TestControlIOPreservesOtherOutcomes leaves short writes and fatal errors to the caller.
func TestControlIOPreservesOtherOutcomes(t *testing.T) {
	for _, outcome := range []struct {
		name   string
		length int
		err    error
	}{
		{name: "short write", length: 2},
		{name: "fatal", err: syscall.EBADF},
		{name: "receive timeout", err: syscall.EAGAIN},
	} {
		t.Run(outcome.name, func(t *testing.T) {
			calls := 0
			length, err := controlIO(context.Background(), func() (int, error) {
				calls++
				return outcome.length, outcome.err
			})
			if length != outcome.length {
				t.Fatalf("length = %d, want %d", length, outcome.length)
			}
			if !errors.Is(err, outcome.err) {
				t.Fatalf("error = %v, want %v", err, outcome.err)
			}
			if calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
		})
	}
}
