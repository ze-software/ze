package bmp

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
)

// findAttr walks an RFC 4271 path-attribute block and returns the value of the
// first attribute with the given type code, or nil if absent. Attribute wire:
// flags(1) + type(1) + length(1 or 2 with the Extended-Length flag) + value.
func findAttr(attrs []byte, code byte) []byte {
	off := 0
	for off+3 <= len(attrs) {
		flags := attrs[off]
		typ := attrs[off+1]
		var vlen, hdr int
		if flags&0x10 != 0 { // Extended Length
			if off+4 > len(attrs) {
				return nil
			}
			vlen = int(attrs[off+2])<<8 | int(attrs[off+3])
			hdr = 4
		} else {
			vlen = int(attrs[off+2])
			hdr = 3
		}
		if off+hdr+vlen > len(attrs) {
			return nil
		}
		if typ == code {
			return attrs[off+hdr : off+hdr+vlen]
		}
		off += hdr + vlen
	}
	return nil
}

// RFC 4271 / RFC 4760 path-attribute type codes exercised here.
const (
	attrORIGIN   = 1
	attrASPATH   = 2
	attrNEXTHOP  = 3
	attrMPReach  = 14
	attrMPUnreac = 15
)

func TestLocRIBPeerHeader(t *testing.T) {
	// VALIDATES: RFC 9069 Section 5.1 PeerType=3 header: Peer Address zero, Peer
	// AS the router's own ASN, BGP ID the router-id, and Flags=0 (F=0
	// in-Loc-RIB; the reserved bits MUST be transmitted as 0).
	ph := locRIBPeerHeader(localIdentity{asn: 4200000001, routerID: 0x0a141e01}, time.Time{})

	if ph.PeerType != PeerTypeLocRIB {
		t.Errorf("PeerType = %d, want %d (Loc-RIB)", ph.PeerType, PeerTypeLocRIB)
	}
	// RFC requirement: RFC9069-x-1 positive -- the Loc-RIB PeerType=3 per-peer header is
	// built with Flags 0, so the V/L/A/O flags are never set (only F, whose 0 value means
	// "route is in the Loc-RIB").
	if ph.Flags != 0 {
		t.Errorf("Flags = %#x, want 0 (RFC 9069: V/L/A/O MUST be 0)", ph.Flags)
	}
	// RFC requirement: RFC9069-x-6 positive -- "Peer Autonomous System (AS): Set to the
	// primary router BGP autonomous system number (ASN)." The value is a 4-octet ASN,
	// which is the case a 2-octet field could not carry.
	if ph.PeerAS != 4200000001 {
		t.Errorf("PeerAS = %d, want the router's own ASN 4200000001 (RFC 9069 Section 5.1)", ph.PeerAS)
	}
	// RFC requirement: RFC9069-x-7 positive -- the Loc-RIB per-peer header carries the local
	// router-id as its Peer BGP ID.
	if ph.PeerBGPID != 0x0a141e01 {
		t.Errorf("PeerBGPID = %#x, want router-id 0x0a141e01", ph.PeerBGPID)
	}
	// RFC requirement: RFC9069-x-5 positive -- the Loc-RIB per-peer header carries an
	// all-zero Peer Address.
	if ph.Address != ([16]byte{}) {
		t.Errorf("Address = %v, want all-zero (RFC 9069)", ph.Address)
	}
}

// RFC requirement: RFC9069-x-6 negative -- the Peer AS is the router's OWN ASN and not
// a constant. A header built for a second identity carries that identity's ASN, so an
// implementation that hardcoded one value, or that zero-filled the field as ze did until
// 2026-08-31, fails here while still passing the positive case.
func TestLocRIBPeerHeaderCarriesTheIdentityItIsGiven(t *testing.T) {
	first := locRIBPeerHeader(localIdentity{asn: 65001, routerID: 0x0a141e01}, time.Time{})
	second := locRIBPeerHeader(localIdentity{asn: 65002, routerID: 0x0a141e02}, time.Time{})

	if first.PeerAS == second.PeerAS {
		t.Errorf("two identities produced one Peer AS (%d): the field does not come from the identity", first.PeerAS)
	}
	if first.PeerAS != 65001 || second.PeerAS != 65002 {
		t.Errorf("PeerAS = (%d, %d), want (65001, 65002)", first.PeerAS, second.PeerAS)
	}
}

