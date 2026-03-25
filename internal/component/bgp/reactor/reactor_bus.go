// Design: docs/architecture/core-design.md — Bus subscription for cross-component events
// Overview: reactor.go — Reactor struct and lifecycle

package reactor

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// BusEventHandler is a function that handles a Bus event.
// It runs inside the Bus's per-consumer delivery goroutine.
// Handlers MUST NOT hold reactor.mu to avoid deadlock with the publish path.
type BusEventHandler func(ze.Event)

// busHandler pairs a topic prefix with its handler function.
type busHandler struct {
	prefix  string
	handler BusEventHandler
}

// OnBusEvent registers a handler for Bus events matching the given topic prefix.
// Must be called before StartWithContext. Returns an error if the reactor is already running.
// The handler runs inside the Bus's delivery goroutine — it must not block or hold reactor.mu.
func (r *Reactor) OnBusEvent(prefix string, handler BusEventHandler) error {
	r.mu.RLock()
	running := r.running
	r.mu.RUnlock()

	if running {
		return fmt.Errorf("cannot register bus handler after start")
	}

	r.busHandlers = append(r.busHandlers, busHandler{prefix: prefix, handler: handler})
	return nil
}

// Deliver implements ze.Consumer. Called by the Bus's per-consumer delivery goroutine.
// Dispatches each event to all handlers whose prefix matches the event topic.
func (r *Reactor) Deliver(events []ze.Event) error {
	for i := range events {
		ev := &events[i]
		for j := range r.busHandlers {
			if strings.HasPrefix(ev.Topic, r.busHandlers[j].prefix) {
				r.busHandlers[j].handler(*ev)
			}
		}
	}
	return nil
}

// subscribeBus subscribes to the Bus for all registered handler prefixes.
// No-op if no handlers are registered or Bus is nil.
// Called from StartWithContext after Bus is set.
func (r *Reactor) subscribeBus() {
	if r.bus == nil || len(r.busHandlers) == 0 {
		return
	}

	// Collect unique prefixes.
	seen := make(map[string]bool, len(r.busHandlers))
	for _, h := range r.busHandlers {
		seen[h.prefix] = true
	}

	// Subscribe once per unique prefix, all pointing to this reactor as Consumer.
	for prefix := range seen {
		sub, err := r.bus.Subscribe(prefix, nil, r)
		if err != nil {
			reactorLogger().Error("bus subscribe failed", "prefix", prefix, "error", err)
			continue
		}
		r.busSubs = append(r.busSubs, sub)
	}
}

// unsubscribeBus removes all Bus subscriptions. Called during cleanup.
func (r *Reactor) unsubscribeBus() {
	if r.bus == nil {
		return
	}
	for _, sub := range r.busSubs {
		r.bus.Unsubscribe(sub)
	}
	r.busSubs = nil
}
