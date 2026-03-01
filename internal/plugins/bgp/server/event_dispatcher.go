// Design: docs/architecture/core-design.md — BGP event dispatch to plugins
// Related: events.go — event delivery functions (onMessageReceived, onPeerStateChange, etc.)
// Related: validate.go — OPEN validation via plugins (broadcastValidateOpen)
// Related: codec.go — codec RPC handler (CodecRPCHandler)

package server

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// EventDispatcher bridges the reactor to plugin event delivery.
// It holds a *plugin.Server for access to subscriptions, context, and processes,
// and a shared JSONEncoder for format encoding.
//
// EventDispatcher satisfies reactor.MessageReceiver implicitly (OnMessageReceived,
// OnMessageBatchReceived, OnMessageSent). The reactor sets it as the message
// receiver instead of using *plugin.Server directly, providing type-safe
// BGP event dispatch without any-typed indirection.
type EventDispatcher struct {
	server  *plugin.Server
	encoder *format.JSONEncoder
}

// NewEventDispatcher creates a new EventDispatcher for the given plugin server.
// The JSONEncoder is created once and reused for all event formatting.
func NewEventDispatcher(server *plugin.Server) *EventDispatcher {
	return &EventDispatcher{
		server:  server,
		encoder: format.NewJSONEncoder("0.0.1"),
	}
}

// OnMessageReceived handles raw BGP messages from peers.
// msg is bgptypes.RawMessage (typed as any to match reactor.MessageReceiver).
// Returns the count of cache-consumer plugins that successfully received the event.
func (d *EventDispatcher) OnMessageReceived(peer plugin.PeerInfo, msg any) int {
	if d.server.ProcessManager() == nil || d.server.Subscriptions() == nil {
		return 0
	}
	typedMsg, ok := msg.(bgptypes.RawMessage)
	if !ok {
		logger().Warn("OnMessageReceived: invalid msg type", "type", fmt.Sprintf("%T", msg))
		return 0
	}
	return onMessageReceived(d.server, d.encoder, peer, typedMsg)
}

// OnMessageBatchReceived handles a batch of received BGP messages from the same peer.
// msgs is []bgptypes.RawMessage (typed as []any to match reactor.MessageReceiver).
// Returns per-message cache-consumer counts for Activate calls.
func (d *EventDispatcher) OnMessageBatchReceived(peer plugin.PeerInfo, msgs []any) []int {
	if d.server.ProcessManager() == nil || d.server.Subscriptions() == nil {
		return make([]int, len(msgs))
	}
	typedMsgs := make([]bgptypes.RawMessage, len(msgs))
	for i, msg := range msgs {
		typedMsg, ok := msg.(bgptypes.RawMessage)
		if !ok {
			logger().Warn("OnMessageBatchReceived: invalid msg type", "type", fmt.Sprintf("%T", msg))
			continue // zero-value RawMessage preserves 1:1 index mapping
		}
		typedMsgs[i] = typedMsg
	}
	return onMessageBatchReceived(d.server, d.encoder, peer, typedMsgs)
}

// OnMessageSent handles BGP messages sent to peers.
// msg is bgptypes.RawMessage (typed as any to match reactor.MessageReceiver).
func (d *EventDispatcher) OnMessageSent(peer plugin.PeerInfo, msg any) {
	if d.server.ProcessManager() == nil || d.server.Subscriptions() == nil {
		return
	}
	typedMsg, ok := msg.(bgptypes.RawMessage)
	if !ok {
		logger().Warn("OnMessageSent: invalid msg type", "type", fmt.Sprintf("%T", msg))
		return
	}
	onMessageSent(d.server, d.encoder, peer, typedMsg)
}

// OnPeerStateChange handles peer state transitions.
// Called by apiStateObserver when peers are established or closed.
func (d *EventDispatcher) OnPeerStateChange(peer plugin.PeerInfo, state string) {
	if d.server.ProcessManager() == nil || d.server.Subscriptions() == nil {
		return
	}
	onPeerStateChange(d.server, peer, state)
}

// OnPeerNegotiated handles capability negotiation completion.
// neg is format.DecodedNegotiated (typed as any from reactor).
func (d *EventDispatcher) OnPeerNegotiated(peer plugin.PeerInfo, neg any) {
	if d.server.ProcessManager() == nil || d.server.Subscriptions() == nil {
		return
	}
	onPeerNegotiated(d.server, d.encoder, peer, neg)
}

// BroadcastValidateOpen validates OPEN messages via all plugins that declared WantsValidateOpen.
// local and remote are *message.Open (typed as any from reactor).
// Returns nil if all accept, or an OpenValidationError on first rejection.
func (d *EventDispatcher) BroadcastValidateOpen(peerAddr string, local, remote any) error {
	localOpen, ok := local.(*message.Open)
	if !ok {
		return fmt.Errorf("BroadcastValidateOpen: local not *message.Open: %T", local)
	}
	remoteOpen, ok := remote.(*message.Open)
	if !ok {
		return fmt.Errorf("BroadcastValidateOpen: remote not *message.Open: %T", remote)
	}
	return broadcastValidateOpen(d.server, peerAddr, localOpen, remoteOpen)
}