func TestBuildLocRIBUpdateBody_IPv4Announce(t *testing.T) {
	// VALIDATES: AC-1 -- an IPv4 announce reconstructs a parseable UPDATE body
	// with ORIGIN + AS_PATH + NEXT_HOP and the prefix in the NLRI section.
	nh := netip.MustParseAddr("192.0.2.1")
	e := ribevents.BestChangeEntry{
		Action:  ribevents.BestChangeAdd,
		Prefix:  netip.MustParsePrefix("10.20.30.0/24"),
		NextHop: nh,
		ASPath:  []uint32{65001, 65002},
	}

	body := buildLocRIBUpdateBody(family.IPv4Unicast, e)
	sec, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatalf("ParseUpdateSections: %v", err)
	}
	if w := sec.Withdrawn(body); w != nil {
		t.Errorf("announce should have no withdrawn routes, got %x", w)
	}

	attrs := sec.Attrs(body)
	if findAttr(attrs, attrORIGIN) == nil {
		t.Error("missing ORIGIN attribute")
	}
	if findAttr(attrs, attrASPATH) == nil {
		t.Error("missing AS_PATH attribute")
	}
	gotNH := findAttr(attrs, attrNEXTHOP)
	wantNH := nh.As4()
	if gotNH == nil || len(gotNH) != 4 || [4]byte(gotNH) != wantNH {
		t.Errorf("NEXT_HOP = %x, want %x", gotNH, wantNH[:])
	}

	nlri := sec.NLRI(body)
	wantNLRI := encodeNLRIPrefix(e.Prefix)
	if !bytes.Equal(nlri, wantNLRI) {
		t.Errorf("NLRI = %x, want %x", nlri, wantNLRI)
	}
}

// RFC requirement: RFC9069-5.4.1-1 positive -- "Loc-RIB Route Monitoring messages MUST
// use a 4-byte ASN encoding as indicated in the Peer Up sent OPEN message (Section 5.2)
// capability."
// RFC requirement: RFC9069-5.4.1-1 negative -- the same sentence read the other way: an
// encoder that is not 4-byte is caught, rather than any encoding being accepted.
//
// One polarity per tag line: the scanner reads a single polarity token, so a combined
// "positive and negative" tag registers only the first and the requirement reads as
// negative-less.
//
// The positive is the width: an AS_PATH segment of two ASNs occupies 2 header octets
// plus 8, not plus 4. The negative is the value: an ASN above 65535 survives the encode
// whole, so a 2-byte encoder is caught by more than an off-by-four -- it cannot represent
// the number at all. The two run together because the same encoded segment answers both.
func TestLocRIBRouteMonitoringUsesFourByteASNs(t *testing.T) {
	e := ribevents.BestChangeEntry{
		Action:  ribevents.BestChangeAdd,
		Prefix:  netip.MustParsePrefix("10.20.30.0/24"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
		ASPath:  []uint32{4200000001, 65002},
	}

	body := buildLocRIBUpdateBody(family.IPv4Unicast, e)
	sec, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatalf("ParseUpdateSections: %v", err)
	}
	asPath := findAttr(sec.Attrs(body), attrASPATH)
	if asPath == nil {
		t.Fatal("missing AS_PATH attribute")
	}

	// RFC 4271 Section 4.3: each AS_PATH segment is type(1) + count(1) + the ASNs.
	const segmentHeader = 2
	if len(asPath) != segmentHeader+len(e.ASPath)*4 {
		t.Fatalf("AS_PATH value is %d octets for %d ASNs, want %d: the ASNs are not 4-byte encoded",
			len(asPath), len(e.ASPath), segmentHeader+len(e.ASPath)*4)
	}
	if count := int(asPath[1]); count != len(e.ASPath) {
		t.Fatalf("AS_PATH segment count = %d, want %d", count, len(e.ASPath))
	}
	for i, want := range e.ASPath {
		off := segmentHeader + i*4
		if got := binary.BigEndian.Uint32(asPath[off : off+4]); got != want {
			t.Errorf("AS_PATH[%d] = %d, want %d", i, got, want)
		}
	}
}

func TestBuildLocRIBUpdateBody_IPv4Withdraw(t *testing.T) {
	// VALIDATES: AC-1 (withdraw) -- an IPv4 withdraw puts the prefix in the
	// Withdrawn Routes section with no path attributes and no NLRI.
	e := ribevents.BestChangeEntry{
		Action: ribevents.BestChangeWithdraw,
		Prefix: netip.MustParsePrefix("10.20.30.0/24"),
	}

	body := buildLocRIBUpdateBody(family.IPv4Unicast, e)
	sec, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatalf("ParseUpdateSections: %v", err)
	}

	wantWithdrawn := encodeNLRIPrefix(e.Prefix)
	if got := sec.Withdrawn(body); !bytes.Equal(got, wantWithdrawn) {
		t.Errorf("Withdrawn = %x, want %x", got, wantWithdrawn)
	}
	if a := sec.Attrs(body); a != nil {
		t.Errorf("withdraw should have no path attributes, got %x", a)
	}
	if n := sec.NLRI(body); n != nil {
		t.Errorf("withdraw should have no NLRI, got %x", n)
	}
}

