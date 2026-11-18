// Design: docs/architecture/l2tp/bng-5-pppoe.md -- registered provider lifecycle
// Related: registry.go -- immutable selection of the subscription factory

package dataplane

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/pkg/ze"
)

// SubscribeLifecycleFunc attaches a selected provider's signal source to the
// daemon bus. It MUST return its original unsubscribe even with an error after
// partial attachment. WatchLifecycle MUST consume that owner on refusal.
// The callback MUST only invoke the supplied signals, never native I/O or a
// subsystem lock. It creates no worker; the subscriber owns its sole worker.
type SubscribeLifecycleFunc func(ze.EventBus, LifecycleSignals) (func(), error)

// LifecycleSignals is the original subscription's notification capability.
// Lost fences admission synchronously. Ready requests another preparation
// attempt; it grants no admission, API generation or physical retirement proof.
// Both methods are safe for concurrent use, including a copied callback after
// unsubscribe. Only WatchLifecycle can construct their private implementation.
type LifecycleSignals interface {
	Lost()
	Ready()
	lifecycleSignals()
}

// LifecycleWatch owns one selected subscription and a capacity-one wake.
// The caller MUST subscribe before preparation, drain every observed loss using
// original owners, and actually prepare all accesses before calling Admit.
// It MUST Close before joining its worker; Close fences before unsubscribe and
// does not join that worker or wait for snapshotted EventBus callbacks.
// Safe for concurrent use. No operation starts a goroutine or performs native I/O.
type LifecycleWatch struct {
	state       atomic.Pointer[lifecycleState] // live CAS storage or an unexposed closed candidate
	terminal    atomic.Pointer[lifecycleState] // nil while live; immutable first public terminal receipt
	changed     chan struct{}
	unsubscribe func()
	closeOnce   sync.Once
}

type lifecyclePhase uint8

const (
	lifecycleUnspecified lifecyclePhase = iota
	lifecycleCandidate
	lifecycleUnavailable
	lifecycleAdmitted
	lifecycleClosed
)

// lifecycleState is immutable once published. Pointer identity binds a complete
// observation, so neither an equal counter nor a copied field can mint authority.
type lifecycleState struct {
	phase   lifecyclePhase
	loss    uint64
	failure error
}

// LifecycleObservation is one original watch's immutable worker observation.
// The zero value is invalid; callers MUST check Valid before reading it.
type LifecycleObservation struct {
	watch *LifecycleWatch
	state *lifecycleState
}

// LifecyclePermit is an admitted discovery generation, never a native resource
// receipt. A caller MUST recheck Current after fallible preparation/construction
// and immediately before its publication. The zero value authorizes nothing.
type LifecyclePermit struct {
	watch *LifecycleWatch
	state *lifecycleState
}

type lifecycleSignals struct {
	watch *LifecycleWatch
}

const lifecycleTransitionsMax = 64

var (
	errLifecycleLossExhausted = errors.New("PPPoE provider loss identity exhausted; subscription permanently closed")
	errLifecycleContention    = errors.New("PPPoE provider lifecycle transition limit reached; subscription permanently closed")
)

// WatchLifecycle returns explicit absence for a stable provider. A registered
// capability instead requires a nonnil bus and a real unsubscribe owner.
// A successful caller MUST Close the returned watch before joining its worker.
func (b *sessionBackend) WatchLifecycle(bus ze.EventBus) (*LifecycleWatch, bool, error) {
	if b.subscribe == nil {
		return nil, false, nil // guard: This provider declared no changing lifecycle.
	}
	if nilResource(bus) {
		return nil, true, fmt.Errorf("PPPoE session backend %q requires an event bus for its lifecycle", b.name)
	}
	watch := &LifecycleWatch{changed: make(chan struct{}, 1)}
	watch.state.Store(&lifecycleState{phase: lifecycleCandidate})
	watch.wake()
	unsubscribe, err := b.subscribe(bus, &lifecycleSignals{watch: watch})
	watch.unsubscribe = unsubscribe
	if unsubscribe == nil {
		err = errors.Join(err, errors.New("provider lifecycle subscription returned no unsubscribe owner"))
	}
	if err != nil {
		watch.Close()
		return nil, true, fmt.Errorf("PPPoE session backend %q lifecycle: %w", b.name, err)
	}
	return watch, true, nil
}

