// Design: docs/architecture/api/process-protocol.md — reactor event callbacks
// Related: server.go — Server struct and lifecycle

package plugin

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"
)

// --- BGP event delegation ---
// These methods delegate to BGPHooks when set.
// They are called by the reactor for BGP event delivery.

// OnMessageReceived handles raw BGP messages from peers.
// msg is bgptypes.RawMessage (typed as any to avoid BGP imports).
// Delegates to BGPHooks.OnMessageReceived when set.
// Returns the count of cache-consumer plugins that successfully received the event.
func (s *Server) OnMessageReceived(peer PeerInfo, msg any) int {
	if s.bgpHooks == nil || s.bgpHooks.OnMessageReceived == nil {
		return 0
	}
	if s.procManager == nil || s.subscriptions == nil {
		return 0
	}
	return s.bgpHooks.OnMessageReceived(s, peer, msg)
}

// OnPeerStateChange handles peer state transitions.
// Delegates to BGPHooks.OnPeerStateChange when set.
func (s *Server) OnPeerStateChange(peer PeerInfo, state string) {
	if s.bgpHooks == nil || s.bgpHooks.OnPeerStateChange == nil {
		return
	}
	if s.procManager == nil || s.subscriptions == nil {
		return
	}
	s.bgpHooks.OnPeerStateChange(s, peer, state)
}

// OnPeerNegotiated handles capability negotiation completion.
// neg is format.DecodedNegotiated (typed as any to avoid BGP imports).
// Delegates to BGPHooks.OnPeerNegotiated when set.
func (s *Server) OnPeerNegotiated(peer PeerInfo, neg any) {
	if s.bgpHooks == nil || s.bgpHooks.OnPeerNegotiated == nil {
		return
	}
	if s.procManager == nil || s.subscriptions == nil {
		return
	}
	s.bgpHooks.OnPeerNegotiated(s, peer, neg)
}

// OnMessageSent handles BGP messages sent to peers.
// msg is bgptypes.RawMessage (typed as any to avoid BGP imports).
// Delegates to BGPHooks.OnMessageSent when set.
func (s *Server) OnMessageSent(peer PeerInfo, msg any) {
	if s.bgpHooks == nil || s.bgpHooks.OnMessageSent == nil {
		return
	}
	if s.procManager == nil || s.subscriptions == nil {
		return
	}
	s.bgpHooks.OnMessageSent(s, peer, msg)
}

// BroadcastValidateOpen sends validate-open to all plugins that declared WantsValidateOpen.
// local and remote are *message.Open (typed as any to avoid BGP imports).
// Returns nil if all accept, or an OpenValidationError on first rejection.
func (s *Server) BroadcastValidateOpen(peerAddr string, local, remote any) error {
	if s.bgpHooks == nil || s.bgpHooks.BroadcastValidateOpen == nil {
		return nil
	}
	return s.bgpHooks.BroadcastValidateOpen(s, peerAddr, local, remote)
}

// EncodeNLRI encodes NLRI by routing to the appropriate family plugin via RPC.
// Returns error if no plugin registered or plugin not running.
func (s *Server) EncodeNLRI(family string, args []string) ([]byte, error) {
	if s.registry == nil || s.procManager == nil {
		return nil, fmt.Errorf("server not configured for plugins")
	}

	pluginName := s.registry.LookupFamily(family)
	if pluginName == "" {
		return nil, fmt.Errorf("no plugin registered for family %s", family)
	}

	proc := s.procManager.GetProcess(pluginName)
	if proc == nil {
		return nil, fmt.Errorf("plugin %s not running", pluginName)
	}

	connB := proc.ConnB()
	if connB == nil {
		return nil, fmt.Errorf("plugin %s connection closed", pluginName)
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	hexStr, err := connB.SendEncodeNLRI(ctx, family, args)
	if err != nil {
		return nil, fmt.Errorf("plugin request failed: %w", err)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("decode plugin hex response: %w", err)
	}
	return data, nil
}

// DecodeNLRI decodes NLRI by routing to the appropriate family plugin via RPC.
// Returns the JSON representation of the decoded NLRI.
// Returns error if no plugin registered or plugin not running.
func (s *Server) DecodeNLRI(family, hexData string) (string, error) {
	if s.registry == nil || s.procManager == nil {
		return "", fmt.Errorf("server not configured for plugins")
	}

	pluginName := s.registry.LookupFamily(family)
	if pluginName == "" {
		return "", fmt.Errorf("no plugin registered for family %s", family)
	}

	proc := s.procManager.GetProcess(pluginName)
	if proc == nil {
		return "", fmt.Errorf("plugin %s not running", pluginName)
	}

	connB := proc.ConnB()
	if connB == nil {
		return "", fmt.Errorf("plugin %s connection closed", pluginName)
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	jsonResult, err := connB.SendDecodeNLRI(ctx, family, hexData)
	if err != nil {
		return "", fmt.Errorf("plugin request failed: %w", err)
	}

	return jsonResult, nil
}
