// Design: docs/architecture/core-design.md — peer lifecycle events and message receiver dispatch
// Overview: reactor.go — BGP reactor event loop and peer management
// Related: received_update.go — ReceivedUpdate created on inbound UPDATE
// Related: forward_build.go — progressive build for egress attribute modification
// Related: update_group.go — update group Add/Remove on peer lifecycle

package reactor

import (
	"maps"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// safeIngressFilter calls an ingress filter with panic recovery.
// Fail-closed: a panicking filter rejects the route (drops the UPDATE).
func safeIngressFilter(filter filterapi.IngressFilterFunc, src filterapi.PeerFilterInfo, payload []byte, meta map[string]any) (accept bool, modified []byte) {
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
//
// panicked reports that the suppression came from the recover, not from the
// filter returning false. Both drop the route; only the second is a policy
// decision, and a caller that counts outcomes must tell them apart (see
// egressStepResult.failed).
func safeEgressFilter(filter filterapi.EgressFilterFunc, src, dest filterapi.PeerFilterInfo, payload []byte, meta map[string]any, mods *filterapi.ModAccumulator) (accept, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			fwdLogger().Error("egress filter panic, suppressing route", "src", src.Address, "dest", dest.Address, "panic", r)
			accept = false // fail-closed: suppress route on filter panic
			panicked = true
		}
	}()
	return filter(src, dest, payload, meta, mods), false
}

// addPeerObserver registers an observer for peer lifecycle events.
// Observers are called synchronously in registration order.
// MUST NOT block; use goroutine for slow processing.
func (r *Reactor) addPeerObserver(obs peerLifecycleObserver) {
	r.observersMu.Lock()
	defer r.observersMu.Unlock()
	r.peerObservers = append(r.peerObservers, obs)
}

// addMessageObserver registers an observer for raw BGP messages.
// Observers are called synchronously on the session goroutine.
// MUST NOT block; buffer internally if I/O is needed.
func (r *Reactor) addMessageObserver(obs MessageObserver) {
	r.observersMu.Lock()
	defer r.observersMu.Unlock()
	r.msgObservers = append(r.msgObservers, obs)
}

// messageCallbackAdapter wraps a registry.MessageCallback as a MessageObserver.
type messageCallbackAdapter struct {
	cb registry.MessageCallback
}

func (a *messageCallbackAdapter) OnBGPMessage(peer *plugin.PeerInfo, msgType msgtype.MessageType, sent bool, rawBytes []byte) {
	a.cb.OnBGPMessage(peer, uint8(msgType), sent, rawBytes)
}

// AddMessageCallback registers an external callback via the any-typed interface.
func (r *Reactor) AddMessageCallback(cb registry.MessageCallback) {
	r.addMessageObserver(&messageCallbackAdapter{cb: cb})
}

// callbackAdapter wraps a registry.PeerLifecycleCallback as a peerLifecycleObserver.
type callbackAdapter struct {
	cb registry.PeerLifecycleCallback
}

func (a *callbackAdapter) OnPeerEstablished(peer *Peer) { a.cb.OnPeerEstablished(peer) }
func (a *callbackAdapter) OnPeerClosed(peer *Peer, reason string) {
	a.cb.OnPeerClosed(peer, reason)
}

