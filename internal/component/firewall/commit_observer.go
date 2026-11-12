// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- the commit observers
// Related: registry.go -- ApplyAll, the one call site of Backend.Apply, which runs the observers

package firewall

import (
	"errors"
	"slices"
	"sync"
)

// CommitObserver is told that Ze is about to commit a reconcile to the backend.
// ApplyAll calls it just before Backend.Apply, and calls the after function it
// returns once Apply has returned, with Apply's error (nil on success). A nil
// after is allowed and means the observer needs no word of the outcome.
//
// An observer runs inside reconcileMu, the outermost firewall lock, so it MUST
// NOT call back into this package (RegisterTables, ApplyAll, FlushAllTables,
// LastApplied, the backend accessors): ApplyAll would wait on itself and every
// firewall owner would hang. Its after function is under the same rule. It MUST
// return quickly too, because every owner's reconcile waits behind it.
//
// The IKE engine registers one (ike/engine/register_firewall_observer.go): its ESP
// form fast path stops rewriting bare ESP while Ze's own commit is in flight, so
// no ESP it would rewrite after the hold begins passes a rule the commit adds
// before the fast path re-reads the ruleset. ESP it rewrote just before the hold
// still reaches the hooks as UDP to port 4500, and if the commit lands before
// those hooks evaluate it, a rule the commit adds for bare ESP does not see it.
type CommitObserver func() (after func(error))

// errBackendApplyPanicked is the error an after function receives when
// Backend.Apply unwound instead of returning: the commit did not complete, and
// what the kernel holds is not known.
var errBackendApplyPanicked = errors.New("firewall: backend apply panicked")

var commitObservers struct {
	mu        sync.Mutex
	observers []CommitObserver
}

// RegisterCommitObserver adds o to the observers ApplyAll runs around every
// Backend.Apply, in registration order. Call it from the init() of the package
// that owns the observer, in its register*.go file. A nil o is a defect in the
// caller and panics.
//
// Safe for concurrent use.
func RegisterCommitObserver(o CommitObserver) {
	if o == nil {
		panic("BUG: firewall: RegisterCommitObserver called with a nil observer")
	}
	commitObservers.mu.Lock()
	defer commitObservers.mu.Unlock()
	commitObservers.observers = append(commitObservers.observers, o)
}

// applyObserved runs b.Apply(all) between the observers' before and after calls,
// for ApplyAll, which holds reconcileMu. Every observer runs, in registration
// order, just before Apply; each after function they answered runs, in the same
// order, once Apply has returned, with its error. The after calls are deferred,
// so a backend that panics still releases what its observers took (the IKE fast
// path hold), with errBackendApplyPanicked, and the panic goes on unwinding.
//
// The observer list is copied under its own lock and the observers run outside
// it, so one registered concurrently is either called with its after or not at
// all.
func applyObserved(b Backend, all []Table) (err error) {
	commitObservers.mu.Lock()
	observers := slices.Clone(commitObservers.observers)
	commitObservers.mu.Unlock()

	afters := make([]func(error), 0, len(observers))
	returned := false
	defer func() {
		outcome := err
		if !returned {
			outcome = errBackendApplyPanicked
		}
		for _, after := range afters {
			after(outcome)
		}
	}()
	for _, o := range observers {
		if after := o(); after != nil {
			afters = append(afters, after)
		}
	}

	err = b.Apply(all)
	returned = true
	return err
}
