// Design: docs/architecture/core-design.md — peer lifecycle events and message receiver dispatch
// Overview: reactor.go — BGP reactor event loop and peer management

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
)

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
		PeerAS:       peer.settings.PeerAS,
		LocalAS:      peer.settings.LocalAS,
	}

	decoded := format.NegotiatedToDecoded(neg)
	r.eventDispatcher.OnPeerNegotiated(peerInfo, decoded)
}

// notifyPeerClosed calls all observers when peer leaves Established.
func (r *Reactor) notifyPeerClosed(peer *Peer, reason string) {
	r.observersMu.RLock()
	observers := r.peerObservers
	r.observersMu.RUnlock()

	for _, obs := range observers {
		obs.OnPeerClosed(peer, reason)
	}

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

// notifyMessageReceiver notifies the message receiver of a raw BGP message.
// Called from session when a BGP message is sent or received.
// peerAddr is used to look up full PeerInfo from the peers map.
// wireUpdate is non-nil for received UPDATE messages (zero-copy path).
// ctxID is the encoding context for zero-copy decisions.
// direction is "sent" or "received".
// buf is the pool buffer for received messages (nil for sent).
// Returns true if buf ownership was taken (caller should not return to pool).
func (r *Reactor) notifyMessageReceiver(peerAddr netip.Addr, msgType message.MessageType, rawBytes []byte, wireUpdate *wireu.WireUpdate, ctxID bgpctx.ContextID, direction string, buf []byte) bool {
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
			LocalAS:      s.LocalAS,
			PeerAS:       s.PeerAS,
			RouterID:     s.RouterID,
			State:        peer.State().String(),
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

	// Cache BEFORE event delivery (only received UPDATEs).
	// Entry is inserted with pending=true so it exists when plugins receive the
	// message-id. After dispatch, Activate(id, N) sets the consumer count.
	// If a fast plugin calls "forward" before Activate(), Get() still works
	// (pending entries are accessible) and Decrement() adjusts the count
	// (negative is corrected when Activate adds N).
	if direction == "received" && wireUpdate != nil && buf != nil {
		r.recentUpdates.Add(&ReceivedUpdate{
			WireUpdate:   wireUpdate, // Zero-copy: slices into buf
			poolBuf:      buf,        // Cache owns buf
			SourcePeerIP: peerAddr,
			ReceivedAt:   timestamp,
		})
		kept = true // Cache always accepts
	}

	// Sent messages: synchronous delivery, no async channel.
	if direction == "sent" {
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
