// Design: docs/architecture/plugin/rib-storage-design.md -- causal receive publication receipts
// Related: receive_publication.go -- bounded broadcast wait, not a route inventory
package reactor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// receiveWaitContext reports entry into wait's select, after its publication
// waiter is registered. Tests use this state rendezvous, never a scheduling sleep.
type receiveWaitContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (ctx *receiveWaitContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.entered) })
	return ctx.Context.Done()
}

// TestReceivePublicationBroadcast requires both cold waiters to be blocked
// before the same completion, failure, or exact-worker stop wakes them.
func TestReceivePublicationBroadcast(t *testing.T) {
	for _, mode := range []string{"complete", "failure", "worker-stop", "replacement"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			var workers sync.WaitGroup
			defer func() { cancel(); workers.Wait() }()
			peer := new(Peer)
			done := make(chan struct{})
			publication := &receivePublication{done: done}
			peer.receivePublication.Store(publication)
			peer.acceptedReceive(41)
			cut := peer.receiveCut()
			results := make(chan error, 2)
			for range 2 {
				waiting := &receiveWaitContext{Context: ctx, entered: make(chan struct{})}
				workers.Go(func() {
					results <- cut.wait(waiting)
				})
				select {
				case <-waiting.entered:
				case <-ctx.Done():
					t.Fatal("publication waiter never registered")
				}
			}
			var want error
			switch mode {
			case "complete":
				publication.complete(41)
			case "failure":
				publication.fail()
				publication.complete(41)
				want = errReceivePublicationFailed
			case "worker-stop":
				close(done)
				want = errReceivePublicationStopped
			case "replacement":
				replacement := new(receivePublication)
				peer.receivePublication.Store(replacement)
				peer.acceptedReceive(99)
				replacement.complete(99)
				if got := cut.state(peer); got != receiveCutEnded {
					t.Fatalf("replacement worker left an old source receipt in state %d, want ended", got)
				}
				close(done)
				want = errReceivePublicationStopped
			}
			for range 2 {
				select {
				case err := <-results:
					if !errors.Is(err, want) {
						t.Fatalf("publication wait = %v, want %v", err, want)
					}
				case <-ctx.Done():
					t.Fatal("publication did not broadcast to both waiters")
				}
			}
			if mode == "failure" {
				if got := cut.state(peer); got != receiveCutEnded {
					t.Fatalf("a failed publication left the cold cut in state %d, want ended", got)
				}
			}
			if mode == "complete" {
				peer.acceptedReceive(42)
				if got := cut.state(peer); got != receiveCutAdvanced {
					t.Fatalf("a later accepted receive left the cold cut in state %d, want advanced", got)
				}
			}
		})
	}
}

func TestReceivePublicationCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	waitingCtx, stopWaiting := context.WithCancel(ctx)
	defer stopWaiting()
	peer := new(Peer)
	peer.acceptedReceive(17)
	cut := peer.receiveCut()
	waiting := &receiveWaitContext{Context: waitingCtx, entered: make(chan struct{})}
	result := make(chan error, 1)
	var worker sync.WaitGroup
	worker.Add(1)
	defer func() { stopWaiting(); worker.Wait() }()
	go func() {
		defer worker.Done()
		result <- cut.wait(waiting)
	}()
	select {
	case <-waiting.entered:
	case <-ctx.Done():
		t.Fatal("publication waiter never registered")
	}
	stopWaiting()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled publication wait = %v", err)
	}
}

func TestReceivePublicationEmptyCut(t *testing.T) {
	peer := new(Peer)
	cut := peer.receiveCut()
	if err := cut.wait(t.Context()); err != nil || cut.state(peer) != receiveCutCurrent {
		t.Fatalf("untouched peer has a pending publication: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := cut.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("empty cut hid caller cancellation: %v", err)
	}
	peer.acceptedReceive(7)
	if cut.state(peer) == receiveCutCurrent {
		t.Fatal("first accepted message did not invalidate an empty cut")
	}
}
