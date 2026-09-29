// Design: docs/guide/bmp.md -- Loc-RIB monitoring and the receiver's per-peer families.
// RFC: rfc/short/rfc9069.md

package bmp

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// openWithOneFamily builds a BGP OPEN whose only optional parameter is one
// Multiprotocol capability for fam.
func openWithOneFamily(asn uint16, routerID uint32, fam family.Family) []byte {
	buf := makeBGPOpen(asn, routerID)
	buf[28] = 8                                             // optional parameters length
	buf = append(buf, 2, 6, 1, 4, 0, 0, 0, uint8(fam.SAFI)) // capability parameter, MP, length 4
	binary.BigEndian.PutUint16(buf[33:35], uint16(fam.AFI))
	binary.BigEndian.PutUint16(buf[16:18], uint16(len(buf)))
	return buf
}

// RFC requirement: RFC9069-6.1.1-1 positive -- two peers of ONE router whose Peer
// Up OPENs advertise different families are recorded each with its own family:
// the receiver keys the association on the peer, not on the router.
//
// VALIDATES: which peer belongs to which address family, with a second peer
// on the same router that a router-keyed store would overwrite or merge.
// Method: two Peer Ups on one router, read back through `show bmp peers`.
func TestReceiverKeepsEachPeersAddressFamiliesApart(t *testing.T) {
	bp := &BMPPlugin{state: newBMPState()}
	bp.state.addRouter("10.0.0.3")

	peers := []struct {
		id  uint32
		fam family.Family
	}{
		{0x0a000001, family.IPv4Unicast},
		{0x0a000002, family.IPv6Unicast},
	}
	for i, p := range peers {
		header := PeerHeader{PeerType: PeerTypeGlobal, PeerAS: 65001, PeerBGPID: p.id}
		header.Address[15] = uint8(i + 1)
		open := openWithOneFamily(65001, p.id, p.fam)
		bp.processPeerUp("10.0.0.3", &PeerUp{Peer: header, SentOpenMsg: open, ReceivedOpenMsg: open})
	}

	_, payload, err := bp.state.peersCommand()
	if err != nil {
		t.Fatalf("peers command: %v", err)
	}
	fields, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("peers payload = %T, want map[string]any", payload)
	}
	recorded, ok := fields["peers"].([]monitoredPeer)
	if !ok {
		t.Fatalf("peers field = %T, want []monitoredPeer", fields["peers"])
	}
	if len(recorded) != len(peers) {
		t.Fatalf("recorded %d peers, want %d", len(recorded), len(peers))
	}
	for _, p := range peers {
		id := netip.AddrFrom4([4]byte{byte(p.id >> 24), byte(p.id >> 16), byte(p.id >> 8), byte(p.id)}).String()
		idx := slices.IndexFunc(recorded, func(m monitoredPeer) bool { return m.PeerBGPID == id })
		if idx < 0 {
			t.Errorf("no peer recorded with BGP ID %s (recorded %+v)", id, recorded)
			continue
		}
		if !slices.Equal(recorded[idx].Families, []string{p.fam.String()}) {
			t.Errorf("peer %s families = %v, want [%s]", id, recorded[idx].Families, p.fam)
		}
	}
}

