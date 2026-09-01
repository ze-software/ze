// RFC: rfc/short/rfc9069.md
//
// The RFC 9069 obligations a walk of the source text found and the summary did
// not declare: the per-peer Timestamp, the VRF/Table Name Information TLV, the
// peer type locally sourced routes are conveyed with, and the receiver's duty
// to read the Peer Up capabilities.

package bmp

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// locRIBTestPlugin returns a plugin with one piped collector session and one
// cached sent OPEN, which is what every Loc-RIB producer reads its identity
// from. The returned conn is the collector end.
func locRIBTestPlugin(t *testing.T) (*BMPPlugin, net.Conn) {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() {
		closeLog(server, "server-pipe")
		closeLog(client, "client-pipe")
	})
	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	bp := &BMPPlugin{
		senders:   []*senderSession{ss},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
	}
	return bp, server
}

// oneBestChange is one best-change batch carrying a single IPv4 best path.
// replayID 0 is an incremental change and any other value is a full-table
// replay (replay.IsReplay).
func oneBestChange(replayID uint64) *ribevents.BestChangeBatch {
	return &ribevents.BestChangeBatch{
		Protocol: "bgp",
		Family:   family.IPv4Unicast,
		ReplayID: replayID,
		Changes: []ribevents.BestChangeEntry{{
			Action:  ribevents.BestChangeAdd,
			Prefix:  netip.MustParsePrefix("10.20.30.0/24"),
			NextHop: netip.MustParseAddr("192.0.2.1"),
			ASPath:  []uint32{65001},
		}},
	}
}

// readLocRIBPeerUpThenRM reads the Peer Up and the Route Monitoring one
// best-change batch produces, in that order.
//
// The producer runs on the TEST goroutine and this reads afterwards, which the
// session's transmit queue makes possible: a producer enqueues and returns, and
// the session's own drain goroutine does the socket write (sender_drain.go
// enqueueLocked). Driving the producer on a goroutine of its own would put a
// panic inside it out of `go test`'s reach, and a break that reddens nothing it
// can attribute proves nothing (rfc/discrimination/README.md).
func readLocRIBPeerUpThenRM(t *testing.T, conn net.Conn) (*PeerUp, *RouteMonitoring) {
	t.Helper()
	first, err := readBMPFromPipe(conn)
	if err != nil {
		t.Fatalf("read peer up: %v", err)
	}
	up, ok := first.(*PeerUp)
	if !ok {
		t.Fatalf("first message = %T, want *PeerUp", first)
	}
	second, err := readBMPFromPipe(conn)
	if err != nil {
		t.Fatalf("read route monitoring: %v", err)
	}
	mon, ok := second.(*RouteMonitoring)
	if !ok {
		t.Fatalf("second message = %T, want *RouteMonitoring", second)
	}
	return up, mon
}

// RFC requirement: RFC9069-5.1-1 positive -- RFC 9069 Section 5.1: "Timestamp: The time
// when the encapsulated routes were installed in the Loc-RIB, expressed in seconds and
// microseconds since midnight (zero hour), January 1, 1970 (UTC)." An incremental best
// change is delivered on the goroutine that installed it, so the Route Monitoring carrying
// it reports that install: a timestamp inside the window this test brackets.
func TestLocRIBIncrementalRouteMonitoringCarriesTheInstallTime(t *testing.T) {
	// VALIDATES: RFC 9069 Section 5.1 -- an incremental Loc-RIB Route Monitoring
	// reports the time its routes entered the Loc-RIB.
	bp, conn := locRIBTestPlugin(t)

	before := time.Now().Add(-2 * time.Second).Unix()
	bp.handleBestChange(oneBestChange(0))
	_, mon := readLocRIBPeerUpThenRM(t, conn)
	after := time.Now().Add(2 * time.Second).Unix()

	got := int64(mon.Peer.TimestampSec)
	if got == 0 {
		t.Fatal("RFC 9069 Section 5.1: an incremental Loc-RIB Route Monitoring reports the install time, and zero says the time is unavailable")
	}
	if got < before || got > after {
		t.Errorf("TimestampSec = %d, want the install time within [%d, %d]", got, before, after)
	}
}

