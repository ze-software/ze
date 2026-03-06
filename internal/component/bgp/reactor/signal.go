// Design: docs/architecture/core-design.md — BGP reactor event loop

package reactor

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"codeberg.org/thomas-mangin/ze/internal/core/syncutil"
)

// SignalCallback is called when a signal is received.
type SignalCallback func()

// SignalHandler manages OS signal handling for the reactor.
//
// It handles:
//   - SIGTERM/SIGINT: Graceful shutdown
//   - SIGHUP: Configuration reload
//   - SIGUSR1: Status dump
type SignalHandler struct {
	onShutdown SignalCallback
	onReload   SignalCallback
	onStatus   SignalCallback

	sigChan chan os.Signal
	running bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex
}

// NewSignalHandler creates a new signal handler.
func NewSignalHandler() *SignalHandler {
	return &SignalHandler{
		sigChan: make(chan os.Signal, 1),
	}
}

// OnShutdown sets the callback for SIGTERM/SIGINT.
func (h *SignalHandler) OnShutdown(cb SignalCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onShutdown = cb
}

// OnReload sets the callback for SIGHUP.
func (h *SignalHandler) OnReload(cb SignalCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onReload = cb
}

// OnStatus sets the callback for SIGUSR1.
func (h *SignalHandler) OnStatus(cb SignalCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onStatus = cb
}

// Running returns true if the handler is active.
func (h *SignalHandler) Running() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

// Start begins signal handling with a background context.
func (h *SignalHandler) Start() {
	h.StartWithContext(context.Background())
}

// StartWithContext begins signal handling with the given context.
func (h *SignalHandler) StartWithContext(ctx context.Context) {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}

	h.ctx, h.cancel = context.WithCancel(ctx)
	h.running = true

	// Register for signals
	signal.Notify(h.sigChan,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGHUP,
		syscall.SIGUSR1,
	)

	h.mu.Unlock()

	h.wg.Add(1)
	go h.run()
}

// Stop signals the handler to stop.
func (h *SignalHandler) Stop() {
	h.mu.Lock()
	cancel := h.cancel
	h.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Wait waits for the handler to stop.
func (h *SignalHandler) Wait(ctx context.Context) error {
	return syncutil.WaitGroupWait(ctx, &h.wg)
}

// run is the main signal handling loop.
func (h *SignalHandler) run() {
	defer h.wg.Done()
	defer h.cleanup()

	for {
		select {
		case <-h.ctx.Done():
			return

		case sig := <-h.sigChan:
			h.safeHandleSignal(sig)
		}
	}
}

// safeHandleSignal wraps handleSignal with panic recovery so that a panic
// in a signal callback doesn't kill the signal handling loop.
func (h *SignalHandler) safeHandleSignal(sig os.Signal) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			reactorLogger().Error("signal handler panic recovered",
				"signal", sig,
				"panic", r,
				"stack", string(buf[:n]),
			)
		}
	}()
	h.handleSignal(sig)
}

// handleSignal dispatches a signal to the appropriate callback.
func (h *SignalHandler) handleSignal(sig os.Signal) {
	h.mu.RLock()
	var cb SignalCallback

	switch sig {
	case syscall.SIGTERM, syscall.SIGINT:
		cb = h.onShutdown
	case syscall.SIGHUP:
		cb = h.onReload
	case syscall.SIGUSR1:
		cb = h.onStatus
	}
	h.mu.RUnlock()

	if cb != nil {
		cb()
	}
}

// cleanup runs when handler stops.
func (h *SignalHandler) cleanup() {
	signal.Stop(h.sigChan)

	h.mu.Lock()
	defer h.mu.Unlock()

	h.running = false
	h.cancel = nil
}
