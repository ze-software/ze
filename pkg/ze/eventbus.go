// Design: docs/architecture/core-design.md -- namespaced event bus
//
// EventBus replaces the older topic-based Bus interface. Internal plugins
// and engine components publish and subscribe through it; the implementation
// lives in internal/component/plugin/server and reuses the same fan-out
// path as plugin-process events (DirectBridge for in-process plugins, TLS
// for external plugins, in-process callbacks for engine components).

package ze

// EventBus is the namespaced pub/sub interface for cross-component
// communication within the Ze process.
//
// Events are addressed by (namespace, eventType) pairs. The namespace
// represents a publishing component (e.g. "bgp", "sysrib", "interface")
// and the eventType identifies the specific event within that namespace
// (e.g. "best-change", "listener-ready", "addr-added").
//
// Payloads are opaque strings; the bus never inspects them. By convention
// payloads are JSON-encoded structs defined by the publishing namespace.
// Publishers and subscribers within the same namespace agree on the
// payload schema out of band; there is no central schema registry.
//
// Both engine components and internal plugins use this single interface,
// so external plugin authors who fork ze and add a new component import
// only this package -- not internal/component/plugin/server.
type EventBus interface {
	// Emit publishes an event to all subscribers matching the given
	// namespace and eventType. Returns the number of plugin process
	// subscribers that received the event; engine subscribers always
	// receive synchronously and are not counted in the return value.
	//
	// The (namespace, eventType) pair MUST be registered in the engine's
	// event registry. Unknown pairs return a non-nil error and the event
	// is not delivered to anybody.
	Emit(namespace, eventType, payload string) (int, error)

	// Subscribe registers a handler for events matching (namespace,
	// eventType). The handler runs synchronously when an event is
	// emitted; it MUST NOT block on I/O.
	//
	// Returns an unsubscribe function. Calling the returned function
	// removes the handler. Safe to call multiple times: subsequent
	// calls are no-ops.
	//
	// A nil handler returns a no-op unsubscribe and registers nothing.
	Subscribe(namespace, eventType string, handler func(payload string)) (unsubscribe func())
}
