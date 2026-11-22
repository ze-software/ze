// Design: docs/architecture/ddos/ddos-detect-enhancements.md -- baseline persistence at removal
// Related: register.go -- OnBye, byeSaveBudget
// Related: detector.go -- stopContext, savePeriodicBaseline

package detect

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/zefs"
)

// slowBaselineStore is a state client whose first put waits for its context to
// end, as a put against a stalled store does, and whose later puts succeed. It
// records how the first put ended and what bound each later put carried.
type slowBaselineStore struct {
	entered chan struct{}

	mu        sync.Mutex
	puts      int
	firstErr  error
	lastKey   string
	lastBound time.Duration
	bounded   bool
}

func (*slowBaselineStore) StateGet(context.Context, string) ([]byte, bool, error) {
	return nil, false, nil
}

func (s *slowBaselineStore) StatePut(ctx context.Context, key string, _ []byte) error {
	s.mu.Lock()
	s.puts++
	first := s.puts == 1
	s.mu.Unlock()
	if first {
		close(s.entered)
		<-ctx.Done()
		s.mu.Lock()
		s.firstErr = ctx.Err()
		s.mu.Unlock()
		return ctx.Err()
	}
	deadline, bounded := ctx.Deadline()
	s.mu.Lock()
	s.lastKey = key
	s.bounded = bounded
	s.lastBound = time.Until(deadline)
	s.mu.Unlock()
	return nil
}

// TestByeStopEndsAPeriodicSaveBlockedInTheStore stops a detector the way its
// bye does, under byeSaveBudget, while a periodic baseline save is blocked in
// a state put.
//
// VALIDATES: the stop ends the blocked put instead of waiting out its timeout,
// and the final baseline is still saved, under the bye's budget.
// PREVENTS: a bye answered after pluginRemovalGrace, which refuses the reload
// that removed the plugin.
func TestByeStopEndsAPeriodicSaveBlockedInTheStore(t *testing.T) {
	store := &slowBaselineStore{entered: make(chan struct{})}
	d := newDetector(DefaultConfig(), newDTestBus(), func(context.Context, string) (string, json.RawMessage, error) {
		return "", nil, nil
	})
	d.state = store

	// What a tick does on a save interval.
	d.wg.Go(d.savePeriodicBaseline)
	<-store.entered

	byeCtx, cancel := context.WithTimeout(context.Background(), byeSaveBudget)
	defer cancel()
	stopped := make(chan struct{})
	go func() {
		d.stopContext(byeCtx)
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("the stop did not return while a periodic save was blocked in the store")
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	// Canceled is the stop ending the put. DeadlineExceeded would be the put
	// running out its own state call timeout, which is what made the bye late.
	if !errors.Is(store.firstErr, context.Canceled) {
		t.Fatalf("blocked periodic save ended with %v, want context.Canceled from the stop", store.firstErr)
	}
	if store.puts != 2 {
		t.Fatalf("state puts = %d, want 2: the blocked periodic save and the final save", store.puts)
	}
	if store.lastKey != zefs.KeyDDoSDetectBaseline.Pattern {
		t.Fatalf("final save key = %q, want %q", store.lastKey, zefs.KeyDDoSDetectBaseline.Pattern)
	}
	if !store.bounded {
		t.Fatal("the final save carried no deadline")
	}
	if store.lastBound > byeSaveBudget {
		t.Fatalf("final save bound = %v, want at most byeSaveBudget (%v)", store.lastBound, byeSaveBudget)
	}
}
