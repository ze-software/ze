// Related: contention_test.go -- the admission, holder and entry helpers these cases reuse.

package job

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// runContextBound is how long a case waits for RunContext to answer before
// it calls the wait unbounded. A wait the root ends answers within one poll.
const runContextBound = 10 * time.Second

// VALIDATES: RunContext under a done root starts no child, and answers a
// failure code rather than 0.
// METHOD: an admission over a free registry runs a command that records it
// ran, under a ctx canceled before the call.
// PREVENTS: a verb whose root a SIGTERM already ended starting a cross-compile
// it will not wait for.
func TestRunContextStartsNoChildUnderADoneRoot(t *testing.T) {
	detach(t)
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")
	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	_, code := admission(t, root).RunContext(canceled, "probe", []string{"sh", "-c", record(ran, "admitted")}, root, nil)
	if code == 0 {
		t.Error("RunContext under a canceled root answered 0")
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("RunContext started the child under a canceled root")
	}
}

// VALIDATES: RunContext stops waiting for a slot when its root ends, so a
// SIGTERM that a verb's signal root captured is not held up by the queue.
// METHOD: a live stand-in holds the only slot; the waiter's root is canceled
// after a few polls, and RunContext must answer, unadmitted, well inside
// runContextBound. A queue on context.Background waits out the bound.
// PREVENTS: a verb rooted on signals waiting up to an hour for a slot after
// the operator stopped it, because NotifyContext took SIGTERM from Go's
// default action and the queue never asked the root.
func TestRunContextEndsTheWaitForASlot(t *testing.T) {
	detach(t)
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")
	holder := sleeper(t)
	heldEntry(t, root, "holder", holder.Process.Pid)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	timer := time.AfterFunc(3*helperPoll, cancel)
	defer timer.Stop()

	answered := make(chan int, 1)
	go func() {
		_, code := admission(t, root).RunContext(ctx, "probe", []string{"sh", "-c", record(ran, "admitted")}, root, nil)
		answered <- code
	}()

	select {
	case code := <-answered:
		if code == 0 {
			t.Error("RunContext answered 0 for a wait its root ended")
		}
	case <-time.After(runContextBound):
		t.Fatal("RunContext kept waiting for a slot after its root ended")
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("RunContext ran the child although the slot was never free")
	}
}
