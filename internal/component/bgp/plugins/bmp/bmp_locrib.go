// RFC: rfc/short/rfc9069.md
// Design: rfc/short/rfc9069.md -- BMP Loc-RIB monitoring (PeerType=3)
//
// Related: bmp.go -- plugin lifecycle, sender fan-out, OPEN cache
// Related: sender.go -- writeRouteMonitoring / writePeerUp / writePeerDown
// Related: header.go -- PeerHeader, PeerTypeLocRIB
//
// RFC 9069 extends BMP (RFC 7854) with Loc-RIB monitoring: the BGP RIB's
// best paths (post best-path selection) are streamed to collectors as Route
// Monitoring messages carrying a PeerType=3 per-peer header. bmp is an
// in-process BGP plugin, so it subscribes to the same EventBus the RIB
// publishes best-change events on (mirrors redistribute_egress), reconstructs
// a minimal UPDATE PDU from the typed best-change entry, and fans it out to
// every configured collector.

package bmp

import (
	"encoding/binary"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/ze"
)

// eventBusPtr holds the in-process EventBus, installed by the plugin's
// registration (register.go ConfigureEventBus). bmp subscribes to the RIB's
// best-change events on it for Loc-RIB monitoring. Package-level (mirrors
// redistribute_egress) because registration runs before any BMPPlugin exists.
var eventBusPtr atomic.Pointer[ze.EventBus]

// setEventBus installs the EventBus (called from register.go ConfigureEventBus).
func setEventBus(eb ze.EventBus) { eventBusPtr.Store(&eb) }

// getEventBus returns the installed EventBus, or nil when none was configured.
func getEventBus() ze.EventBus {
	if p := eventBusPtr.Load(); p != nil {
		return *p
	}
	return nil
}

// locRIBPeerHeader builds the RFC 9069 PeerType=3 per-peer header for a Loc-RIB
// Route Monitoring or Peer Up message.
//
// RFC 9069: for Loc-RIB (Peer Type 3) the Peer Address and Peer AS are 0 (not
// applicable), the Peer BGP ID is the local router-id, and the V-flag position
// (bit 7) is reused as the F flag. F=0 means "route is in the Loc-RIB" (a
// best path); the V/L/A/O flags MUST NOT be set, so Flags is 0.
func locRIBPeerHeader(routerID uint32) PeerHeader {
	return PeerHeader{
		PeerType:     PeerTypeLocRIB,
		Flags:        0, // RFC 9069: F=0 (in Loc-RIB); V/L/A/O MUST be 0.
		PeerBGPID:    routerID,
		TimestampSec: uint32(time.Now().Unix()), //nolint:gosec // wall-clock seconds
	}
}

// bgpIdentifierFromSentOpen extracts the 4-byte BGP Identifier (the local
// router-id) from a sent BGP OPEN PDU. RFC 4271 Section 4.2: the OPEN body
// begins after the 19-byte message header with Version(1) + My AS(2) +
// Hold Time(2), so the BGP Identifier lives at offset 24. Returns (0, false)
// when the PDU is too short to contain it.
func bgpIdentifierFromSentOpen(open []byte) (uint32, bool) {
	const bgpIDOffset = message.HeaderLen + 1 + 2 + 2 // header + version + myAS + holdtime
	if len(open) < bgpIDOffset+4 {
		return 0, false
	}
	return binary.BigEndian.Uint32(open[bgpIDOffset : bgpIDOffset+4]), true
}

// encodeNLRIPrefix encodes a prefix as RFC 4271 Section 4.3 NLRI:
// [prefix-length:1][prefix-bytes], where prefix-bytes is the minimal number of
// octets holding prefix-length bits. Returns nil for an invalid prefix.
func encodeNLRIPrefix(p netip.Prefix) []byte {
	bits := p.Bits()
	if bits < 0 {
		return nil
	}
	addr := p.Addr()
	var raw []byte
	if addr.Is4() {
		a := addr.As4()
		raw = a[:]
	} else {
		a := addr.As16()
		raw = a[:]
	}
	nbytes := min((bits+7)/8, len(raw))
	out := make([]byte, 1+nbytes)
	out[0] = byte(bits)
	copy(out[1:], raw[:nbytes])
	return out
}

// writeAttr encodes a single path attribute (header + value) to fresh bytes.
// The 4-byte allowance covers the attribute header (flags + type + up to a
// 2-byte extended length); WriteAttrTo returns the exact bytes written.
func writeAttr(a attribute.Attribute) []byte {
	buf := make([]byte, 4+a.Len())
	n := attribute.WriteAttrTo(a, buf, 0)
	return buf[:n]
}

