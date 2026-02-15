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
		OnMessageReceived: func(s *plugin.Server, peer plugin.PeerInfo, msg any) {
			typedMsg, ok := msg.(bgptypes.RawMessage)
			if !ok {
				logger().Warn("OnMessageReceived: invalid msg type", "type", fmt.Sprintf("%T", msg))
				return
			}
			onMessageReceived(s, encoder, peer, typedMsg)
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
			onMessageSent(s, peer, typedMsg)
		},
		BroadcastValidateOpen: broadcastValidateOpen,
		CodecRPCHandler:       codecRPCHandler,
	}
}