// RFC requirement: RFC9069-5.1-1 negative -- the same sentence's own answer for the case
// ze cannot report: "If zero, the time is unavailable." A full-table replay re-reads a
// table installed at times nobody recorded, and the Peer Up that precedes it encapsulates
// no route at all, so both carry zero. An implementation reading a wall clock into the
// field -- which is what ze did until 2026-09-01 -- passes the positive above and fails
// here, because it dates every replayed route to the moment the collector connected.
func TestLocRIBReplayAndPeerUpCarryNoTimestamp(t *testing.T) {
	// PREVENTS: a wall-clock read standing in for an install time nobody
	// recorded, which dates every replayed route to the collector's connect.
	bp, conn := locRIBTestPlugin(t)

	bp.handleBestChange(oneBestChange(1))
	up, mon := readLocRIBPeerUpThenRM(t, conn)

	if up.Peer.TimestampSec != 0 || up.Peer.TimestampUsec != 0 {
		t.Errorf("Peer Up Timestamp = (%d, %d), want (0, 0): a Peer Up encapsulates no route",
			up.Peer.TimestampSec, up.Peer.TimestampUsec)
	}
	if mon.Peer.TimestampSec != 0 || mon.Peer.TimestampUsec != 0 {
		t.Errorf("replay Route Monitoring Timestamp = (%d, %d), want (0, 0): the install time of a replayed route is unavailable",
			mon.Peer.TimestampSec, mon.Peer.TimestampUsec)
	}
}

// RFC requirement: RFC9069-4.2-1 positive -- RFC 9069 Section 4.2: "If locally sourced
// routes are communicated using BMP, they MUST be conveyed using the Loc-RIB Instance Peer
// Type." The Loc-RIB feed is the path that communicates them, and both messages it emits
// -- the Peer Up announcing the emulated peer and the Route Monitoring carrying the route
// -- carry Peer Type 3.
func TestLocRIBFeedConveysRoutesWithTheLocRIBPeerType(t *testing.T) {
	// VALIDATES: RFC 9069 Section 4.2 -- locally sourced routes reach a collector
	// under Peer Type 3.
	bp, conn := locRIBTestPlugin(t)

	bp.handleBestChange(oneBestChange(0))
	up, mon := readLocRIBPeerUpThenRM(t, conn)

	if up.Peer.PeerType != PeerTypeLocRIB {
		t.Errorf("Peer Up PeerType = %d, want %d (Loc-RIB Instance Peer)", up.Peer.PeerType, PeerTypeLocRIB)
	}
	if mon.Peer.PeerType != PeerTypeLocRIB {
		t.Errorf("Route Monitoring PeerType = %d, want %d (Loc-RIB Instance Peer)", mon.Peer.PeerType, PeerTypeLocRIB)
	}
}

// RFC requirement: RFC9069-4.2-1 negative -- RFC 9069 Section 4.2 reserves the peer type
// for locally sourced routes: "If locally sourced routes are communicated using BMP, they
// MUST be conveyed using the Loc-RIB Instance Peer Type." A monitored BGP peer's routes
// are not locally sourced, so the Route Monitoring carrying them reaches the collector
// under Peer Type 0. An implementation stamping Peer Type 3 on every message passes the
// positive above and fails here, having told the collector that every peer's Adj-RIB-In is
// the router's Loc-RIB.
func TestMonitoredPeerRouteMonitoringIsNotTheLocRIBPeerType(t *testing.T) {
	// VALIDATES: the Loc-RIB Instance Peer Type discriminates the Loc-RIB feed.
	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")
	defer closeLog(client, "client-pipe")

	bp := newPipeSender(client, false)
	bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, []byte{0x00, 0x00, 0x00, 0x00, 0x00}))

	msg, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("read route monitoring: %v", err)
	}
	mon, ok := msg.(*RouteMonitoring)
	if !ok {
		t.Fatalf("message = %T, want *RouteMonitoring", msg)
	}
	if mon.Peer.PeerType != PeerTypeGlobal {
		t.Errorf("a monitored peer's Route Monitoring carries PeerType %d, want %d: Peer Type 3 names the Loc-RIB instance peer",
			mon.Peer.PeerType, PeerTypeGlobal)
	}
}