// assembleUpdateBody frames the three UPDATE body sections (RFC 4271
// Section 4.3): Withdrawn Routes Length + Withdrawn Routes + Total Path
// Attribute Length + Path Attributes + NLRI. It returns the UPDATE body only
// (no 19-byte BGP header); writeRouteMonitoring synthesizes that header.
func assembleUpdateBody(withdrawn, attrs, nlri []byte) []byte {
	body := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[0:], uint16(len(withdrawn))) //nolint:gosec // bounded by BGP max message size
	off := 2
	off += copy(body[off:], withdrawn)
	binary.BigEndian.PutUint16(body[off:], uint16(len(attrs))) //nolint:gosec // bounded by BGP max message size
	off += 2
	off += copy(body[off:], attrs)
	copy(body[off:], nlri)
	return body
}

// buildLocRIBUpdateBody reconstructs a BGP UPDATE message body from a Loc-RIB
// best-change entry. It is a minimal Route Monitoring UPDATE per RFC 9069
// "Route Monitoring Content": ORIGIN + AS_PATH + NEXT_HOP + NLRI for an
// announce, the prefix in Withdrawn Routes (IPv4) or MP_UNREACH_NLRI (IPv6)
// for a withdraw. BestChangeEntry does not carry communities or LOCAL_PREF, so
// those attributes are absent (documented fidelity limit; the spec forbids a
// RIB back-door for the full attribute set).
//
// Returns nil when the entry has no usable prefix.
func buildLocRIBUpdateBody(fam family.Family, e ribevents.BestChangeEntry) []byte {
	nlri := encodeNLRIPrefix(e.Prefix)
	if nlri == nil {
		return nil
	}
	isV4 := e.Prefix.Addr().Is4()

	if e.Action == routeaction.Withdraw {
		if isV4 {
			// RFC 4271 Section 4.3: IPv4 withdrawn routes in the Withdrawn field.
			return assembleUpdateBody(nlri, nil, nil)
		}
		// RFC 4760 Section 4: IPv6 withdraw via MP_UNREACH_NLRI (type 15).
		mp := &attribute.MPUnreachNLRI{
			AFI:  attribute.AFI(fam.AFI),
			SAFI: attribute.SAFI(fam.SAFI),
			NLRI: nlri,
		}
		return assembleUpdateBody(nil, writeAttr(mp), nil)
	}

	// Announce. ORIGIN defaults to IGP; AS_PATH is empty for locally originated
	// routes (RFC 9069 allows this). The attribute encoder is the same one
	// injectRoute uses, so no parallel encoder is introduced.
	ab := attribute.NewBuilder()
	ab.SetOrigin(uint8(attribute.OriginIGP))
	if len(e.ASPath) > 0 {
		ab.SetASPath(e.ASPath)
	}

	nh := e.NextHop
	if isV4 && nh.Is4() {
		// Legacy NEXT_HOP (type 3, IPv4 only) + IPv4 NLRI in the NLRI field.
		ab.SetNextHopAddr(nh)
		return assembleUpdateBody(nil, ab.Build(), nlri)
	}

	// IPv6 NLRI (or an IPv4 NLRI reachable via an IPv6 next-hop): reachability
	// and next-hop travel together in MP_REACH_NLRI (RFC 4760 / RFC 5549).
	attrs := ab.Build()
	if nh.IsValid() {
		mp := attribute.NewMPReachNLRI(
			attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI),
			[]netip.Addr{nh}, nlri,
		)
		attrs = append(attrs, writeAttr(mp)...)
	}
	return assembleUpdateBody(nil, attrs, nil)
}

// localRouterID returns the local router-id (BGP Identifier) for the Loc-RIB
// peer header, read from any cached sent OPEN PDU. Returns 0 when no OPEN has
// been cached yet (no peer has come up).
func (bp *BMPPlugin) localRouterID() uint32 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	for _, pair := range bp.openCache {
		if pair == nil {
			continue
		}
		if id, ok := bgpIdentifierFromSentOpen(pair.sent); ok {
			return id
		}
	}
	return 0
}