func TestBuildLocRIBUpdateBody_IPv6Announce(t *testing.T) {
	// VALIDATES: AC-1 (IPv6) -- an IPv6 announce carries reachability and
	// next-hop in MP_REACH_NLRI, with an empty legacy NLRI section.
	e := ribevents.BestChangeEntry{
		Action:  ribevents.BestChangeAdd,
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		ASPath:  []uint32{65010},
	}

	body := buildLocRIBUpdateBody(family.IPv6Unicast, e)
	sec, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatalf("ParseUpdateSections: %v", err)
	}
	if n := sec.NLRI(body); n != nil {
		t.Errorf("IPv6 announce should carry NLRI in MP_REACH, legacy NLRI got %x", n)
	}
	attrs := sec.Attrs(body)
	if findAttr(attrs, attrORIGIN) == nil {
		t.Error("missing ORIGIN attribute")
	}
	if findAttr(attrs, attrMPReach) == nil {
		t.Error("missing MP_REACH_NLRI attribute")
	}
	if findAttr(attrs, attrNEXTHOP) != nil {
		t.Error("IPv6 announce must not use the IPv4-only NEXT_HOP attribute")
	}
}

func TestBuildLocRIBUpdateBody_IPv6Withdraw(t *testing.T) {
	// VALIDATES: an IPv6 withdraw uses MP_UNREACH_NLRI (no IPv4 withdrawn field).
	e := ribevents.BestChangeEntry{
		Action: ribevents.BestChangeWithdraw,
		Prefix: netip.MustParsePrefix("2001:db8::/32"),
	}

	body := buildLocRIBUpdateBody(family.IPv6Unicast, e)
	sec, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatalf("ParseUpdateSections: %v", err)
	}
	if w := sec.Withdrawn(body); w != nil {
		t.Errorf("IPv6 withdraw must not use the IPv4 withdrawn field, got %x", w)
	}
	if findAttr(sec.Attrs(body), attrMPUnreac) == nil {
		t.Error("missing MP_UNREACH_NLRI attribute")
	}
}

func TestBgpIdentityFromSentOpen(t *testing.T) {
	// VALIDATES: the router's own ASN and router-id are read out of a sent OPEN,
	// which is where the Loc-RIB emulated peer gets both (RFC 9069 Sections 5.1
	// and 5.2).
	open := makeBGPOpen(65000, 0x01020305)
	id, ok := bgpIdentityFromSentOpen(open)
	if !ok {
		t.Fatal("expected to read an identity from a full OPEN")
	}
	if id.routerID != 0x01020305 {
		t.Errorf("BGP Identifier = %#x, want 0x01020305", id.routerID)
	}
	if id.asn != 65000 {
		t.Errorf("ASN = %d, want the My AS field's 65000", id.asn)
	}

	if _, ok := bgpIdentityFromSentOpen(open[:20]); ok {
		t.Error("a truncated OPEN must not yield an identity")
	}
	if _, ok := bgpIdentityFromSentOpen(nil); ok {
		t.Error("a nil OPEN must not yield an identity")
	}
}

// RFC requirement: RFC9069-x-6 negative -- "the primary router BGP autonomous system
// number" is the 4-octet ASN, not the two-octet field a 4-byte speaker fills with
// AS_TRANS. An OPEN carrying My AS 23456 and a 4-octet ASN capability yields the
// capability's value, so an implementation that read only the fixed field would publish
// 23456 as the router's AS while still passing the positive case above.
//
// RFC 6793 Section 4.1: "When a NEW BGP speaker processes an OPEN message from another
// NEW BGP speaker, it MUST use the AS number encoded in this capability in lieu of the
// 'My Autonomous System' field of the OPEN message".
func TestBgpIdentityPrefersTheFourOctetASNCapability(t *testing.T) {
	open := fabricateLocRIBOpen(localIdentity{asn: 4200000001, routerID: 0x01020305})

	// The fabricated OPEN is itself the fixture: it carries AS_TRANS in My AS
	// and the real ASN in the capability, which is exactly the shape ze's own
	// sessions put on the wire for a 4-byte ASN.
	body := open[message.HeaderLen:]
	if got := binary.BigEndian.Uint16(body[1:3]); got != message.AS_TRANS {
		t.Fatalf("My AS = %d, want AS_TRANS %d: the fixture does not exercise the capability", got, message.AS_TRANS)
	}

	id, ok := bgpIdentityFromSentOpen(open)
	if !ok {
		t.Fatal("expected to read an identity from the fabricated OPEN")
	}
	if id.asn != 4200000001 {
		t.Errorf("ASN = %d, want the capability's 4200000001", id.asn)
	}
	if id.routerID != 0x01020305 {
		t.Errorf("BGP Identifier = %#x, want 0x01020305", id.routerID)
	}
}

