// Package server provides BGP-specific hook implementations for the plugin server.
// These functions are registered as BGPHooks callbacks, keeping BGP protocol
// knowledge out of the generic plugin infrastructure.
package server

import (
	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/format"
)

// NewBGPHooks creates the BGPHooks callbacks for the plugin server.
// The JSONEncoder is created once and captured in closures.
func NewBGPHooks() *plugin.BGPHooks {
	encoder := format.NewJSONEncoder("0.0.1")

	return &plugin.BGPHooks{
		OnMessageReceived: func(s *plugin.Server, peer plugin.PeerInfo, msg plugin.RawMessage) {
			onMessageReceived(s, encoder, peer, msg)
		},
		OnPeerStateChange: func(s *plugin.Server, peer plugin.PeerInfo, state string) {
			onPeerStateChange(s, peer, state)
		},
		OnPeerNegotiated: func(s *plugin.Server, peer plugin.PeerInfo, neg any) {
			onPeerNegotiated(s, encoder, peer, neg)
		},
		OnMessageSent: func(s *plugin.Server, peer plugin.PeerInfo, msg plugin.RawMessage) {
			onMessageSent(s, peer, msg)
		},
		BroadcastValidateOpen: broadcastValidateOpen,
		CodecRPCHandler:       codecRPCHandler,
	}
}
