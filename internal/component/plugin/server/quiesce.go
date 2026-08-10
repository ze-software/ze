// Design: docs/architecture/api/commands.md — quiesce barrier (test synchronization)
//
// `request quiesce` (ze-system:quiesce) blocks until every registered subsystem
// has drained its pending asynchronous work, then replies. It is the general
// form of ze-bgp:peer-flush: a test does `send(change); request quiesce; assert`
// and never sleeps. Subsystems register a Quiescer at runtime (they need a live
// reference such as the reactor), and the handler discovers them through the
// registry — no per-subsystem switch here.

package server

import (
	"context"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// QuiesceFunc drains a subsystem's pending asynchronous work, returning when the
// subsystem has settled or ctx is canceled (deadline reached).
//
// A QuiesceFunc MUST honor ctx cancellation. quiesceAll bounds each drain with a
// per-quiescer deadline by canceling ctx, but it cannot force-return a drain that
// ignores ctx — such a quiescer would hang the barrier (and the caller) despite
// the timeout. The only current registrant, the reactor's FlushForwardPool,
// selects on ctx.Done() (forward_pool_barrier.go).
type QuiesceFunc func(ctx context.Context) error

// Quiescer is a named subsystem drain invoked by `request quiesce`.
type Quiescer struct {
	Name    string
	Quiesce QuiesceFunc
}

// QuiescerRegistry holds subsystem drains invoked by `request quiesce`.
// Registration is at runtime (a subsystem needs a live reference such as the
// reactor), so this is a lock-guarded registry rather than an init() table.
type QuiescerRegistry struct {
	mu        sync.RWMutex
	quiescers []Quiescer
}

// Register adds a named subsystem drain. Safe for concurrent use.
func (r *QuiescerRegistry) Register(name string, fn QuiesceFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.quiescers = append(r.quiescers, Quiescer{Name: name, Quiesce: fn})
}

// All returns a snapshot of the registered quiescers.
func (r *QuiescerRegistry) All() []Quiescer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Quiescer, len(r.quiescers))
	copy(out, r.quiescers)
	return out
}

// quiesceTimeout bounds each subsystem drain so a wedged subsystem can never
// hang `request quiesce` (and thus the caller) indefinitely.
const quiesceTimeout = 10 * time.Second

// quiesceAll drains every quiescer concurrently, each bounded by perTimeout, and
// returns StatusDone only when all settle. If any drain fails or times out it
// returns StatusError naming the offending subsystem(s); the Data map always
// lists which subsystems were asked to quiesce.
func quiesceAll(ctx context.Context, quiescers []Quiescer, perTimeout time.Duration) *plugin.Response {
	names := make([]string, len(quiescers))
	errs := make([]error, len(quiescers))
	var wg sync.WaitGroup
	for i := range quiescers {
		names[i] = quiescers[i].Name
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			qctx, cancel := context.WithTimeout(ctx, perTimeout)
			defer cancel()
			errs[i] = quiescers[i].Quiesce(qctx)
		}(i)
	}
	wg.Wait()

	var failed []string
	for i, err := range errs {
		if err != nil {
			failed = append(failed, names[i])
		}
	}
	if len(failed) > 0 {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("quiesce failed for: ").Str(textbuf.Join(failed, ", ")).String(),
			Data:   plugin.Map{"quiesced": names, "failed": failed},
		}
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"quiesced": names},
	}
}

// registerQuiescer registers a named subsystem drain, invoked by `request
// quiesce`. Called at wiring time (e.g. when the reactor is attached to the
// server), not from init(), because the drain closes over a live reference.
func (s *Server) registerQuiescer(name string, fn QuiesceFunc) {
	s.quiescers.Register(name, fn)
}

// Quiescers returns a snapshot of the registered subsystem drains.
func (s *Server) Quiescers() []Quiescer {
	return s.quiescers.All()
}

// registerReactorQuiescer registers the reactor's forward pool as the
// "bgp-forward-pool" Quiescer, so `request quiesce` drains queued routes to peer
// sockets — the general form of ze-bgp:peer-flush. A nil reactor (bare/test or
// web-only server) registers nothing. FlushForwardPool's signature already
// matches QuiesceFunc, so it is registered as a method value.
func registerReactorQuiescer(s *Server, reactor plugin.ReactorLifecycle) {
	if reactor == nil {
		return
	}
	// Two independent drain paths: the forward pool (post-establishment routes)
	// and the per-peer initial-sync opQueue (routes sent during establishment go
	// direct to the session, bypassing the forward pool). `request quiesce` runs
	// both concurrently so a "routes on the wire" barrier covers both.
	s.registerQuiescer("bgp-forward-pool", reactor.FlushForwardPool)
	s.registerQuiescer("bgp-peer-sync", reactor.DrainPeerSync)
}

// handleQuiesce implements `request quiesce` (ze-system:quiesce): drain every
// registered subsystem and reply when all have settled. Tests use it as a
// barrier in place of a fixed sleep.
func handleQuiesce(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	var quiescers []Quiescer
	if ctx.Server != nil {
		quiescers = ctx.Server.Quiescers()
	}
	return quiesceAll(ctx.Context(), quiescers, quiesceTimeout), nil
}

func init() {
	RegisterRPCs(RPCRegistration{WireMethod: "ze-system:quiesce", Handler: handleQuiesce})
}