// AddPeerLifecycleCallback registers an external callback via the any-typed interface.
func (r *Reactor) AddPeerLifecycleCallback(cb registry.PeerLifecycleCallback) {
	r.addPeerObserver(&callbackAdapter{cb: cb})
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
		Address:         peer.settings.Address,
		LocalAddress:    peer.settings.LocalAddress,
		AddressStr:      peer.addrString,
		LocalAddressStr: peer.localAddrString,
		Name:            peer.settings.Name,
		GroupName:       peer.settings.GroupName,
		PeerAS:          peer.settings.PeerAS,
		LocalAS:         peer.settings.LocalAS,
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

	// Schedule cleanup for dynamic peers after idle timeout.
	if peer.Settings().IsDynamic {
		r.scheduleDynamicPeerCleanup(peer)
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
	addrStr := peer.addrString
	peerInfo := plugin.PeerInfo{
		Address:         s.Address,
		LocalAddress:    s.LocalAddress,
		AddressStr:      addrStr,
		LocalAddressStr: peer.localAddrString,
		Name:            s.Name,
		GroupName:       s.GroupName,
		LocalAS:         s.LocalAS,
		// PeerAS via the guarded accessor: this runs on the congestion controller
		// goroutine, which can race a dynamic peer's establishment write of PeerAS.
		PeerAS:   peer.PeerAS(),
		RouterID: s.RouterID,
		State:    peer.State().PluginState(),
	}
	r.mu.RUnlock()

	if r.eventDispatcher == nil {
		return
	}
	r.eventDispatcher.OnPeerCongestionChange(&peerInfo, eventType)

	// Cross-component consumers receive (bgp, congested) or (bgp, resumed) via the EventBus.
	// eventType is bgpevents.EventCongested or bgpevents.EventResumed -- pass through directly.
	r.emitCongestionEventBus(addrStr, eventType)
}

// notifyMessageReceiver notifies the message receiver of a raw BGP message.
// Called from session when a BGP message is sent or received.
// peerAddr is used to look up full PeerInfo from the peers map.
// wireUpdate is non-nil for received UPDATE messages (zero-copy path).
// ctxID is the encoding context for zero-copy decisions.
// direction is rpc.DirectionSent or rpc.DirectionReceived.
// buf is the pool buffer for received messages (nil for sent).
// Returns true if buf ownership was taken (caller should not return to pool).
func (r *Reactor) notifyMessageReceiver(peerAddr netip.Addr, msgType msgtype.MessageType, rawBytes []byte, wireUpdate *wireu.WireUpdate, ctxID bgpctx.ContextID, direction rpc.MessageDirection, buf BufHandle, meta map[string]any, sentSourcePeerStr string) bool {
	r.mu.RLock()
	receiver := r.messageReceiver
	peer, hasPeer := r.findPeerByAddr(peerAddr)

	// Build PeerInfo while holding lock to avoid race on state
	var peerInfo plugin.PeerInfo
	if hasPeer {
		s := peer.Settings()
		peerInfo = plugin.PeerInfo{
			Address:         s.Address,
			LocalAddress:    s.LocalAddress,
			AddressStr:      peer.addrString,
			LocalAddressStr: peer.localAddrString,
			Name:            s.Name,
			GroupName:       s.GroupName,
			LocalAS:         s.LocalAS,
			// PeerAS via the guarded accessor: the sent direction runs on forward-pool /
			// timer / plugin-RPC goroutines, which can race a dynamic peer's establishment
			// write of PeerAS. (On the received direction this is the peer's own read
			// goroutine, where the accessor is merely redundant, not wrong.)
			PeerAS:   peer.PeerAS(),
			RouterID: s.RouterID,
			State:    peer.State().PluginState(),
		}
		// Increment per-peer counters (lock-free atomics).
		// Engine counts updates, keepalives, and EOR. NLRI-level counters
		// (announce vs withdraw per prefix) belong in the RIB plugin.
		if direction == rpc.DirectionReceived {
			switch msgType { //nolint:exhaustive // only counting updates and keepalives
			case msgtype.TypeUPDATE:
				peer.IncrUpdatesReceived()
				// Additionally count EOR as a subset of updates.
				if wireUpdate != nil {
					if _, isEOR := wireUpdate.IsEOR(); isEOR {
						peer.IncrEORReceived()
						// Cancel EOR timeout warning (AC-11).
						if peer.health != nil {
							peer.health.onEORReceived()
						}
						// Notify weight tracker: may transition pre-EOR to
						// post-EOR when all family EORs received, shrinking
						// pool allocation (AC-28).
						if r.fwdWeights != nil {
							r.fwdWeights.PeerEORReceived(peer.peerAddrLabel())
						}
					}
				}
			case msgtype.TypeKEEPALIVE:
				peer.IncrKeepalivesReceived()
			}
		} else {
			switch msgType { //nolint:exhaustive // only counting updates and keepalives
			case msgtype.TypeUPDATE:
				peer.IncrUpdatesSent()
				// EOR sent is counted at BuildEOR call sites via IncrEORSent()
				// because wireUpdate is nil for sent messages.
			case msgtype.TypeKEEPALIVE:
				peer.IncrKeepalivesSent()
			}
		}
	} else {
		peerInfo = plugin.PeerInfo{Address: peerAddr, AddressStr: peerAddr.String()}
	}

	if r.capture != nil {
		var errCode, errSub uint8
		if msgType == msgtype.TypeNOTIFICATION && len(rawBytes) >= 2 {
			errCode = rawBytes[0]
			errSub = rawBytes[1]
		}
		r.capture.Append(direction == rpc.DirectionSent, peerAddr, msgType, len(rawBytes), errCode, errSub)
	}
	if rc := r.rawCapture.Load(); rc != nil {
		var dir uint8
		if direction == rpc.DirectionSent {
			dir = 1
		}
		rc.Append(dir, rawBytes)
	}

	r.observersMu.RLock()
	msgObs := r.msgObservers
	r.observersMu.RUnlock()

	r.mu.RUnlock()

	isSent := direction == rpc.DirectionSent
	for _, obs := range msgObs {
		obs.OnBGPMessage(&peerInfo, msgType, isSent, rawBytes)
	}

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

		// Tag config-static routes so the RIB plugin skips ribOut storage.
		// The sendingConfigStatic flag is set by sendInitialRoutes during
		// static route sending and cleared before opQueue drain.
		sentMeta := meta
		if direction == rpc.DirectionSent && hasPeer && peer.sendingConfigStatic.Load() {
			if sentMeta == nil {
				sentMeta = map[string]any{"config-static": true}
			} else {
				merged := make(map[string]any, len(sentMeta)+1)
				maps.Copy(merged, sentMeta)
				merged["config-static"] = true
				sentMeta = merged
			}
		}

		// Source peer for ribOut stale-scoping on sent forward-pool writes. The value
		// is captured at the sender's write site inside its writeMu critical section
		// (session_write.go) and arrives as the sentSourcePeerStr argument. This
		// replaces an unlocked double-read of peer.session here, which raced the peer
		// run goroutine that nils/replaces peer.session under peer.mu (non-nil-then-nil
		// panic, or reading a reconnected session's writeMu-guarded field without its
		// writeMu). "" for received and non-forward sends.
		msg = bgptypes.RawMessage{
			Type:          msgType,
			RawBytes:      bytes,
			Timestamp:     timestamp,
			Direction:     direction,
			MessageID:     messageID,
			Meta:          sentMeta,
			SourcePeerStr: sentSourcePeerStr,
		}

		// For sent UPDATE messages, create WireUpdate + AttrsWire from body.
		// WireUpdate is needed by structured handlers (e.g., RIB plugin's
		// handleSentStructured) to extract NLRIs via wu.NLRI()/MPReach().
		// AttrsWire is needed to extract path attributes for ribOut storage.
		if msgType == msgtype.TypeUPDATE && len(bytes) >= 4 {
			wu := wireu.NewWireUpdate(bytes, ctxID)
			wu.SetMessageID(messageID)
			msg.WireUpdate = wu
			if aw, parseErr := wu.Attrs(); parseErr == nil {
				msg.AttrsWire = aw
			}
		}
	}

	// Unified ingress filter pass: ONE stage-ordered pipeline over the in-process
	// filters (loop, community, redistribute, OTC) and the reactor-bound per-peer
	// policy chain (FilterStagePeerChain, which sorts LAST -- after OTC). This
	// replaces the former two back-to-back blocks; the cross-system order is now a
	// declared Stage, not code position. Only for received UPDATEs.
	var routeMeta map[string]any
	if direction == rpc.DirectionReceived && wireUpdate != nil && len(r.orderedIngressSteps) > 0 {
		src := filterapi.PeerFilterInfo{
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
		// AC-5 verified-benign unlocked peer.session read: this whole block runs only on
		// the received path (direction == DirectionReceived, guarded above), which
		// executes on the peer's session read goroutine — the SAME goroutine that writes
		// p.session under p.mu in runOnce (peer_run.go). Program order gives a
		// happens-before edge, so no lock is needed here and none is added (unlike the
		// sent path, whose callbacks run on other goroutines). Negotiated() itself takes
		// s.mu.RLock for the field it reads.
		if hasPeer && peer.session != nil {
			if neg := peer.session.Negotiated(); neg != nil {
				src.ASN4 = neg.ASN4
			}
		}
		payload := wireUpdate.Payload()
		var ingressMeta map[string]any
		for i := range r.orderedIngressSteps {
			step := &r.orderedIngressSteps[i]
			var res ingressStepResult
			if step.policyChain {
				// The external per-peer chain runs last, on the (possibly
				// in-process-modified) payload. No-op accept when the peer has no
				// import filters or no API server (hot-path gate inside).
				res = r.runIngressPolicyChain(peer, peerAddr, peerInfo.PeerAS, wireUpdate, payload)
			} else {
				if ingressMeta == nil {
					ingressMeta = make(map[string]any, 2)
				}
				accept, modifiedPayload := safeIngressFilter(step.inproc, src, payload, ingressMeta)
				res = ingressStepResult{accept: accept, modifiedPayload: modifiedPayload}
			}
			// Honor a policy teardown request (e.g. filter_family tear-down):
			// queue a NOTIFICATION + session close for the session read loop to
			// run after this callback (session_read.go), and drop the route.
			// AC-5 verified-benign unlocked peer.session read: peer.session is the
			// session whose read goroutine invoked us — the same goroutine that writes
			// p.session under p.mu (peer_run.go), so program order gives happens-before
			// and no lock is needed (received path only).
			if res.teardown {
				if hasPeer && peer.session != nil {
					code := message.NotifyErrorCode(res.notifyCode)
					subcode := res.notifySubcode
					if res.notifyCode == 0 {
						code = message.NotifyCease
						subcode = message.NotifyCeaseConnectionRejected
					}
					peer.session.requestPolicyTeardown(code, subcode)
				}
				return false
			}
			if !res.accept {
				return false // Route rejected by filter; don't cache or dispatch.
			}
			if res.modifiedPayload != nil {
				payload = res.modifiedPayload
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
		if len(ingressMeta) > 0 {
			routeMeta = ingressMeta
		}
	}

	// Source peer identity for ribOut stale-scoping.
	// Stored as typed field on ReceivedUpdate (SourcePeerStr) instead of
	// allocating a map per UPDATE. The forward path threads it into
	// fwdItem.sourcePeerStr for sent event callbacks.
	var sourcePeerStr string
	if direction == rpc.DirectionReceived {
		if hasPeer {
			sourcePeerStr = peer.addrString
		} else {
			sourcePeerStr = peerAddr.String()
		}
	}

	// Cache BEFORE event delivery (only received UPDATEs).
	// Entry is inserted with pending=true so it exists when plugins receive the
	// message-id. After dispatch, Activate(id, N) sets the consumer count.
	// If a fast plugin calls "forward" before Activate(), Get() still works
	// (pending entries are accessible) and Decrement() adjusts the count
	// (negative is corrected when Activate adds N).
	if direction == rpc.DirectionReceived && wireUpdate != nil && buf.Buf != nil {
		ru := &ReceivedUpdate{
			poolBuf:       buf, // Cache owns buf
			SourcePeerIP:  peerAddr,
			SourcePeerStr: sourcePeerStr,
			ReceivedAt:    timestamp,
			Meta:          routeMeta,
		}
		// Initialize WireUpdate inline to co-locate it within the ReceivedUpdate
		// allocation (one fewer heap object per UPDATE). Fresh init from the same
		// payload; lazy parse re-fires on first consumer access (~35ns, cheaper
		// than the saved allocation).
		wireu.InitWireUpdate(&ru.wireUpdateInline, wireUpdate.Payload(), wireUpdate.SourceCtxID())
		ru.wireUpdateInline.SetMessageID(wireUpdate.MessageID())
		ru.wireUpdateInline.SetSourceID(wireUpdate.SourceID())
		ru.WireUpdate = &ru.wireUpdateInline
		r.recentUpdates.Add(ru)
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
		r.emitUpdateNotificationEvent(addrStr, direction.String())
	}

	// Sent messages: synchronous delivery, no async channel.
	if direction == rpc.DirectionSent {
		receiver.OnMessageSent(&peerInfo, msg)
		return kept
	}

	// Reactor RS fast path: forward UPDATE directly from the session read
	// goroutine, bypassing the delivery goroutine and plugin dispatch chain.
	// Runs after cache Add (buffer lifetime via cache entry) and before
	// deliverChan enqueue (plugins still get fire-and-forget delivery).
	// Activate is NOT called here -- the delivery goroutine calls it after
	// OnMessageBatchReceived returns the cache consumer count.
	//
	// r.rsForwardingEnabled is defense-in-depth. rs-fast-path/rs-client are
	// plugin-owned YANG leaves, so with the rs plugin absent they are rejected at
	// config load and peer.settings.RSFastPath can never become true. The cached
	// capability bool (set once in New from the filterapi seam the rs plugin
	// activates) makes the fast path inert even if that field were somehow set
	// without the plugin, keeping the "delete the plugin, RS forwarding vanishes"
	// invariant enforced at this gate and not only by the schema. The other
	// RSFastPath/RSClient readers (session_negotiate PATHS-LIMIT suppression,
	// peer_forward_facts AS-path-skip) stay schema-gated only, which suffices
	// because they are unreachable without the plugin-owned config.
	if kept && hasPeer && r.rsForwardingEnabled && peer.settings.RSFastPath && msgType == msgtype.TypeUPDATE {
		// ReactorForwarded is a claim of delivery, and bgp-rs believes it: with
		// the flag set and no FastPathSkipped it takes `default: releaseCache`
		// (rs/server_withdrawal.go) and forwards to nobody. So set it ONLY when
		// this rail actually dispatched to someone, and say something on every
		// path that declines -- a guard that neither denies nor speaks does not
		// exist (ai/rules/fail-closed-guards.md). Both branches below used to be
		// silent, which is why a rail switch was invisible in the logs.
		update, ok := r.recentUpdates.Get(messageID)
		switch {
		case !ok:
			// The entry was Add()ed a few lines above, so a miss means it was
			// evicted under concurrent load. bgp-rs still delivers via
			// ForwardCached -> forwardUpdateCore, which applies the same egress
			// policy, so this is a performance fallback and not a delivery gap.
			fwdLogger().Warn("rs fast path declined: update evicted from cache before forwarding",
				"id", messageID, "peer", peer.addrString)
		default:
			skipped, dispatched := reactorForwardRS(r, update, messageID, peerAddr, peer)
			if dispatched == 0 && len(skipped) == 0 {
				// Matched no destination at all. Leaving the flag clear hands the
				// UPDATE to bgp-rs's own target selection, which releases the
				// cache itself when it likewise finds no target. Claiming
				// "forwarded" here is what turned "no eligible peer yet" into a
				// silently dropped UPDATE.
				fwdLogger().Debug("rs fast path matched no destination; deferring to the rs plugin",
					"id", messageID, "peer", peer.addrString)
				break
			}
			msg.ReactorForwarded = true
			if len(skipped) > 0 {
				msg.FastPathSkipped = skipped
			}
		}
	}

	// Received UPDATE with per-peer delivery channel: enqueue for async delivery.
	// The delivery goroutine (started by Peer.runOnce) drains a batch and calls
	// OnMessageBatchReceived + Activate per message. This decouples the TCP read
	// goroutine from plugin processing.
	// Non-UPDATE messages (OPEN, KEEPALIVE, NOTIFICATION) stay synchronous
	// because they are infrequent and FSM-critical.
	if hasPeer && peer.deliverChan != nil && msgType == msgtype.TypeUPDATE {
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
