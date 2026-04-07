// Design: docs/architecture/api/process-protocol.md -- engine-side stream pub/sub
// Related: dispatch.go -- deliverEvent fans out to engine handlers as well as plugins
// Related: subscribe.go -- parallel mechanism for plugin-process subscriptions
// Related: engine_event_gateway.go -- ConfigEventGateway adapter for the transaction package

package server

import (
	"sync"
)

// EngineEventHandler is invoked when a stream event matches an engine subscription.
// The event string is the same payload that plugin subscribers receive (typically
// JSON). Handlers are called synchronously from deliverEvent. Handlers MUST NOT
// block on external I/O; push to a buffered channel and return if work is needed.
//
// A handler that panics is recovered by the dispatch loop, logged, and the
// remaining handlers for the same event still fire. The panic does NOT
// propagate to the emitter (whoever called EmitEngineEvent or any other
// path that ends in deliverEvent).
type EngineEventHandler func(event string)

// engineEventSubscribers tracks engine-side subscriptions to stream events.
// Parallel to SubscriptionManager (which tracks plugin-process subscriptions).
// Engine handlers fire from deliverEvent in addition to plugin process delivery.
type engineEventSubscribers struct {
	mu       sync.RWMutex
	nextID   uint64
	handlers map[engineSubKey]map[uint64]EngineEventHandler
}

// engineSubKey identifies an engine subscription by namespace and event type.
type engineSubKey struct {
	Namespace string
	EventType string
}

func newEngineEventSubscribers() *engineEventSubscribers {
	return &engineEventSubscribers{
		handlers: make(map[engineSubKey]map[uint64]EngineEventHandler),
	}
}

// register adds a handler and returns its unique id (always non-zero on success).
// Returns 0 without storing anything if handler is nil; callers must treat 0
// as a registration failure. The caller MUST hold no other locks (acquires mu).
func (e *engineEventSubscribers) register(namespace, eventType string, handler EngineEventHandler) uint64 {
	if handler == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nextID++
	id := e.nextID
	key := engineSubKey{Namespace: namespace, EventType: eventType}
	if _, ok := e.handlers[key]; !ok {
		e.handlers[key] = make(map[uint64]EngineEventHandler)
	}
	e.handlers[key][id] = handler
	return id
}

// unregister removes a handler by id. No-op if id is unknown or zero.
func (e *engineEventSubscribers) unregister(namespace, eventType string, id uint64) {
	if id == 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	key := engineSubKey{Namespace: namespace, EventType: eventType}
	if m, ok := e.handlers[key]; ok {
		delete(m, id)
		if len(m) == 0 {
			delete(e.handlers, key)
		}
	}
}

// dispatch invokes all handlers registered for (namespace, eventType).
//
// Handlers are copied to a local slice under read lock and invoked OUTSIDE
// the lock so handler code can call register/unregister without deadlocking.
//
// Mid-dispatch contract: the local slice is taken at the moment dispatch
// acquires the read lock. If a handler unregisters another handler during
// the same dispatch, the unregistered handler IS still called once more
// (because it is in the local slice). Effective semantics: register and
// unregister take effect on the NEXT dispatch, not the current one.
//
// Each handler invocation is wrapped in a deferred recover so a single
// panicking handler does not abort the loop or propagate the panic out
// to the emitter. Panics are logged via the package logger.
func (e *engineEventSubscribers) dispatch(namespace, eventType, event string) {
	e.mu.RLock()
	key := engineSubKey{Namespace: namespace, EventType: eventType}
	m, ok := e.handlers[key]
	if !ok || len(m) == 0 {
		e.mu.RUnlock()
		return
	}
	handlers := make([]EngineEventHandler, 0, len(m))
	for _, h := range m {
		handlers = append(handlers, h)
	}
	e.mu.RUnlock()

	for _, h := range handlers {
		invokeEngineHandler(namespace, eventType, event, h)
	}
}

// invokeEngineHandler runs a single handler with panic recovery.
// Extracted so the dispatch loop is straight-line and the deferred
// recover scope is exactly one handler invocation.
func invokeEngineHandler(namespace, eventType, event string, h EngineEventHandler) {
	defer func() {
		if r := recover(); r != nil {
			logger().Error("engine event handler panicked",
				"namespace", namespace, "event-type", eventType, "panic", r)
		}
	}()
	h(event)
}

