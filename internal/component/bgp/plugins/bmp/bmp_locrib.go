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
	"math"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/ze"
)

// The two BGP constants the fabricated Loc-RIB OPEN needs.
//
// RFC 4271 Section 4.2: "Version: ... The current BGP version number is 4."
// RFC 5492 Section 4: the Capabilities Optional Parameter is "Parameter Type: 2".
const (
	bgpVersion           uint8 = 4
	optParamCapabilities byte  = 2
)

// locRIBTableName is the VRF/Table Name ze reports for its one Loc-RIB.
//
// RFC 9069 Section 5.2.1: "The default value of "global" MUST be used for the
// default Loc-RIB instance with a zero-filled distinguisher." Ze runs exactly
// one Loc-RIB, it is the default instance, and locRIBPeerHeader leaves the
// distinguisher zero -- so this string is the name the RFC fixes for it, not a
// name an operator chose.
//
// The size the same section requires is a property of this constant: "The
// string size MUST be within the range of 1 to 255 bytes", and "global" is six
// UTF-8 bytes.
const locRIBTableName = "global"

// locRIBTableNameTLV is the Peer Up Information TLV that carries it.
//
// RFC 9069 Section 5.2.1 registers "Type = 3: VRF/Table Name. The Information
// field contains a UTF-8 string whose value MUST be equal to the value of the
// VRF or table name (e.g., RD instance name) being conveyed", and Section 5.3
// repeats the TLV for the Peer Down: "The VRF/Table Name informational TLV MUST
// be included if it was in the Peer Up." One TLV, built in one place, so the
// Peer Up and the Peer Down cannot convey two different names.
func locRIBTableNameTLV() TLV {
	return makeStringTLV(PeerTLVVRFTableName, locRIBTableName)
}

// locRIBTableNameTLVBytes is the same TLV encoded, for the Peer Down, whose
// body is "the reason ... followed by data in TLV format" (RFC 9069 Section
// 5.3) rather than a TLV list the message writer appends.
func locRIBTableNameTLVBytes() []byte {
	tlv := locRIBTableNameTLV()
	buf := make([]byte, TLVHeaderSize+len(tlv.Value))
	writeTLV(buf, 0, tlv)
	return buf
}

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

// localIdentity is the router's own BGP identity: the two values RFC 9069
// requires the Loc-RIB emulated peer to describe itself with, read from the
// `bgp` configuration section (BMPPlugin.localIdentity).
//
// One struct rather than two uint32 arguments: the ASN and the router-id are
// the same Go type, so a caller that transposed them would compile and would
// put each value in the other's wire field.
type localIdentity struct {
	asn      uint32
	routerID uint32
}

// locRIBPeerHeader builds the RFC 9069 PeerType=3 per-peer header for a Loc-RIB
// Route Monitoring or Peer Up message.
//
// RFC 9069 Section 5.1 fixes every field of it:
//   - "Peer Type: Set to 3 to indicate Loc-RIB Instance Peer."
//   - "Peer Address: Zero-filled. The remote peer address is not applicable.
//     The V flag is not applicable with the Loc-RIB Instance Peer Type
//     considering addresses are zero-filled."
//   - "Peer Autonomous System (AS): Set to the primary router BGP autonomous
//     system number (ASN)."
//   - "Peer BGP ID: Set the ID to the router-id of the VRF instance if VRF is
//     used; otherwise, set to the global instance router-id."
//   - "Timestamp: The time when the encapsulated routes were installed in the
//     Loc-RIB, expressed in seconds and microseconds since midnight (zero hour),
//     January 1, 1970 (UTC). If zero, the time is unavailable. Precision of the
//     timestamp is implementation dependent."
//
// The Peer Distinguisher stays zero: "Zero-filled if the Loc-RIB represents the
// global instance", and ze runs exactly one Loc-RIB, the global one. The
// "otherwise" branch names a route distinguisher per VRF instance, and VRF is
// out of ze's scope (owner decision, 2026-09-01).
//
// installed is the time the encapsulated routes entered the Loc-RIB, and the
// ZERO Time says ze does not know it. The RFC states the answer for that case
// rather than leaving it to the sender: a zero Timestamp means unavailable. Ze
// knows the install time for an incremental best change only, where the RIB
// delivers the event on the installing goroutine, so an initial full-table
// replay, a Peer Up, a Peer Down and an End-of-RIB marker each carry zero. A
// wall clock read here would date every replayed route to the moment the
// collector connected, which is a claim about the network that is false.
//
// RFC 9069 Section 4.2 defines the flags byte: bit 7 is the F flag, "set when a
// filter is applied to Loc-RIB routes sent to the BMP collector", and ze filters
// none, so F is 0. The remaining bits are "reserved for future use. They MUST be
// transmitted as 0", which the V, L, A and O flags of RFC 7854 and RFC 8671 sit
// in for this peer type.
func locRIBPeerHeader(id localIdentity, installed time.Time) PeerHeader {
	header := PeerHeader{
		PeerType:  PeerTypeLocRIB,
		Flags:     0, // RFC 9069 Section 4.2: F=0 (unfiltered), reserved bits 0.
		PeerAS:    id.asn,
		PeerBGPID: id.routerID,
	}
	if !installed.IsZero() {
		header.TimestampSec = uint32(installed.Unix())               //nolint:gosec // wall-clock seconds
		header.TimestampUsec = uint32(installed.Nanosecond() / 1000) //nolint:gosec // bounded by 1e9/1e3
	}
	return header
}