// RFC requirement: RFC9069-5.2-1 positive -- the fabricated OPEN carries exactly
// one 4-octet ASN capability, one Multiprotocol capability per family the
// Loc-RIB dump delivers, and one Extended Next Hop capability naming only the
// IPv4 unicast with IPv6 next-hop pair the Route Monitoring encodes, and NO
// other capability: Route Refresh, ADD-PATH, Graceful Restart or any other code
// would be a capability "not used for Loc-RIB monitoring messages".
//
// VALIDATES: the "Only include capabilities if they will be used" clause over
// every capability code, not only Multiprotocol. Method: decode the OPEN and
// classify every capability it carries.
func TestFabricatedLocRIBOpenCarriesOnlyCapabilitiesTheRouteMonitoringUses(t *testing.T) {
	open := fabricateLocRIBOpen(localIdentity{asn: 4200000001, routerID: 0x01020305})
	parsed, err := message.UnpackOpen(open[message.HeaderLen:])
	if err != nil {
		t.Fatalf("the fabricated OPEN does not decode: %v", err)
	}
	caps, err := capability.ParseFromOptionalParams(parsed.OptionalParams, parsed.ExtendedParams)
	if err != nil {
		t.Fatalf("the fabricated OPEN's capabilities do not decode: %v", err)
	}
	asn4 := 0
	var families []family.Family
	var nextHops [][]capability.ExtendedNextHopFamily
	for _, capa := range caps {
		switch c := capa.(type) {
		case *capability.ASN4:
			asn4++
		case *capability.Multiprotocol:
			families = append(families, family.Family{AFI: c.AFI, SAFI: c.SAFI})
		case *capability.ExtendedNextHop:
			nextHops = append(nextHops, c.Families)
		default:
			t.Errorf("the fabricated OPEN carries capability %T, which Loc-RIB monitoring never uses", capa)
		}
	}
	if asn4 != 1 {
		t.Errorf("4-octet ASN capabilities = %d, want 1", asn4)
	}
	onlyPair := []capability.ExtendedNextHopFamily{{NLRIAFI: 1, NLRISAFI: 1, NextHopAFI: 2}}
	if len(nextHops) != 1 || !slices.Equal(nextHops[0], onlyPair) {
		t.Errorf("Extended Next Hop capabilities = %v, want exactly one naming %v", nextHops, onlyPair)
	}
	if !slices.Equal(families, dumpFamilies[:]) {
		t.Errorf("Multiprotocol families = %v, want exactly the dump's %v", families, dumpFamilies)
	}
}