// RFC requirement: RFC9069-5.2.1-1 positive -- RFC 9069 Section 5.2.1 registers "Type = 3:
// VRF/Table Name. The Information field contains a UTF-8 string whose value MUST be equal
// to the value of the VRF or table name (e.g., RD instance name) being conveyed. The
// string size MUST be within the range of 1 to 255 bytes", and fixes the name of the one
// instance ze runs: "The default value of "global" MUST be used for the default Loc-RIB
// instance with a zero-filled distinguisher." Section 5.3 carries the same TLV onto the
// Peer Down: "The VRF/Table Name informational TLV MUST be included if it was in the Peer
// Up." This drives the pair over one session and reads both off the wire.
func TestLocRIBPeerUpAndPeerDownCarryTheGlobalTableName(t *testing.T) {
	// VALIDATES: RFC 9069 Sections 5.2.1 and 5.3 -- the VRF/Table Name TLV, its
	// value for the default instance, and its repeat on the Peer Down.
	bp, conn := locRIBTestPlugin(t)

	bp.handleBestChange(oneBestChange(0))
	up, _ := readLocRIBPeerUpThenRM(t, conn)

	if up.Peer.Distinguisher != 0 {
		t.Errorf("Peer Distinguisher = %d, want 0: %q names the DEFAULT instance, the one with a zero-filled distinguisher",
			up.Peer.Distinguisher, locRIBTableName)
	}
	if len(up.InfoTLVs) != 1 {
		t.Fatalf("Peer Up carries %d Information TLV(s), want exactly the VRF/Table Name one", len(up.InfoTLVs))
	}
	assertTableNameTLV(t, "peer up", up.InfoTLVs[0])

	bp.sendLocRIBPeerDown()
	msg, err := readBMPFromPipe(conn)
	if err != nil {
		t.Fatalf("read peer down: %v", err)
	}
	down, ok := msg.(*PeerDown)
	if !ok {
		t.Fatalf("message = %T, want *PeerDown", msg)
	}
	if down.Reason != PeerDownTLVData {
		t.Fatalf("Peer Down reason = %d, want %d (local system closed, TLV data follows)", down.Reason, PeerDownTLVData)
	}
	tlvs, err := DecodeTLVs(down.Data, 0, len(down.Data))
	if err != nil {
		t.Fatalf("the Peer Down data does not decode as TLVs: %v", err)
	}
	if len(tlvs) != 1 {
		t.Fatalf("Peer Down carries %d TLV(s) after the reason, want the VRF/Table Name one that was in the Peer Up", len(tlvs))
	}
	assertTableNameTLV(t, "peer down", tlvs[0])
}

// assertTableNameTLV checks one TLV against every property RFC 9069 Section
// 5.2.1 states of the VRF/Table Name: the registered type, the UTF-8 value of
// the default instance, and the 1-to-255-byte size bound.
func assertTableNameTLV(t *testing.T, where string, tlv TLV) {
	t.Helper()
	if tlv.Type != PeerTLVVRFTableName {
		t.Errorf("%s TLV type = %d, want %d (VRF/Table Name)", where, tlv.Type, PeerTLVVRFTableName)
	}
	if string(tlv.Value) != locRIBTableName {
		t.Errorf("%s TLV value = %q, want %q (the default Loc-RIB instance name)", where, tlv.Value, locRIBTableName)
	}
	if len(tlv.Value) < 1 || len(tlv.Value) > 255 {
		t.Errorf("%s TLV value is %d bytes, want within 1 to 255", where, len(tlv.Value))
	}
	if int(tlv.Length) != len(tlv.Value) {
		t.Errorf("%s TLV length field = %d, value is %d bytes", where, tlv.Length, len(tlv.Value))
	}
}