// fabricateLocRIBOpen builds the BGP OPEN message a Loc-RIB Peer Up carries.
//
// RFC 9069 Section 5.2: "Sent OPEN Message: This is a fabricated BGP OPEN
// message. Capabilities MUST include the 4-octet ASN and all necessary
// capabilities to represent the Loc-RIB Route Monitoring messages. Only include
// capabilities if they will be used for Loc-RIB monitoring messages." The
// capabilities are therefore the 4-octet ASN and one Multiprotocol capability
// per family in dumpFamilies -- the same list the dump closes with an End-of-RIB
// marker, so what the OPEN advertises and what the dump delivers are one
// declaration rather than two that can disagree.
//
// RFC 9069 Section 6.1.1 is what the receiver does with them: "Each emulated
// peer instance MUST send a Peer Up with the OPEN message indicating the address
// family capabilities. A BMP receiver MUST process these capabilities to know
// which peer belongs to which address family."
//
// The Hold Time is 0. RFC 4271 Section 4.2 allows "either zero or at least three
// seconds", and this peer has no session to keep alive.
func fabricateLocRIBOpen(id localIdentity) []byte {
	caps := make([]capability.Capability, 0, 1+len(dumpFamilies))
	caps = append(caps, &capability.ASN4{ASN: id.asn})
	for _, fam := range dumpFamilies {
		// capability.AFI and capability.SAFI are aliases of the family package's
		// own types, so the fields take a family.Family's halves unconverted.
		caps = append(caps, &capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI})
	}

	capBytes := 0
	for _, capa := range caps {
		capBytes += capa.Len()
	}

	// RFC 5492 Section 4: the Capabilities Optional Parameter is parameter type
	// 2, whose value is the sequence of capability triples.
	params := make([]byte, 2+capBytes)
	params[0] = optParamCapabilities
	params[1] = byte(capBytes) // bounded: three capabilities of 6 octets each
	off := 2
	for _, capa := range caps {
		off += capa.WriteTo(params, off)
	}

	open := &message.Open{
		Version:        bgpVersion,
		HoldTime:       0,
		BGPIdentifier:  id.routerID,
		ASN4:           id.asn,
		OptionalParams: params,
	}
	// RFC 6793 Section 3: a speaker whose ASN does not fit the two-octet field
	// sends AS_TRANS in it, which Open.WriteTo substitutes from ASN4.
	if id.asn <= math.MaxUint16 {
		open.MyAS = uint16(id.asn)
	}

	buf := make([]byte, open.Len(nil))
	return buf[:open.WriteTo(buf, 0, nil)]
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
	//
	// RFC 9069 Section 5.4.1: "Loc-RIB Route Monitoring messages MUST use a
	// 4-byte ASN encoding as indicated in the Peer Up sent OPEN message
	// (Section 5.2) capability." attribute.ASPath.WriteTo writes 4-byte ASNs
	// unconditionally (WriteToWithASN4(buf, off, true)), and the fabricated OPEN
	// advertises the matching 4-octet ASN capability, so the two agree by
	// construction rather than by negotiation.
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

