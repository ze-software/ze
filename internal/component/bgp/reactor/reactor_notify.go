// Design: docs/architecture/core-design.md — peer lifecycle events and message receiver dispatch
// Overview: reactor.go — BGP reactor event loop and peer management
// Related: received_update.go — ReceivedUpdate created on inbound UPDATE

package reactor

import (
	"encoding/binary"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
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
	r.observersMu.RLock()
	observers := r.peerObservers
	r.observersMu.RUnlock()

	for _, obs := range observers {
		obs.OnPeerEstablished(peer)
	}

	// Bus notification for cross-component consumers.
	r.publishBusNotification("bgp/state", map[string]string{
		"peer":  peer.settings.Address.String(),
		"state": "up",
	})
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
	r.eventDispatcher.OnPeerNegotiated(peerInfo, decoded)

	// Bus notification for cross-component consumers.
	r.publishBusNotification("bgp/negotiated", map[string]string{
		"peer": peer.settings.Address.String(),
	})
}

// notifyPeerClosed calls all observers when peer leaves Established.
func (r *Reactor) notifyPeerClosed(peer *Peer, reason string) {
	r.observersMu.RLock()
	observers := r.peerObservers
	r.observersMu.RUnlock()

	for _, obs := range observers {
		obs.OnPeerClosed(peer, reason)
	}

	// Bus notification for cross-component consumers.
	r.publishBusNotification("bgp/state", map[string]string{
		"peer":   peer.settings.Address.String(),
		"state":  "down",
		"reason": reason,
	})

	// Track session count for MaxSessions feature (tcp.once/tcp.attempts)
	if r.config.MaxSessions > 0 {
		r.sessionCountMu.Lock()
		r.sessionCount++
		count := r.sessionCount
		r.sessionCountMu.Unlock()

		if int(count) >= r.config.MaxSessions {
			// MaxSessions reached - trigger shutdown
			go r.Stop()
		}
	}
}

// emitCongestionEvent emits a congestion state change event to subscribed plugins.
// Called from fwdPool congestion callbacks. peerAddr is the string form of the
// destination peer address. eventType is plugin.EventCongested or plugin.EventResumed.
// Safe to call before the eventDispatcher is initialized (nil check after peer lookup).
//
// Validates address and looks up peer before checking eventDispatcher, so that
// invalid addresses and missing peers are caught independently of dispatcher state.
func (r *Reactor) emitCongestionEvent(peerAddr, eventType string) {
	addr, err := netip.ParseAddr(peerAddr)
	if err != nil {
		return
	}

	r.mu.RLock()
	peer, ok := r.findPeerByAddr(addr)
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
	r.eventDispatcher.OnPeerCongestionChange(peerInfo, eventType)

	// Bus notification for cross-component consumers.
	r.publishBusNotification("bgp/congestion", map[string]string{
		"peer":  peerAddr,
		"event": eventType,
	})
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
		if direction == plugin.DirectionReceived {
			switch msgType { //nolint:exhaustive // only counting updates and keepalives
			case message.TypeUPDATE:
				peer.IncrUpdatesReceived()
				// Additionally count EOR as a subset of updates.
				if wireUpdate != nil {
					if _, isEOR := wireUpdate.IsEOR(); isEOR {
						peer.IncrEORReceived()
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
			Direction:  direction,
			MessageID:  messageID,
			WireUpdate: wireUpdate,
			AttrsWire:  attrsWire, // Derived from WireUpdate
			ParseError: parseErr,  // Propagate parse error to plugins
		}
	} else {
		// Non-UPDATE or sent messages: copy bytes for async processing safety
		bytes := make([]byte, len(rawBytes))
		copy(bytes, rawBytes)

		msg = bgptypes.RawMessage{
			Type:      msgType,
			RawBytes:  bytes,
			Timestamp: timestamp,
			Direction: direction,
			MessageID: messageID,
			Meta:      meta, // Route metadata from ReceivedUpdate (sent events).
		}

		// For sent UPDATE messages, create AttrsWire from body if we have a context ID
		if msgType == message.TypeUPDATE && ctxID != 0 && len(bytes) >= 4 {
			// Parse UPDATE body to extract attribute bytes
			// RFC 4271: withdrawnLen(2) + withdrawn(...) + attrLen(2) + attrs(...) + nlri(...)
			withdrawnLen := int(binary.BigEndian.Uint16(bytes[0:2]))
			attrOffset := 2 + withdrawnLen
			if len(bytes) >= attrOffset+2 {
				attrLen := int(binary.BigEndian.Uint16(bytes[attrOffset : attrOffset+2]))
				if len(bytes) >= attrOffset+2+attrLen {
					attrBytes := bytes[attrOffset+2 : attrOffset+2+attrLen]
					msg.AttrsWire = attribute.NewAttributesWire(attrBytes, ctxID)
				}
			}
		}
	}

	// Ingress peer filter chain: reject routes before caching/dispatching.
	// Only for received UPDATEs. Filter closures check peer role, OTC, etc.
	var routeMeta map[string]any
	if direction == plugin.DirectionReceived && wireUpdate != nil && len(r.ingressFilters) > 0 {
		src := registry.PeerFilterInfo{Address: peerAddr, PeerAS: peerInfo.PeerAS}
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

	// Cache BEFORE event delivery (only received UPDATEs).
	// Entry is inserted with pending=true so it exists when plugins receive the
	// message-id. After dispatch, Activate(id, N) sets the consumer count.
	// If a fast plugin calls "forward" before Activate(), Get() still works
	// (pending entries are accessible) and Decrement() adjusts the count
	// (negative is corrected when Activate adds N).
	if direction == plugin.DirectionReceived && wireUpdate != nil && buf.Buf != nil {
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
	r.publishBusNotification("bgp/update", map[string]string{
		"peer":      peerAddr.String(),
		"direction": direction,
	})

	// Sent messages: synchronous delivery, no async channel.
	if direction == plugin.DirectionSent {
		receiver.OnMessageSent(peerInfo, msg)
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
	consumerCount := receiver.OnMessageReceived(peerInfo, msg)
	if kept {
		r.recentUpdates.Activate(messageID, consumerCount)
	}

	return kept
}
