// Design: docs/architecture/config/transaction-protocol.md -- orchestrator's view of the stream system
// Related: orchestrator.go -- the consumer of this interface
// Related: topics.go -- event type constants the orchestrator passes to gateway methods
// Related: ../../plugin/server/engine_event_gateway.go -- ConfigEventGateway adapter that satisfies this interface

package transaction

// EventGateway is the orchestrator's view of the stream event system.
//
// The orchestrator publishes config namespace events (verify-<plugin>,
// apply-<plugin>, rollback, committed, applied, rolled-back, verify-abort)
// and subscribes to plugin-emitted ack events (verify-ok, verify-failed,
// apply-ok, apply-failed, rollback-ok). All events are in the config
// namespace; the gateway hides the namespace parameter from the
// orchestrator.
//
// The Server in internal/component/plugin/server provides a
// ConfigEventGateway adapter that satisfies this interface by delegating
// to Server.EmitEngineEvent / Server.SubscribeEngineEvent in the config
// namespace.
type EventGateway interface {
	// EmitConfigEvent publishes a stream event in the config namespace.
	// Returns the number of plugin processes that received it. Engine
	// subscribers (other orchestrators, observers) also receive the event
	// but are not counted in the return value.
	//
	// eventType MUST be a registered event type in the config namespace
	// (see plugin.IsValidEvent). Per-plugin event types
	// (verify-<plugin>, apply-<plugin>) must be registered before they
	// can be emitted.
	EmitConfigEvent(eventType string, payload []byte) (int, error)

	// SubscribeConfigEvent registers a handler for a config namespace
	// event type. The handler fires synchronously when a matching event
	// is published; it must not block on external I/O (push to a buffered
	// channel and return).
	//
	// Returns an unsubscribe function. Calling it removes the handler.
	// Safe to call multiple times -- subsequent calls are no-ops.
	// If handler is nil, the returned function is a no-op (no
	// registration is performed).
	SubscribeConfigEvent(eventType string, handler func(payload []byte)) func()
}