// localIdentity returns the router's own ASN and router-id for the Loc-RIB
// emulated peer, and ok=false when no `bgp` configuration has been parsed yet.
//
// RFC 9069 Section 5.1 names both: "Peer Autonomous System (AS): Set to the
// primary router BGP autonomous system number (ASN)", and "Peer BGP ID: Set the
// ID to the router-id of the VRF instance if VRF is used; otherwise, set to the
// global instance router-id."
//
// CONFIG is the source, and the only one. Reading them off a cached sent OPEN
// instead -- which this did until 2026-09-03 -- answered zero for the case RFC
// 9069 Section 1.1 exists to serve, a Loc-RIB monitored on a router with no BGP
// peer up: the OPEN cache is filled by peers coming up, so before the first one
// there is nothing to read, and the zero it returned was indistinguishable at
// the collector from AS 0 on router 0.0.0.0. Configuration is present from the
// first apply, before any session, and both leaves are `mandatory true`.
//
// The bool is the guard, and it is named rather than inferred from a zero: a
// caller that cannot get an identity MUST NOT emit (ai/rules/principles.md).
func (bp *BMPPlugin) localIdentity() (localIdentity, bool) {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	if bp.identity == nil {
		return localIdentity{}, false
	}
	return *bp.identity, true
}

// setLocalIdentity installs the identity a config apply parsed. nil is what a
// `bgp` section carrying no usable router-id or ASN leaves behind, and it puts
// the plugin back to declining rather than leaving the previous router's
// identity on the wire.
func (bp *BMPPlugin) setLocalIdentity(id *localIdentity) {
	bp.mu.Lock()
	bp.identity = id
	bp.mu.Unlock()
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
	if err := bp.emitReplayRequest(bus, nil); err != nil {
		logger().Warn("bmp: loc-rib replay-request emit failed", "error", err)
	}
	logger().Info("bmp: loc-rib monitoring started (RFC 9069 PeerType=3)")
}

// emitReplayRequest asks the RIB for a full-table replay and publishes the
// scope of the resulting dump for handleBestChange to read. A nil session means
// the dump is for every connected collector.
//
// The whole window -- publish, emit, retract -- is held under dumpMu, and that
// serialization is the point rather than an accident. In-process EventBus
// delivery is synchronous, so the entire dump runs inside the Emit on the
// caller's goroutine; two collectors reconnecting together (the common case,
// since a collector host restart plus the reconnect backoff aligns them) would
// otherwise interleave their scopes, and the first dump would be delivered to
// the second collector while the first got nothing.
//
// Serialized rather than queued: the work and its order are identical either
// way, and queueing would add a goroutine, a bound and a failure mode to
// arrange the same outcome. The cost is that the second collector's dump waits
// for the first to finish, on its own session goroutine, where nothing else is
// waiting -- its producers enqueue independently and never block on this.
//
// dumpMu is deliberately NOT bp.mu: handleBestChange takes bp.mu.RLock inside
// this Emit, so holding bp.mu across it would deadlock.
func (bp *BMPPlugin) emitReplayRequest(bus ze.EventBus, ss *senderSession) error {
	bp.dumpMu.Lock()
	defer bp.dumpMu.Unlock()

	token := nextDumpToken()
	scope := &dumpScope{session: ss, replayID: token}
	bp.dumpScope.Store(scope)

	_, err := ribevents.ReplayRequest.Emit(bus, &replay.Request{ReplayID: token})

	// Retract BEFORE the End-of-RIB sweep: the dump is over, and leaving the
	// scope published would let a batch arriving now be counted into it.
	bp.dumpScope.Store(nil)
	if err != nil {
		return err
	}

	bp.closeDumpFamilies(scope, ss)
	return nil
}

// dumpTokens allocates the per-dump correlation tokens. Monotonic from 1, which
// the replay vocabulary reserves for exactly this ("any other -> a per-target
// token the emitter maps back to one consumer", internal/core/replay).
//
// Disjointness on this hop is what makes the token decisive: 0 means incremental
// (replay.IsReplay), replay.Broadcast is the reserved everyone-token, and the
// ONLY other emitter of (bgp-rib, replay-request) is sysrib, which emits
// Broadcast (internal/component/sysrib/sysrib.go:898). So a batch stamped with
// our token can only be the batch our request produced. A uint64 counter cannot
// realistically reach Broadcast (MaxUint64), and 0 is skipped by starting at 1.
var dumpTokens atomic.Uint64

// nextDumpToken returns a fresh correlation token for one dump.
func nextDumpToken() uint64 {
	return dumpTokens.Add(1)
}