// RFC requirement: RFC9069-5.2-1 positive -- "Sent OPEN Message: This is a fabricated BGP
// OPEN message. Capabilities MUST include the 4-octet ASN and all necessary capabilities
// to represent the Loc-RIB Route Monitoring messages. Only include capabilities if they
// will be used for Loc-RIB monitoring messages."
//
// The fabricated OPEN decodes as a BGP OPEN, carries the 4-octet ASN capability with the
// router's own ASN, and carries one Multiprotocol capability for each family the dump
// delivers -- and none for a family it does not.
func TestFabricatedLocRIBOpenCarriesTheRequiredCapabilities(t *testing.T) {
	open := fabricateLocRIBOpen(localIdentity{asn: 4200000001, routerID: 0x01020305})

	if got := msgtype.MessageType(open[message.MarkerLen+2]); got != msgtype.TypeOPEN {
		t.Fatalf("message type = %d, want OPEN", got)
	}
	parsed, err := message.UnpackOpen(open[message.HeaderLen:])
	if err != nil {
		t.Fatalf("the fabricated OPEN does not decode: %v", err)
	}
	if parsed.Version != 4 {
		t.Errorf("Version = %d, want 4", parsed.Version)
	}
	if parsed.BGPIdentifier != 0x01020305 {
		t.Errorf("BGP Identifier = %#x, want the router-id 0x01020305", parsed.BGPIdentifier)
	}

	caps, err := capability.ParseFromOptionalParams(parsed.OptionalParams, parsed.ExtendedParams)
	if err != nil {
		t.Fatalf("the fabricated OPEN's capabilities do not decode: %v", err)
	}

	var asn uint32
	families := map[family.Family]bool{}
	for _, capa := range caps {
		switch c := capa.(type) {
		case *capability.ASN4:
			asn = c.ASN
		case *capability.Multiprotocol:
			families[family.Family{AFI: c.AFI, SAFI: c.SAFI}] = true
		}
	}
	if asn != 4200000001 {
		t.Errorf("4-octet ASN capability = %d, want the router's own 4200000001", asn)
	}
	for _, fam := range dumpFamilies {
		if !families[fam] {
			t.Errorf("no Multiprotocol capability for %s, which the dump delivers", fam)
		}
	}
	if len(families) != len(dumpFamilies) {
		t.Errorf("the OPEN advertises %d families and the dump delivers %d: a capability that will not be used", len(families), len(dumpFamilies))
	}
}

