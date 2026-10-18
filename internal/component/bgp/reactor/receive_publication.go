// Design: docs/architecture/plugin/rib-storage-design.md -- received publication before cold recovery
// Related: peer_run.go -- one publication lifetime per delivery worker
// Related: reactor_notify.go -- acceptance precedes reactor-native forwarding
package reactor

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var errReceivePublicationStopped = errors.New("received UPDATE delivery stopped before publication")
var errReceivePublicationFailed = errors.New("received UPDATE delivery failed before publication")

// receivePublication is a scalar frontier, not a message queue or route store.
// One peer's read loop accepts UPDATEs in order; its delivery worker completes
// that same FIFO after publishing each batch into the plugin queues. Message IDs
// are monotonic. A nil done channel denotes the synchronous receive fallback.
// The immutable worker lifetime prevents a replacement from satisfying its
// predecessor's unfinished publication receipt.
type receivePublication struct {
	accepted  atomic.Uint64
	completed atomic.Uint64
	failed    atomic.Bool
	done      <-chan struct{}
	waiting   atomic.Bool
	mu        sync.Mutex
	changed   chan struct{}
}

// receivePublicationCut is captured only by cold recovery, after the destination
// writer sequence. Waiting on it never holds writeMu or any source dispatch lock.
type receivePublicationCut struct {
	publication *receivePublication
	messageID   uint64
}

// acceptedReceive MUST precede the fast forward and delivery enqueue. Production
// installs the worker's frontier before Session.Run; the CAS initializes only a
// peer using synchronous delivery, once rather than once per UPDATE.
func (p *Peer) acceptedReceive(messageID uint64) *receivePublication {
	publication := p.receivePublication.Load()
	if publication == nil {
		publication = new(receivePublication)
		if !p.receivePublication.CompareAndSwap(nil, publication) {
			publication = p.receivePublication.Load()
		}
	}
	publication.accepted.Store(messageID)
	return publication
}

func (p *Peer) receiveCut() receivePublicationCut {
	publication := p.receivePublication.Load()
	if publication == nil {
		return receivePublicationCut{}
	}
	return receivePublicationCut{publication: publication, messageID: publication.accepted.Load()}
}

// receiveCutState is what became of a source since a cold recovery captured
// its cut. The two stale states call for opposite answers, so they are never
// one false: a session that ended is excluded from the key, while a live
// session that accepted more UPDATEs only makes the round's lookup possibly
// stale for the keys those UPDATEs touched, and excluding it would withdraw a
// route the source still holds.
type receiveCutState uint8

const (
	receiveCutUnspecified receiveCutState = iota
	// receiveCutCurrent: same publication, nothing accepted since the cut.
	receiveCutCurrent
	// receiveCutAdvanced: the same live publication accepted UPDATEs since
	// the cut; decide again from a fresh cut.
	receiveCutAdvanced
	// receiveCutEnded: the publication was replaced (the session ended or
	// restarted) or failed, so its received routes are no candidates.
	receiveCutEnded
)

// state reports what became of the source's publication since the cut. Safe
// for concurrent use: it reads atomics only.
func (cut receivePublicationCut) state(peer *Peer) receiveCutState {
	publication := peer.receivePublication.Load()
	if publication != cut.publication {
		return receiveCutEnded
	}
	if publication == nil {
		return receiveCutCurrent
	}
	if publication.failed.Load() {
		return receiveCutEnded
	}
	if publication.accepted.Load() != cut.messageID {
		return receiveCutAdvanced
	}
	return receiveCutCurrent
}

// complete follows successful publication, not plugin application. The caller
// must still drain the selecting process's applied-event barrier before lookup.
func (publication *receivePublication) complete(messageID uint64) {
	publication.completed.Store(messageID)
	publication.wake()
}

// fail is sticky for this worker lifetime: a later successful batch cannot hide
// an earlier batch that never reached the plugin queues. It reports whether
// this call was the first failure, so the delivery worker tears the session
// down once.
func (publication *receivePublication) fail() bool {
	first := !publication.failed.Swap(true)
	publication.wake()
	return first
}

// wake allocates nothing. With no cold waiter, completion is two atomic stores
// or loads; the peer's hot receive loop does not acquire a publication mutex.
func (publication *receivePublication) wake() {
	if !publication.waiting.Load() {
		return
	}
	publication.mu.Lock()
	if publication.changed != nil {
		close(publication.changed)
		publication.changed = nil
	}
	publication.waiting.Store(false)
	publication.mu.Unlock()
}

// wait broadcasts completion to every waiter, rather than consuming one shared
// notification. Channels are allocated only by a cold caller that must block;
// cancellation leaves at most one reusable channel until the next completion.
func (cut receivePublicationCut) wait(ctx context.Context) error {
	publication := cut.publication
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if publication == nil {
			return nil
		}
		if publication.failed.Load() {
			return errReceivePublicationFailed
		}
		if publication.completed.Load() >= cut.messageID {
			return nil
		}
		publication.mu.Lock()
		if publication.changed == nil {
			publication.changed = make(chan struct{})
		}
		publication.waiting.Store(true)
		changed := publication.changed
		publication.mu.Unlock()
		// Completion may have raced the first check before waiter registration.
		// Recheck after publishing the waiter, so no completion wake is lost.
		if publication.failed.Load() || publication.completed.Load() >= cut.messageID {
			continue
		}
		select {
		case <-changed:
		case <-publication.done:
			if publication.failed.Load() {
				return errReceivePublicationFailed
			}
			if publication.completed.Load() < cut.messageID {
				return errReceivePublicationStopped
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