// dumpFamilies are the families a Loc-RIB dump carries: the families it owes an
// End-of-RIB marker for, and the families the fabricated OPEN advertises a
// Multiprotocol capability for (fabricateLocRIBOpen).
//
// It is one declaration because RFC 9069 Section 5.2 binds the two together:
// "Only include capabilities if they will be used for Loc-RIB monitoring
// messages." A second list would let the emulated peer advertise a family the
// dump never delivers, or deliver one it never advertised.
//
// There is no negotiated set to derive it from -- the Loc-RIB peer has no
// session -- so these are the two a collector is realistically waiting on.
var dumpFamilies = [...]family.Family{family.IPv4Unicast, family.IPv6Unicast}

// closeDumpFamilies sends the End-of-RIB markers a dump still owes: every family
// in dumpFamilies that no batch closed.
//
// The RIB emits NO batch for a family with zero best paths -- replayBestPaths
// only publishes a family whose change list is non-empty
// (internal/component/bgp/plugins/rib/rib_bestchange.go) -- so on an empty
// Loc-RIB handleBestChange never runs and no marker is ever sent, precisely in
// the case the marker exists to describe. A collector attached to a router with
// an empty table then waits forever for a dump that already finished.
//
// Runs after the replay-request Emit returns. In-process EventBus delivery is
// synchronous, so by then every batch this dump produced has been processed and
// scope.closed is final.
//
// The MIXED table is covered too, and that is the point of working from
// scope.closed rather than from "did the dump produce anything at all". A table
// with IPv6 populated and IPv4 empty used to yield an IPv6 marker and no IPv4
// one, which contradicts RFC 4724 Section 4: the End-of-RIB marker "MUST be
// sent by a BGP speaker to its peer once it completes the initial routing
// update (including the case when there is no update to send) for an address
// family". RFC 7854 Section 5 makes the BMP dump's completion "MUST be
// indicated by sending an End-of-RIB marker for that peer (as specified in
// Section 2 of [RFC4724])", importing that per-<AFI, SAFI> definition verbatim,
// so a silent family is precisely the case the marker exists to describe: a
// collector waiting on IPv4 waits forever while the dump is in fact complete.
func (bp *BMPPlugin) closeDumpFamilies(scope *dumpScope, ss *senderSession) {
	missing := scope.unclosed(dumpFamilies[:])
	if len(missing) == 0 {
		return // every family this dump owes a marker for already got one
	}

	senders := []*senderSession{ss}
	if ss == nil {
		bp.mu.RLock()
		senders = bp.senders
		bp.mu.RUnlock()
	}
	if len(senders) == 0 {
		return
	}

	id, ok := bp.localIdentity()
	if !ok {
		logger().Warn("bmp: loc-rib dump not closed: the router identity is unknown; set `bgp router-id` and `bgp session asn local`")
		return
	}

	// An End-of-RIB marker IS a Route Monitoring message, so RFC 9069 still
	// requires the Loc-RIB Peer Up to precede it. On a wholly empty table
	// nothing else has sent one; the guard inside is per-session and idempotent.
	bp.ensureLocRIBPeerUp(senders)

	// Zero Timestamp: an End-of-RIB marker encapsulates no route, so there is no
	// install time to report (RFC 9069 Section 5.1, "If zero, the time is
	// unavailable").
	peer := locRIBPeerHeader(id, time.Time{})
	for _, fam := range missing {
		bp.sendLocRIBEndOfRIB(senders, fam, peer)
	}
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
// Monitoring (RFC 9069 requires Peer Up to precede Route Monitoring), and a
// full-table replay batch is closed with an End-of-RIB marker.
//
// This runs on the RIB's publisher goroutine -- engine EventBus subscribers
// fire synchronously from deliverEvent
// (internal/component/plugin/server/engine_event.go SubscribeEngineEvent) --
// and the bus contract is explicit that a handler "MUST NOT block on I/O"
// (pkg/ze/eventbus.go EventBus.Subscribe). It honors that: every write* call
// below encodes into the session's scratch buffer and copies the message into
// that session's bounded transmit queue (sender.go enqueueLocked), and the
// session's own drain goroutine does the socket write. A wedged collector
// therefore costs this goroutine a memcpy, not a 10s write deadline per
// message, and when the queue fills the SESSION is reset rather than messages
// being dropped.
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

	// A replay batch this plugin asked for is scoped to what it asked for: one
	// collector's fresh session, or every connected session when Loc-RIB
	// monitoring has just started. A replay somebody ELSE asked for (sysrib
	// emits on the same handle) is delivered here too; those routes are still
	// real, so they fan out as before, but they are not this plugin's dump and
	// must not be closed with an End-of-RIB marker.
	//
	// The claim is made on the TOKEN, which answers "is this batch mine", not on
	// "is a dump of mine in flight", which answered a weaker question and got it
	// wrong whenever the two overlapped. sysrib emits its own replay-request from
	// its own goroutine (internal/component/sysrib/sysrib.go:898) and a dump of a
	// million routes takes seconds, so overlap is ordinary. A foreign replay
	// landing in that window used to be treated as ours: senders narrowed to
	// scope.session, so every OTHER collector silently lost the batch, and the
	// family was closed with an End-of-RIB asserting a dump they never requested
	// had completed -- a marker RFC 7854 Section 5 defines as meaning the dump IS
	// complete. dumpMu never helped: it serializes only this plugin's own emits.
	//
	// The RIB ignores the token for routing (it walks the whole table either way,
	// rib_bestchange.go:1143-1211) and echoes it verbatim onto each batch (:1202),
	// which is what makes the comparison exact. BMP is in-process, so the token
	// survives delivery; the JSON codec on BestChangeBatch would flatten it for a
	// forked plugin, which is why this stays an in-process claim.
	var ourScope *dumpScope
	if batch.IsReplay() {
		if scope := bp.dumpScope.Load(); scope != nil && scope.replayID == batch.ReplayID {
			ourScope = scope
			if scope.session != nil {
				senders = []*senderSession{scope.session}
			}
		}
	}

	bp.ensureLocRIBPeerUp(senders)

	// RFC 9069 Section 5.1 asks for "the time when the encapsulated routes were
	// installed in the Loc-RIB", and ze can answer for an INCREMENTAL batch
	// only: the RIB emits it on the goroutine that installed the change, so the
	// clock read here is that install to the precision this implementation
	// offers ("Precision of the timestamp is implementation dependent"). A
	// replay batch re-reads a table installed at times nobody recorded, so it
	// carries the zero the same paragraph defines as "the time is unavailable".
	installed := time.Now()
	if batch.IsReplay() {
		installed = time.Time{}
	}
	id, ok := bp.localIdentity()
	if !ok {
		logger().Warn("bmp: loc-rib route monitoring suppressed: the router identity is unknown; set `bgp router-id` and `bgp session asn local`")
		return
	}
	peer := locRIBPeerHeader(id, installed)
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

	// A replay batch IS the full table for its family (the RIB emits one batch
	// per family, rib_bestchange.go replayBestPaths), so the End-of-RIB marker
	// belongs right here: it tells the collector the dump for this family is
	// complete.
	//
	// Only for a dump this plugin requested. RFC 4724 Section 2 gives the marker
	// one meaning -- "my initial routing update is complete" -- so emitting it
	// on the back of another subsystem's replay would assert that mid-stream to
	// collectors that never asked for anything.
	if ourScope != nil {
		bp.sendLocRIBEndOfRIB(senders, batch.Family, peer)
		// Recorded so emitReplayRequest can tell which families the RIB never
		// produced a batch for, and close those too.
		ourScope.noteClosed(batch.Family)
	}
}