func TestHandleBestChangeEmitsPeerUpThenRM(t *testing.T) {
	// VALIDATES: AC-2 -- a (replay) best-change batch emits a Loc-RIB Peer Up
	// (PeerType=3) followed by a Route Monitoring (PeerType=3) per best path.
	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")
	defer closeLog(client, "client-pipe")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	bp := &BMPPlugin{
		senders:   []*senderSession{ss},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
	}

	batch := &ribevents.BestChangeBatch{
		Protocol: "bgp",
		Family:   family.IPv4Unicast,
		ReplayID: 1, // replay batch
		Changes: []ribevents.BestChangeEntry{{
			Action:  ribevents.BestChangeAdd,
			Prefix:  netip.MustParsePrefix("10.20.30.0/24"),
			NextHop: netip.MustParseAddr("192.0.2.1"),
			ASPath:  []uint32{65001},
		}},
	}

	go bp.handleBestChange(batch)

	// First message MUST be the Loc-RIB Peer Up (RFC 9069: Peer Up precedes RM).
	pu, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("read peer up: %v", err)
	}
	up, ok := pu.(*PeerUp)
	if !ok {
		t.Fatalf("first message = %T, want *PeerUp", pu)
	}
	if up.Peer.PeerType != PeerTypeLocRIB {
		t.Errorf("Peer Up PeerType = %d, want %d (Loc-RIB)", up.Peer.PeerType, PeerTypeLocRIB)
	}
	if up.Peer.PeerBGPID != 0x01020305 {
		t.Errorf("Peer Up BGP ID = %#x, want local router-id 0x01020305", up.Peer.PeerBGPID)
	}
	// RFC requirement: RFC9069-x-3 positive -- the Loc-RIB Peer Up carries the fabricated
	// OPEN of RFC 9069 Section 5.2 in BOTH fields: "Received OPEN Message: Repeat of the
	// same sent OPEN message. The duplication allows the BMP receiver to parse the expected
	// received OPEN message as defined in Section 4.10 of [RFC7854]."
	if len(up.SentOpenMsg) == 0 {
		t.Fatal("RFC 9069 Section 5.2: the Loc-RIB Peer Up must carry a fabricated sent OPEN")
	}
	if !bytes.Equal(up.SentOpenMsg, up.ReceivedOpenMsg) {
		t.Errorf("the received OPEN is not a repeat of the sent one: %x vs %x", up.ReceivedOpenMsg, up.SentOpenMsg)
	}
	// The OPEN describes this router: the same 4-octet ASN the per-peer header carries.
	sent, err := message.UnpackOpen(up.SentOpenMsg[message.HeaderLen:])
	if err != nil {
		t.Fatalf("the Peer Up's sent OPEN does not decode: %v", err)
	}
	if sent.BGPIdentifier != 0x01020305 {
		t.Errorf("sent OPEN BGP Identifier = %#x, want the local router-id 0x01020305", sent.BGPIdentifier)
	}
	caps, err := capability.ParseFromOptionalParams(sent.OptionalParams, sent.ExtendedParams)
	if err != nil {
		t.Fatalf("the Peer Up's sent OPEN carries capabilities that do not decode: %v", err)
	}
	sawASN4, sawFamily := false, false
	for _, capa := range caps {
		switch capa.(type) {
		case *capability.ASN4:
			sawASN4 = true
		case *capability.Multiprotocol:
			sawFamily = true
		}
	}
	if !sawASN4 {
		t.Error("RFC 9069 Section 5.2: capabilities MUST include the 4-octet ASN")
	}
	if !sawFamily {
		t.Error("RFC 9069 Section 6.1.1: the OPEN must indicate the address family capabilities")
	}

	// Then a Route Monitoring with a PeerType=3 header and a valid UPDATE PDU.
	rm, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("read route monitoring: %v", err)
	}
	mon, ok := rm.(*RouteMonitoring)
	if !ok {
		t.Fatalf("second message = %T, want *RouteMonitoring", rm)
	}
	if mon.Peer.PeerType != PeerTypeLocRIB {
		t.Errorf("RM PeerType = %d, want %d (Loc-RIB)", mon.Peer.PeerType, PeerTypeLocRIB)
	}
	// The embedded PDU is a complete BGP UPDATE (marker + length + type).
	if len(mon.BGPUpdate) < message.HeaderLen {
		t.Fatalf("BGPUpdate too short: %d bytes", len(mon.BGPUpdate))
	}
	if mon.BGPUpdate[message.MarkerLen+2] != byte(msgtype.TypeUPDATE) {
		t.Errorf("embedded PDU type = %d, want %d (UPDATE)", mon.BGPUpdate[message.MarkerLen+2], msgtype.TypeUPDATE)
	}
}

