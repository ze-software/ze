// Design: docs/architecture/config/transaction-protocol.md -- engine-side stream pub/sub adapter
// Related: engine_event.go -- the underlying Server methods
// Related: ../../config/transaction/gateway.go -- the EventGateway interface this adapter satisfies

package server

import (
	"context"

	txevents "github.com/ze-software/ze/internal/component/config/transaction/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

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
	ctx    context.Context
}

// newConfigEventGateway creates a new adapter wrapping the given Server.
// The Server must outlive the gateway; the gateway holds a reference but
// does not manage Server lifecycle.
func newConfigEventGateway(s *Server) *ConfigEventGateway {
	return newConfigEventGatewayWithContext(s, context.Background())
}

// newConfigEventGatewayWithContext creates a ConfigEventGateway bound to ctx so
// config transaction events carry the reload's operation-id in EventRing.
func newConfigEventGatewayWithContext(s *Server, ctx context.Context) *ConfigEventGateway {
	if ctx == nil {
		ctx = context.Background()
	}
	return &ConfigEventGateway{server: s, ctx: ctx}
}

// EmitConfigEvent publishes a stream event in the config namespace.
// Returns the number of plugin processes that received the event.
// Empty payloads are rejected — config acks always carry at least a
// transaction-id envelope, so an empty []byte is a programmer error.
func (g *ConfigEventGateway) EmitConfigEvent(eventType string, payload []byte) (int, error) {
	if len(payload) == 0 {
		return 0, &rpc.RPCCallError{Message: "emit-config-event requires non-empty payload"}
	}
	return g.server.EmitEngineEventContext(g.ctx, txevents.Namespace, eventType, string(payload))
}

// SubscribeConfigEvent registers a handler for a config namespace event type.
// The handler is invoked synchronously from deliverEvent. Returns an
// unsubscribe function; nil handler returns a no-op unsubscribe.
func (g *ConfigEventGateway) SubscribeConfigEvent(eventType string, handler func(payload []byte)) func() {
	if handler == nil {
		return func() {}
	}
	return g.server.SubscribeEngineEvent(txevents.Namespace, eventType, func(p any) {
		if event, ok := p.(string); ok {
			handler([]byte(event))
			return
		}
		if raw, ok := p.([]byte); ok {
			handler(raw)
		}
	})
}

// SubscribeEvent registers a handler for any namespace/event pair. Operation
// settlement uses this to wait for side-effect events outside config.
func (g *ConfigEventGateway) SubscribeEvent(namespace, eventType string, handler func(payload any)) func() {
	if handler == nil {
		return func() {}
	}
	return g.server.SubscribeEngineEvent(namespace, eventType, handler)
}