// ensureLocRIBPeerUp sends the RFC 9069 Loc-RIB Peer Up before the first Route
// Monitoring.
//
// RFC 9069 Section 5.2 fixes what it carries: "Local Address: Zero-filled",
// "Local Port: Set to 0", "Remote Port: Set to 0", a fabricated sent OPEN
// (fabricateLocRIBOpen), and "Received OPEN Message: Repeat of the same sent
// OPEN message. The duplication allows the BMP receiver to parse the expected
// received OPEN message as defined in Section 4.10 of [RFC7854]."
//
// The guard is per SESSION, not per plugin: RFC 9069's "per-instance, not
// per-peer" means one Peer Up per Loc-RIB instance per BMP session, and a
// collector that reconnects gets a brand new session that has been told
// nothing. bp.locRIBUp stays as the plugin-wide record that Loc-RIB monitoring
// has been announced at all, which is what sendLocRIBPeerDown keys off.
func (bp *BMPPlugin) ensureLocRIBPeerUp(senders []*senderSession) {
	id, ok := bp.localIdentity()
	if !ok {
		logger().Warn("bmp: loc-rib peer up suppressed: the router identity is unknown; set `bgp router-id` and `bgp session asn local`")
		return
	}
	// Zero Timestamp: a Peer Up encapsulates no route (RFC 9069 Section 5.1).
	peer := locRIBPeerHeader(id, time.Time{})
	open := fabricateLocRIBOpen(id)
	tlvs := []TLV{locRIBTableNameTLV()}
	var zeroAddr [16]byte
	announced := false
	for _, ss := range senders {
		// The claim and the enqueue happen under ONE writeMu critical section.
		// That is what orders this against a best-change batch being processed
		// concurrently on another goroutine: the loser of the claim blocks here
		// until the winner's Peer Up is in the queue, so its Route Monitoring
		// cannot overtake the Peer Up that RFC 9069 requires to precede it.
		ss.writeMu.Lock()
		claimed := ss.locRIBUpSent.CompareAndSwap(false, true)
		var err error
		if claimed {
			err = ss.writePeerUpLocked(peer, zeroAddr, 0, 0, open, open, tlvs)
		}
		ss.writeMu.Unlock()

		if !claimed {
			continue // already announced on this session's current connection
		}
		if err != nil {
			// The claim is given back: nothing reached the collector (typically
			// the session is not connected yet), and the Peer Up MUST precede
			// this session's first Route Monitoring, so the next batch has to
			// try again rather than assume it was announced.
			ss.locRIBUpSent.Store(false)
			logger().Debug("bmp: loc-rib peer up failed", "collector", ss.name, "error", err)
			continue
		}
		announced = true
	}
	if !announced {
		return
	}

	bp.mu.Lock()
	bp.locRIBUp = true
	bp.mu.Unlock()
}

