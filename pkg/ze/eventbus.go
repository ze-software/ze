// Design: docs/architecture/core-design.md -- namespaced event bus (typed payloads)
//
// EventBus is the typed pub/sub interface for cross-component communication
// within the Ze process. In-process subscribers (engine Go code and internal
// plugins running as goroutines) receive the publisher's Go value directly
// via `any`. External plugin processes receive a JSON string marshaled
// lazily inside the bus, at most once per Emit, only when at least one
// external subscriber exists.
//
// The canonical payload type for each (namespace, eventType) pair is fixed by
// the publishing package and documented next to the event constant in that
// package's events subpackage. Consumers type-assert on receive.

package ze

// EventBus is the namespaced pub/sub interface for cross-component
// communication within the Ze process.
//
// Events are addressed by (namespace, eventType) pairs. The namespace
// represents a publishing component (e.g. "bgp", "sysrib", "interface")
// and the eventType identifies the specific event within that namespace
// (e.g. "best-change", "listener-ready", "addr-added").
//
// Payloads are typed Go values, passed as `any`. In-process subscribers
// receive the original value with zero serialization. External plugin
// processes receive JSON bytes; the marshal happens at most once per Emit,
// only when at least one external subscriber exists. For signal-only events
// a nil payload is valid.
//
// Both engine components and internal plugins use this single interface,
// so external plugin authors who fork ze and add a new component import
// only this package -- not internal/component/plugin/server.
type EventBus interface {
	// Emit publishes an event to all subscribers matching the given
	// namespace and eventType. In-process subscribers receive payload
	// directly. External plugin-process subscribers receive a JSON
	// marshaling of payload (produced once per Emit when any external
	// subscriber exists). Returns the number of plugin-process
	// subscribers that received the event; engine subscribers always
	// receive synchronously and are not counted.
	//
	// The (namespace, eventType) pair MUST be registered in the engine's
	// event registry. Unknown pairs return a non-nil error and the event
	// is not delivered to anybody.
	//
	// payload may be nil for signal-only events. Producers that have
	// nothing but bytes may pass a json.RawMessage; subscribers receive
	// the same json.RawMessage.
	Emit(namespace, eventType string, payload any) (int, error)

	// Subscribe registers an in-process handler for events matching
	// (namespace, eventType). The handler runs synchronously when an
	// event is emitted and MUST NOT block on I/O.
	//
	// The handler receives the publisher's typed payload via `any`;
	// consumers type-assert to the documented payload type for that
	// (namespace, eventType). The canonical type is documented next to
	// the event constant in the publishing package's events subpackage.
	//
	// Subscribers MUST treat the payload as read-only. When the payload
	// is a pointer (the common case for large structs like
	// *BestChangeBatch), every engine subscriber receives the SAME
	// pointer; mutating the value would race with peer subscribers. If
	// a subscriber needs to retain or modify state derived from the
	// payload, it must copy what it needs.
	//
	// Returns an unsubscribe function. Calling the returned function
	// removes the handler. Safe to call multiple times: subsequent
	// calls are no-ops.
	//
	// A nil handler returns a no-op unsubscribe and registers nothing.
	Subscribe(namespace, eventType string, handler func(payload any)) (unsubscribe func())
}
