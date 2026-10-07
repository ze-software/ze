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

func (cut receivePublicationCut) current(peer *Peer) bool {
	publication := peer.receivePublication.Load()
	if publication != cut.publication {
		return false
	}
	if publication == nil {
		return true
	}
	return !publication.failed.Load() && publication.accepted.Load() == cut.messageID
}

// complete follows successful publication, not plugin application. The caller
// must still drain the selecting process's applied-event barrier before lookup.
func (publication *receivePublication) complete(messageID uint64) {
	publication.completed.Store(messageID)
	publication.wake()
}

// fail is sticky for this worker lifetime: a later successful batch cannot hide
// an earlier batch that never reached the plugin queues.
func (publication *receivePublication) fail() {
	publication.failed.Store(true)
	publication.wake()
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
