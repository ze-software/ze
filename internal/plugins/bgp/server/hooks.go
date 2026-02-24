// Design: docs/architecture/core-design.md — BGP server events and hooks
//
// Package server provides BGP-specific hook implementations for the plugin server.
// These functions are registered as BGPHooks callbacks, keeping BGP protocol
// knowledge out of the generic plugin infrastructure.
package server

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/format"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// NewBGPHooks creates the BGPHooks callbacks for the plugin server.
// The JSONEncoder is created once and captured in closures.
func NewBGPHooks() *plugin.BGPHooks {
	encoder := format.NewJSONEncoder("0.0.1")

	return &plugin.BGPHooks{
		OnMessageReceived: func(s *plugin.Server, peer plugin.PeerInfo, msg any) int {
			typedMsg, ok := msg.(bgptypes.RawMessage)
			if !ok {
				logger().Warn("OnMessageReceived: invalid msg type", "type", fmt.Sprintf("%T", msg))
				return 0
			}
			return onMessageReceived(s, encoder, peer, typedMsg)
		},
		OnMessageBatchReceived: func(s *plugin.Server, peer plugin.PeerInfo, msgs []any) []int {
			typedMsgs := make([]bgptypes.RawMessage, len(msgs))
			for i, msg := range msgs {
				typedMsg, ok := msg.(bgptypes.RawMessage)
				if !ok {
					logger().Warn("OnMessageBatchReceived: invalid msg type", "type", fmt.Sprintf("%T", msg))
					continue // zero-value RawMessage preserves 1:1 index mapping
				}
				typedMsgs[i] = typedMsg
			}
			return onMessageBatchReceived(s, encoder, peer, typedMsgs)
		},
		OnPeerStateChange: func(s *plugin.Server, peer plugin.PeerInfo, state string) {
			onPeerStateChange(s, peer, state)
		},
		OnPeerNegotiated: func(s *plugin.Server, peer plugin.PeerInfo, neg any) {
			onPeerNegotiated(s, encoder, peer, neg)
		},
		OnMessageSent: func(s *plugin.Server, peer plugin.PeerInfo, msg any) {
			typedMsg, ok := msg.(bgptypes.RawMessage)
			if !ok {
				logger().Warn("OnMessageSent: invalid msg type", "type", fmt.Sprintf("%T", msg))
				return
			}
			onMessageSent(s, encoder, peer, typedMsg)
		},
		BroadcastValidateOpen: broadcastValidateOpen,
		CodecRPCHandler:       codecRPCHandler,
	}
}