// RFC requirement: RFC9069-5.2-1 positive -- an IPv4 best path whose next hop is
// IPv6 is conveyed in MP_REACH_NLRI with AFI 1 and a 16-octet next hop, which a
// receiver can only decode under the Extended Next Hop capability of RFC 8950,
// so the fabricated OPEN carries that capability for exactly the pair the
// Route Monitoring uses: NLRI AFI 1, SAFI 1, next-hop AFI 2.
//
// VALIDATES: "all necessary capabilities to represent the Loc-RIB Route
// Monitoring messages" for the one Route Monitoring shape the 4-octet ASN and
// Multiprotocol capabilities do not cover. Method: encode the route with the
// Loc-RIB encoder, read the MP_REACH_NLRI octets, then decode the OPEN.
func TestLocRIBIPv4RouteWithIPv6NextHopIsBackedByExtendedNextHop(t *testing.T) {
	body := buildLocRIBUpdateBody(family.IPv4Unicast, &ribevents.BestChangeEntry{
		Action:  routeaction.Add,
		Prefix:  netip.MustParsePrefix("192.0.2.0/24"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
	})
	if len(body) < 4 {
		t.Fatalf("UPDATE body = %x, too short", body)
	}
	attrs := body[4 : 4+int(binary.BigEndian.Uint16(body[2:4]))]
	var mpReach []byte
	for off := 0; off+3 <= len(attrs); {
		flags, code := attrs[off], attrs[off+1]
		hdr, length := 3, int(attrs[off+2])
		if flags&0x10 != 0 {
			hdr, length = 4, int(binary.BigEndian.Uint16(attrs[off+2:off+4]))
		}
		if code == 14 {
			mpReach = attrs[off+hdr : off+hdr+length]
		}
		off += hdr + length
	}
	if len(mpReach) < 4 {
		t.Fatalf("no MP_REACH_NLRI in the IPv4 route with an IPv6 next hop: attrs %x", attrs)
	}
	if afi, nhLen := binary.BigEndian.Uint16(mpReach[0:2]), mpReach[3]; afi != 1 || nhLen != 16 {
		t.Fatalf("MP_REACH_NLRI AFI %d next-hop length %d, want AFI 1 with a 16-octet IPv6 next hop", afi, nhLen)
	}

	open := fabricateLocRIBOpen(localIdentity{asn: 65001, routerID: 0x01020305})
	parsed, err := message.UnpackOpen(open[message.HeaderLen:])
	if err != nil {
		t.Fatalf("the fabricated OPEN does not decode: %v", err)
	}
	caps, err := capability.ParseFromOptionalParams(parsed.OptionalParams, parsed.ExtendedParams)
	if err != nil {
		t.Fatalf("the fabricated OPEN's capabilities do not decode: %v", err)
	}
	want := capability.ExtendedNextHopFamily{NLRIAFI: 1, NLRISAFI: 1, NextHopAFI: 2}
	for _, capa := range caps {
		if enh, ok := capa.(*capability.ExtendedNextHop); ok && slices.Contains(enh.Families, want) {
			return
		}
	}
	t.Errorf("the fabricated OPEN carries no Extended Next Hop capability for %+v, which the IPv4 route with an IPv6 next hop needs", want)
}

// RFC requirement: RFC9069-6.1.1-2 positive -- the Peer Up of the Loc-RIB
// emulated peer, as the collector reads it, carries an OPEN (both OPEN fields)
// whose Multiprotocol capabilities indicate exactly the address families the
// peer's Route Monitoring carries.
//
// VALIDATES: the sender half of Section 6.1.1 on the octets a collector reads.
// Method: one best change drives the Loc-RIB Peer Up onto a pipe.
func TestLocRIBPeerUpOpenIndicatesTheAddressFamilies(t *testing.T) {
	bp, conn := locRIBTestPlugin(t)
	bp.handleBestChange(oneBestChange(0))
	up, _ := readLocRIBPeerUpThenRM(t, conn)
	if up.Peer.PeerType != PeerTypeLocRIB {
		t.Fatalf("peer type = %d, want %d (Loc-RIB)", up.Peer.PeerType, PeerTypeLocRIB)
	}
	for name, open := range map[string][]byte{"sent": up.SentOpenMsg, "received": up.ReceivedOpenMsg} {
		got := openMultiprotocolFamilies(open)
		if !slices.Equal(got, dumpFamilies[:]) {
			t.Errorf("%s OPEN of the Loc-RIB Peer Up indicates %v, want %v", name, got, dumpFamilies)
		}
	}
}

// RFC requirement: RFC9069-x-4 positive -- starting Loc-RIB monitoring puts the
// initial synchronization on the collector's wire as Route Monitoring messages:
// after the Loc-RIB Peer Up, the route the RIB replays arrives as a Route
// Monitoring of Peer Type 3 carrying that prefix.
//
// VALIDATES: the dump reaches the collector, not only the replay-request.
// Method: a stand-in RIB answers the replay-request startLocRIB emits with one
// best change carrying the request's token, and the collector end of a pipe reads
// what follows.
func TestStartLocRIBDeliversTheInitialDumpAsRouteMonitoring(t *testing.T) {
	bp, conn := locRIBTestPlugin(t)
	bus := newLocRIBTestBus()
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })

	unsub := ribevents.ReplayRequest.Subscribe(bus, func(r *replay.Request) {
		if _, err := ribevents.BestChange.Emit(bus, oneBestChange(r.ReplayID)); err != nil {
			t.Errorf("stand-in RIB failed to emit the replay batch: %v", err)
		}
	})
	defer unsub()

	bp.startLocRIB()
	t.Cleanup(bp.stopLocRIB)

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, mon := readLocRIBPeerUpThenRM(t, conn)
	if mon.Peer.PeerType != PeerTypeLocRIB {
		t.Errorf("dump Route Monitoring peer type = %d, want %d", mon.Peer.PeerType, PeerTypeLocRIB)
	}
	prefix := []byte{24, 10, 20, 30}
	if !bytes.Contains(mon.BGPUpdate[message.HeaderLen:], prefix) {
		t.Errorf("dump Route Monitoring does not carry 10.20.30.0/24: % X", mon.BGPUpdate)
	}
}

// RFC requirement: RFC9069-4.2-1 positive -- a locally sourced route (one this
// router originated, so its AS_PATH is empty) that the Loc-RIB feed carries
// reaches the collector as a Route Monitoring of Peer Type 3, after a Peer Up of
// Peer Type 3.
//
// VALIDATES: the locally sourced case itself, not a BGP-learned route. Method:
// one best change with no AS_PATH, read off the collector end of a pipe.
func TestLocallySourcedRouteIsConveyedWithTheLocRIBPeerType(t *testing.T) {
	bp, conn := locRIBTestPlugin(t)
	local := oneBestChange(0)
	local.Changes[0].ASPath = nil
	bp.handleBestChange(local)
	up, mon := readLocRIBPeerUpThenRM(t, conn)
	if up.Peer.PeerType != PeerTypeLocRIB {
		t.Errorf("Peer Up peer type = %d, want %d", up.Peer.PeerType, PeerTypeLocRIB)
	}
	if mon.Peer.PeerType != PeerTypeLocRIB {
		t.Errorf("Route Monitoring peer type = %d, want %d", mon.Peer.PeerType, PeerTypeLocRIB)
	}
	if !bytes.Contains(mon.BGPUpdate[message.HeaderLen:], []byte{24, 10, 20, 30}) {
		t.Errorf("Route Monitoring does not carry the locally sourced 10.20.30.0/24: % X", mon.BGPUpdate)
	}
}

