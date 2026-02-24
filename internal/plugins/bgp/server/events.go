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
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// logger is the bgp.server subsystem logger.
var logger = slogutil.LazyLogger("bgp.server")

// onMessageReceived handles raw BGP messages from peers.
// Forwards to processes based on API subscriptions.
// Returns the count of cache-consumer plugins that successfully received the event.
// Only plugins that declared cache-consumer: true during registration AND where
// delivery succeeded are counted. Non-cache-consumer plugins receive the event
// but are not counted (they don't participate in cache lifecycle tracking).
//
// Delivery is parallel via long-lived per-process goroutines (see rules/goroutine-lifecycle.md).
// Events are enqueued to each process's delivery channel; no per-event goroutines are created.
// Format encoding is pre-computed once per distinct format mode.
func onMessageReceived(s *plugin.Server, encoder *format.JSONEncoder, peer plugin.PeerInfo, msg bgptypes.RawMessage) int {
	if s.Context().Err() != nil {
		return 0 // Server shutting down, skip event delivery
	}

	eventType := messageTypeToEventType(msg.Type)
	if eventType == "" {
		logger().Debug("OnMessageReceived: unknown event type", "msgType", msg.Type)
		return 0
	}

	procs := s.Subscriptions().GetMatching(plugin.NamespaceBGP, eventType, msg.Direction, peer.Address.String())
	if len(procs) == 0 {
		return 0
	}
	logger().Debug("OnMessageReceived", "peer", peer.Address.String(), "event", eventType, "dir", msg.Direction, "count", len(procs))

	// Pre-format: encode once per distinct format mode.
	formatOutputs := make(map[string]string, 2)
	for _, proc := range procs {
		fmtMode := proc.Format()
		if _, ok := formatOutputs[fmtMode]; !ok {
			formatOutputs[fmtMode] = formatMessageForSubscription(encoder, peer, msg, fmtMode)
		}
	}

	// Enqueue to long-lived per-process delivery goroutines and collect results.
	results := make(chan plugin.EventResult, len(procs))
	sent := 0

	for _, proc := range procs {
		output := formatOutputs[proc.Format()]
		logger().Debug("OnMessageReceived writing", "proc", proc.Name(), "outputLen", len(output))

		if !proc.Deliver(plugin.EventDelivery{Output: output, Result: results}) {
			continue
		}
		sent++
	}

	// Collect results from all deliveries.
	var cacheCount int
	for range sent {
		r := <-results
		if r.Err != nil && s.Context().Err() == nil {
			logger().Error("OnMessageReceived write failed", "proc", r.ProcName, "err", r.Err)
		} else if r.CacheConsumer {
			cacheCount++
		}
	}

	return cacheCount
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

// deliverToProcs enqueues events to long-lived per-process delivery goroutines and
// waits for all deliveries to complete. Used by non-cache-consumer event functions.
func deliverToProcs(s *plugin.Server, procs []*plugin.Process, output, eventName string) {
	results := make(chan plugin.EventResult, len(procs))
	sent := 0

	for _, proc := range procs {
		logger().Debug(eventName+" writing", "proc", proc.Name())

		if !proc.Deliver(plugin.EventDelivery{Output: output, Result: results}) {
			continue
		}
		sent++
	}

	for range sent {
		r := <-results
		if r.Err != nil && s.Context().Err() == nil {
			logger().Warn(eventName+" write failed", "proc", r.ProcName, "err", r.Err)
		}
	}
}

// onPeerStateChange handles peer state transitions.
// Called by reactor when peer state changes (not a BGP message).
// Delivery is parallel via long-lived per-process goroutines (see rules/goroutine-lifecycle.md).
func onPeerStateChange(s *plugin.Server, peer plugin.PeerInfo, state string) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	procs := s.Subscriptions().GetMatching(plugin.NamespaceBGP, plugin.EventState, "", peer.Address.String())
	if len(procs) == 0 {
		return
	}
	logger().Debug("OnPeerStateChange", "peer", peer.Address.String(), "state", state, "count", len(procs))

	// Format once — state change output is identical for all plugins.
	output := format.FormatStateChange(peer, state, plugin.EncodingJSON)

	deliverToProcs(s, procs, output, "OnPeerStateChange")
}

// onPeerNegotiated handles capability negotiation completion.
// neg is format.DecodedNegotiated passed as any from the generic hook.
// Delivery is parallel via long-lived per-process goroutines (see rules/goroutine-lifecycle.md).
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

	// Format once — negotiated output is identical for all plugins.
	output := format.FormatNegotiated(peer, decoded, encoder)

	deliverToProcs(s, procs, output, "OnPeerNegotiated")
}

// onMessageSent handles BGP messages sent to peers.
// Forwards to processes that subscribed to sent events.
// Uses the JSONEncoder for non-UPDATE messages (same as onMessageReceived),
// and FormatSentMessage for UPDATEs (which adds the "type":"sent" marker).
//
// Delivery is parallel via long-lived per-process goroutines (see rules/goroutine-lifecycle.md).
// Format encoding is pre-computed once per distinct format mode.
func onMessageSent(s *plugin.Server, encoder *format.JSONEncoder, peer plugin.PeerInfo, msg bgptypes.RawMessage) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	eventType := messageTypeToEventType(msg.Type)
	if eventType == "" {
		return
	}

	procs := s.Subscriptions().GetMatching(plugin.NamespaceBGP, eventType, plugin.DirectionSent, peer.Address.String())
	if len(procs) == 0 {
		return
	}
	logger().Debug("OnMessageSent", "peer", peer.Address.String(), "type", eventType, "count", len(procs))

	// Pre-format: encode once per distinct format mode.
	formatOutputs := make(map[string]string, 2)
	for _, proc := range procs {
		fmtMode := proc.Format()
		if _, ok := formatOutputs[fmtMode]; !ok {
			if msg.Type == message.TypeUPDATE {
				content := bgptypes.ContentConfig{
					Encoding: plugin.EncodingJSON,
					Format:   fmtMode,
				}
				formatOutputs[fmtMode] = format.FormatSentMessage(peer, msg, content)
			} else {
				formatOutputs[fmtMode] = formatMessageForSubscription(encoder, peer, msg, fmtMode)
			}
		}
	}

	// Enqueue to long-lived per-process delivery goroutines and collect results.
	results := make(chan plugin.EventResult, len(procs))
	sent := 0

	for _, proc := range procs {
		output := formatOutputs[proc.Format()]
		logger().Debug("OnMessageSent writing", "proc", proc.Name())

		if !proc.Deliver(plugin.EventDelivery{Output: output, Result: results}) {
			continue
		}
		sent++
	}

	for range sent {
		r := <-results
		if r.Err != nil && s.Context().Err() == nil {
			logger().Warn("OnMessageSent write failed", "proc", r.ProcName, "err", r.Err)
		}
	}
}
