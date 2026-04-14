// Design: docs/architecture/config/transaction-protocol.md -- engine-side stream pub/sub adapter
// Related: engine_event.go -- the underlying Server methods
// Related: ../../config/transaction/gateway.go -- the EventGateway interface this adapter satisfies

package server

import txevents "codeberg.org/thomas-mangin/ze/internal/component/config/transaction/events"

// ConfigEventGateway adapts Server to the
// internal/component/config/transaction.EventGateway interface used by the
// config transaction orchestrator.
//
// The adapter hides the namespace parameter (always "config")
// and converts between the orchestrator's []byte payloads and Server's
// string event payloads.
//
// Performance note: each emit/dispatch round-trips through []byte -> string
// -> []byte (one copy in EmitConfigEvent, one copy in the SubscribeConfigEvent
// handler bridge). This is acceptable for small config transaction payloads
// (~hundreds of bytes per ack) and trades two small allocations against the
// simpler string-based deliverEvent path. If this ever becomes a hot path,
// the right fix is to add a []byte-native variant to deliverEvent rather
// than complicating the adapter.
type ConfigEventGateway struct {
	server *Server
}

// NewConfigEventGateway creates a new adapter wrapping the given Server.
// The Server must outlive the gateway; the gateway holds a reference but
// does not manage Server lifecycle.
func NewConfigEventGateway(s *Server) *ConfigEventGateway {
	return &ConfigEventGateway{server: s}
}

// EmitConfigEvent publishes a stream event in the config namespace.
// Returns the number of plugin processes that received the event.
func (g *ConfigEventGateway) EmitConfigEvent(eventType string, payload []byte) (int, error) {
	return g.server.EmitEngineEvent(txevents.Namespace, eventType, string(payload))
}

// SubscribeConfigEvent registers a handler for a config namespace event type.
// The handler is invoked synchronously from deliverEvent. Returns an
// unsubscribe function; nil handler returns a no-op unsubscribe.
func (g *ConfigEventGateway) SubscribeConfigEvent(eventType string, handler func(payload []byte)) func() {
	if handler == nil {
		return func() {}
	}
	return g.server.SubscribeEngineEvent(txevents.Namespace, eventType, func(event string) {
		handler([]byte(event))
	})
}
