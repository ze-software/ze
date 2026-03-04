// Design: (none — predates documentation)

// Package syncutil provides concurrency helpers.
package syncutil

import (
	"context"
	"sync"
)

// WaitGroupWait waits for wg to complete or ctx to be canceled.
// Returns nil if the WaitGroup finished, or ctx.Err() if the context expired first.
func WaitGroupWait(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
