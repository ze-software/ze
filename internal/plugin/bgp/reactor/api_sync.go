package reactor

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultAPITimeout is how long to wait for all "api ready" signals at startup.
const DefaultAPITimeout = 5 * time.Second

// APISyncState tracks API process synchronization state.
type APISyncState struct {
	// processCount is the number of API processes that must send ready.
	processCount int

	// readyCount tracks how many "api ready" signals received.
	readyCount atomic.Int32

	// apiReady is closed when all processes are ready (or timeout).
	apiReady chan struct{}

	// apiReadyOnce ensures apiReady is closed only once.
	apiReadyOnce sync.Once

	// apiTimeout is configurable for testing.
	apiTimeout time.Duration

	// startupComplete is closed when plugin startup (all phases) is done.
	startupComplete chan struct{}

	// startupCompleteOnce ensures startupComplete is closed only once.
	startupCompleteOnce sync.Once
}

// SetAPIProcessCount sets the number of API processes to wait for.
// Must be called before WaitForAPIReady.
func (r *Reactor) SetAPIProcessCount(count int) {
	r.processCount = count
	r.readyCount.Store(0)
	r.apiReady = make(chan struct{})
	r.apiReadyOnce = sync.Once{}
	r.startupComplete = make(chan struct{})
	r.startupCompleteOnce = sync.Once{}
	if r.apiTimeout == 0 {
		r.apiTimeout = DefaultAPITimeout
	}
}

// AddAPIProcessCount adds to the number of API processes to wait for.
// Used for two-phase plugin startup: Phase 1 (explicit) + Phase 2 (auto-load).
// Safe to call while WaitForAPIReady is blocking.
func (r *Reactor) AddAPIProcessCount(count int) {
	r.processCount += count
	slog.Debug("added api process count", "added", count, "total", r.processCount)
}

// WaitForAPIReady blocks until all API processes signal readiness or timeout.
// Called after spawning API processes but before starting peer connections.
//
// Thread-safe: can be called multiple times (subsequent calls return immediately).
func (r *Reactor) WaitForAPIReady() {
	// No processes = no wait
	if r.processCount == 0 {
		return
	}

	// Already ready - return immediately
	select {
	case <-r.apiReady:
		return
	default:
	}

	// Wait for all ready signals or timeout
	select {
	case <-r.apiReady:
		return
	case <-time.After(r.apiTimeout):
		slog.Warn("api timeout", "ready", r.readyCount.Load(), "expected", r.processCount)
		r.signalAllReady()
	}
}

// SignalAPIReady is called when "plugin session ready" is received.
// When all processes have signaled, unblocks WaitForAPIReady.
func (r *Reactor) SignalAPIReady() {
	count := r.readyCount.Add(1)
	slog.Debug("api ready signal", "count", count, "expected", r.processCount)
	if int(count) >= r.processCount {
		r.signalAllReady()
	}
}

// signalAllReady closes the apiReady channel safely.
func (r *Reactor) signalAllReady() {
	r.apiReadyOnce.Do(func() {
		close(r.apiReady)
	})
}

// SignalPluginStartupComplete signals that all plugin phases are done.
// Called by Server after Phase 1 + Phase 2 complete.
func (r *Reactor) SignalPluginStartupComplete() {
	r.startupCompleteOnce.Do(func() {
		if r.startupComplete != nil {
			close(r.startupComplete)
		}
	})
}

// WaitForPluginStartupComplete blocks until plugin startup is complete or timeout.
// This waits for both Phase 1 (explicit) and Phase 2 (auto-load) to finish.
// Uses 3x the API timeout since it covers multiple plugin phases.
func (r *Reactor) WaitForPluginStartupComplete() {
	if r.startupComplete == nil {
		return
	}

	// Use longer timeout for startup complete (covers Phase 1 + Phase 2)
	startupTimeout := 3 * r.apiTimeout
	if startupTimeout == 0 {
		startupTimeout = 3 * DefaultAPITimeout
	}

	select {
	case <-r.startupComplete:
		return
	case <-time.After(startupTimeout):
		slog.Warn("plugin startup timeout", "timeout", startupTimeout)
	}
}

// SignalPeerAPIReady signals that a peer-specific API initialization is complete.
// Called when "peer <addr> plugin session ready" is received (e.g., after route replay).
// Routes the signal to the specified peer.
func (r *Reactor) SignalPeerAPIReady(peerAddr string) {
	r.mu.RLock()
	peer, ok := r.peers[peerAddr]
	r.mu.RUnlock()

	slog.Debug("peer api ready signal", "peer", peerAddr, "found", ok)

	if ok && peer != nil {
		peer.SignalAPIReady()
	}
}
