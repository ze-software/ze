package bmp

import (
	"bytes"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/msgtype"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/ribevents"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/wire"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/replay"
)

// rfc-test-change-approved: 2026-07-22 Thomas approved the msgtype/routeaction
// package rename (spec-feature-gate-10-bgp). MessageType/Type* moved to
// internal/core/bgp/msgtype and the route-action enum to
// internal/core/bgp/routeaction so MRT, sysrib and the FIB backends keep
// compiling when the BGP engine is compiled out (//go:build ze_bgp). Every hunk
// in this file is a package-qualifier requalification: no assertion was added,
// removed, reworded, weakened or re-tagged, verified by normalising the diff
// under the renaming and confirming the add/delete multisets cancel.

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
	// VALIDATES: RFC 9069 PeerType=3 header: Address/AS zero, BGP ID=router-id,
	// and Flags=0 (F=0 in-Loc-RIB; V/L/A/O MUST NOT be set).
	ph := locRIBPeerHeader(0x0a141e01)

	if ph.PeerType != PeerTypeLocRIB {
		t.Errorf("PeerType = %d, want %d (Loc-RIB)", ph.PeerType, PeerTypeLocRIB)
	}
	// RFC requirement: RFC9069-x-1 positive -- the Loc-RIB PeerType=3 per-peer header is
	// built with Flags 0, so the V/L/A/O flags are never set (only F, whose 0 value means
	// "route is in the Loc-RIB").
	if ph.Flags != 0 {
		t.Errorf("Flags = %#x, want 0 (RFC 9069: V/L/A/O MUST be 0)", ph.Flags)
	}
	// RFC requirement: RFC9069-x-6 positive -- the Loc-RIB per-peer header carries Peer AS 0.
	if ph.PeerAS != 0 {
		t.Errorf("PeerAS = %d, want 0 (RFC 9069)", ph.PeerAS)
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

func TestBgpIdentifierFromSentOpen(t *testing.T) {
	// VALIDATES: the local router-id is extracted from a sent OPEN's BGP
	// Identifier (RFC 4271 offset 24).
	open := makeBGPOpen(65000, 0x01020305)
	id, ok := bgpIdentifierFromSentOpen(open)
	if !ok {
		t.Fatal("expected to extract BGP Identifier from a full OPEN")
	}
	if id != 0x01020305 {
		t.Errorf("BGP Identifier = %#x, want 0x01020305", id)
	}

	if _, ok := bgpIdentifierFromSentOpen(open[:20]); ok {
		t.Error("a truncated OPEN must not yield a BGP Identifier")
	}
	if _, ok := bgpIdentifierFromSentOpen(nil); ok {
		t.Error("a nil OPEN must not yield a BGP Identifier")
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
	// RFC requirement: RFC9069-x-3 positive -- the Loc-RIB Peer Up carries zero-length sent
	// and received OPEN messages.
	if len(up.SentOpenMsg) != 0 || len(up.ReceivedOpenMsg) != 0 {
		t.Error("RFC 9069: Loc-RIB Peer Up OPENs must be zero-length")
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

// RFC requirement: RFC9069-x-2 positive -- Loc-RIB monitoring is per RIB instance, not per
// BGP peer: two best-change batches standing in for best paths selected from two different
// peers emit exactly ONE Loc-RIB Peer Up (the one-shot BMPPlugin.locRIBUp guard), never one
// Peer Up per peer.
func TestLocRIBSinglePeerUpPerInstance(t *testing.T) {
	// VALIDATES: RFC 9069 -- exactly one Loc-RIB Peer Up per RIB instance, independent of
	// how many BGP peers contribute best paths.
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

	// The instance emits: Peer Up, RM(batchA), RM(batchB) -- exactly one Peer Up despite two
	// peers' worth of best changes. The read deadline turns a regression (a second Peer Up,
	// or a missing message) into a clear failure instead of a hang.
	_ = server.SetReadDeadline(time.Now().Add(3 * time.Second))
	var peerUps, routeMons int
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
			routeMons++
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
	if got[0].ReplayID != replay.Broadcast {
		t.Errorf("replay-request ReplayID = %#x, want replay.Broadcast %#x (full-table dump)", got[0].ReplayID, replay.Broadcast)
	}
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