// Follow-up, same day: the widening was superseded and the read is back at
// three. An End-of-RIB marker is now emitted only for a dump this plugin
// requested (bmp_locrib.go handleBestChange, ourDump), and this test calls
// handleBestChange directly, which is the shape of a replay another subsystem
// asked for. So this stream is Peer Up, RM(batchA), RM(batchB) again -- exactly
// what it read before -- and the vacuity the widening existed to fix was
// removed at its source instead. Verified by mutation: breaking the
// one-Peer-Up guard makes this fail on peerUps=2.
// RFC requirement: RFC9069-x-2 positive -- Loc-RIB monitoring is per RIB instance, not per
// BGP peer: two best-change batches standing in for best paths selected from two different
// peers emit exactly ONE Loc-RIB Peer Up (the one-shot BMPPlugin.locRIBUp guard), never one
// Peer Up per peer.
func TestLocRIBSinglePeerUpPerInstance(t *testing.T) {
	// VALIDATES: RFC 9069 -- exactly one Loc-RIB Peer Up per RIB instance, independent of
	// how many BGP peers contribute best paths. Reads the WHOLE stream both batches
	// produce, so a second Peer Up cannot hide behind a read that stops inside batch A.
	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")
	defer closeLog(client, "client-pipe")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	bp := &BMPPlugin{
		senders:   []*senderSession{ss},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
	}

	// Two batches with distinct prefixes and next-hops, standing in for best paths selected
	// from two different BGP peers.
	batchA := &ribevents.BestChangeBatch{
		Protocol: "bgp",
		Family:   family.IPv4Unicast,
		ReplayID: 1,
		Changes: []ribevents.BestChangeEntry{{
			Action:  ribevents.BestChangeAdd,
			Prefix:  netip.MustParsePrefix("10.20.30.0/24"),
			NextHop: netip.MustParseAddr("192.0.2.1"),
			ASPath:  []uint32{65001},
		}},
	}
	batchB := &ribevents.BestChangeBatch{
		Protocol: "bgp",
		Family:   family.IPv4Unicast,
		ReplayID: 1,
		Changes: []ribevents.BestChangeEntry{{
			Action:  ribevents.BestChangeAdd,
			Prefix:  netip.MustParsePrefix("10.40.50.0/24"),
			NextHop: netip.MustParseAddr("192.0.2.2"),
			ASPath:  []uint32{65002},
		}},
	}

	// net.Pipe is unbuffered, so each write blocks until the reader below drains it. Both
	// batches are processed in order in one goroutine.
	go func() {
		bp.handleBestChange(batchA)
		bp.handleBestChange(batchB)
	}()

	// The instance emits: Peer Up, RM(batchA), RM(batchB) -- exactly one Peer Up despite
	// two peers' worth of best changes, and no End-of-RIB, because a batch delivered to
	// handleBestChange without this plugin having requested the dump is somebody else's
	// replay. A per-peer regression puts a second Peer Up at index 2, inside this read.
	// The read deadline turns a regression (a second Peer Up, or a missing message) into
	// a clear failure instead of a hang.
	_ = server.SetReadDeadline(time.Now().Add(3 * time.Second))
	var peerUps, routeMons, endOfRIBs int
	for i := range 3 {
		msg, err := readBMPFromPipe(server)
		if err != nil {
			t.Fatalf("read message %d: %v", i, err)
		}
		switch m := msg.(type) {
		case *PeerUp:
			peerUps++
			if m.Peer.PeerType != PeerTypeLocRIB {
				t.Errorf("Peer Up PeerType = %d, want %d (Loc-RIB)", m.Peer.PeerType, PeerTypeLocRIB)
			}
		case *RouteMonitoring:
			// An End-of-RIB marker is a Route Monitoring too, so it is counted apart from
			// the best changes rather than passing for one of them.
			if isEndOfRIB(m) {
				endOfRIBs++
			} else {
				routeMons++
			}
		default:
			t.Fatalf("unexpected message type %T", msg)
		}
	}

	if peerUps != 1 {
		t.Errorf("Loc-RIB Peer Up count = %d, want exactly 1 per RIB instance (RFC 9069: per-instance, not per-peer)", peerUps)
	}
	if routeMons != 2 {
		t.Errorf("Route Monitoring count = %d, want 2 (one per best change)", routeMons)
	}
	if endOfRIBs != 0 {
		t.Errorf("End-of-RIB count = %d, want 0: this plugin did not request this replay", endOfRIBs)
	}
	bp.mu.RLock()
	up := bp.locRIBUp
	bp.mu.RUnlock()
	if !up {
		t.Error("locRIBUp guard should be set once, after the single Loc-RIB Peer Up")
	}
}

// RFC requirement: RFC9069-x-4 positive -- starting Loc-RIB monitoring triggers a full-table
// dump: startLocRIB broadcasts a replay-request (ReplayID = replay.Broadcast) so the RIB
// re-emits its entire best-path table as Loc-RIB Route Monitoring.
func TestStartLocRIBTriggersInitialDump(t *testing.T) {
	// VALIDATES: RFC 9069 -- on Loc-RIB monitoring start ze requests a full-table replay.
	bus := newLocRIBTestBus()
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })

	var got []*replay.Request
	unsub := ribevents.ReplayRequest.Subscribe(bus, func(r *replay.Request) {
		got = append(got, r)
	})
	defer unsub()

	bp := &BMPPlugin{}
	bp.startLocRIB()
	t.Cleanup(bp.stopLocRIB)

	if len(got) != 1 {
		t.Fatalf("startLocRIB emitted %d replay-request(s), want exactly 1 (RFC 9069 initial full-table dump)", len(got))
	}
	if got[0] == nil {
		t.Fatal("replay-request payload is nil")
	}
	if !replay.IsReplay(got[0].ReplayID) {
		t.Errorf("replay-request ReplayID = %d, want a replay token (RFC 9069: initial dump must request a full-table replay)", got[0].ReplayID)
	}

	second := bp.dumpToken(t, bus)
	if !replay.IsReplay(second) {
		t.Errorf("second dump's ReplayID = %d, want a replay token", second)
	}
	if second == got[0].ReplayID {
		t.Errorf("both dumps used ReplayID %#x; per-dump tokens must differ, or a batch "+
			"cannot be attributed to the dump that asked for it", second)
	}
}

// dumpToken runs one more dump on bus and returns the token it requested.
func (bp *BMPPlugin) dumpToken(t *testing.T, bus *locRIBTestBus) uint64 {
	t.Helper()
	var seen []*replay.Request
	unsub := ribevents.ReplayRequest.Subscribe(bus, func(r *replay.Request) { seen = append(seen, r) })
	defer unsub()
	if err := bp.emitReplayRequest(bus, nil); err != nil {
		t.Fatalf("second dump request: %v", err)
	}
	if len(seen) != 1 {
		t.Fatalf("second dump emitted %d replay-request(s), want 1", len(seen))
	}
	return seen[0].ReplayID
}

