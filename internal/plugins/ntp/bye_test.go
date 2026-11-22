// Design: docs/architecture/api/process-protocol.md -- the bye a live removal sends
// Related: register.go -- leaveNTPWorker, byeSaveBudget

package ntp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/zefs"
)

// byeStore is a state client that records the final save and the bound it was
// given. With stall set its put waits for its context to end, as a put against
// a stalled store does.
type byeStore struct {
	stall bool

	mu      sync.Mutex
	keys    []string
	bound   time.Duration
	bounded bool
}

func (*byeStore) StateGet(context.Context, string) ([]byte, bool, error) {
	return nil, false, nil
}

func (s *byeStore) StatePut(ctx context.Context, key string, _ []byte) error {
	deadline, bounded := ctx.Deadline()
	s.mu.Lock()
	s.keys = append(s.keys, key)
	s.bounded = bounded
	s.bound = time.Until(deadline)
	s.mu.Unlock()
	if !s.stall {
		return nil
	}
	<-ctx.Done()
	return ctx.Err()
}

// queryBlockedWorker returns a worker whose loop stands inside an ntp.Query:
// it ignores the stop signal and ends only when the returned release is
// called, which closes done as the real loop does on its way out.
func queryBlockedWorker(t *testing.T, store stateClient) (*syncWorker, func()) {
	t.Helper()
	worker := newSyncWorker(ntpConfig{PersistPath: "/perm/ze/timefile"}, nil)
	worker.state = store
	var once sync.Once
	release := func() { once.Do(func() { close(worker.done) }) }
	t.Cleanup(release)
	return worker, release
}

// leave runs leaveNTPWorker and fails the test when it does not return while
// the worker is still inside its query.
func leave(t *testing.T, worker *syncWorker) (refused bool) {
	t.Helper()
	answered := make(chan bool, 1)
	go func() { answered <- leaveNTPWorker(context.Background(), worker) != nil }()
	select {
	case refused = <-answered:
		return refused
	case <-time.After(10 * time.Second):
		t.Fatal("the bye did not answer while the worker was blocked in a query")
		return false
	}
}

// TestByeAnswersWhileTheWorkerIsInAQuery runs the bye's body against a worker
// that does not react to its stop signal.
//
// VALIDATES: the bye saves the last-known time, signals the worker and answers
// without waiting for it.
// PREVENTS: a bye that waits out an in-flight ntp.Query (about 5 seconds) past
// pluginRemovalGrace, which refuses the reload that removed ntp.
func TestByeAnswersWhileTheWorkerIsInAQuery(t *testing.T) {
	store := &byeStore{}
	worker, release := queryBlockedWorker(t, store)

	if refused := leave(t, worker); refused {
		t.Fatal("the bye reported a refusal for an accepted save")
	}

	select {
	case <-worker.done:
		t.Fatal("the worker ended before the bye answered, so the test proved nothing about a blocked worker")
	default:
	}
	if !worker.isStopped() {
		t.Fatal("the bye did not signal the worker to stop")
	}
	store.mu.Lock()
	keys, bounded, bound := append([]string(nil), store.keys...), store.bounded, store.bound
	store.mu.Unlock()
	if len(keys) != 1 || keys[0] != zefs.KeyNTPLastTime.Pattern {
		t.Fatalf("saved keys = %v, want one write of %q", keys, zefs.KeyNTPLastTime.Pattern)
	}
	if !bounded {
		t.Fatal("the bye's save carried no deadline")
	}
	if bound > byeSaveBudget {
		t.Fatalf("the bye's save bound = %v, want at most byeSaveBudget (%v)", bound, byeSaveBudget)
	}

	// The join the plugin runs after Run returns: the signal is already sent,
	// and the wait ends when the query does.
	release()
	worker.stopAndWait()
}

// TestByeAnswersWhileTheStoreStalls runs the bye's body against a store that
// never answers the put.
//
// VALIDATES: a stalled state put ends at byeSaveBudget, is reported as the
// plugin's refusal, and still leaves the worker signaled.
// PREVENTS: a bye held for the whole 5-second state call timeout.
func TestByeAnswersWhileTheStoreStalls(t *testing.T) {
	store := &byeStore{stall: true}
	worker, _ := queryBlockedWorker(t, store)

	if refused := leave(t, worker); !refused {
		t.Fatal("a save that did not land was not reported")
	}
	if !worker.isStopped() {
		t.Fatal("the bye did not signal the worker to stop after a failed save")
	}
	store.mu.Lock()
	bounded, bound := store.bounded, store.bound
	store.mu.Unlock()
	if !bounded {
		t.Fatal("the bye's save carried no deadline")
	}
	if bound > byeSaveBudget {
		t.Fatalf("the bye's save bound = %v, want at most byeSaveBudget (%v)", bound, byeSaveBudget)
	}
}
