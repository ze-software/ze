// Design: docs/architecture/api/commands.md -- a command that starts work outliving its request
// Overview: server.go -- plugin server lifecycle, whose Stop runs these hooks

package server

import (
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/core/opererr"
)

// errServerStopped refuses a stop hook offered after Stop already ran: the
// hook would never run, so the work it bounds would outlive the daemon.
var errServerStopped = errors.New("the daemon is stopping, so nothing new can start: start it again once ze runs")

// CheckAdmission refuses new command-owned work as soon as Stop enters. This
// check is not a lifetime lease: resource owners MUST still register OnStop
// before exposure and serialize their own active operations with Close.
// A nil Server has no lifetime and refuses. Safe for concurrent use.
func (s *Server) CheckAdmission() error {
	if s == nil {
		return errServerStopped
	}
	if s.stopEntered.Load() {
		return errServerStopped
	}
	return nil
}

// stopHooks holds the work a command started that must end when the server
// stops: a listener a builtin opened at runtime, which no config transaction
// and no plugin process owns. Each hook is keyed so its owner can withdraw it
// when the work ends first. Safe for concurrent use.
type stopHooks struct {
	mu      sync.Mutex
	next    uint64
	hooks   map[uint64]func() error
	stopped bool
}

// add registers fn and answers the function that withdraws it, or
// errServerStopped once run has started.
func (h *stopHooks) add(fn func() error) (func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.stopped {
		return nil, errServerStopped
	}
	if h.hooks == nil {
		h.hooks = make(map[uint64]func() error)
	}
	id := h.next
	h.next++
	h.hooks[id] = fn
	return func() { h.remove(id) }, nil
}

// remove withdraws one hook. A hook already taken by run is gone, so a
// withdrawal from inside that hook finds nothing and takes no lock run holds.
func (h *stopHooks) remove(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.hooks, id)
}

// run takes every hook and calls each in turn, outside the lock, so a hook
// may withdraw itself. Every hook runs even if another fails, and the answer
// retains every failure. Calls after the first find no hook and return nil.
func (h *stopHooks) run() error {
	h.mu.Lock()
	hooks := h.hooks
	h.hooks = nil
	h.stopped = true
	h.mu.Unlock()

	var failure error
	for _, fn := range hooks {
		failure = errors.Join(failure, fn())
	}
	return failure
}

// OnStop registers fn to run when Stop stands the server down, after the
// server's context is canceled and before Stop returns, so the daemon's
// ordered shutdown waits for fn. fn MUST return only once the work it ends has
// ended, its goroutines included, and MUST return any cleanup failure.
// StopFailure retains that error and any fatal plugin cause for the daemon's
// exit result. The answer withdraws fn, and its owner MUST
// call it when the work ends before the server does. Once Stop has started,
// OnStop refuses, because fn would never run. Safe for concurrent use.
func (s *Server) OnStop(fn func() error) (func(), error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopEntered.Load() {
		return nil, errServerStopped
	}
	return s.stopHooks.add(fn)
}

// recordStopFailure retains both a fatal plugin cause and resource cleanup
// failures when their owners finish concurrently. A successful stop never
// calls it, so an existing plugin diagnosis remains unchanged without errors.
func (s *Server) recordStopFailure(failure *opererr.Error) {
	// Only the elected fatal plugin and the once-only Stop write this value,
	// so at most one competing writer can make this CAS retry.
	for {
		previous := s.stopFailure.Load()
		combined := failure
		if previous != nil {
			combined = opererr.Wrap(errors.Join(previous, failure),
				"the daemon could not stop cleanly",
				"plugin or resource cleanup failed, so daemon resources can remain",
				"ze doctor")
		}
		if s.stopFailure.CompareAndSwap(previous, combined) {
			return
		}
	}
}
