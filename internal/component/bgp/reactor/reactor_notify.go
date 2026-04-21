// Design: docs/architecture/core-design.md — peer lifecycle events and message receiver dispatch
// Overview: reactor.go — BGP reactor event loop and peer management
// Related: received_update.go — ReceivedUpdate created on inbound UPDATE
// Related: forward_build.go — progressive build for egress attribute modification
// Related: update_group.go — update group Add/Remove on peer lifecycle

package reactor

import (
	"maps"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// safeIngressFilter calls an ingress filter with panic recovery.
// Fail-closed: a panicking filter rejects the route (drops the UPDATE).
func safeIngressFilter(filter registry.IngressFilterFunc, src registry.PeerFilterInfo, payload []byte, meta map[string]any) (accept bool, modified []byte) {
	defer func() {
		if r := recover(); r != nil {
			sessionLogger().Error("ingress filter panic, rejecting route", "peer", src.Address, "panic", r)
			accept = false // fail-closed: reject route on filter panic
			modified = nil
		}
	}()
	return filter(src, payload, meta)
}

// safeEgressFilter calls an egress filter with panic recovery.
// Fail-closed: a panicking filter suppresses the route for this peer.
func safeEgressFilter(filter registry.EgressFilterFunc, src, dest registry.PeerFilterInfo, payload []byte, meta map[string]any, mods *registry.ModAccumulator) (accept bool) {
	defer func() {
		if r := recover(); r != nil {
			fwdLogger().Error("egress filter panic, suppressing route", "src", src.Address, "dest", dest.Address, "panic", r)
			accept = false // fail-closed: suppress route on filter panic
		}
	}()
	return filter(src, dest, payload, meta, mods)
}

// AddPeerObserver registers an observer for peer lifecycle events.
// Observers are called synchronously in registration order.
// MUST NOT block; use goroutine for slow processing.
func (r *Reactor) AddPeerObserver(obs PeerLifecycleObserver) {
	r.observersMu.Lock()
	defer r.observersMu.Unlock()
	r.peerObservers = append(r.peerObservers, obs)
}

// notifyPeerEstablished calls all observers when peer reaches Established.
func (r *Reactor) notifyPeerEstablished(peer *Peer) {
	// Update weight tracker with actual negotiated family count (AC-28).
	// Config-declared familyCount may differ from negotiated families.
	// RFC 8654: update ExtMsg flag and per-peer pool buffer size.
	if nc := peer.negotiated.Load(); nc != nil {
		if r.fwdWeights != nil {
			r.fwdWeights.UpdateFamilyCount(peer.peerAddrLabel(), len(nc.Families()))
			r.fwdWeights.UpdateExtMsg(peer.peerAddrLabel(), nc.ExtendedMessage)
		}
		if r.fwdPool != nil && nc.ExtendedMessage {
			r.fwdPool.RegisterOutgoingPool(fwdKey{peerAddr: peer.Settings().PeerKey()}, message.ExtMsgLen)
		}
	}

	// Register peer in update group index by sendCtxID.
	if r.updateGroups != nil {
		r.updateGroups.Add(peer)
	}

	r.observersMu.RLock()
	observers := r.peerObservers
	r.observersMu.RUnlock()

	for _, obs := range observers {
		obs.OnPeerEstablished(peer)
	}
}

// notifyPeerNegotiated sends negotiated capabilities to subscribed plugins.
// Called after OPEN exchange completes and peer reaches Established.
func (r *Reactor) notifyPeerNegotiated(peer *Peer, neg *capability.Negotiated) {
	if r.eventDispatcher == nil || neg == nil {
		return
	}

	peerInfo := plugin.PeerInfo{
		Address:      peer.settings.Address,
		LocalAddress: peer.settings.LocalAddress,
		Name:         peer.settings.Name,
		GroupName:    peer.settings.GroupName,
		PeerAS:       peer.settings.PeerAS,
		LocalAS:      peer.settings.LocalAS,
	}

	decoded := format.NegotiatedToDecoded(neg)
	r.eventDispatcher.OnPeerNegotiated(&peerInfo, decoded)
}

// notifyPeerClosed calls all observers when peer leaves Established.
func (r *Reactor) notifyPeerClosed(peer *Peer, reason string) {
	// Remove peer from update group index before notifying observers.
	// Must happen before clearEncodingContexts resets sendCtxID to 0.
	if r.updateGroups != nil {
		r.updateGroups.Remove(peer)
	}

	r.observersMu.RLock()
	observers := r.peerObservers
	r.observersMu.RUnlock()

	for _, obs := range observers {
		obs.OnPeerClosed(peer, reason)
	}
}

// emitCongestionEvent emits a congestion state change event to subscribed plugins.
// Called from fwdPool congestion callbacks. peerAddr is the destination peer address.
// eventType is bgpevents.EventCongested or bgpevents.EventResumed.
// Safe to call before the eventDispatcher is initialized (nil check after peer lookup).
//
// Looks up peer before checking eventDispatcher, so that missing peers are
// caught independently of dispatcher state.
func (r *Reactor) emitCongestionEvent(peerAddr netip.Addr, eventType string) {
	r.mu.RLock()
	peer, ok := r.findPeerByAddr(peerAddr)
	if !ok {
		r.mu.RUnlock()
		return
	}
	s := peer.Settings()
	peerInfo := plugin.PeerInfo{
		Address:      s.Address,
		LocalAddress: s.LocalAddress,
		Name:         s.Name,
		GroupName:    s.GroupName,
		LocalAS:      s.LocalAS,
		PeerAS:       s.PeerAS,
		RouterID:     s.RouterID,
		State:        peer.State().String(),
	}
	r.mu.RUnlock()

	if r.eventDispatcher == nil {
		return
	}
	r.eventDispatcher.OnPeerCongestionChange(&peerInfo, eventType)

	// Cross-component consumers receive (bgp, congested) or (bgp, resumed) via the EventBus.
	// eventType is bgpevents.EventCongested or bgpevents.EventResumed -- pass through directly.
	r.emitCongestionEventBus(peerAddr.String(), eventType)
}

// notifyMessageReceiver notifies the message receiver of a raw BGP message.
// Called from session when a BGP message is sent or received.
// peerAddr is used to look up full PeerInfo from the peers map.
// wireUpdate is non-nil for received UPDATE messages (zero-copy path).
// ctxID is the encoding context for zero-copy decisions.
// direction is "sent" or "received".
// buf is the pool buffer for received messages (nil for sent).
// Returns true if buf ownership was taken (caller should not return to pool).
func (r *Reactor) notifyMessageReceiver(peerAddr netip.Addr, msgType message.MessageType, rawBytes []byte, wireUpdate *wireu.WireUpdate, ctxID bgpctx.ContextID, direction string, buf BufHandle, meta map[string]any) bool {
	var typedDirection rpc.MessageDirection
	switch direction {
	case events.DirectionSent:
		typedDirection = rpc.DirectionSent
	case events.DirectionReceived:
		typedDirection = rpc.DirectionReceived
	}
	r.mu.RLock()
	receiver := r.messageReceiver
	peer, hasPeer := r.findPeerByAddr(peerAddr)

	// Build PeerInfo while holding lock to avoid race on state
	var peerInfo plugin.PeerInfo
	if hasPeer {
		s := peer.Settings()
		peerInfo = plugin.PeerInfo{
			Address:      s.Address,
			LocalAddress: s.LocalAddress,
			Name:         s.Name,
			GroupName:    s.GroupName,
			LocalAS:      s.LocalAS,
			PeerAS:       s.PeerAS,
			RouterID:     s.RouterID,
			State:        peer.State().String(),
		}
		// Increment per-peer counters (lock-free atomics).
		// Engine counts updates, keepalives, and EOR. NLRI-level counters
		// (announce vs withdraw per prefix) belong in the RIB plugin.
		if direction == events.DirectionReceived {
			switch msgType { //nolint:exhaustive // only counting updates and keepalives
			case message.TypeUPDATE:
				peer.IncrUpdatesReceived()
				// Additionally count EOR as a subset of updates.
				if wireUpdate != nil {
					if _, isEOR := wireUpdate.IsEOR(); isEOR {
						peer.IncrEORReceived()
						// Notify weight tracker: may transition pre-EOR to
						// post-EOR when all family EORs received, shrinking
						// pool allocation (AC-28).
						if r.fwdWeights != nil {
							r.fwdWeights.PeerEORReceived(peer.peerAddrLabel())
						}
					}
				}
			case message.TypeKEEPALIVE:
				peer.IncrKeepalivesReceived()
			}
		} else {
			switch msgType { //nolint:exhaustive // only counting updates and keepalives
			case message.TypeUPDATE:
				peer.IncrUpdatesSent()
				// EOR sent is counted at BuildEOR call sites via IncrEORSent()
				// because wireUpdate is nil for sent messages.
			case message.TypeKEEPALIVE:
				peer.IncrKeepalivesSent()
			}
		}
	} else {
		peerInfo = plugin.PeerInfo{Address: peerAddr}
	}
	r.mu.RUnlock()

	if receiver == nil {
		return false
	}

	// Assign message ID for all message types
	messageID := nextMsgID()
	timestamp := r.clock.Now()

	var msg bgptypes.RawMessage
	var kept bool

	// Zero-copy path for received UPDATE messages
	if wireUpdate != nil {
		// Set messageID on WireUpdate (single source of truth for UPDATEs)
		wireUpdate.SetMessageID(messageID)

		// Derive AttrsWire for observation callback
		// Errors logged but not fatal - handleUpdate() validates separately
		attrsWire, parseErr := wireUpdate.Attrs()
		if parseErr != nil {
			sessionLogger().Debug("WireUpdate.Attrs error", "peer", peerAddr, "error", parseErr)
		}

		// RawMessage uses zero-copy for synchronous callback processing
		msg = bgptypes.RawMessage{
			Type:       msgType,
			RawBytes:   wireUpdate.Payload(), // Zero-copy: valid during callback
			Timestamp:  timestamp,
			Direction:  typedDirection,
			MessageID:  messageID,
			WireUpdate: wireUpdate,
			AttrsWire:  attrsWire, // Derived from WireUpdate
			ParseError: parseErr,  // Propagate parse error to plugins
		}
	} else {
		// Non-UPDATE or sent messages: copy bytes for async processing safety
		bytes := make([]byte, len(rawBytes))
		copy(bytes, rawBytes)

		// Tag config-static routes so the RIB plugin skips ribOut storage.
		// The sendingConfigStatic flag is set by sendInitialRoutes during
		// static route sending and cleared before opQueue drain.
		sentMeta := meta
		if direction == events.DirectionSent && hasPeer && peer.sendingConfigStatic.Load() {
			if sentMeta == nil {
				sentMeta = map[string]any{"config-static": true}
			} else {
				merged := make(map[string]any, len(sentMeta)+1)
				maps.Copy(merged, sentMeta)
				merged["config-static"] = true
				sentMeta = merged
			}
		}

		msg = bgptypes.RawMessage{
			Type:      msgType,
			RawBytes:  bytes,
			Timestamp: timestamp,
			Direction: typedDirection,
			MessageID: messageID,
			Meta:      sentMeta,
		}

		// For sent UPDATE messages, create WireUpdate + AttrsWire from body.
		// WireUpdate is needed by structured handlers (e.g., RIB plugin's
		// handleSentStructured) to extract NLRIs via wu.NLRI()/MPReach().
		// AttrsWire is needed to extract path attributes for ribOut storage.
		if msgType == message.TypeUPDATE && len(bytes) >= 4 {
			wu := wireu.NewWireUpdate(bytes, ctxID)
			wu.SetMessageID(messageID)
			msg.WireUpdate = wu
			if aw, parseErr := wu.Attrs(); parseErr == nil {
				msg.AttrsWire = aw
			}
		}
	}

	// Ingress peer filter chain: reject routes before caching/dispatching.
	// Only for received UPDATEs. Filter closures check peer role, OTC, etc.
	var routeMeta map[string]any
	if direction == events.DirectionReceived && wireUpdate != nil && len(r.ingressFilters) > 0 {
		src := registry.PeerFilterInfo{
			Address:  peerAddr,
			PeerAS:   peerInfo.PeerAS,
			LocalAS:  peerInfo.LocalAS,
			RouterID: peerInfo.RouterID,
		}
		if hasPeer {
			src.Name = peer.settings.Name
			src.GroupName = peer.settings.GroupName
			src.AllowOwnAS = peer.settings.LoopAllowOwnAS
			src.ClusterID = peer.settings.LoopClusterID
			src.LoopDisabled = peer.settings.LoopDisabled
		}
		// ASN4 from negotiated capabilities (peer may have disconnected).
		if hasPeer && peer.session != nil {
			if neg := peer.session.Negotiated(); neg != nil {
				src.ASN4 = neg.ASN4
			}
		}
		payload := wireUpdate.Payload()
		ingressMeta := make(map[string]any, 2) // Non-nil: filters may write to it.
		for _, filter := range r.ingressFilters {
			accept, modifiedPayload := safeIngressFilter(filter, src, payload, ingressMeta)
			if !accept {
				return false // Route rejected by ingress filter; don't cache or dispatch.
			}
			if modifiedPayload != nil {
				payload = modifiedPayload
				// Create new WireUpdate from modified payload.
				// The modified buffer is heap-allocated (not from pool).
				wireUpdate = wireu.NewWireUpdate(payload, wireUpdate.SourceCtxID())
				wireUpdate.SetMessageID(messageID)
				// Update RawMessage to use modified WireUpdate.
				attrsWire, parseErr := wireUpdate.Attrs()
				if parseErr != nil {
					sessionLogger().Debug("modified WireUpdate.Attrs error", "peer", peerAddr, "error", parseErr)
				}
				msg.RawBytes = payload
				msg.WireUpdate = wireUpdate
				msg.AttrsWire = attrsWire
				msg.ParseError = parseErr
			}
		}
		// Only store metadata on ReceivedUpdate if any filter wrote to it.
		if len(ingressMeta) > 0 {
			routeMeta = ingressMeta
		}
	}

	// Policy filter chain: external plugin filters (after in-process filters).
	// Only for received UPDATEs when the peer has import filters configured.
	if direction == events.DirectionReceived && wireUpdate != nil && hasPeer {
		if filters := peer.settings.ImportFilters; len(filters) > 0 && r.api != nil {
			attrsWire, _ := wireUpdate.Attrs()
			// Stack-local scratch for zero-alloc AppendUpdateForFilter path.
			// One `string(scratch)` conversion at the IPC boundary below.
			// Size rationale: 4096B covers typical UPDATEs (12 attrs,
			// 50-500B each) including extreme community / large-community
			// lists up to ~2-4KB. Pathological inputs (200+ large-communities)
			// spill to heap via `append` growth -- correct but not zero alloc.
			// See plan/learned/614-fmt-0-append.md invariant 4.
			var scratchArr [4096]byte
			scratch := AppendUpdateForFilter(scratchArr[:0], attrsWire, wireUpdate, nil)
			updateText := string(scratch)
			action, modifiedText := PolicyFilterChain(filters, "import", peerAddr.String(), peerInfo.PeerAS,
				updateText, r.policyFilterFunc(wireUpdate.Payload()),
			)
			if action == PolicyReject {
				return false // Route rejected by policy filter; don't cache or dispatch.
			}
			// Wire-level dirty tracking: convert text delta to wire attribute
			// modifications. Same pattern as in-process ingress filter
			// modification above (lines 337-352). Additionally, when the
			// filter chain produced a per-prefix modify (subset of the
			// legacy IPv4 NLRI section), pass the re-encoded prefix bytes
			// to buildModifiedPayload so step 8 of the progressive build
			// writes the filtered NLRI tail instead of copying the original.
			if modifiedText != updateText {
				var importMods registry.ModAccumulator
				textDeltaToModOps(updateText, modifiedText, &importMods)
				ExtractASPathPrependOps(modifiedText, peer.settings.LocalAS, &importMods)
				nlriOverride := extractLegacyNLRIOverride(updateText, modifiedText)
				if importMods.Len() > 0 || nlriOverride != nil {
					if modPayload, _ := buildModifiedPayload(wireUpdate.Payload(), &importMods, r.attrModHandlers, nil, nlriOverride); modPayload != nil {
						wireUpdate = wireu.NewWireUpdate(modPayload, wireUpdate.SourceCtxID())
						wireUpdate.SetMessageID(messageID)
						newAttrsWire, parseErr := wireUpdate.Attrs()
						msg.RawBytes = modPayload
						msg.WireUpdate = wireUpdate
						msg.AttrsWire = newAttrsWire
						msg.ParseError = parseErr
					}
				}
			}
		}
	}

	// Cache BEFORE event delivery (only received UPDATEs).
	// Entry is inserted with pending=true so it exists when plugins receive the
	// message-id. After dispatch, Activate(id, N) sets the consumer count.
	// If a fast plugin calls "forward" before Activate(), Get() still works
	// (pending entries are accessible) and Decrement() adjusts the count
	// (negative is corrected when Activate adds N).
	if direction == events.DirectionReceived && wireUpdate != nil && buf.Buf != nil {
		r.recentUpdates.Add(&ReceivedUpdate{
			WireUpdate:   wireUpdate, // Zero-copy: slices into buf
			poolBuf:      buf,        // Cache owns buf
			SourcePeerIP: peerAddr,
			ReceivedAt:   timestamp,
			Meta:         routeMeta,
		})
		kept = true // Cache always accepts
	}

	// Bus notification for cross-component consumers.
	// Skip map allocation entirely when no bus is configured.
	// (bgp, update) lightweight notification on the EventBus. Cross-component
	// consumers that just want to know an UPDATE arrived (without the wire
	// payload) subscribe here. Plugins that need the full UPDATE go through
	// the EventDispatcher delivery path instead.
	if r.eventBus != nil {
		// Use cached addrString when available to avoid per-message String() allocation.
		addrStr := peerAddr.String()
		if hasPeer {
			addrStr = peer.addrString
		}
		r.emitUpdateNotificationEvent(addrStr, direction)
	}

	// Sent messages: synchronous delivery, no async channel.
	if direction == events.DirectionSent {
		receiver.OnMessageSent(&peerInfo, msg)
		return kept
	}

	// Received UPDATE with per-peer delivery channel: enqueue for async delivery.
	// The delivery goroutine (started by Peer.runOnce) drains a batch and calls
	// OnMessageBatchReceived + Activate per message. This decouples the TCP read
	// goroutine from plugin processing.
	// Non-UPDATE messages (OPEN, KEEPALIVE, NOTIFICATION) stay synchronous
	// because they are infrequent and FSM-critical.
	if hasPeer && peer.deliverChan != nil && msgType == message.TypeUPDATE {
		peer.deliverChan <- deliveryItem{peerInfo: peerInfo, msg: msg}
		return kept
	}

	// Synchronous fallback: no delivery channel or non-UPDATE message.
	consumerCount := receiver.OnMessageReceived(&peerInfo, msg)
	if kept {
		r.recentUpdates.Activate(messageID, consumerCount)
	}

	return kept
}
