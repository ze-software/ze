// Design: docs/architecture/api/process-protocol.md -- typed event handles
// Related: events.go -- namespace/eventType name registry
//
// Event[T] and SignalEvent are generic handles that bind a (namespace,
// eventType) pair to a concrete Go payload type. The type registration is
// the single source of truth — callers never type-assert in their own code.
//
// Usage:
//
//   // in foo/events/events.go
//   var BestChange = events.Register[*BestChangeBatch]("bgp-rib", "best-change")
//
//   // producer
//   BestChange.Emit(bus, &BestChangeBatch{...})
//
//   // consumer
//   BestChange.Subscribe(bus, func(b *BestChangeBatch) { ... })
//
// For signal-only events with no payload, use RegisterSignal:
//
//   var ReplayRequest = events.RegisterSignal("bgp-rib", "replay-request")
//   ReplayRequest.Emit(bus)
//   ReplayRequest.Subscribe(bus, func() { ... })

package events

import (
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// loggerFn is the function-value type stored in loggerPtr. Wrapping in a
// named type lets atomic.Pointer hold it safely.
type loggerFn func() *slog.Logger

// loggerPtr is the package-level logger for type-mismatch diagnostics.
// Held in atomic.Pointer so concurrent SetLogger and logger() calls do
// not race. A nil load (no SetLogger ever called) falls back to
// slog.Default() inside logger().
var loggerPtr atomic.Pointer[loggerFn]

// SetLogger overrides the logger used by typed-handle drop diagnostics.
// Pass a func that returns the live engine logger so subsequent log-level
// changes propagate. Safe to call concurrently with Emit/Subscribe; the
// stored pointer is swapped atomically.
func SetLogger(fn func() *slog.Logger) {
	if fn == nil {
		return
	}
	wrapped := loggerFn(fn)
	loggerPtr.Store(&wrapped)
}

func logger() *slog.Logger {
	if fn := loggerPtr.Load(); fn != nil {
		return (*fn)()
	}
	return slog.Default()
}

// typeRegistry maps (namespace, eventType) to the canonical payload type.
// Populated by Register / RegisterSignal at package init. Protected by its
// own mutex so it can be read without blocking on the ValidEvents mu.
var (
	typeRegistryMu sync.RWMutex
	typeRegistry   = map[typeKey]reflect.Type{}
)

type typeKey struct {
	namespace string
	eventType string
}

// signalType is the sentinel reflect.Type for signal (no-payload) events.
// Distinct from any user type so the registry can report "signal vs typed".
type signalType struct{}

// Event is a typed handle for a (namespace, eventType) pair whose payload is
// a value of type T. Emit and Subscribe are compile-time checked against T;
// the bus receives and delivers `any`, but the type assertion lives inside
// the handle's Subscribe wrapper rather than in every consumer.
type Event[T any] struct {
	namespace string
	eventType string
}

// Namespace returns the event's namespace (publisher component).
func (e *Event[T]) Namespace() string { return e.namespace }

// EventType returns the event's type within the namespace.
func (e *Event[T]) EventType() string { return e.eventType }

// Emit publishes payload to bus under the handle's (namespace, eventType).
// Returns the number of plugin-process subscribers delivered (engine
// subscribers always deliver synchronously; see EventBus.Emit).
func (e *Event[T]) Emit(bus ze.EventBus, payload T) (int, error) {
	return bus.Emit(e.namespace, e.eventType, payload)
}

// Subscribe registers a typed handler. The bus dispatches `any`; the wrapper
// here type-asserts to T once per event. A type mismatch (publisher emitted
// a value that does not match the registered T, or the bus received an
// undecoded RPC string for a typed event) logs a warn so silent drops do
// not mask publisher / consumer drift. Returns an unsubscribe function.
func (e *Event[T]) Subscribe(bus ze.EventBus, handler func(T)) func() {
	if handler == nil {
		return func() {}
	}
	return bus.Subscribe(e.namespace, e.eventType, func(p any) {
		v, ok := p.(T)
		if !ok {
			logger().Warn("eventbus: payload type mismatch, dropping",
				"namespace", e.namespace,
				"event-type", e.eventType,
				"want", reflect.TypeFor[T]().String(),
				"got", reflect.TypeOf(p))
			return
		}
		handler(v)
	})
}

// SignalEvent is a typed handle for a (namespace, eventType) pair with no
// payload. Producers call Emit(bus); subscribers register `func()` handlers.
// Under the hood the payload is nil on the bus.
type SignalEvent struct {
	namespace string
	eventType string
}

// Namespace returns the event's namespace.
func (e *SignalEvent) Namespace() string { return e.namespace }

// EventType returns the event's type within the namespace.
func (e *SignalEvent) EventType() string { return e.eventType }

// Emit publishes a signal with a nil payload. Returns the number of
// plugin-process subscribers delivered.
func (e *SignalEvent) Emit(bus ze.EventBus) (int, error) {
	return bus.Emit(e.namespace, e.eventType, nil)
}

// Subscribe registers a no-payload handler. The bus still delivers nil as
// an `any`; the wrapper discards it and invokes the handler.
func (e *SignalEvent) Subscribe(bus ze.EventBus, handler func()) func() {
	if handler == nil {
		return func() {}
	}
	return bus.Subscribe(e.namespace, e.eventType, func(_ any) {
		handler()
	})
}

// Register declares a typed event and returns the handle. Call from package
// init at the top level:
//
//	var BestChange = events.Register[*BestChangeBatch]("bgp-rib", "best-change")
//
// Register performs three actions:
//   - Registers the eventType string under the namespace (same as
//     RegisterNamespace).
//   - Records the reflect.Type of T in the type registry.
//   - Returns a typed handle whose Emit and Subscribe are bound to T.
//
// Panics with "BUG:" prefix on any registration error (empty namespace,
// empty eventType, interface-typed payload, or duplicate registration with
// a different Go type). These are programmer errors detectable at init
// time. Two init calls with the same (ns, et, T) are idempotent.
func Register[T any](namespace, eventType string) *Event[T] {
	if namespace == "" {
		panic("BUG: events.Register: empty namespace for event " + eventType)
	}
	if eventType == "" {
		panic("BUG: events.Register: empty eventType in namespace " + namespace)
	}
	typ := reflect.TypeFor[T]()
	// Interfaces (e.g. `any`) defeat the registry: consumers would receive
	// anything a publisher hands over. Reject at Register time.
	if typ.Kind() == reflect.Interface {
		panic("BUG: events.Register: " + namespace + "/" + eventType +
			": payload type must be concrete, not an interface")
	}

	if err := RegisterNamespace(namespace, eventType); err != nil {
		panic("BUG: events.Register: " + namespace + "/" + eventType +
			": namespace registration failed: " + err.Error())
	}

	key := typeKey{namespace: namespace, eventType: eventType}
	typeRegistryMu.Lock()
	if existing, ok := typeRegistry[key]; ok && existing != typ {
		typeRegistryMu.Unlock()
		panic("BUG: events.Register: " + namespace + "/" + eventType +
			": already registered with type " + existing.String() +
			", cannot re-register as " + typ.String())
	}
	typeRegistry[key] = typ
	typeRegistryMu.Unlock()

	return &Event[T]{namespace: namespace, eventType: eventType}
}

// RegisterSignal declares a no-payload (signal) event and returns the handle.
// Call from package init:
//
//	var ReplayRequest = events.RegisterSignal("bgp-rib", "replay-request")
//
// Panics with "BUG:" prefix if (namespace, eventType) was already registered
// as a typed event. Two init calls with the same (ns, et) as signals are
// idempotent.
func RegisterSignal(namespace, eventType string) *SignalEvent {
	if namespace == "" {
		panic("BUG: events.RegisterSignal: empty namespace for event " + eventType)
	}
	if eventType == "" {
		panic("BUG: events.RegisterSignal: empty eventType in namespace " + namespace)
	}

	if err := RegisterNamespace(namespace, eventType); err != nil {
		panic("BUG: events.RegisterSignal: " + namespace + "/" + eventType +
			": namespace registration failed: " + err.Error())
	}

	sigType := reflect.TypeFor[signalType]()
	key := typeKey{namespace: namespace, eventType: eventType}
	typeRegistryMu.Lock()
	if existing, ok := typeRegistry[key]; ok && existing != sigType {
		typeRegistryMu.Unlock()
		panic("BUG: events.RegisterSignal: " + namespace + "/" + eventType +
			": already registered with type " + existing.String() +
			", cannot re-register as signal")
	}
	typeRegistry[key] = sigType
	typeRegistryMu.Unlock()

	return &SignalEvent{namespace: namespace, eventType: eventType}
}

// PayloadType returns the canonical payload type registered for
// (namespace, eventType), or nil if no type is registered. Signal events
// return the sentinel signalType; a nil return means no Register call has
// happened for this pair.
func PayloadType(namespace, eventType string) reflect.Type {
	typeRegistryMu.RLock()
	defer typeRegistryMu.RUnlock()
	return typeRegistry[typeKey{namespace: namespace, eventType: eventType}]
}

// PayloadInfo returns both the payload type and whether the event is a
// signal in a single locked query. Use this when both pieces of
// information are needed together to avoid the registry mutating between
// two separate lookups (in practice the registry is init-only, but this
// keeps the contract explicit). typ is nil when no Register call has
// happened for this pair.
func PayloadInfo(namespace, eventType string) (typ reflect.Type, isSignal bool) {
	typeRegistryMu.RLock()
	defer typeRegistryMu.RUnlock()
	typ = typeRegistry[typeKey{namespace: namespace, eventType: eventType}]
	if typ == nil {
		return nil, false
	}
	return typ, typ == reflect.TypeFor[signalType]()
}

// AsString adapts a legacy string-payload handler to the typed-bus
// signature. Wraps fn in a type assertion to string. The wrapper logs
// the first non-string drop for each distinct dynamic payload type seen
// on the wrapper — so a producer migrating to Event[T] does not vanish
// silently, and a cascade of distinct wrong types each surfaces once
// instead of being suppressed by the first.
//
// Prefer Event[T].Subscribe for new code: it registers the payload type
// and the bus does the conversion automatically.
func AsString(fn func(string)) func(any) {
	if fn == nil {
		return func(_ any) {}
	}
	// seen tracks distinct dynamic payload types that have already been
	// logged on this wrapper. Bounded in practice by the (small) set of
	// payload types real producers emit on a single (ns, et); a
	// pathological producer cycling through many types would grow this
	// map unboundedly, but no real workload does that.
	var seen sync.Map // reflect.Type -> struct{}
	return func(p any) {
		if s, ok := p.(string); ok {
			fn(s)
			return
		}
		typ := reflect.TypeOf(p)
		if _, loaded := seen.LoadOrStore(typ, struct{}{}); loaded {
			return // this dynamic type already logged once
		}
		logger().Warn("eventbus: AsString wrapper received non-string payload, dropping (further drops of this type on this wrapper suppressed)",
			"got", typ)
	}
}

// IsSignal reports whether (namespace, eventType) is registered as a signal
// (no-payload) event.
func IsSignal(namespace, eventType string) bool {
	typeRegistryMu.RLock()
	defer typeRegistryMu.RUnlock()
	typ, ok := typeRegistry[typeKey{namespace: namespace, eventType: eventType}]
	if !ok {
		return false
	}
	return typ == reflect.TypeFor[signalType]()
}