// RFC requirement: RFC9069-4.2-1 negative -- the inputs are pushed toward the
// violation and the output still complies. The locally sourced route (empty
// AS_PATH) names as its next hop the address of a BGP peer the plugin monitors,
// the one fact a producer attributing a route to "the peer it came from" would
// use to convey it under that peer's Global Instance Peer header. The route
// still reaches the collector under the Loc-RIB Instance Peer Type with the
// router's own identity, and no message carrying it names the monitored peer.
//
// VALIDATES: no Route Monitoring conveys the locally sourced route under another
// peer type or another peer's identity. Method: one best change on a pipe, then
// the Peer Up and Route Monitoring headers compared field by field.
func TestLocallySourcedRouteIsNotConveyedUnderTheMonitoredPeer(t *testing.T) {
	bp, conn := locRIBTestPlugin(t)
	local := oneBestChange(0)
	local.Changes[0].ASPath = nil
	local.Changes[0].NextHop = netip.MustParseAddr("10.0.0.1") // the monitored peer in bp.openCache
	bp.handleBestChange(local)
	up, mon := readLocRIBPeerUpThenRM(t, conn)
	for name, peer := range map[string]PeerHeader{"Peer Up": up.Peer, "Route Monitoring": mon.Peer} {
		if peer.PeerType != PeerTypeLocRIB {
			t.Errorf("%s peer type = %d, want %d: the locally sourced route went out under another peer type", name, peer.PeerType, PeerTypeLocRIB)
		}
		if peer.Address != [16]byte{} {
			t.Errorf("%s peer address = % X, want zero: the locally sourced route is attributed to a peer", name, peer.Address)
		}
		if peer.PeerAS != 65000 || peer.PeerBGPID != 0x01020305 {
			t.Errorf("%s peer AS %d BGP ID %#x, want the router's own 65000 and 0x01020305", name, peer.PeerAS, peer.PeerBGPID)
		}
	}
	if !bytes.Contains(mon.BGPUpdate[message.HeaderLen:], []byte{24, 10, 20, 30}) {
		t.Errorf("Route Monitoring does not carry the locally sourced 10.20.30.0/24: % X", mon.BGPUpdate)
	}
}

// RFC requirement: RFC9069-6.1.3-1 positive -- a behavior change on a session
// that carries the Loc-RIB puts a Peer Down and then a Peer Up of the Loc-RIB
// emulated peer (Peer Type 3) on the wire: the emulated peer is bounced too.
//
// VALIDATES: the Loc-RIB half of the bounce. Method: the reload fixture with the
// Loc-RIB Peer Up already sent, one behavior leaf moved, and every message the
// collector reads classified by peer type.
func TestBehaviorChangeBouncesTheLocRIBEmulatedPeer(t *testing.T) {
	bp, conn, inForce := locRIBReloadPlugin(t)
	id := &localIdentity{asn: 65000, routerID: 0x01020305}
	inForce.identity = id
	bp.identity = id
	bp.locRIBUp = true
	bp.senders[0].locRIBUpSent.Store(true)

	changed := *inForce
	changed.RouteMonitoringPolicy = policyPostPolicy
	bp.applySenderConfig(inForce, &changed)

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	// The global peer's bounce and its snapshot interleave; read until the
	// Loc-RIB pair is complete, with a bound on the messages read.
	var locRIB []string
	for i := 0; i < 20 && len(locRIB) < 2; i++ {
		msg, err := readBMPFromPipe(conn)
		if err != nil {
			t.Fatalf("read after the change: %v (Loc-RIB messages so far %v)", err, locRIB)
		}
		switch m := msg.(type) {
		case *PeerDown:
			if m.Peer.PeerType == PeerTypeLocRIB {
				locRIB = append(locRIB, "down")
			}
		case *PeerUp:
			if m.Peer.PeerType == PeerTypeLocRIB {
				locRIB = append(locRIB, "up")
			}
		}
	}
	if !slices.Equal(locRIB, []string{"down", "up"}) {
		t.Errorf("Loc-RIB emulated peer messages = %v, want [down up]", locRIB)
	}
}