// startLocRIB subscribes to (bgp-rib, best-change) and requests an initial
// full-table replay so Loc-RIB Route Monitoring reflects the RIB an operator
// already has when they enable monitoring (RFC 9069: "Initial dump sends full
// Loc-RIB contents"). Idempotent: a repeat call while already subscribed (a
// config reload) is a no-op.
func (bp *BMPPlugin) startLocRIB() {
	bus := getEventBus()
	if bus == nil {
		logger().Warn("bmp: loc-rib monitoring enabled but no event bus is available")
		return
	}

	bp.mu.Lock()
	if bp.locRIBUnsub != nil {
		bp.mu.Unlock()
		return
	}
	bp.locRIBUnsub = ribevents.BestChange.Subscribe(bus, bp.handleBestChange)
	bp.mu.Unlock()

	// Broadcast replay-request: the RIB re-emits its whole best-path table as
	// replay batches (mirrors sysrib.go). The hop is broadcast, so every
	// best-change subscriber re-processes; those paths dedup unchanged entries,
	// so a redundant replay is safe.
	if _, err := ribevents.ReplayRequest.Emit(bus, &replay.Request{ReplayID: replay.Broadcast}); err != nil {
		logger().Warn("bmp: loc-rib replay-request emit failed", "error", err)
	}
	logger().Info("bmp: loc-rib monitoring started (RFC 9069 PeerType=3)")
}

// stopLocRIB unsubscribes from best-change events.
func (bp *BMPPlugin) stopLocRIB() {
	bp.mu.Lock()
	unsub := bp.locRIBUnsub
	bp.locRIBUnsub = nil
	bp.mu.Unlock()
	if unsub != nil {
		unsub()
	}
}

// handleBestChange turns a RIB best-change batch into Loc-RIB Route Monitoring
// messages. The Loc-RIB Peer Up is sent lazily before the first Route
// Monitoring (RFC 9069 requires Peer Up to precede Route Monitoring).
//
// KNOWN DEFECT (pre-existing, not introduced by the writeMu fix): this runs on
// the RIB's publisher goroutine -- engine EventBus subscribers fire
// synchronously from deliverEvent (internal/component/plugin/server/engine_event.go
// SubscribeEngineEvent) -- and the bus contract is explicit that a handler
// "MUST NOT block on I/O" (pkg/ze/eventbus.go EventBus.Subscribe). The writes
// below are blocking socket writes bounded only by writeTimeout, so a wedged
// collector stalls RIB best-change publication for up to that long per message.
// This is not a regression from writeMu: concurrent conn.Write calls already
// serialize on the socket's own write lock (internal/poll.FD.Write holds it for
// the whole call), so a producer already waited out any in-flight write. The
// real fix is a bounded per-session send queue so no socket write happens on a
// subscriber goroutine; dropping instead (TryLock) would silently lose Route
// Monitoring messages, which is worse for a monitoring protocol.
// Tracked in plan/deferrals/.
func (bp *BMPPlugin) handleBestChange(batch *ribevents.BestChangeBatch) {
	if batch == nil || len(batch.Changes) == 0 {
		return
	}

	bp.mu.RLock()
	senders := bp.senders
	bp.mu.RUnlock()
	if len(senders) == 0 {
		return
	}

	bp.ensureLocRIBPeerUp(senders)

	peer := locRIBPeerHeader(bp.localRouterID())
	for i := range batch.Changes {
		body := buildLocRIBUpdateBody(batch.Family, batch.Changes[i])
		if body == nil {
			continue
		}
		for _, ss := range senders {
			if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body); err != nil {
				logger().Debug("bmp: loc-rib route monitoring failed", "collector", ss.name, "error", err)
			}
		}
	}
}

// ensureLocRIBPeerUp sends the RFC 9069 Loc-RIB Peer Up exactly once, before
// the first Route Monitoring. Per RFC 9069 the sent/received OPENs are
// zero-length and the local/remote ports are 0.
func (bp *BMPPlugin) ensureLocRIBPeerUp(senders []*senderSession) {
	bp.mu.Lock()
	if bp.locRIBUp {
		bp.mu.Unlock()
		return
	}
	bp.locRIBUp = true
	bp.mu.Unlock()

	peer := locRIBPeerHeader(bp.localRouterID())
	var zeroAddr [16]byte
	for _, ss := range senders {
		if err := ss.writePeerUp(peer, zeroAddr, 0, 0, nil, nil); err != nil {
			logger().Debug("bmp: loc-rib peer up failed", "collector", ss.name, "error", err)
		}
	}
}

// sendLocRIBPeerDown emits a Loc-RIB Peer Down (RFC 9069: signals end of
// Loc-RIB monitoring) best-effort on shutdown, before the sender sessions are
// torn down. No-op when Peer Up was never sent.
func (bp *BMPPlugin) sendLocRIBPeerDown() {
	bp.mu.Lock()
	if !bp.locRIBUp {
		bp.mu.Unlock()
		return
	}
	bp.locRIBUp = false
	senders := bp.senders
	bp.mu.Unlock()

	peer := locRIBPeerHeader(bp.localRouterID())
	for _, ss := range senders {
		if err := ss.writePeerDown(peer, PeerDownLocalNoNotify, nil); err != nil {
			logger().Debug("bmp: loc-rib peer down failed", "collector", ss.name, "error", err)
		}
	}
}