// locRIBTestBus is a minimal in-memory ze.EventBus for exercising the Loc-RIB replay-request
// path: it delivers Emit synchronously to matching Subscribe handlers (mirrors the testBus in
// redistribute_egress/redistribute_test.go).
type locRIBTestBus struct {
	mu   sync.Mutex
	subs []*locRIBTestSub
}

type locRIBTestSub struct {
	ns, et  string
	handler func(any)
}

func newLocRIBTestBus() *locRIBTestBus { return &locRIBTestBus{} }

func (b *locRIBTestBus) Emit(ns, et string, payload any) (int, error) {
	b.mu.Lock()
	hs := make([]func(any), 0, len(b.subs))
	for _, s := range b.subs {
		if s.ns == ns && s.et == et {
			hs = append(hs, s.handler)
		}
	}
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return 0, nil
}

func (b *locRIBTestBus) Subscribe(ns, et string, handler func(any)) func() {
	if handler == nil {
		return func() {}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s := &locRIBTestSub{ns: ns, et: et, handler: handler}
	b.subs = append(b.subs, s)
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, ss := range b.subs {
			if ss == s {
				b.subs = append(b.subs[:i], b.subs[i+1:]...)
				return
			}
		}
	}
}

// TestMixedFamilyDumpClosesTheSilentFamily proves the marker is owed per family,
// not per dump.
//
// VALIDATES: a dump whose RIB answers for ONE family still closes the other.
// RFC 4724 Section 4: the End-of-RIB marker "MUST be sent by a BGP speaker to
// its peer once it completes the initial routing update (including the case
// when there is no update to send) for an address family". RFC 7854 Section 5
// makes the BMP initial dump's completion "MUST be indicated by sending an
// End-of-RIB marker for that peer (as specified in Section 2 of [RFC4724])",
// importing that per-<AFI, SAFI> definition.
// PREVENTS: the mixed-table gap. replayBestPaths publishes NO batch for a family
// with no best paths (rib_bestchange.go:1193-1196), so on an IPv6-only table the
// IPv4 marker was never sent and a collector waiting on IPv4 waited forever for
// a dump that had already finished.
func TestMixedFamilyDumpClosesTheSilentFamily(t *testing.T) {
	conn := newRecordingConn()
	ss := newTestSession(t, "mixed-family", conn)
	bp := &BMPPlugin{
		senders:   []*senderSession{ss},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
	}

	// The stand-in RIB answers with IPv6 only -- IPv4 is empty, so it stays
	// silent about it exactly as the real replayBestPaths does.
	bus := dumpBus(t, func() *ribevents.BestChangeBatch {
		return &ribevents.BestChangeBatch{
			Protocol: "bgp",
			Family:   family.IPv6Unicast,
			ReplayID: replay.Broadcast,
			Changes: []ribevents.BestChangeEntry{{
				Action:  ribevents.BestChangeAdd,
				Prefix:  netip.MustParsePrefix("2001:db8::/32"),
				NextHop: netip.MustParseAddr("2001:db8::1"),
				ASPath:  []uint32{65001},
			}},
		}
	})

	// startLocRIB subscribes AND runs a first dump; drain and reset so the
	// counts below describe exactly one dump.
	bp.startLocRIB()
	t.Cleanup(bp.stopLocRIB)
	waitQueueDrained(t, ss)
	conn.reset()

	if err := bp.emitReplayRequest(bus, ss); err != nil {
		t.Fatalf("dump request: %v", err)
	}
	waitQueueDrained(t, ss)

	var v4Markers, v6Markers, routes int
	for _, m := range decodeBMPStream(t, conn.written()) {
		rm, ok := m.(*RouteMonitoring)
		if !ok {
			continue
		}
		if !isEndOfRIB(rm) {
			routes++
			continue
		}
		// The IPv4 unicast marker is the minimum-length UPDATE; any other
		// family's is an MP_UNREACH_NLRI carrying its AFI/SAFI (RFC 4724 S2).
		if bytes.Equal(rm.BGPUpdate[message.HeaderLen:], []byte{0, 0, 0, 0}) {
			v4Markers++
		} else {
			v6Markers++
		}
	}

	if routes != 1 {
		t.Errorf("Route Monitoring count = %d, want 1 (the single IPv6 best path)", routes)
	}
	if v6Markers != 1 {
		t.Errorf("IPv6 End-of-RIB count = %d, want 1 (the family the RIB answered for)", v6Markers)
	}
	if v4Markers != 1 {
		t.Errorf("IPv4 End-of-RIB count = %d, want 1: RFC 4724 S4 owes a marker for a family "+
			"even when there is no update to send, and a collector waiting on IPv4 otherwise waits forever", v4Markers)
	}
}