// RFC requirement: RFC8671-x-3 positive -- the Peer Down a session carries when a
// monitored peer goes down is the same, octet for octet apart from the
// timestamp, whether the session monitors the Adj-RIB-In (pre-policy), the
// Adj-RIB-Out (post-policy) or both: its presence, its per-peer header, its
// reason and its data do not depend on which RIBs route monitoring covers.
//
// VALIDATES: the Peer Down half of the sentence through the producer, under all
// three policies. Method: one plugin per policy, the same down event, and the
// Peer Down each collector reads compared with the first.
func TestRFC8671PeerDownDoesNotDependOnTheMonitoredRIB(t *testing.T) {
	var first *PeerDown
	for _, policy := range []string{policyPrePolicy, policyPostPolicy, policyAll} {
		server, client := net.Pipe()
		bp := &BMPPlugin{
			state:              newBMPState(),
			openCache:          make(map[string]*openPair),
			stopCh:             make(chan struct{}),
			routeMonitorPolicy: policy,
			senders:            []*senderSession{{name: "test", conn: client, stopCh: make(chan struct{})}},
		}
		result := asyncRead(server)
		bp.handleStructuredEvent(&rpc.StructuredEvent{
			PeerAddress: "10.0.0.1",
			PeerAS:      65001,
			EventType:   rpc.EventKindState,
			State:       rpc.SessionStateDown,
			Reason:      "notification",
		})
		r := <-result
		closeLog(server, "server")
		closeLog(client, "client")
		if r.err != nil {
			t.Fatalf("%s: read: %v", policy, r.err)
		}
		down, ok := r.msg.(*PeerDown)
		if !ok {
			t.Fatalf("%s: the session carries %T, want the Peer Down", policy, r.msg)
		}
		down.Peer.TimestampSec, down.Peer.TimestampUsec = 0, 0
		if first == nil {
			first = down
			continue
		}
		if down.Peer != first.Peer {
			t.Errorf("%s: per-peer header %+v differs from %+v", policy, down.Peer, first.Peer)
		}
		if down.Reason != first.Reason || !bytes.Equal(down.Data, first.Data) {
			t.Errorf("%s: Peer Down reason %d data %x differ from the %s session's reason %d data %x",
				policy, down.Reason, down.Data, policyPrePolicy, first.Reason, first.Data)
		}
	}
}

// RFC requirement: RFC8671-x-3 positive -- the Peer Up a new session carries for
// an Established peer is the same, octet for octet apart from the timestamp,
// whether the session monitors the Adj-RIB-In (pre-policy), the Adj-RIB-Out
// (post-policy) or both: its presence and its content do not depend on which
// RIBs route monitoring covers.
//
// VALIDATES: the independence through the producer, under all three policies.
// Method: prime one collector per policy over the same peer and compare the Peer Ups.
func TestRFC8671PeerUpDoesNotDependOnTheMonitoredRIB(t *testing.T) {
	var first *PeerUp
	for _, policy := range []string{policyPrePolicy, policyPostPolicy, policyAll} {
		bp := replayPlugin(t)
		bp.routeMonitorPolicy = policy
		_, conn := primeReplayCollector(t, bp)
		var up *PeerUp
		for _, m := range decodeBMPStream(t, conn.written()) {
			if u, ok := m.(*PeerUp); ok {
				if up != nil {
					t.Fatalf("%s: more than one Peer Up", policy)
				}
				up = u
			}
		}
		if up == nil {
			t.Fatalf("%s: the session carries no Peer Up for the Established peer", policy)
		}
		up.Peer.TimestampSec, up.Peer.TimestampUsec = 0, 0
		if first == nil {
			first = up
			continue
		}
		if up.Peer != first.Peer {
			t.Errorf("%s: per-peer header %+v differs from %+v", policy, up.Peer, first.Peer)
		}
		if !bytes.Equal(up.SentOpenMsg, first.SentOpenMsg) || !bytes.Equal(up.ReceivedOpenMsg, first.ReceivedOpenMsg) {
			t.Errorf("%s: the Peer Up OPENs differ from the %s session's", policy, policyPrePolicy)
		}
		if up.LocalAddress != first.LocalAddress || up.LocalPort != first.LocalPort || up.RemotePort != first.RemotePort {
			t.Errorf("%s: the Peer Up transport fields differ", policy)
		}
	}
}
