// Design: docs/architecture/core-design.md -- shared netlink route subscription

package routewatch

import (
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/rtproto"
)

type Action uint8

const (
	ActionAdd    Action = 1
	ActionRemove Action = 2
)

type RouteEvent struct {
	Prefix   netip.Prefix
	NextHop  netip.Addr
	Protocol int
	Metric   uint32
	Action   Action
}

type Handler func(RouteEvent)

type handlerEntry struct {
	id int
	fn Handler
}

type Watcher struct {
	mu       sync.Mutex
	handlers []handlerEntry
	nextID   int

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
	errCb     func(error)

	// platform holds per-OS subscription state (the pinned network
	// namespace on Linux); see platformState in routewatch_linux.go and
	// routewatch_other.go.
	platform platformState
}

func New() *Watcher {
	return &Watcher{
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
		platform: newPlatformState(),
	}
}

func (w *Watcher) Register(fn Handler) func() {
	w.mu.Lock()
	id := w.nextID
	w.nextID++
	w.handlers = append(w.handlers, handlerEntry{id: id, fn: fn})
	w.mu.Unlock()
	return func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		for i, h := range w.handlers {
			if h.id == id {
				w.handlers = append(w.handlers[:i], w.handlers[i+1:]...)
				return
			}
		}
	}
}

func (w *Watcher) Start(errCb func(error)) {
	w.startOnce.Do(func() {
		w.errCb = errCb
		// Capture platform state (the network namespace on Linux) on the
		// CALLER's goroutine: the subscription goroutine below runs on an
		// arbitrary OS thread whose namespace depends on when the runtime
		// cloned it, so it cannot capture this itself.
		w.captureNamespace()
		go func() {
			defer close(w.doneCh)
			w.subscribe()
		}()
	})
}

func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
}

func (w *Watcher) Wait() {
	<-w.doneCh
}

func (w *Watcher) deliver(ev RouteEvent) {
	if !ev.Prefix.IsValid() {
		return
	}
	if rtproto.IsZe(ev.Protocol) {
		return
	}
	w.mu.Lock()
	snap := make([]Handler, len(w.handlers))
	for i, h := range w.handlers {
		snap[i] = h.fn
	}
	w.mu.Unlock()
	for _, fn := range snap {
		fn(ev)
	}
}

var defaultWatcher = New()

func Global() *Watcher { return defaultWatcher }