// Changed coalesces notifications; the worker MUST inspect Snapshot after a
// wake and again before sleeping. The channel is never closed or used as proof
// of readiness. Close publishes a final wake for a worker waiting on it.
func (w *LifecycleWatch) Changed() <-chan struct{} { return w.changed }

// Snapshot captures one immutable subscription state. LossSequence is sticky
// across Ready, so coalescing never erases a loss the worker has not retired.
// A closed candidate MUST bind its exact first-terminal receipt before exposure.
func (w *LifecycleWatch) Snapshot() LifecycleObservation {
	return LifecycleObservation{watch: w, state: w.snapshotState()}
}

// snapshotState MUST supply every public state reader's terminal authority.
// It helps bind a normal Close/loss-overflow candidate before exposing it, so
// a producer paused after its live CAS cannot lose an already observed receipt.
// At most three loads and one nil-to-terminal CAS; no I/O, callbacks or locks.
// Nil is only the invalid zero watch, never a live or admitted observation.
func (w *LifecycleWatch) snapshotState() *lifecycleState {
	if terminal := w.terminal.Load(); terminal != nil {
		return terminal
	}
	state := w.state.Load()
	if state == nil {
		return nil // guard: The zero watch has no constructor-owned state.
	}
	if state.phase == lifecycleClosed {
		w.terminal.CompareAndSwap(nil, state)
	}
	if terminal := w.terminal.Load(); terminal != nil {
		return terminal
	}
	return state
}

// Admit publishes preparation only for the unchanged original observation.
// The caller MUST have retired all earlier losses and completed actual access
// preparation first. A Ready event alone MUST NOT cause this call.
func (w *LifecycleWatch) Admit(observation LifecycleObservation) bool {
	if w.terminal.Load() != nil {
		return false
	}
	if observation.watch != w {
		return false
	}
	if !observation.Valid() {
		return false
	}
	if observation.state.phase != lifecycleCandidate {
		return false
	}
	admitted := &lifecycleState{phase: lifecycleAdmitted, loss: observation.state.loss}
	if !w.state.CompareAndSwap(observation.state, admitted) {
		return false
	}
	// A terminal receipt may win after the first check while this exact live
	// CAS is in flight. Hidden live storage cannot grant public admission.
	return w.terminal.Load() == nil
}

// Admission borrows the exact currently admitted generation. The caller MUST
// recheck Current after any operation that could outlast provider loss.
func (w *LifecycleWatch) Admission() (LifecyclePermit, bool) {
	state := w.snapshotState()
	if state == nil {
		return LifecyclePermit{}, false
	}
	if state.phase != lifecycleAdmitted {
		return LifecyclePermit{}, false
	}
	return LifecyclePermit{watch: w, state: state}, true
}

// Current accepts only this watch's still-admitted original permit. It MUST
// be rechecked immediately before discovery/session publication; it conveys
// no authority over a replacement native session or connection.
func (w *LifecycleWatch) Current(permit LifecyclePermit) bool {
	if permit.watch != w {
		return false
	}
	if permit.state == nil {
		return false
	}
	if permit.state.phase != lifecycleAdmitted {
		return false
	}
	return w.snapshotState() == permit.state
}

// Close MUST precede joining the subscriber's worker. It seals admission first
// and then releases exactly the original subscription. The bus may already have
// copied a callback; its signal methods cannot replace the first terminal
// receipt or resurrect admission, even if its final live CAS was already in flight.
func (w *LifecycleWatch) Close() {
	w.closeOnce.Do(func() {
		w.transition(lifecycleClosed)
		if w.unsubscribe != nil {
			w.unsubscribe()
		}
	})
}