// TestForeignReplayIsNotClaimedAsOurDump is the reason the per-dump correlation
// token exists.
//
// VALIDATES: a replay batch somebody ELSE asked for, landing while this
// plugin's own dump is in flight, is treated as ordinary routes: it fans out to
// every connected collector and is NOT closed with an End-of-RIB marker.
// PREVENTS: the mis-claim. Claiming on "a dump of ours is in flight" rather than
// on the token meant a sysrib-initiated replay (sysrib.go:898, emitted from its
// own goroutine, overlapping ours because a large dump takes seconds) was taken
// as ours: senders narrowed to the one collector the dump was addressed to, so
// every OTHER collector silently lost the batch, and the family was closed with
// an End-of-RIB -- which RFC 7854 Section 5 defines as "the initial dump is
// completed for a given peer", told to collectors that had requested nothing.
func TestForeignReplayIsNotClaimedAsOurDump(t *testing.T) {
	bus := newLocRIBTestBus()
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })

	// The stand-in RIB answers our request with TWO batches, delivered
	// synchronously inside our Emit and therefore both inside the window where
	// our dump scope is published: somebody else's broadcast replay first, then
	// ours carrying our token back.
	unsubRIB := ribevents.ReplayRequest.Subscribe(bus, func(req *replay.Request) {
		foreign := &ribevents.BestChangeBatch{
			Protocol: "bgp",
			Family:   family.IPv4Unicast,
			ReplayID: replay.Broadcast, // sysrib's token, not ours
			Changes: []ribevents.BestChangeEntry{{
				Action:  ribevents.BestChangeAdd,
				Prefix:  netip.MustParsePrefix("10.99.0.0/24"),
				NextHop: netip.MustParseAddr("192.0.2.9"),
				ASPath:  []uint32{65009},
			}},
		}
		if _, err := ribevents.BestChange.Emit(bus, foreign); err != nil {
			t.Errorf("foreign replay emit: %v", err)
		}
		ours := &ribevents.BestChangeBatch{
			Protocol: "bgp",
			Family:   family.IPv6Unicast,
			ReplayID: req.ReplayID, // echoed, as the real RIB does
			Changes: []ribevents.BestChangeEntry{{
				Action:  ribevents.BestChangeAdd,
				Prefix:  netip.MustParsePrefix("2001:db8::/32"),
				NextHop: netip.MustParseAddr("2001:db8::1"),
				ASPath:  []uint32{65001},
			}},
		}
		if _, err := ribevents.BestChange.Emit(bus, ours); err != nil {
			t.Errorf("our replay emit: %v", err)
		}
	})
	defer unsubRIB()

	connA, connB := newRecordingConn(), newRecordingConn()
	ssA := newTestSession(t, "dump-target", connA)
	ssB := newTestSession(t, "other-collector", connB)
	bp := &BMPPlugin{
		senders:   []*senderSession{ssA, ssB},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
	}

	bp.startLocRIB()
	t.Cleanup(bp.stopLocRIB)
	waitQueueDrained(t, ssA)
	waitQueueDrained(t, ssB)
	connA.reset()
	connB.reset()

	// A dump addressed to ssA alone.
	if err := bp.emitReplayRequest(bus, ssA); err != nil {
		t.Fatalf("dump request: %v", err)
	}
	waitQueueDrained(t, ssA)
	waitQueueDrained(t, ssB)

	countB := func(conn *recordingConn) (routes, eors int) {
		for _, m := range decodeBMPStream(t, conn.written()) {
			if rm, ok := m.(*RouteMonitoring); ok {
				if isEndOfRIB(rm) {
					eors++
				} else {
					routes++
				}
			}
		}
		return routes, eors
	}

	routesB, eorsB := countB(connB)
	if routesB != 1 {
		t.Errorf("other collector got %d routes, want 1: the foreign replay is not our dump and "+
			"must still fan out to every collector", routesB)
	}
	if eorsB != 0 {
		t.Errorf("other collector got %d End-of-RIB markers, want 0: it requested no dump, and a "+
			"marker tells it one of ITS dumps completed (RFC 7854 S5)", eorsB)
	}

	routesA, eorsA := countB(connA)
	if routesA != 2 {
		t.Errorf("dump target got %d routes, want 2 (the foreign one plus its own dump's)", routesA)
	}
	if eorsA != 2 {
		t.Errorf("dump target got %d End-of-RIB markers, want 2 (its own dump closes IPv4 and IPv6, "+
			"and the foreign batch closes nothing)", eorsA)
	}
}
