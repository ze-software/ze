// Design: docs/architecture/l2tp/subscriber-session-model.md -- original PPP construction receipt

package l2tp

import "github.com/ze-software/ze/internal/component/l2tp/ppp"

// pppConstruction retains one original row's descriptor-consuming operation.
// Its zero value is invalid; newPPPConstruction creates the completion signal.
// The producer MUST call finish exactly once after its last descriptor/owner
// action. Observers MUST read outcome fields only after done closes.
type pppConstruction struct {
	done    chan struct{}
	cleanup error
	owner   ppp.SessionHandle
}

func newPPPConstruction() *pppConstruction {
	return &pppConstruction{done: make(chan struct{})}
}

// finish MUST be called once by the producer that owns this construction.
// A failed unused Close keeps the actual owner alive beside its retained error;
// no observer gains authority to retry a recyclable descriptor number.
func (construction *pppConstruction) finish(cleanup error, owner ppp.SessionHandle) {
	construction.cleanup = cleanup
	construction.owner = owner
	close(construction.done)
}

// cleanupStatus performs no native I/O and is safe for concurrent observation.
// The channel close publishes the immutable result; an in-flight constructor
// cannot be mistaken for the absence of a PPP owner or attempt.
func (construction *pppConstruction) cleanupStatus() error {
	select {
	case <-construction.done:
		return construction.cleanup
	default:
		return ErrSessionCleanupPending
	}
}
