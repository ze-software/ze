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
}

// SetAPIProcessCount sets the number of API processes to wait for.
// Must be called before WaitForAPIReady.
func (r *Reactor) SetAPIProcessCount(count int) {
	r.processCount = count
	r.readyCount.Store(0)
	r.apiReady = make(chan struct{})
	r.apiReadyOnce = sync.Once{}
	if r.apiTimeout == 0 {
		r.apiTimeout = DefaultAPITimeout
	}
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

// SignalAPIReady is called when "session api ready" is received.
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
