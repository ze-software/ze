// Design: docs/architecture/core-design.md — BGP event delivery to plugins
// Overview: event_dispatcher.go — EventDispatcher type that calls these functions
//
// Package server provides BGP-specific event delivery for the plugin server.
// These functions implement BGP protocol event dispatch, keeping BGP protocol
// knowledge out of the generic plugin infrastructure.
package server

import (
	"fmt"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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
// For received UPDATEs, also checks for EOR markers (RFC 4724 Section 2) and
// fires a separate EventEOR to subscribers if detected.
//
// Delivery is parallel via long-lived per-process goroutines (see rules/goroutine-lifecycle.md).
// Events are enqueued to each process's delivery channel; no per-event goroutines are created.
// Format encoding is pre-computed once per distinct format mode.
func onMessageReceived(s *pluginserver.Server, encoder *format.JSONEncoder, peer plugin.PeerInfo, msg bgptypes.RawMessage) int {
	if s.Context().Err() != nil {
		return 0 // Server shutting down, skip event delivery
	}

	eventType := messageTypeToEventType(msg.Type)
	if eventType == "" {
		logger().Debug("OnMessageReceived: unknown event type", "msgType", msg.Type)
		return 0
	}

	peerAddr := peer.Address.String()
	procs := s.Subscriptions().GetMatching(plugin.NamespaceBGP, eventType, msg.Direction, peerAddr)
	if len(procs) == 0 {
		return 0
	}
	logger().Debug("OnMessageReceived", "peer", peerAddr, "event", eventType, "dir", msg.Direction, "count", len(procs))

	// Check if this is an UPDATE (only UPDATEs use DirectBridge structured delivery).
	isUpdate := msg.Type == message.TypeUPDATE

	// Pre-format text for text/JSON consumers per distinct format+encoding key.
	// DirectBridge consumers skip text formatting entirely.
	formatOutputs := make(map[string]string, 2)
	for _, proc := range procs {
		if isUpdate && proc.HasStructuredHandler() {
			continue // DirectBridge — no text formatting needed
		}
		cacheKey := proc.Format() + "+" + proc.Encoding()
		if _, ok := formatOutputs[cacheKey]; !ok {
			formatOutputs[cacheKey] = formatMessageForSubscription(encoder, peer, msg, proc.Format(), proc.Encoding())
		}
	}

	// Enqueue to long-lived per-process delivery goroutines and collect results.
	results := make(chan process.EventResult, len(procs))
	sent := 0

	for _, proc := range procs {
		var delivery process.EventDelivery
		if isUpdate && proc.HasStructuredHandler() {
			delivery = process.EventDelivery{Event: &rpc.StructuredUpdate{PeerAddress: peerAddr, Event: &msg}, Result: results}
		} else {
			output := formatOutputs[proc.Format()+"+"+proc.Encoding()]
			delivery = process.EventDelivery{Output: output, Result: results}
		}
		if !proc.Deliver(delivery) {
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

	// RFC 4724 Section 2: detect EOR markers in received UPDATEs.
	// EOR is delivered as a separate event so plugins can subscribe to "eor" independently.
	if isUpdate && msg.Direction == plugin.DirectionReceived && msg.WireUpdate != nil {
		if family, ok := msg.WireUpdate.IsEOR(); ok {
			onEORReceived(s, peer, family.String())
		}
	}

	return cacheCount
}

// onMessageBatchReceived handles a batch of BGP messages from the same peer.
// Subscription lookup and format-mode map are computed once for the batch.
// Each message gets its own delivery and result collection, preserving per-message
// cacheCount semantics for cache lifecycle tracking.
// Returns a slice of cache-consumer counts, one per message.
func onMessageBatchReceived(s *pluginserver.Server, encoder *format.JSONEncoder, peer plugin.PeerInfo, msgs []bgptypes.RawMessage) []int {
	counts := make([]int, len(msgs))
	if len(msgs) == 0 || s.Context().Err() != nil {
		return counts
	}

	// All messages share the same peer → same event type for subscription lookup.
	// Use the first message to determine event type (all are from same peer delivery goroutine).
	eventType := messageTypeToEventType(msgs[0].Type)
	if eventType == "" {
		return counts
	}

	procs := s.Subscriptions().GetMatching(plugin.NamespaceBGP, eventType, msgs[0].Direction, peer.Address.String())
	if len(procs) == 0 {
		return counts
	}
	logger().Debug("OnMessageBatchReceived", "peer", peer.Address.String(), "event", eventType, "dir", msgs[0].Direction, "procs", len(procs), "msgs", len(msgs))

	// Check if this is an UPDATE batch (only UPDATEs use DirectBridge structured delivery).
	isUpdate := msgs[0].Type == message.TypeUPDATE
	peerAddr := peer.Address.String()

	// Deliver each message: pre-format text for text/JSON consumers,
	// pass RawMessage directly for DirectBridge consumers.
	for i := range msgs {
		msg := &msgs[i]

		// Pre-format text for text/JSON consumers per distinct format+encoding key.
		// DirectBridge consumers skip text formatting entirely.
		formatOutputs := make(map[string]string, 2)
		for _, proc := range procs {
			if isUpdate && proc.HasStructuredHandler() {
				continue // DirectBridge — no text formatting needed
			}
			cacheKey := proc.Format() + "+" + proc.Encoding()
			if _, ok := formatOutputs[cacheKey]; !ok {
				formatOutputs[cacheKey] = formatMessageForSubscription(encoder, peer, *msg, proc.Format(), proc.Encoding())
			}
		}

		results := make(chan process.EventResult, len(procs))
		sent := 0
		for _, proc := range procs {
			var delivery process.EventDelivery
			if isUpdate && proc.HasStructuredHandler() {
				delivery = process.EventDelivery{Event: &rpc.StructuredUpdate{PeerAddress: peerAddr, Event: msg}, Result: results}
			} else {
				output := formatOutputs[proc.Format()+"+"+proc.Encoding()]
				delivery = process.EventDelivery{Output: output, Result: results}
			}
			if !proc.Deliver(delivery) {
				continue
			}
			sent++
		}

		var cacheCount int
		for range sent {
			r := <-results
			if r.Err != nil && s.Context().Err() == nil {
				logger().Error("OnMessageBatchReceived write failed", "proc", r.ProcName, "err", r.Err)
			} else if r.CacheConsumer {
				cacheCount++
			}
		}
		counts[i] = cacheCount

		// RFC 4724 Section 2: detect EOR markers in received UPDATEs.
		if isUpdate && msg.Direction == plugin.DirectionReceived && msg.WireUpdate != nil {
			if family, ok := msg.WireUpdate.IsEOR(); ok {
				onEORReceived(s, peer, family.String())
			}
		}
	}

	return counts
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
// Uses the specified encoding and format (from process settings).
// For text encoding, non-UPDATE types use dedicated text formatters instead of JSONEncoder.
func formatMessageForSubscription(encoder *format.JSONEncoder, peer plugin.PeerInfo, msg bgptypes.RawMessage, fmtMode, encoding string) string {
	switch msg.Type { //nolint:exhaustive // Only supported types; unsupported are filtered by caller
	case message.TypeUPDATE:
		content := bgptypes.ContentConfig{
			Encoding: encoding,
			Format:   fmtMode,
		}
		return format.FormatMessage(peer, msg, content, "")

	case message.TypeOPEN:
		decoded := format.DecodeOpen(msg.RawBytes)
		if encoding == plugin.EncodingText {
			return format.FormatOpen(peer, decoded, msg.Direction, msg.MessageID)
		}
		return encoder.Open(peer, decoded, msg.Direction, msg.MessageID)

	case message.TypeNOTIFICATION:
		decoded := format.DecodeNotification(msg.RawBytes)
		if encoding == plugin.EncodingText {
			return format.FormatNotification(peer, decoded, msg.Direction, msg.MessageID)
		}
		return encoder.Notification(peer, decoded, msg.Direction, msg.MessageID)

	case message.TypeKEEPALIVE:
		if encoding == plugin.EncodingText {
			return format.FormatKeepalive(peer, msg.Direction, msg.MessageID)
		}
		return encoder.Keepalive(peer, msg.Direction, msg.MessageID)

	case message.TypeROUTEREFRESH:
		decoded := format.DecodeRouteRefresh(msg.RawBytes)
		if encoding == plugin.EncodingText {
			return format.FormatRouteRefresh(peer, decoded, msg.Direction, msg.MessageID)
		}
		return encoder.RouteRefresh(peer, decoded, msg.Direction, msg.MessageID)

	default: // Unsupported type — filtered by messageTypeToEventType before reaching here
		return ""
	}
}

// deliverToProcs enqueues events to long-lived per-process delivery goroutines and
// waits for all deliveries to complete. Used by non-cache-consumer event functions.
func deliverToProcs(s *pluginserver.Server, procs []*process.Process, output, eventName string) {
	results := make(chan process.EventResult, len(procs))
	sent := 0

	for _, proc := range procs {
		if !proc.Deliver(process.EventDelivery{Output: output, Result: results}) {
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

// sortByReverseDependencyTier sorts processes so that plugins with MORE dependencies
// (higher topological tier) come first. This ensures that dependent plugins (e.g., bgp-gr)
// process events before the plugins they depend on (e.g., bgp-rib).
//
// Used for state and EOR events where ordering matters for inter-plugin coordination
// (e.g., bgp-gr must send retain-routes before bgp-rib processes peer-down).
//
// If TopologicalTiers fails (cycle or unknown plugins), falls back to name-based sort
// for deterministic but arbitrary ordering.
func sortByReverseDependencyTier(procs []*process.Process) {
	if len(procs) <= 1 {
		return
	}

	// Collect process names for tier computation.
	names := make([]string, len(procs))
	for i, p := range procs {
		names[i] = p.Name()
	}

	tiers, err := registry.TopologicalTiers(names)
	if err != nil {
		// Fallback: sort by name for deterministic ordering.
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].Name() < procs[j].Name()
		})
		return
	}

	// Build name → tier index map.
	tierOf := make(map[string]int, len(names))
	for tierIdx, tier := range tiers {
		for _, name := range tier {
			tierOf[name] = tierIdx
		}
	}

	// Sort: higher tier first (reverse topological order).
	sort.Slice(procs, func(i, j int) bool {
		ti, tj := tierOf[procs[i].Name()], tierOf[procs[j].Name()]
		if ti != tj {
			return ti > tj // Higher tier first
		}
		return procs[i].Name() < procs[j].Name() // Deterministic within tier
	})
}

// onPeerStateChange handles peer state transitions.
// Called by reactor when peer state changes (not a BGP message).
// reason is the close reason (empty for "up"): "tcp-failure", "notification", etc.
// State events are delivered sequentially in reverse dependency order so that
// plugins with dependencies (e.g., bgp-gr) process before their dependencies (e.g., bgp-rib).
func onPeerStateChange(s *pluginserver.Server, peer plugin.PeerInfo, state, reason string) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	procs := s.Subscriptions().GetMatching(plugin.NamespaceBGP, plugin.EventState, "", peer.Address.String())
	if len(procs) == 0 {
		return
	}

	// Sort by reverse dependency tier: dependents first, dependencies last.
	sortByReverseDependencyTier(procs)

	logger().Debug("OnPeerStateChange", "peer", peer.Address.String(), "state", state, "reason", reason, "count", len(procs))

	// Pre-format once per distinct encoding.
	formatOutputs := make(map[string]string, 2)
	for _, proc := range procs {
		enc := proc.Encoding()
		if _, ok := formatOutputs[enc]; !ok {
			formatOutputs[enc] = format.FormatStateChange(peer, state, reason, enc)
		}
	}

	// Deliver sequentially in dependency order — each process must complete
	// before the next starts, enabling inter-plugin coordination.
	for _, proc := range procs {
		output := formatOutputs[proc.Encoding()]
		results := make(chan process.EventResult, 1)
		if !proc.Deliver(process.EventDelivery{Output: output, Result: results}) {
			continue
		}
		r := <-results
		if r.Err != nil && s.Context().Err() == nil {
			logger().Warn("OnPeerStateChange write failed", "proc", r.ProcName, "err", r.Err)
		}
	}
}

// onPeerNegotiated handles capability negotiation completion.
// neg is format.DecodedNegotiated passed as any from the generic hook.
// Delivery is parallel via long-lived per-process goroutines (see rules/goroutine-lifecycle.md).
func onPeerNegotiated(s *pluginserver.Server, encoder *format.JSONEncoder, peer plugin.PeerInfo, neg any) {
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

// onEORReceived handles End-of-RIB marker detection.
// RFC 4724 Section 2: EOR signals completion of initial routing exchange for a family.
// Called when an incoming UPDATE is detected as an EOR marker.
// EOR events are delivered sequentially in reverse dependency order, like state events,
// to enable inter-plugin coordination (e.g., bgp-gr triggers stale route purge).
func onEORReceived(s *pluginserver.Server, peer plugin.PeerInfo, family string) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	procs := s.Subscriptions().GetMatching(plugin.NamespaceBGP, plugin.EventEOR, plugin.DirectionReceived, peer.Address.String())
	if len(procs) == 0 {
		return
	}

	// Sort by reverse dependency tier: dependents first, dependencies last.
	sortByReverseDependencyTier(procs)

	logger().Debug("OnEORReceived", "peer", peer.Address.String(), "family", family, "count", len(procs))

	// Pre-format once per distinct encoding.
	formatOutputs := make(map[string]string, 2)
	for _, proc := range procs {
		enc := proc.Encoding()
		if _, ok := formatOutputs[enc]; !ok {
			formatOutputs[enc] = format.FormatEOR(peer, family, enc)
		}
	}

	// Deliver sequentially in dependency order.
	for _, proc := range procs {
		output := formatOutputs[proc.Encoding()]
		results := make(chan process.EventResult, 1)
		if !proc.Deliver(process.EventDelivery{Output: output, Result: results}) {
			continue
		}
		r := <-results
		if r.Err != nil && s.Context().Err() == nil {
			logger().Warn("OnEORReceived write failed", "proc", r.ProcName, "err", r.Err)
		}
	}
}

// onMessageSent handles BGP messages sent to peers.
// Forwards to processes that subscribed to sent events.
// Uses the JSONEncoder for non-UPDATE messages (same as onMessageReceived),
// and FormatSentMessage for UPDATEs (which adds the "type":"sent" marker).
//
// Delivery is parallel via long-lived per-process goroutines (see rules/goroutine-lifecycle.md).
// Format encoding is pre-computed once per distinct format mode.
func onMessageSent(s *pluginserver.Server, encoder *format.JSONEncoder, peer plugin.PeerInfo, msg bgptypes.RawMessage) {
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

	// Check if this is an UPDATE (only UPDATEs use DirectBridge structured delivery).
	isUpdate := msg.Type == message.TypeUPDATE
	peerAddr := peer.Address.String()

	// Pre-format: encode once per distinct format+encoding combination.
	// DirectBridge consumers skip text formatting entirely.
	formatOutputs := make(map[string]string, 2)
	for _, proc := range procs {
		if isUpdate && proc.HasStructuredHandler() {
			continue // DirectBridge — no text formatting needed
		}
		cacheKey := proc.Format() + "+" + proc.Encoding()
		if _, ok := formatOutputs[cacheKey]; !ok {
			if isUpdate {
				content := bgptypes.ContentConfig{Encoding: proc.Encoding(), Format: proc.Format()}
				formatOutputs[cacheKey] = format.FormatSentMessage(peer, msg, content)
			} else {
				formatOutputs[cacheKey] = formatMessageForSubscription(encoder, peer, msg, proc.Format(), proc.Encoding())
			}
		}
	}

	// Enqueue to long-lived per-process delivery goroutines and collect results.
	results := make(chan process.EventResult, len(procs))
	sent := 0

	for _, proc := range procs {
		var delivery process.EventDelivery
		if isUpdate && proc.HasStructuredHandler() {
			delivery = process.EventDelivery{Event: &rpc.StructuredUpdate{PeerAddress: peerAddr, Event: &msg}, Result: results}
		} else {
			output := formatOutputs[proc.Format()+"+"+proc.Encoding()]
			delivery = process.EventDelivery{Output: output, Result: results}
		}
		if !proc.Deliver(delivery) {
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