// primeLocRIBPeerUp queues the RFC 9069 Loc-RIB Peer Up for a session that has
// just connected, claiming the session's once-per-connection guard. It carries
// the same fabricated OPEN, twice, that ensureLocRIBPeerUp does.
//
// This is where the Loc-RIB Peer Up belongs: the connection is known, and the
// caller holds writeMu, so it cannot be overtaken by a Route Monitoring from a
// best-change landing on another goroutine. ensureLocRIBPeerUp remains only for
// the other order of events -- monitoring switched on while a collector is
// already connected.
//
// Caller MUST hold ss.writeMu.
func (bp *BMPPlugin) primeLocRIBPeerUp(ss *senderSession) {
	if !ss.locRIBUpSent.CompareAndSwap(false, true) {
		return
	}
	id, ok := bp.localIdentity()
	if !ok {
		// The claim is given back: nothing was announced, so the next batch has
		// to try again once the configuration has arrived.
		ss.locRIBUpSent.Store(false)
		logger().Warn("bmp: loc-rib peer up suppressed: the router identity is unknown; set `bgp router-id` and `bgp session asn local`",
			"collector", ss.name)
		return
	}
	open := fabricateLocRIBOpen(id)
	var zeroAddr [16]byte
	peer := locRIBPeerHeader(id, time.Time{}) // a Peer Up encapsulates no route
	if err := ss.writePeerUpLocked(peer, zeroAddr, 0, 0, open, open, []TLV{locRIBTableNameTLV()}); err != nil {
		ss.locRIBUpSent.Store(false) // nothing reached the collector; let the next batch retry
		logger().Debug("bmp: loc-rib peer up failed", "collector", ss.name, "error", err)
		return
	}

	bp.mu.Lock()
	bp.locRIBUp = true
	bp.mu.Unlock()
}

// sendLocRIBEndOfRIB closes a full-table Loc-RIB dump with an End-of-RIB marker
// for the family that was dumped, so a collector can tell "the table is empty
// so far" from "the dump is still arriving". BIRD ends its BMP table dump the
// same way (proto/bmp/bmp.c:1040-1065).
func (bp *BMPPlugin) sendLocRIBEndOfRIB(senders []*senderSession, fam family.Family, peer PeerHeader) {
	body := buildEndOfRIBBody(fam)
	for _, ss := range senders {
		if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body); err != nil {
			logger().Debug("bmp: loc-rib end-of-rib failed", "collector", ss.name, "error", err)
		}
	}
}

