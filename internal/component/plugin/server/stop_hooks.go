// Design: docs/architecture/api/commands.md -- a command that starts work outliving its request
// Overview: server.go -- plugin server lifecycle, whose Stop runs these hooks

package server

import (
	"errors"
	"sync"
)

// errServerStopped refuses a stop hook offered after Stop already ran: the
// hook would never run, so the work it bounds would outlive the daemon.
var errServerStopped = errors.New("the daemon is stopping, so nothing new can start: start it again once ze runs")

// stopHooks holds the work a command started that must end when the server
// stops: a listener a builtin opened at runtime, which no config transaction
// and no plugin process owns. Each hook is keyed so its owner can withdraw it
// when the work ends first. Safe for concurrent use.
type stopHooks struct {
	mu      sync.Mutex
	next    uint64
	hooks   map[uint64]func()
	stopped bool
}

// add registers fn and answers the function that withdraws it, or
// errServerStopped once run has started.
func (h *stopHooks) add(fn func()) (func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.stopped {
		return nil, errServerStopped
	}
	if h.hooks == nil {
		h.hooks = make(map[uint64]func())
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
// may withdraw itself. Calls after the first find no hook.
func (h *stopHooks) run() {
	h.mu.Lock()
	hooks := h.hooks
	h.hooks = nil
	h.stopped = true
	h.mu.Unlock()

	for _, fn := range hooks {
		fn()
	}
}

// OnStop registers fn to run when Stop stands the server down, after the
// server's context is cancelled and before Stop returns, so the daemon's
// ordered shutdown waits for fn. fn MUST return only once the work it ends has
// ended, its goroutines included. The answer withdraws fn, and its owner MUST
// call it when the work ends before the server does. Once Stop has started,
// OnStop refuses, because fn would never run. Safe for concurrent use.
func (s *Server) OnStop(fn func()) (func(), error) {
	return s.stopHooks.add(fn)
}
