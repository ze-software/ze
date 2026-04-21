// Design: docs/architecture/core-design.md — BGP event delivery to plugins
// Overview: event_dispatcher.go — EventDispatcher type that calls these functions
//
// Package server provides BGP-specific event delivery for the plugin server.
// These functions implement BGP protocol event dispatch, keeping BGP protocol
// knowledge out of the generic plugin infrastructure.
package server

import (
	"encoding/json"
	"fmt"
	"sort"

	bgpevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// logger is the bgp.server subsystem logger.
var logger = slogutil.LazyLogger("bgp.server")

// eorEventBus holds the EventBus reference for EOR notifications.
// Set by EventDispatcher.SetEventBus(). Package-level to avoid threading
// through the onMessageReceived → onEORReceived call chain.
var eorEventBus ze.EventBus //nolint:gochecknoglobals // EventBus reference set once at startup

// monitorFormatKey is the format+encoding cache key for CLI monitors (always json+parsed).
const monitorFormatKey = "parsed+json"

// getStructuredEvent returns a StructuredEvent from the pool with peer fields populated.
func getStructuredEvent(peer *plugin.PeerInfo, msg *bgptypes.RawMessage) *rpc.StructuredEvent {
	se := rpc.GetStructuredEvent()
	se.PeerAddress = peer.Address.String()
	se.PeerName = peer.Name
	se.PeerGroup = peer.GroupName
	se.PeerAS = peer.PeerAS
	se.LocalAS = peer.LocalAS
	se.LocalAddress = peer.LocalAddress.String()
	se.EventType = messageTypeToEventKind(msg.Type)
	se.Direction = msg.Direction
	se.MessageID = msg.MessageID
	se.RawMessage = msg
	se.Meta = msg.Meta
	return se
}

// getStructuredStateEvent returns a StructuredEvent for a peer state change.
func getStructuredStateEvent(peer *plugin.PeerInfo, state rpc.SessionState, reason string) *rpc.StructuredEvent {
	se := rpc.GetStructuredEvent()
	se.PeerAddress = peer.Address.String()
	se.PeerName = peer.Name
	se.PeerGroup = peer.GroupName
	se.PeerAS = peer.PeerAS
	se.LocalAS = peer.LocalAS
	se.LocalAddress = peer.LocalAddress.String()
	se.EventType = rpc.EventKindState
	se.State = state
	se.Reason = reason
	return se
}

// formatCache is a stack-allocated cache for pre-formatted event outputs.
// Replaces make(map[string]string, 2) to avoid per-event map allocation.
// The key space is tiny: typically 1-2 distinct format+encoding combinations.
const formatCacheSlots = 4

type formatCache struct {
	keys   [formatCacheSlots]string
	values [formatCacheSlots]string
	n      int
}

// get returns the cached value for key, or empty string and false if not found.
func (c *formatCache) get(key string) (string, bool) {
	for i := range c.n {
		if c.keys[i] == key {
			return c.values[i], true
		}
	}
	return "", false
}

// set stores a key-value pair. If the cache is full, the entry is silently dropped
// (caller falls through to format on the spot -- correctness is preserved).
func (c *formatCache) set(key, value string) {
	if c.n < formatCacheSlots {
		c.keys[c.n] = key
		c.values[c.n] = value
		c.n++
	}
}

// reset clears the cache for reuse across batch iterations.
func (c *formatCache) reset() {
	for i := range c.n {
		c.keys[i] = ""
		c.values[i] = ""
	}
	c.n = 0
}

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
func onMessageReceived(s *pluginserver.Server, encoder *format.JSONEncoder, peer *plugin.PeerInfo, msg bgptypes.RawMessage) int {
	if s.Context().Err() != nil {
		return 0 // Server shutting down, skip event delivery
	}

	eventType := messageTypeToEventKind(msg.Type)
	if eventType == rpc.EventKindUnspecified {
		logger().Debug("OnMessageReceived: unknown event type", "msgType", msg.Type)
		return 0
	}

	peerAddr := peer.Address.String()
	eventTypeStr := eventType.String()
	procs := s.Subscriptions().GetMatching(bgpevents.Namespace, eventTypeStr, msg.Direction.String(), peerAddr, peer.Name)
	hasMonitors := s.Monitors().Count() > 0
	if len(procs) == 0 && !hasMonitors {
		return 0
	}
	logger().Debug("OnMessageReceived", "peer", peerAddr, "event", eventTypeStr, "dir", msg.Direction, "count", len(procs))

	// Pre-format text for text/JSON consumers per distinct format+encoding key.
	// DirectBridge structured consumers skip text formatting entirely.
	// Stack-allocated cache avoids per-event map allocation.
	var fmtCache formatCache
	for _, proc := range procs {
		if proc.HasStructuredHandler() {
			continue // DirectBridge — no text formatting needed
		}
		cacheKey := proc.FormatCacheKey()
		if _, ok := fmtCache.get(cacheKey); !ok {
			fmtCache.set(cacheKey, formatMessageForSubscription(encoder, peer, msg, proc.Format(), proc.Encoding()))
		}
	}

	// Enqueue to long-lived per-process delivery goroutines and collect results.
	results := make(chan process.EventResult, len(procs))
	sent := 0

	// Track pooled StructuredEvents for return after result collection.
	var pooled [4]*rpc.StructuredEvent
	pooledN := 0

	for _, proc := range procs {
		var delivery process.EventDelivery
		if proc.HasStructuredHandler() {
			se := getStructuredEvent(peer, &msg)
			delivery = process.EventDelivery{Event: se, Result: results}
			if pooledN < len(pooled) {
				pooled[pooledN] = se
				pooledN++
			}
		} else {
			output, _ := fmtCache.get(proc.FormatCacheKey())
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

	// Return pooled StructuredEvents after all consumers are done.
	for i := range pooledN {
		rpc.PutStructuredEvent(pooled[i])
	}

	// RFC 4724 Section 2: detect EOR markers in received UPDATEs.
	// EOR is delivered as a separate event so plugins can subscribe to "eor" independently.
	isUpdate := msg.Type == message.TypeUPDATE
	if isUpdate && msg.Direction == rpc.DirectionReceived && msg.WireUpdate != nil {
		if fam, ok := msg.WireUpdate.IsEOR(); ok {
			onEORReceived(s, peer, fam.String())
		}
	}

	// Deliver to CLI monitors. Reuse json+parsed from format cache if available.
	jsonOutput, ok := fmtCache.get(monitorFormatKey)
	if !ok {
		jsonOutput = formatMessageForSubscription(encoder, peer, msg, "parsed", "json")
	}
	monitorDeliver(s, eventTypeStr, msg.Direction.String(), peerAddr, peer.Name, jsonOutput)

	return cacheCount
}

// onMessageBatchReceived handles a batch of BGP messages from the same peer.
// Subscription lookup and format-mode map are computed once for the batch.
// Each message gets its own delivery and result collection, preserving per-message
// cacheCount semantics for cache lifecycle tracking.
// Returns a slice of cache-consumer counts, one per message.
func onMessageBatchReceived(s *pluginserver.Server, encoder *format.JSONEncoder, peer *plugin.PeerInfo, msgs []bgptypes.RawMessage) []int {
	counts := make([]int, len(msgs))
	if len(msgs) == 0 || s.Context().Err() != nil {
		return counts
	}

	// All messages share the same peer → same event type for subscription lookup.
	// Use the first message to determine event type (all are from same peer delivery goroutine).
	eventType := messageTypeToEventKind(msgs[0].Type)
	if eventType == rpc.EventKindUnspecified {
		return counts
	}

	peerAddr := peer.Address.String()
	eventTypeStr := eventType.String()
	procs := s.Subscriptions().GetMatching(bgpevents.Namespace, eventTypeStr, msgs[0].Direction.String(), peerAddr, peer.Name)
	hasMonitors := s.Monitors().Count() > 0
	if len(procs) == 0 && !hasMonitors {
		return counts
	}
	logger().Debug("OnMessageBatchReceived", "peer", peerAddr, "event", eventTypeStr, "dir", msgs[0].Direction, "procs", len(procs), "msgs", len(msgs))

	// Deliver each message: pre-format text for text/JSON consumers,
	// pass StructuredEvent directly for DirectBridge consumers.
	// Stack-allocated cache is reused across batch iterations.
	isUpdate := msgs[0].Type == message.TypeUPDATE
	var fmtCache formatCache
	results := make(chan process.EventResult, len(procs))

	for i := range msgs {
		msg := &msgs[i]

		// Pre-format text for text/JSON consumers per distinct format+encoding key.
		// DirectBridge structured consumers skip text formatting entirely.
		fmtCache.reset()
		for _, proc := range procs {
			if proc.HasStructuredHandler() {
				continue // DirectBridge — no text formatting needed
			}
			cacheKey := proc.FormatCacheKey()
			if _, ok := fmtCache.get(cacheKey); !ok {
				fmtCache.set(cacheKey, formatMessageForSubscription(encoder, peer, *msg, proc.Format(), proc.Encoding()))
			}
		}

		var pooled [4]*rpc.StructuredEvent
		pooledN := 0
		sent := 0
		for _, proc := range procs {
			var delivery process.EventDelivery
			if proc.HasStructuredHandler() {
				se := getStructuredEvent(peer, msg)
				delivery = process.EventDelivery{Event: se, Result: results}
				if pooledN < len(pooled) {
					pooled[pooledN] = se
					pooledN++
				}
			} else {
				output, _ := fmtCache.get(proc.FormatCacheKey())
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

		// Return pooled StructuredEvents after all consumers are done.
		for j := range pooledN {
			rpc.PutStructuredEvent(pooled[j])
		}

		// RFC 4724 Section 2: detect EOR markers in received UPDATEs.
		if isUpdate && msg.Direction == rpc.DirectionReceived && msg.WireUpdate != nil {
			if fam, ok := msg.WireUpdate.IsEOR(); ok {
				onEORReceived(s, peer, fam.String())
			}
		}

		// Deliver to CLI monitors per message.
		jsonOutput, ok := fmtCache.get(monitorFormatKey)
		if !ok {
			jsonOutput = formatMessageForSubscription(encoder, peer, *msg, "parsed", "json")
		}
		monitorDeliver(s, eventTypeStr, msg.Direction.String(), peerAddr, peer.Name, jsonOutput)
	}

	return counts
}

// messageTypeToEventKind converts BGP message type to typed EventKind.
// Returns EventKindUnspecified for unsupported types (caller checks for zero).
func messageTypeToEventKind(msgType message.MessageType) rpc.EventKind {
	switch msgType { //nolint:exhaustive // Only supported types; caller checks zero return
	case message.TypeUPDATE:
		return rpc.EventKindUpdate
	case message.TypeOPEN:
		return rpc.EventKindOpen
	case message.TypeNOTIFICATION:
		return rpc.EventKindNotification
	case message.TypeKEEPALIVE:
		return rpc.EventKindKeepalive
	case message.TypeROUTEREFRESH:
		return rpc.EventKindRefresh
	default:
		return rpc.EventKindUnspecified
	}
}

// formatMessageForSubscription formats a BGP message for subscription-based delivery.
// Uses the specified encoding and format (from process settings).
// For text encoding, non-UPDATE types use dedicated text formatters instead of JSONEncoder.
func formatMessageForSubscription(encoder *format.JSONEncoder, peer *plugin.PeerInfo, msg bgptypes.RawMessage, fmtMode, encoding string) string {
	// Stack-local scratch for the non-UPDATE Append path. Single string
	// conversion at each return is the named AC-9 boundary edge.
	// Size rationale: typical OPEN (~6 caps) / NOTIFICATION / KEEPALIVE /
	// ROUTE-REFRESH lines fit in 512B. Pathological inputs (many caps,
	// long data hex) spill to heap transparently via `append` growth.
	// See plan/learned/614-fmt-0-append.md invariant 4.
	var scratchArr [512]byte
	switch msg.Type { //nolint:exhaustive // Only supported types; unsupported are filtered by caller
	case message.TypeUPDATE:
		content := bgptypes.ContentConfig{
			Encoding: encoding,
			Format:   fmtMode,
		}
		var updateScratch [4096]byte
		return string(format.AppendMessage(updateScratch[:0], peer, msg, content, ""))

	case message.TypeOPEN:
		decoded := format.DecodeOpen(msg.RawBytes)
		if encoding == plugin.EncodingText {
			return string(format.AppendOpen(scratchArr[:0], peer, decoded, msg.Direction.String(), msg.MessageID))
		}
		return encoder.Open(peer, decoded, msg.Direction.String(), msg.MessageID)

	case message.TypeNOTIFICATION:
		decoded := format.DecodeNotification(msg.RawBytes)
		if encoding == plugin.EncodingText {
			return string(format.AppendNotification(scratchArr[:0], peer, decoded, msg.Direction.String(), msg.MessageID))
		}
		return encoder.Notification(peer, decoded, msg.Direction.String(), msg.MessageID)

	case message.TypeKEEPALIVE:
		if encoding == plugin.EncodingText {
			return string(format.AppendKeepalive(scratchArr[:0], peer, msg.Direction.String(), msg.MessageID))
		}
		return encoder.Keepalive(peer, msg.Direction.String(), msg.MessageID)

	case message.TypeROUTEREFRESH:
		decoded := format.DecodeRouteRefresh(msg.RawBytes)
		if encoding == plugin.EncodingText {
			return string(format.AppendRouteRefresh(scratchArr[:0], peer, decoded, msg.Direction.String(), msg.MessageID))
		}
		return encoder.RouteRefresh(peer, decoded, msg.Direction.String(), msg.MessageID)

	default: // Unsupported type — filtered by messageTypeToEventKind before reaching here
		return ""
	}
}

// monitorDeliver delivers a pre-formatted JSON event to matching CLI monitors.
// Called after plugin delivery in each event function. The output must be the
// json+parsed format string. This is a no-op if no monitors match.
// All events in this package are BGP namespace, so namespace is hardcoded.
func monitorDeliver(s *pluginserver.Server, eventType, direction, peerAddr, peerName, jsonOutput string) {
	s.Monitors().Deliver(bgpevents.Namespace, eventType, direction, peerAddr, peerName, jsonOutput)
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
func onPeerStateChange(s *pluginserver.Server, peer *plugin.PeerInfo, state rpc.SessionState, reason string) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	peerAddr := peer.Address.String()
	procs := s.Subscriptions().GetMatching(bgpevents.Namespace, bgpevents.EventState, "", peerAddr, peer.Name)
	hasMonitors := s.Monitors().Count() > 0
	if len(procs) == 0 && !hasMonitors {
		return
	}

	// Sort by reverse dependency tier: dependents first, dependencies last.
	sortByReverseDependencyTier(procs)

	logger().Debug("OnPeerStateChange", "peer", peerAddr, "state", state, "reason", reason, "count", len(procs))

	// Pre-format once per distinct encoding (text consumers only).
	// DirectBridge structured consumers get StructuredEvent with State/Reason fields.
	// Stack-local scratch; single string(scratch) conversion at the cache edge.
	// Size rationale: a typical state-change line (peer, ASN, state, reason,
	// optional group/local) fits well under 512B. Long peer/group names
	// spill transparently. See plan/learned/614-fmt-0-append.md.
	var fmtCache formatCache
	var scratchArr [512]byte
	for _, proc := range procs {
		if proc.HasStructuredHandler() {
			continue // DirectBridge — no text formatting needed
		}
		enc := proc.Encoding()
		if _, ok := fmtCache.get(enc); !ok {
			scratch := format.AppendStateChange(scratchArr[:0], peer, state, reason, enc)
			fmtCache.set(enc, string(scratch))
		}
	}

	// Deliver sequentially in dependency order — each process must complete
	// before the next starts, enabling inter-plugin coordination.
	// Structured consumers get StructuredEvent; text consumers get formatted text.
	var pooled [4]*rpc.StructuredEvent
	pooledN := 0
	for _, proc := range procs {
		var delivery process.EventDelivery
		results := make(chan process.EventResult, 1)
		if proc.HasStructuredHandler() {
			se := getStructuredStateEvent(peer, state, reason)
			delivery = process.EventDelivery{Event: se, Result: results}
			if pooledN < len(pooled) {
				pooled[pooledN] = se
				pooledN++
			}
		} else {
			output, _ := fmtCache.get(proc.Encoding())
			delivery = process.EventDelivery{Output: output, Result: results}
		}
		if !proc.Deliver(delivery) {
			continue
		}
		r := <-results
		if r.Err != nil && s.Context().Err() == nil {
			logger().Warn("OnPeerStateChange write failed", "proc", r.ProcName, "err", r.Err)
		}
	}

	// Return pooled StructuredEvents after all consumers are done.
	for i := range pooledN {
		rpc.PutStructuredEvent(pooled[i])
	}

	// Deliver to CLI monitors.
	jsonOutput, ok := fmtCache.get("json")
	if !ok {
		jsonOutput = string(format.AppendStateChange(scratchArr[:0], peer, state, reason, "json"))
	}
	monitorDeliver(s, bgpevents.EventState, "", peerAddr, peer.Name, jsonOutput)
}

// onPeerNegotiated handles capability negotiation completion.
// neg is format.DecodedNegotiated passed as any from the generic hook.
// Delivery is parallel via long-lived per-process goroutines (see rules/goroutine-lifecycle.md).
func onPeerNegotiated(s *pluginserver.Server, encoder *format.JSONEncoder, peer *plugin.PeerInfo, neg any) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	decoded, ok := neg.(format.DecodedNegotiated)
	if !ok {
		logger().Warn("OnPeerNegotiated: invalid neg type", "type", fmt.Sprintf("%T", neg))
		return
	}

	peerAddr := peer.Address.String()
	procs := s.Subscriptions().GetMatching(bgpevents.Namespace, bgpevents.EventNegotiated, "", peerAddr, peer.Name)
	hasMonitors := s.Monitors().Count() > 0
	if len(procs) == 0 && !hasMonitors {
		return
	}

	// Format once — negotiated output is identical for all plugins (always JSON).
	// FormatNegotiated was a one-line wrapper around encoder.Negotiated; rewire
	// directly now that the wrapper is deleted (spec-fmt-1-text-update AC-3).
	output := encoder.Negotiated(peer, decoded)

	deliverToProcs(s, procs, output, "OnPeerNegotiated")

	// Deliver to CLI monitors (negotiated is always JSON format).
	monitorDeliver(s, bgpevents.EventNegotiated, "", peerAddr, peer.Name, output)
}

// onEORReceived handles End-of-RIB marker detection.
// RFC 4724 Section 2: EOR signals completion of initial routing exchange for a family.
// Called when an incoming UPDATE is detected as an EOR marker.
// EOR events are delivered sequentially in reverse dependency order, like state events,
// to enable inter-plugin coordination (e.g., bgp-gr triggers stale route purge).
func onEORReceived(s *pluginserver.Server, peer *plugin.PeerInfo, family string) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	peerAddr := peer.Address.String()
	procs := s.Subscriptions().GetMatching(bgpevents.Namespace, bgpevents.EventEOR, events.DirectionReceived, peerAddr, peer.Name)
	hasMonitors := s.Monitors().Count() > 0
	if len(procs) == 0 && !hasMonitors {
		return
	}

	// Sort by reverse dependency tier: dependents first, dependencies last.
	sortByReverseDependencyTier(procs)

	logger().Debug("OnEORReceived", "peer", peerAddr, "family", family, "count", len(procs))

	// Pre-format once per distinct encoding. Stack-local scratch; single
	// string(scratch) conversion at the cache edge (AC-9 site).
	// Size rationale: EOR line is "peer X remote as N eor <family>" or the
	// equivalent JSON envelope; always under 256B.
	// See plan/learned/614-fmt-0-append.md.
	var fmtCache formatCache
	var scratchArr [256]byte
	for _, proc := range procs {
		enc := proc.Encoding()
		if _, ok := fmtCache.get(enc); !ok {
			scratch := format.AppendEOR(scratchArr[:0], peer, family, enc)
			fmtCache.set(enc, string(scratch))
		}
	}

	// Deliver sequentially in dependency order.
	for _, proc := range procs {
		output, _ := fmtCache.get(proc.Encoding())
		results := make(chan process.EventResult, 1)
		if !proc.Deliver(process.EventDelivery{Output: output, Result: results}) {
			continue
		}
		r := <-results
		if r.Err != nil && s.Context().Err() == nil {
			logger().Warn("OnEORReceived write failed", "proc", r.ProcName, "err", r.Err)
		}
	}

	// Deliver to CLI monitors.
	jsonOutput, ok := fmtCache.get("json")
	if !ok {
		jsonOutput = string(format.AppendEOR(scratchArr[:0], peer, family, "json"))
	}
	monitorDeliver(s, bgpevents.EventEOR, events.DirectionReceived, peerAddr, peer.Name, jsonOutput)

	// Cross-component consumers receive (bgp, eor) via the EventBus.
	if eorEventBus != nil {
		payload, err := json.Marshal(map[string]string{
			"peer":   peerAddr,
			"family": family,
		})
		if err == nil {
			if _, err := eorEventBus.Emit(bgpevents.Namespace, bgpevents.EventEOR, string(payload)); err != nil {
				logger().Debug("emit bgp eor failed", "peer", peerAddr, "family", family, "error", err)
			}
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
func onMessageSent(s *pluginserver.Server, encoder *format.JSONEncoder, peer *plugin.PeerInfo, msg bgptypes.RawMessage) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	eventType := messageTypeToEventKind(msg.Type)
	if eventType == rpc.EventKindUnspecified {
		return
	}

	peerAddr := peer.Address.String()
	eventTypeStr := eventType.String()
	procs := s.Subscriptions().GetMatching(bgpevents.Namespace, eventTypeStr, events.DirectionSent, peerAddr, peer.Name)
	hasMonitors := s.Monitors().Count() > 0
	if len(procs) == 0 && !hasMonitors {
		return
	}
	logger().Debug("OnMessageSent", "peer", peerAddr, "type", eventTypeStr, "count", len(procs))

	isUpdate := msg.Type == message.TypeUPDATE

	// Pre-format: encode once per distinct format+encoding combination.
	// DirectBridge structured consumers skip text formatting entirely.
	var fmtCache formatCache
	// Hoist the stack scratch above the loop: re-declaring inside the loop
	// pays a 4KB zero-init per iteration; one declaration + scratch[:0]
	// reset costs nothing after the first pass.
	var sentScratch [4096]byte
	for _, proc := range procs {
		if proc.HasStructuredHandler() {
			continue // DirectBridge — no text formatting needed
		}
		cacheKey := proc.FormatCacheKey()
		if _, ok := fmtCache.get(cacheKey); !ok {
			if isUpdate {
				content := bgptypes.ContentConfig{Encoding: proc.Encoding(), Format: proc.Format()}
				fmtCache.set(cacheKey, string(format.AppendSentMessage(sentScratch[:0], peer, msg, content)))
			} else {
				fmtCache.set(cacheKey, formatMessageForSubscription(encoder, peer, msg, proc.Format(), proc.Encoding()))
			}
		}
	}

	// Fire-and-forget delivery without result channels.
	// Unlike received events, sent event delivery MUST NOT block on results.
	// A plugin's event handler may send a route (e.g., RIB refresh replay),
	// which triggers onMessageSent, which delivers the "sent" event back to
	// the same plugin. Blocking here causes a deadlock: the delivery goroutine
	// is processing the original event and can't handle the re-entrant delivery.
	// No result channel = no pooled StructuredEvent return (GC handles cleanup).
	for _, proc := range procs {
		var delivery process.EventDelivery
		if proc.HasStructuredHandler() {
			se := getStructuredEvent(peer, &msg)
			delivery = process.EventDelivery{Event: se}
		} else {
			output, _ := fmtCache.get(proc.FormatCacheKey())
			delivery = process.EventDelivery{Output: output}
		}
		proc.Deliver(delivery)
	}

	// Deliver to CLI monitors. Reuse json+parsed from format cache if available.
	jsonOutput, ok := fmtCache.get(monitorFormatKey)
	if !ok {
		if isUpdate {
			content := bgptypes.ContentConfig{Encoding: "json", Format: "parsed"}
			var monScratch [4096]byte
			jsonOutput = string(format.AppendSentMessage(monScratch[:0], peer, msg, content))
		} else {
			jsonOutput = formatMessageForSubscription(encoder, peer, msg, "parsed", "json")
		}
	}
	monitorDeliver(s, eventTypeStr, events.DirectionSent, peerAddr, peer.Name, jsonOutput)
}

// onPeerCongestionChange handles forward-path congestion state transitions.
// eventType is bgpevents.EventCongested or bgpevents.EventResumed.
// Delivery is parallel via long-lived per-process goroutines (see rules/goroutine-lifecycle.md).
func onPeerCongestionChange(s *pluginserver.Server, peer *plugin.PeerInfo, eventType string) {
	if s.Context().Err() != nil {
		return // Server shutting down, skip event delivery
	}

	peerAddr := peer.Address.String()
	procs := s.Subscriptions().GetMatching(bgpevents.Namespace, eventType, "", peerAddr, peer.Name)
	hasMonitors := s.Monitors().Count() > 0
	if len(procs) == 0 && !hasMonitors {
		return
	}

	logger().Debug("OnPeerCongestionChange", "peer", peerAddr, "event", eventType, "count", len(procs))

	// Pre-format once per distinct encoding. Stack-local scratch; single
	// string(scratch) conversion at the cache edge (AC-9 site).
	// Size rationale: congestion line is "peer X remote as N <eventType>"
	// or the JSON envelope; always under 256B.
	// See plan/learned/614-fmt-0-append.md.
	var fmtCache formatCache
	var scratchArr [256]byte
	for _, proc := range procs {
		enc := proc.Encoding()
		if _, ok := fmtCache.get(enc); !ok {
			scratch := format.AppendCongestion(scratchArr[:0], peer, eventType, enc)
			fmtCache.set(enc, string(scratch))
		}
	}

	// Enqueue to long-lived per-process delivery goroutines and collect results.
	results := make(chan process.EventResult, len(procs))
	sent := 0

	for _, proc := range procs {
		output, _ := fmtCache.get(proc.Encoding())
		if !proc.Deliver(process.EventDelivery{Output: output, Result: results}) {
			continue
		}
		sent++
	}

	for range sent {
		r := <-results
		if r.Err != nil && s.Context().Err() == nil {
			logger().Warn("OnPeerCongestionChange write failed", "proc", r.ProcName, "err", r.Err)
		}
	}

	// Deliver to CLI monitors.
	jsonOutput, ok := fmtCache.get("json")
	if !ok {
		jsonOutput = string(format.AppendCongestion(scratchArr[:0], peer, eventType, "json"))
	}
	monitorDeliver(s, eventType, "", peerAddr, peer.Name, jsonOutput)
}