// buildEndOfRIBBody returns the UPDATE body of an End-of-RIB marker.
//
// RFC 4724 Section 2: "An UPDATE message with no reachable Network Layer
// Reachability Information (NLRI) and empty Withdrawn NLRI is specified as the
// End-of-RIB marker"; for any other <AFI, SAFI> it is an UPDATE carrying only
// an MP_UNREACH_NLRI attribute with no withdrawn routes.
func buildEndOfRIBBody(fam family.Family) []byte {
	if fam == family.IPv4Unicast {
		return assembleUpdateBody(nil, nil, nil)
	}
	mp := attribute.NewMPUnreachEndOfRIB(attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI))
	return assembleUpdateBody(nil, writeAttr(mp), nil)
}

// requestLocRIBDump asks the RIB to re-emit its whole best-path table, which
// handleBestChange turns into a fresh Loc-RIB dump ending in End-of-RIB. Called
// when a collector connects: a new BMP session starts with the collector
// knowing nothing, and RFC 9069's initial dump is per session, not per process.
//
// The replay-request hop itself is broadcast -- the RIB has no per-subscriber
// replay -- so ss is published as the dump target for the duration of the Emit
// and handleBestChange sends the resulting replay batches to that session only.
// Without it, one collector reconnecting would re-dump the whole table to every
// other collector AND hand each of them a second End-of-RIB, which per RFC 4724
// Section 2 semantics claims their initial dump just completed.
//
// Targeting works because in-process EventBus delivery is synchronous (the RIB
// replays inside this Emit call, on this goroutine); a subscriber that deferred
// delivery would land outside the window and fall back to the full fan-out,
// which is the pre-existing behavior rather than a new failure.
//
// No-op when Loc-RIB monitoring is not enabled or no bus is installed.
func (bp *BMPPlugin) requestLocRIBDump(ss *senderSession) {
	bp.mu.RLock()
	subscribed := bp.locRIBUnsub != nil
	bp.mu.RUnlock()
	if !subscribed {
		return
	}

	bus := getEventBus()
	if bus == nil {
		return
	}

	if err := bp.emitReplayRequest(bus, ss); err != nil {
		logger().Warn("bmp: loc-rib replay-request emit failed", "collector", ss.name, "error", err)
		return
	}
	logger().Info("bmp: loc-rib dump requested for collector session", "collector", ss.name)
}

// sendLocRIBPeerDown emits a Loc-RIB Peer Down, which signals the end of
// Loc-RIB monitoring. Sent best-effort on shutdown before the sender sessions
// are torn down, and on the reload that turns `loc-rib` off while they stay up.
// No-op when Peer Up was never sent.
//
// RFC 9069 Section 5.3: "The Peer Down notification MUST use reason code 6."
// The VRF/Table Name Information TLV follows it, because the same section
// requires it "if it was in the Peer Up" and every ze Loc-RIB Peer Up carries
// it (ensureLocRIBPeerUp, primeLocRIBPeerUp).
//
// The per-session guard is given back with the Peer Down, because the pair is
// what makes Loc-RIB monitoring restartable on a session that survives the
// reload: monitoring turned off and on again owes the collector a second Peer
// Up before any further Loc-RIB Route Monitoring, and the guard is what would
// otherwise swallow it.
func (bp *BMPPlugin) sendLocRIBPeerDown() {
	bp.mu.Lock()
	if !bp.locRIBUp {
		bp.mu.Unlock()
		return
	}
	bp.locRIBUp = false
	senders := bp.senders
	bp.mu.Unlock()

	// Zero Timestamp: a Peer Down encapsulates no route (RFC 9069 Section 5.1).
	//
	// RFC 9069 Section 5.3: "Following the reason is data in TLV format", and
	// "The VRF/Table Name informational TLV MUST be included if it was in the
	// Peer Up." Every ze Loc-RIB Peer Up carries it, so every Loc-RIB Peer Down
	// carries it too.
	id, ok := bp.localIdentity()
	if !ok {
		logger().Warn("bmp: loc-rib peer down suppressed: the router identity is unknown; set `bgp router-id` and `bgp session asn local`")
		return
	}
	peer := locRIBPeerHeader(id, time.Time{})
	tlv := locRIBTableNameTLVBytes()
	for _, ss := range senders {
		if err := ss.writePeerDown(peer, PeerDownTLVData, tlv); err != nil {
			logger().Debug("bmp: loc-rib peer down failed", "collector", ss.name, "error", err)
		}
		ss.locRIBUpSent.Store(false)
	}
}