// EmitEngineEvent publishes an event from the engine to the stream system.
// Both engine subscribers and plugin process subscribers receive it.
// Returns the number of plugin processes that received the event (engine
// handler count is intentionally not reported because engine subscribers
// are in-process and always receive synchronously when matching).
//
// The event must use a registered (namespace, eventType) per plugin.IsValidEvent;
// unknown pairs return an error and deliver to nobody (neither engine handlers
// nor plugin subscribers).
func (s *Server) EmitEngineEvent(namespace, eventType, event string) (int, error) {
	// Reuse the existing deliverEvent path. Passing nil emitter means no plugin
	// process will be excluded from delivery (the existing exclusion check
	// "if p == emitter" never matches a real process when emitter is nil).
	return s.deliverEvent(nil, namespace, eventType, "", "", event)
}

// Emit satisfies the pkg/ze.EventBus interface. It is a thin alias for
// EmitEngineEvent so engine components can depend on the public ze.EventBus
// type without importing this package directly.
func (s *Server) Emit(namespace, eventType, payload string) (int, error) {
	return s.EmitEngineEvent(namespace, eventType, payload)
}

// Subscribe satisfies the pkg/ze.EventBus interface. It is a thin alias
// for SubscribeEngineEvent that adapts the handler signature from
// EngineEventHandler (a named type) to a plain func, which is what
// ze.EventBus declares.
func (s *Server) Subscribe(namespace, eventType string, handler func(payload string)) func() {
	if handler == nil {
		return func() {}
	}
	return s.SubscribeEngineEvent(namespace, eventType, EngineEventHandler(handler))
}

// SubscribeEngineEvent registers an engine-side handler for stream events
// matching the given namespace and event type. The returned function
// unregisters the handler when called; safe to call multiple times.
//
// Handlers fire synchronously from deliverEvent. They must not block on
// external I/O. The handler receives the same event string that plugin
// process subscribers would receive.
//
// Engine subscriptions are parallel to plugin process subscriptions managed
// by SubscriptionManager. Both fire on the same deliverEvent call.
//
// Subscriptions are NOT validated against the event registry: subscribing
// to an unknown (namespace, eventType) pair, or to a per-plugin event type
// that is not yet registered, succeeds silently. Such a subscription is
// dead until the matching emit arrives. This avoids races with per-plugin
// event types like "verify-bgp" that are registered dynamically when the
// plugin starts.
//
// Engine subscribers receive ALL events for the given (namespace, eventType)
// regardless of direction or peer address. Plugin process subscribers can
// filter on direction and peer; engine subscribers cannot. This is intended
// for engine-internal coordination (e.g. config transactions) where direction
// has no meaning.
//
// A nil handler is rejected: the call returns a no-op unsubscribe function
// without registering anything. This catches programmer errors loudly via
// "the handler I just registered never fires" rather than via a nil-pointer
// panic at first dispatch.
func (s *Server) SubscribeEngineEvent(namespace, eventType string, handler EngineEventHandler) func() {
	if s.engineSubscribers == nil {
		// Defensive: should never happen because NewServer initializes it.
		// Return a no-op unsubscribe so callers can defer it safely.
		return func() {}
	}
	if handler == nil {
		logger().Error("SubscribeEngineEvent called with nil handler",
			"namespace", namespace, "event-type", eventType)
		return func() {}
	}
	id := s.engineSubscribers.register(namespace, eventType, handler)
	return func() {
		s.engineSubscribers.unregister(namespace, eventType, id)
	}
}

// dispatchEngineEvent is called from deliverEvent's defer to fire any
// engine-side handlers after plugin process delivery completes. The
// (namespace, eventType) pair is already validated by deliverEvent before
// the defer is registered, so this method does no further validation
// beyond the nil-check on the registry.
func (s *Server) dispatchEngineEvent(namespace, eventType, event string) {
	if s.engineSubscribers == nil {
		return
	}
	s.engineSubscribers.dispatch(namespace, eventType, event)
}