// RFC requirement: RFC9069-5.2.1-1 negative -- the TLV names the LOC-RIB instance, so a
// monitored BGP peer's Peer Up carries none of it. RFC 9069 Section 5.2.1 sits under
// Section 5, "Loc-RIB Monitoring", and the name it fixes is "for the default Loc-RIB
// instance with a zero-filled distinguisher" -- an implementation appending the TLV to
// every Peer Up would tell a collector that a peer's Adj-RIB-In is the router's Loc-RIB.
// It passes the positive above and fails here.
func TestMonitoredPeerUpCarriesNoTableNameTLV(t *testing.T) {
	// VALIDATES: the VRF/Table Name TLV is scoped to the Loc-RIB instance peer.
	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")
	defer closeLog(client, "client-pipe")
	peerSS := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	peer := PeerHeader{PeerType: PeerTypeGlobal, PeerAS: 65001}

	if err := peerSS.writePeerUp(peer, [16]byte{}, 179, 54321, makeBGPOpen(65000, 1), makeBGPOpen(65001, 2)); err != nil {
		t.Fatalf("write peer up: %v", err)
	}
	msg, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("read peer up: %v", err)
	}
	up, ok := msg.(*PeerUp)
	if !ok {
		t.Fatalf("message = %T, want *PeerUp", msg)
	}
	for _, tlv := range up.InfoTLVs {
		if tlv.Type == PeerTLVVRFTableName {
			t.Errorf("a monitored peer's Peer Up carries a VRF/Table Name TLV %q: the name belongs to the Loc-RIB instance peer", tlv.Value)
		}
	}
}

