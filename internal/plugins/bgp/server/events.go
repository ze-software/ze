// Package server provides BGP-specific hook implementations for the plugin server.
// These functions are registered as BGPHooks callbacks, keeping BGP protocol
// knowledge out of the generic plugin infrastructure.
package server

import (
	"context"
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// logger is the bgp.server subsystem logger.
var logger = slogutil.LazyLogger("bgp.server")

// onMessageReceived handles raw BGP messages from peers.
// Forwards to processes based on API subscriptions.
func onMessageReceived(s *plugin.Server, encoder *format.JSONEncoder, peer plugin.PeerInfo, msg bgptypes.RawMessage) int {
	if s.Context().Err() != nil {
		return 0 // Server shutting down, skip event delivery
	}

	eventType := messageTypeToEventType(msg.Type)
	if eventType == "" {
		logger().Debug("OnMessageReceived: unknown event type", "msgType", msg.Type)
		return 0
	}

	logger().Debug("OnMessageReceived", "peer", peer.Address.String(), "event", eventType, "dir", msg.Direction)
	procs := s.Subscriptions().GetMatching(plugin.NamespaceBGP, eventType, msg.Direction, peer.Address.String())
	logger().Debug("OnMessageReceived matched", "count", len(procs))

	for _, proc := range procs {
		output := formatMessageForSubscription(encoder, peer, msg, proc.Format())
		logger().Debug("OnMessageReceived writing", "proc", proc.Name(), "outputLen", len(output))

		connB := proc.ConnB()
		if connB == nil {
			continue
		}

		deliverCtx, cancel := context.WithTimeout(s.Context(), 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()

		if err != nil && s.Context().Err() == nil {
			logger().Warn("OnMessageReceived write failed", "proc", proc.Name(), "err", err)
		}
	}

	return len(procs)
}

// messageTypeToEventType converts BGP message type to event type string.
// Returns empty string for unsupported types (caller checks for empty).
func messageTypeToEventType(msgType message.MessageType) string {
	switch msgType { //nolint:exhaustive // Only supported types; caller checks empty return
	case message.TypeUPDATE:
		return plugin.EventUpdate
	case message.TypeOPEN:
		return plugin.EventOpen
	case message.TypeNOTIFICATION:
		return plugin.EventNotification
	case message.TypeKEEPALIVE:
		return plugin.EventKeepalive
	case message.TypeROUTEREFRESH:
		return plugin.EventRefresh
	default: // Unsupported type — caller checks for empty string
		return ""
	}
}

// formatMessageForSubscription formats a BGP message for subscription-based delivery.
// Uses JSON encoding with the specified format (from process settings).
func formatMessageForSubscription(encoder *format.JSONEncoder, peer plugin.PeerInfo, msg bgptypes.RawMessage, fmtMode string) string {
	switch msg.Type { //nolint:exhaustive // Only supported types; unsupported are filtered by caller
	case message.TypeUPDATE:
		content := bgptypes.ContentConfig{
			Encoding: plugin.EncodingJSON,
			Format:   fmtMode,
		}
		return format.FormatMessage(peer, msg, content, "")

	case message.TypeOPEN:
		decoded := format.DecodeOpen(msg.RawBytes)
		return encoder.Open(peer, decoded, msg.Direction, msg.MessageID)

	case message.TypeNOTIFICATION:
		decoded := format.DecodeNotification(msg.RawBytes)
		return encoder.Notification(peer, decoded, msg.Direction, msg.MessageID)

	case message.TypeKEEPALIVE:
		return encoder.Keepalive(peer, msg.Direction, msg.MessageID)

	case message.TypeROUTEREFRESH:
		decoded := format.DecodeRouteRefresh(msg.RawBytes)
		return encoder.RouteRefresh(peer, decoded, msg.Direction, msg.MessageID)

	default: // Unsupported type — filtered by messageTypeToEventType before reaching here
		return ""
	}
}

// onPeerStateChange handles peer state transitions.
// Called by reactor when peer state changes (not a BGP message).
func onPeerStateChange(s *plugin.Server, peer plugin.PeerInfo, state string) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	logger().Debug("OnPeerStateChange", "peer", peer.Address.String(), "state", state)

	procs := s.Subscriptions().GetMatching(plugin.NamespaceBGP, plugin.EventState, "", peer.Address.String())
	logger().Debug("OnPeerStateChange matched", "count", len(procs))

	for _, proc := range procs {
		output := format.FormatStateChange(peer, state, plugin.EncodingJSON)
		logger().Debug("OnPeerStateChange writing", "proc", proc.Name())

		connB := proc.ConnB()
		if connB == nil {
			continue
		}

		deliverCtx, cancel := context.WithTimeout(s.Context(), 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()

		if err != nil && s.Context().Err() == nil {
			logger().Warn("OnPeerStateChange write failed", "proc", proc.Name(), "err", err)
		}
	}
}

// onPeerNegotiated handles capability negotiation completion.
// neg is format.DecodedNegotiated passed as any from the generic hook.
func onPeerNegotiated(s *plugin.Server, encoder *format.JSONEncoder, peer plugin.PeerInfo, neg any) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	decoded, ok := neg.(format.DecodedNegotiated)
	if !ok {
		logger().Warn("OnPeerNegotiated: invalid neg type", "type", fmt.Sprintf("%T", neg))
		return
	}

	procs := s.Subscriptions().GetMatching(plugin.NamespaceBGP, plugin.EventNegotiated, "", peer.Address.String())
	for _, proc := range procs {
		output := format.FormatNegotiated(peer, decoded, encoder)

		connB := proc.ConnB()
		if connB == nil {
			continue
		}

		deliverCtx, cancel := context.WithTimeout(s.Context(), 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()

		if err != nil && s.Context().Err() == nil {
			logger().Warn("OnPeerNegotiated write failed", "proc", proc.Name(), "err", err)
		}
	}
}

// onMessageSent handles BGP messages sent to peers.
// Forwards to processes that subscribed to sent events.
// Uses the JSONEncoder for non-UPDATE messages (same as onMessageReceived),
// and FormatSentMessage for UPDATEs (which adds the "type":"sent" marker).
func onMessageSent(s *plugin.Server, encoder *format.JSONEncoder, peer plugin.PeerInfo, msg bgptypes.RawMessage) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	eventType := messageTypeToEventType(msg.Type)
	logger().Debug("OnMessageSent", "peer", peer.Address.String(), "type", eventType)

	if eventType == "" {
		return
	}

	procs := s.Subscriptions().GetMatching(plugin.NamespaceBGP, eventType, plugin.DirectionSent, peer.Address.String())
	logger().Debug("OnMessageSent matched", "count", len(procs))

	for _, proc := range procs {
		var output string
		if msg.Type == message.TypeUPDATE {
			// UPDATE sent events use FormatSentMessage for the "type":"sent" marker.
			content := bgptypes.ContentConfig{
				Encoding: plugin.EncodingJSON,
				Format:   proc.Format(),
			}
			output = format.FormatSentMessage(peer, msg, content)
		} else {
			// Non-UPDATE sent events use the same JSON formatters as received events.
			// formatMessageForSubscription dispatches to encoder.Open(), encoder.Notification(),
			// etc., which always produce JSON. msg.Direction is already "sent".
			output = formatMessageForSubscription(encoder, peer, msg, proc.Format())
		}
		logger().Debug("OnMessageSent writing", "proc", proc.Name())

		connB := proc.ConnB()
		if connB == nil {
			continue
		}

		deliverCtx, cancel := context.WithTimeout(s.Context(), 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()

		if err != nil && s.Context().Err() == nil {
			logger().Warn("OnMessageSent write failed", "proc", proc.Name(), "err", err)
		}
	}
}