// Valid distinguishes a constructor-owned observation from its invalid zero.
func (o LifecycleObservation) Valid() bool {
	if o.watch == nil {
		return false
	}
	return o.state != nil
}

// LossSequence counts distinct outages; Ready never clears an unretired loss.
func (o LifecycleObservation) LossSequence() uint64 {
	if !o.Valid() {
		return 0 // guard: Invalid observations are never eligible for Admit.
	}
	return o.state.loss
}

// RetryRequested permits a bounded preparation attempt, never publication.
func (o LifecycleObservation) RetryRequested() bool {
	if !o.Valid() {
		return false
	}
	if o.state.phase == lifecycleCandidate {
		return true
	}
	return o.state.phase == lifecycleAdmitted
}

// Closed identifies a terminal subscription; its callbacks can never reopen it.
func (o LifecycleObservation) Closed() bool {
	if !o.Valid() {
		return false
	}
	return o.state.phase == lifecycleClosed
}

// Failure preserves a permanent subscription refusal, including loss overflow.
func (o LifecycleObservation) Failure() error {
	if !o.Valid() {
		return errors.New("PPPoE provider lifecycle observation is invalid")
	}
	return o.state.failure
}

func (s *lifecycleSignals) Lost()           { s.watch.transition(lifecycleUnavailable) }
func (s *lifecycleSignals) Ready()          { s.watch.transition(lifecycleCandidate) }
func (*lifecycleSignals) lifecycleSignals() {}

// transition performs bounded lock-free publication, with no callback or I/O.
// One failed CAS means another transition made progress. Even adversarial
// contention cannot spin forever: after 64 live CAS attempts the watch has an
// immutable terminal receipt. Live storage and that once-bound receipt retain
// no history; observations are held only by their bounded worker/call owners.
func (w *LifecycleWatch) transition(next lifecyclePhase) {
	for range lifecycleTransitionsMax {
		if w.terminal.Load() != nil {
			return
		}
		before := w.state.Load()
		if before == nil {
			return // guard: Only the constructor can create a live watch.
		}
		if before.phase == lifecycleClosed {
			// Help the exact normal Close/overflow candidate before exposing it.
			w.terminal.CompareAndSwap(nil, before)
			return
		}
		if next == lifecycleCandidate {
			if before.phase == lifecycleCandidate {
				return
			}
			if before.phase == lifecycleAdmitted {
				return // Duplicate Ready preserves held discovery permits.
			}
		}
		if before.phase == next {
			return // Duplicate loss remains fenced without another loss report.
		}
		after := &lifecycleState{phase: next, loss: before.loss, failure: before.failure}
		if next == lifecycleUnavailable {
			if before.loss == math.MaxUint64 {
				after.phase = lifecycleClosed
				after.failure = errLifecycleLossExhausted
			} else {
				after.loss++
			}
		}
		if w.state.CompareAndSwap(before, after) {
			if after.phase == lifecycleClosed {
				w.terminal.CompareAndSwap(nil, after)
			}
			w.wake()
			return
		}
	}
	w.refuseContention()
}

// refuseContention is the actual terminal-refusal action after transition's
// bounded live CAS budget is spent. It binds one first-terminal receipt or
// observes an already terminal winner; it never overwrites live/closed storage.
// A normal Close/overflow candidate keeps its exact original loss and cause.
// One load and one nil-to-terminal CAS are bounded even under live contention.
// Direct controls prove this producer, not scheduling 64 failed CAS attempts.
func (w *LifecycleWatch) refuseContention() {
	state := w.state.Load()
	if state == nil {
		return // guard: The zero watch has no constructor-owned state.
	}
	if state.phase != lifecycleClosed {
		state = &lifecycleState{phase: lifecycleClosed, loss: math.MaxUint64, failure: errLifecycleContention}
	}
	w.terminal.CompareAndSwap(nil, state)
	w.wake()
}

func (w *LifecycleWatch) wake() {
	select {
	case w.changed <- struct{}{}:
	default: // Capacity one: Snapshot retains every unprocessed loss.
	}
}