// RFC requirement: RFC9069-6.1.1-1 positive -- RFC 9069 Section 6.1.1: "Each emulated peer
// instance MUST send a Peer Up with the OPEN message indicating the address family
// capabilities. A BMP receiver MUST process these capabilities to know which peer belongs
// to which address family." The receiver reads them off the Peer Up's OPEN and records the
// association against that peer, where `show bmp peers` reports it.
func TestReceiverRecordsThePeerUpAddressFamilies(t *testing.T) {
	// VALIDATES: RFC 9069 Section 6.1.1 -- the receiver reads the Peer Up OPEN
	// capabilities and records which families the peer carries.
	bp := &BMPPlugin{state: newBMPState()}
	bp.state.addRouter("10.0.0.1")

	open := fabricateLocRIBOpen(localIdentity{asn: 65000, routerID: 0x01020305})
	bp.processPeerUp("10.0.0.1", &PeerUp{
		Peer:            PeerHeader{PeerType: PeerTypeLocRIB, PeerAS: 65000, PeerBGPID: 0x01020305},
		SentOpenMsg:     open,
		ReceivedOpenMsg: open,
	})

	want := make([]string, 0, len(dumpFamilies))
	for _, fam := range dumpFamilies {
		want = append(want, fam.String())
	}
	got := recordedFamilies(t, bp, "10.0.0.1")
	if len(got) != len(want) {
		t.Fatalf("recorded families = %v, want %v (one per Multiprotocol capability the OPEN advertises)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("recorded family %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// RFC requirement: RFC9069-6.1.1-1 negative -- the association is READ, never assumed. A
// Peer Up whose OPEN advertises no Multiprotocol capability records no family, because a
// guessed family is indistinguishable from an advertised one at every reader
// (ai/rules/principles.md) and would answer "which peer belongs to which address family"
// with an invention. An implementation defaulting to IPv4 unicast passes the positive
// above and fails here.
func TestReceiverRecordsNoFamiliesWhenThePeerUpAdvertisesNone(t *testing.T) {
	// PREVENTS: a defaulted family set, which answers the association with an
	// invention rather than with what the peer advertised.
	bp := &BMPPlugin{state: newBMPState()}
	bp.state.addRouter("10.0.0.2")

	open := makeBGPOpen(65001, 0x0a141e01) // no optional parameters, so no capabilities
	bp.processPeerUp("10.0.0.2", &PeerUp{
		Peer:            PeerHeader{PeerType: PeerTypeGlobal, PeerAS: 65001, PeerBGPID: 0x0a141e01},
		SentOpenMsg:     open,
		ReceivedOpenMsg: open,
	})

	if got := recordedFamilies(t, bp, "10.0.0.2"); len(got) != 0 {
		t.Errorf("recorded families = %v, want none: the OPEN advertised no address family capability", got)
	}
}

// recordedFamilies answers the families the receiver state holds for the one
// peer of a router, read through the same command `show bmp peers` renders.
func recordedFamilies(t *testing.T, bp *BMPPlugin, router string) []string {
	t.Helper()
	_, payload, err := bp.state.peersCommand()
	if err != nil {
		t.Fatalf("peers command: %v", err)
	}
	fields, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("peers payload = %T, want map[string]any", payload)
	}
	peers, ok := fields["peers"].([]monitoredPeer)
	if !ok {
		t.Fatalf("peers field = %T, want []monitoredPeer", fields["peers"])
	}
	for _, peer := range peers {
		if peer.Router == router {
			return peer.Families
		}
	}
	t.Fatalf("no peer recorded for router %s", router)
	return nil
}

// TestLocRIBPeerKeyDistinguishesTheInstancesOfOneRouter covers the defect the
// requirement above exposed rather than a requirement of its own, so it carries
// no tag.
//
// RFC 9069 Section 5.1 zero-fills the peer address of every Loc-RIB Instance
// Peer, so the "<router>:<peer-address>" identity every BMP peer used to be
// keyed by was "<router>:0.0.0.0" for all of them: two Loc-RIB instances of one
// router shared one RIB-In pool, and a Peer Down for either withdrew both.
// Section 6.1.1 names the fields that identify one: "The BMP receiver
// identifies the Loc-RIB by the peer header distinguisher and BGP ID." Those
// two are what the key carries now.
func TestLocRIBPeerKeyDistinguishesTheInstancesOfOneRouter(t *testing.T) {
	// PREVENTS: two Loc-RIB instances of one router sharing a RIB-In pool.
	first := bmpCompositeKey("10.0.0.1", PeerHeader{PeerType: PeerTypeLocRIB, Distinguisher: 0, PeerBGPID: 0x01020304})
	second := bmpCompositeKey("10.0.0.1", PeerHeader{PeerType: PeerTypeLocRIB, Distinguisher: 42, PeerBGPID: 0x01020304})
	third := bmpCompositeKey("10.0.0.1", PeerHeader{PeerType: PeerTypeLocRIB, Distinguisher: 42, PeerBGPID: 0x01020305})

	if first == second {
		t.Errorf("two Loc-RIB instances of one router share the key %q: their distinguishers differ", first)
	}
	if second == third {
		t.Errorf("two Loc-RIB instances of one router share the key %q: their BGP IDs differ", second)
	}

	// A monitored BGP peer is still keyed by its address, which is the identity
	// RFC 7854 gives it.
	var addr [16]byte
	copy(addr[:], net.ParseIP("::ffff:10.20.30.40").To16())
	monitored := bmpCompositeKey("10.0.0.1", PeerHeader{PeerType: PeerTypeGlobal, Address: addr})
	if monitored != "10.0.0.1:10.20.30.40" {
		t.Errorf("monitored peer key = %q, want %q", monitored, "10.0.0.1:10.20.30.40")
	}
}

// TestOpenMultiprotocolFamiliesRefusesAMalformedOPEN covers the producer's
// failure path rather than a requirement, so it carries no tag: a PDU that is
// not an OPEN answers no families instead of a default.
func TestOpenMultiprotocolFamiliesRefusesAMalformedOPEN(t *testing.T) {
	// PREVENTS: a PDU that is not an OPEN answering with a default family.
	for name, open := range map[string][]byte{
		"empty":     nil,
		"truncated": make([]byte, message.HeaderLen-1),
		"not-open":  append(make([]byte, message.HeaderLen), 0xff),
	} {
		if got := openMultiprotocolFamilies(open); got != nil {
			t.Errorf("%s: families = %v, want none", name, got)
		}
	}

	// The fabricated Loc-RIB OPEN is the shape that DOES answer, so the refusals
	// above are about the input rather than about the parser never working.
	open := fabricateLocRIBOpen(localIdentity{asn: 65000, routerID: 1})
	if got := openMultiprotocolFamilies(open); len(got) != len(dumpFamilies) {
		t.Errorf("families = %v, want %d", got, len(dumpFamilies))
	}
}

// locRIBReloadPlugin returns a plugin with one live collector session, one
// established monitored peer, and the collector configuration that session runs
// under. applySenderConfig is driven directly, on the TEST goroutine: the
// reload engine of rfc8671_test.go runs the plugin's event loop on a goroutine
// of its own, which puts every producer on that rail out of reach of a break
// `go test` can attribute (rfc/discrimination/README.md).
func locRIBReloadPlugin(t *testing.T) (*BMPPlugin, net.Conn, *senderConfig) {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() {
		closeLog(server, "server-pipe")
		closeLog(client, "client-pipe")
	})
	collector := collectorConfig{Address: testCollectorAddr, Port: testCollectorPort}
	ss := newSenderSession(testCollectorName, collector)
	ss.conn = client
	bp := &BMPPlugin{
		stopCh:  make(chan struct{}),
		senders: []*senderSession{ss},
		peerUps: map[string]*peerUpState{"10.0.0.1": establishedPeer("10.0.0.1", 65001)},
	}
	t.Cleanup(func() {
		bp.stopSenders()
		bp.sessions.Wait()
	})
	inForce := &senderConfig{
		Collectors:            map[string]collectorConfig{testCollectorName: collector},
		RouteMonitoringPolicy: policyAll,
		LocRIB:                yangTrue,
	}
	return bp, server, inForce
}

// RFC requirement: RFC9069-6.1.3-1 positive -- RFC 9069 Section 6.1.3: "In case of any
// change that results in the alteration of behavior of an existing BMP session, i.e.,
// changes to filtering and table names, the session MUST be bounced with a Peer Down /
// Peer Up sequence." Ze bounces the peers reported on the session and leaves the session
// itself up, which is the answer RFC8671-7.2-1 records for the identical sentence in RFC
// 8671 Section 7.2. Moving one behavior leaf puts a Peer Down and then a Peer Up on the
// wire for the established peer.
func TestBehaviorChangeBouncesThePeersOfALocRIBSession(t *testing.T) {
	// VALIDATES: RFC 9069 Section 6.1.3 -- a change to what the session carries
	// bounces its peers.
	bp, conn, inForce := locRIBReloadPlugin(t)

	changed := *inForce
	changed.RouteMonitoringPolicy = policyPostPolicy
	bp.applySenderConfig(inForce, &changed)

	first, err := readBMPFromPipe(conn)
	if err != nil {
		t.Fatalf("read the peer down: %v", err)
	}
	down, ok := first.(*PeerDown)
	if !ok {
		t.Fatalf("first message = %T, want *PeerDown: the bounce leads with it", first)
	}
	if down.Reason != PeerDownDeconfigured {
		t.Errorf("Peer Down reason = %d, want %d (configuration reasons)", down.Reason, PeerDownDeconfigured)
	}
	second, err := readBMPFromPipe(conn)
	if err != nil {
		t.Fatalf("read the peer up: %v", err)
	}
	if _, ok := second.(*PeerUp); !ok {
		t.Fatalf("second message = %T, want *PeerUp: the sequence is Peer Down then Peer Up", second)
	}
}

// RFC requirement: RFC9069-6.1.3-1 negative -- the bounce is owed to "any change that
// results in the alteration of behavior", so a configuration that alters none owes the
// collector nothing. Ze is handed the whole `bgp` root and hears about every edit an
// operator makes, so an implementation bouncing on arrival rather than on comparison
// passes the positive above and costs every collector a bounce for a change no collector
// can see.
func TestAConfigurationThatAltersNoBehaviorBouncesNothing(t *testing.T) {
	// PREVENTS: a bounce on every commit rather than on a change.
	bp, conn, inForce := locRIBReloadPlugin(t)

	same := *inForce
	bp.applySenderConfig(inForce, &same)

	requireCollectorSilent(t, asyncRead(conn), "after a configuration that moved no leaf")
}
