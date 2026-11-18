// Design: docs/guide/route-reflection.md -- joining admitted peer-up replay lifecycles.
// Related: replay_settlement.go -- the applied-event handler and stop owner.
package rs

import (
	"context"
	"sync"
)

// replayLifetimes holds only active peer-up lifecycles. The number of entries is
// bounded by admitted peer-UP lifecycles still executing; completion removes its
// entry. begin MUST precede launching the lifecycle goroutine. Its owner MUST
// call finish after the last engine call exactly once, including
// on cancellation. stop MUST precede SDK connection closure. Safe for concurrent
// use; the zero value admits work.
type replayLifetimes struct {
	mu      sync.Mutex
	active  map[chan struct{}]context.CancelFunc
	stopped bool
}

func (r *replayLifetimes) begin(parent context.Context, done chan struct{}) (context.Context, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return nil, false
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	if r.active == nil {
		r.active = make(map[chan struct{}]context.CancelFunc)
	}
	r.active[done] = cancel
	return ctx, true
}

// finish removes one completed lifecycle and closes its actual completion gate.
// It MUST run once, after the last engine call. Direct synchronous replay callers
// own no admission, but still close their completion through the same function.
func (r *replayLifetimes) finish(done chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cancel := r.active[done]; cancel != nil {
		cancel()
		delete(r.active, done)
	}
	close(done)
}

// snapshot captures the actual active lifecycles, including any superseded by
// a reconnect. It does not look them up through a peer's current replay gate.
func (r *replayLifetimes) snapshot() []chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := make([]chan struct{}, 0, len(r.active))
	for done := range r.active {
		pending = append(pending, done)
	}
	return pending
}

func joinReplays(ctx context.Context, pending []chan struct{}) error {
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return ctx.Err()
}

// stop seals admission before cancellation, then joins the captured lifecycles.
// It does not launch a waiter goroutine, retain history or create worker lanes.
func (r *replayLifetimes) stop() {
	r.mu.Lock()
	r.stopped = true
	pending := make([]chan struct{}, 0, len(r.active))
	for done, cancel := range r.active {
		cancel()
		pending = append(pending, done)
	}
	r.mu.Unlock()
	for _, done := range pending {
		<-done
	}
}
