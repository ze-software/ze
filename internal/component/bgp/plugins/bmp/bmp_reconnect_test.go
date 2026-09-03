package bmp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
)

// isEndOfRIB reports whether a Route Monitoring carries an RFC 4724 Section 2
// End-of-RIB marker, in EITHER of the two forms the RFC defines:
//
//   - IPv4 unicast: an UPDATE with no withdrawn routes, no path attributes and
//     no NLRI (a body of four zero bytes).
//   - any other <AFI, SAFI>: an UPDATE carrying ONLY an MP_UNREACH_NLRI
//     attribute with no withdrawn routes.
//
// Recognizing only the IPv4 form made this helper report an IPv6 End-of-RIB as
// an ordinary route, so every count of "routes" in these tests silently
// included it.
func isEndOfRIB(rm *RouteMonitoring) bool {
	body := rm.BGPUpdate
	if len(body) < message.HeaderLen+4 {
		return false
	}
	update := body[message.HeaderLen:]

	// IPv4 unicast form: withdrawn-len 0, attribute-len 0, no NLRI.
	if len(update) == 4 && bytes.Equal(update, []byte{0, 0, 0, 0}) {
		return true
	}

	// Non-IPv4 form: withdrawn-len 0, one MP_UNREACH_NLRI (type 15) attribute
	// whose value is just AFI(2)+SAFI(1), and no NLRI after it.
	if binary.BigEndian.Uint16(update[0:2]) != 0 {
		return false
	}
	attrLen := int(binary.BigEndian.Uint16(update[2:4]))
	attrs := update[4:]
	if attrLen != len(attrs) || attrLen != 6 {
		return false
	}
	// flags(1) + type(1) + length(1) + AFI(2) + SAFI(1)
	return attrs[1] == 15 && attrs[2] == 3
}

// dumpBus installs a test EventBus whose stand-in RIB answers every
// replay-request with one full-table batch, and returns it. This is the real
// dump path: request -> RIB -> handleBestChange.
func dumpBus(t *testing.T, batch func() *ribevents.BestChangeBatch) *locRIBTestBus {
	t.Helper()
	bus := newLocRIBTestBus()
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })

	unsub := ribevents.ReplayRequest.Subscribe(bus, func(req *replay.Request) {
		b := batch()
		// Model the real RIB: replayBestPaths echoes the REQUEST's correlation
		// token onto every batch it produces in answer
		// (internal/component/bgp/plugins/rib/rib_bestchange.go:1202). A
		// stand-in that stamped its own fixed token would answer every dump
		// with a batch addressed to nobody, which is not what the RIB does.
		// An intentionally-incremental batch (token 0) stays incremental.
		if b != nil && b.IsReplay() {
			b.ReplayID = req.ReplayID
		}
		if _, err := ribevents.BestChange.Emit(bus, b); err != nil {
			t.Errorf("stand-in RIB emit: %v", err)
		}
	})
	t.Cleanup(unsub)
	return bus
}

func TestHandleBestChangeReplayEndsWithEndOfRIB(t *testing.T) {
	// VALIDATES: a full-table dump THIS plugin requested is closed with an
	// End-of-RIB Route Monitoring for the family that was dumped, and an
	// incremental batch is not -- End-of-RIB means "the dump is complete", not
	// "this update is done".
	// PREVENTS: a collector that cannot tell an empty Loc-RIB from a dump still
	// in flight (BIRD ends its dump the same way, proto/bmp/bmp.c:1040-1065).
	conn := newRecordingConn()
	ss := newTestSession(t, "eor", conn)
	bp := &BMPPlugin{
		senders:   []*senderSession{ss},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
		identity:  &localIdentity{asn: 65000, routerID: 0x01020305},
	}
	bus := dumpBus(t, func() *ribevents.BestChangeBatch { return locRIBBatch(1, true) })
	bp.startLocRIB()
	t.Cleanup(bp.stopLocRIB)
	_ = bus

	waitQueueDrained(t, ss)
	msgs := decodeBMPStream(t, conn.written())
	if len(msgs) < 3 {
		t.Fatalf("requested dump produced %d messages, want Peer Up + Route Monitoring + End-of-RIB", len(msgs))
	}
	last, ok := msgs[len(msgs)-1].(*RouteMonitoring)
	if !ok {
		t.Fatalf("last message of a dump is %T, want *RouteMonitoring (End-of-RIB)", msgs[len(msgs)-1])
	}
	if !isEndOfRIB(last) {
		t.Errorf("last message of a dump is not an End-of-RIB marker: body %x", last.BGPUpdate)
	}
	if last.Peer.PeerType != PeerTypeLocRIB {
		t.Errorf("End-of-RIB PeerType = %d, want %d (Loc-RIB)", last.Peer.PeerType, PeerTypeLocRIB)
	}

	// Incremental batch: no End-of-RIB.
	conn.reset()
	bp.handleBestChange(locRIBBatch(1, false))
	waitQueueDrained(t, ss)
	for i, m := range decodeBMPStream(t, conn.written()) {
		if rm, ok := m.(*RouteMonitoring); ok && isEndOfRIB(rm) {
			t.Errorf("message %d of an incremental batch is an End-of-RIB marker", i)
		}
	}
}

func TestThirdPartyReplayEmitsNoEndOfRIB(t *testing.T) {
	// VALIDATES: a full-table replay somebody ELSE asked for delivers its routes
	// but no End-of-RIB marker.
	// PREVENTS: asserting RFC 4724 Section 2's "my initial routing update is
	// complete" mid-stream to collectors that never asked for anything.
	// ribevents.ReplayRequest is a broadcast handle with other emitters --
	// internal/component/sysrib/sysrib.go emits on it whenever sysrib needs the
	// BGP table replayed -- and those batches arrive here too.
	conn := newRecordingConn()
	ss := newTestSession(t, "third-party", conn)
	bp := &BMPPlugin{
		senders:   []*senderSession{ss},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
		identity:  &localIdentity{asn: 65000, routerID: 0x01020305},
	}

	bus := newLocRIBTestBus()
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })
	bp.startLocRIB() // subscribes handleBestChange; its own initial dump has no RIB to answer it
	t.Cleanup(bp.stopLocRIB)
	waitQueueDrained(t, ss)
	conn.reset()

	// Somebody else's replay-request produced this batch: emitted on the bus by
	// another subscriber, with no dump requested by this plugin.
	if _, err := ribevents.BestChange.Emit(bus, locRIBBatch(2, true)); err != nil {
		t.Fatalf("third-party replay emit: %v", err)
	}
	waitQueueDrained(t, ss)

	msgs := decodeBMPStream(t, conn.written())
	if len(msgs) == 0 {
		t.Fatal("the replayed routes never reached the collector; the no-marker check would be vacuous")
	}
	for i, m := range msgs {
		if rm, ok := m.(*RouteMonitoring); ok && isEndOfRIB(rm) {
			t.Errorf("message %d is an End-of-RIB marker for a replay this plugin never requested", i)
		}
	}
}

func TestLocRIBEndOfRIBForNonIPv4Family(t *testing.T) {
	// VALIDATES: the End-of-RIB marker for a family other than IPv4 unicast is
	// the RFC 4724 Section 2 MP form -- an UPDATE whose only path attribute is
	// MP_UNREACH_NLRI with no withdrawn routes.
	// PREVENTS: shipping the IPv4 four-zero-bytes form for every family, which a
	// collector would read as an IPv4-unicast End-of-RIB (or as malformed).
	body := buildEndOfRIBBody(family.IPv6Unicast)
	sec, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatalf("ParseUpdateSections: %v", err)
	}
	if w := sec.Withdrawn(body); len(w) != 0 {
		t.Errorf("End-of-RIB must carry no withdrawn routes, got %x", w)
	}
	attrs := sec.Attrs(body)
	mp := findAttr(attrs, attrMPUnreac)
	if mp == nil {
		t.Fatal("non-IPv4 End-of-RIB must carry MP_UNREACH_NLRI (RFC 4724 Section 2)")
	}
	// AFI(2) + SAFI(1) and nothing else: an MP_UNREACH with no NLRI.
	if len(mp) != 3 {
		t.Errorf("MP_UNREACH_NLRI value is %d bytes (%x), want exactly AFI+SAFI with no NLRI", len(mp), mp)
	}

	// The IPv4-unicast form stays the bare four zero bytes.
	if got := buildEndOfRIBBody(family.IPv4Unicast); !bytes.Equal(got, []byte{0, 0, 0, 0}) {
		t.Errorf("IPv4 unicast End-of-RIB body = %x, want 00000000", got)
	}
}

func TestShutdownDeliversLocRIBPeerDownThenTermination(t *testing.T) {
	// VALIDATES: the RFC 9069 Loc-RIB Peer Down the plugin sends on shutdown
	// reaches the collector, and the Termination follows it.
	// PREVENTS: the regression the transmit queue introduced. The plugin's
	// teardown enqueues the Peer Down and then stops the senders; the drain
	// checks its stop channel BEFORE the queue, so without a bounded flush in
	// stop() the last message of the session was thrown away and the collector
	// saw a Termination with no Peer Down before it.
	conn := newRecordingConn()
	ss := newTestSession(t, "shutdown", conn)
	ss.locRIBUpSent.Store(true)

	bp := &BMPPlugin{
		senders:   []*senderSession{ss},
		locRIBUp:  true,
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
		identity:  &localIdentity{asn: 65000, routerID: 0x01020305},
	}

	bp.sendLocRIBPeerDown() // what RunBMPPlugin's defer does...
	ss.stop()               // ...immediately before this

	msgs := decodeBMPStream(t, conn.written())
	var sawPeerDown, sawTermination bool
	for _, m := range msgs {
		switch v := m.(type) {
		case *PeerDown:
			if v.Peer.PeerType != PeerTypeLocRIB {
				t.Errorf("Peer Down PeerType = %d, want %d (Loc-RIB)", v.Peer.PeerType, PeerTypeLocRIB)
			}
			// RFC 9069 Section 5.3: "The Peer Down notification MUST use reason
			// code 6." The Loc-RIB instance peer has no BGP session, so the
			// reasons that report an FSM event or a NOTIFICATION cannot apply.
			if v.Reason != PeerDownTLVData {
				t.Errorf("Loc-RIB Peer Down reason = %d, want %d (RFC 9069 Section 5.3)", v.Reason, PeerDownTLVData)
			}
			sawPeerDown = true
		case *Termination:
			if !sawPeerDown {
				t.Error("Termination arrived before the Loc-RIB Peer Down")
			}
			sawTermination = true
		}
	}
	if !sawPeerDown {
		t.Errorf("collector never received the Loc-RIB Peer Down (%d messages delivered)", len(msgs))
	}
	if !sawTermination {
		t.Error("collector never received the Termination")
	}
}

func TestLocRIBOnePeerUpAcrossReplayBatches(t *testing.T) {
	// VALIDATES: RFC 9069 per-instance semantics under the End-of-RIB change --
	// two full-table replay batches produce exactly ONE Loc-RIB Peer Up, two
	// Route Monitoring messages for the routes, and two End-of-RIB markers (one
	// closing each dump).
	// PREVENTS: a Peer Up per batch. TestLocRIBSinglePeerUpPerInstance is the
	// RFC-tagged proof of the same requirement, but it reads a fixed count of
	// messages that the End-of-RIB markers now shift, so it no longer sees the
	// second batch; this reads the whole stream instead. Both should hold.
	conn := newRecordingConn()
	ss := newTestSession(t, "per-instance", conn)
	bp := &BMPPlugin{
		senders:   []*senderSession{ss},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
		identity:  &localIdentity{asn: 65000, routerID: 0x01020305},
	}

	bus := dumpBus(t, func() *ribevents.BestChangeBatch { return locRIBBatch(1, true) })
	bp.startLocRIB() // requests dump 1
	t.Cleanup(bp.stopLocRIB)
	if err := bp.emitReplayRequest(bus, nil); err != nil { // dump 2
		t.Fatalf("second dump request: %v", err)
	}
	waitQueueDrained(t, ss)

	var peerUps, routes, eors int
	for _, m := range decodeBMPStream(t, conn.written()) {
		switch v := m.(type) {
		case *PeerUp:
			peerUps++
		case *RouteMonitoring:
			if isEndOfRIB(v) {
				eors++
			} else {
				routes++
			}
		default:
			t.Errorf("unexpected message type %T", m)
		}
	}
	if peerUps != 1 {
		t.Errorf("Loc-RIB Peer Up count = %d, want exactly 1 per RIB instance per session", peerUps)
	}
	if routes != 2 {
		t.Errorf("Route Monitoring count = %d, want 2 (one per best change)", routes)
	}
	// Two dumps, and each one closes BOTH families: IPv4 by its batch, IPv6 by
	// closeDumpFamilies because the RIB stayed silent about it (RFC 4724
	// Section 4 -- the marker is owed "including the case when there is no
	// update to send" for that family).
	if eors != 4 {
		t.Errorf("End-of-RIB count = %d, want 4 (two families closed by each of the two replay batches)", eors)
	}
}

func TestLocRIBDumpTargetsOnlyTheReconnectingCollector(t *testing.T) {
	// VALIDATES: the full-table dump one collector's fresh session asks for goes
	// to that collector alone.
	// PREVENTS: re-dumping the whole table to every other collector each time
	// one of them reconnects -- and, worse, handing those collectors a second
	// End-of-RIB, which per RFC 4724 Section 2 semantics tells them their own
	// initial dump just completed.
	bus := newLocRIBTestBus()
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })

	unsubRIB := ribevents.ReplayRequest.Subscribe(bus, func(req *replay.Request) {
		b := locRIBBatch(1, true)
		// Echo the request's correlation token, as replayBestPaths does
		// (rib_bestchange.go:1202); that echo is what lets the requester tell
		// its own dump from somebody else's.
		b.ReplayID = req.ReplayID
		if _, err := ribevents.BestChange.Emit(bus, b); err != nil {
			t.Errorf("stand-in RIB emit: %v", err)
		}
	})
	defer unsubRIB()

	connA, connB := newRecordingConn(), newRecordingConn()
	ssA := newTestSession(t, "reconnecting", connA)
	ssB := newTestSession(t, "settled", connB)

	bp := &BMPPlugin{
		senders:   []*senderSession{ssA, ssB},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
		identity:  &localIdentity{asn: 65000, routerID: 0x01020305},
	}
	bp.startLocRIB() // subscribes handleBestChange (and emits one initial dump)
	t.Cleanup(bp.stopLocRIB)

	// That initial dump is untargeted and legitimately reaches both sessions.
	// Wait for it to be WRITTEN before clearing the recordings: the drain is
	// asynchronous, so resetting on the producer's schedule would let those
	// bytes land after the reset and be misread as the targeted dump leaking.
	waitQueueDrained(t, ssA)
	waitQueueDrained(t, ssB)
	connA.reset()
	connB.reset()

	// Both collectors reconnect at once -- the common case, since a collector
	// host restart plus the reconnect backoff aligns them. Each request runs on
	// its own session goroutine, exactly as run() drives it.
	var wg sync.WaitGroup
	wg.Go(func() { bp.requestLocRIBDump(ssA) })
	wg.Wait()
	waitQueueDrained(t, ssA)
	waitQueueDrained(t, ssB)

	if len(decodeBMPStream(t, connA.written())) == 0 {
		t.Error("the reconnecting collector received no dump at all")
	}
	for i, m := range decodeBMPStream(t, connB.written()) {
		t.Errorf("settled collector received message %d (%T) from another collector's dump", i, m)
	}
}

func TestPrimingPrecedesConcurrentRouteMonitoring(t *testing.T) {
	// VALIDATES: on a fresh connection the priming messages (Peer Up for every
	// established peer, and the RFC 9069 Loc-RIB Peer Up) are queued before
	// ANY producer's Route Monitoring, even with a producer hammering the
	// session throughout the connect.
	// PREVENTS: publishing the connection and then priming as two steps. In
	// between, a Route Monitoring from the delivery goroutine reaches the
	// collector for a peer it has never been told about -- an RFC 7854 Section
	// 4.10 ordering break, and the same class of defect this work exists to
	// remove. run() closes the window by publishing the conn and queueing the
	// priming inside one writeMu critical section, which every producer must
	// take before it can enqueue.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer closeLog(ln, "test-listener")
	addr, _ := ln.Addr().(*net.TCPAddr)

	sentOpen := makeBGPOpen(65000, 0x0A000064)
	recvOpen := makeBGPOpen(65001, 0x0A000001)

	ss := newSenderSession("primed", collectorConfig{
		Address: "127.0.0.1",
		Port:    strconv.Itoa(addr.Port),
	})
	bp := &BMPPlugin{
		senders: []*senderSession{ss},
		peerUps: map[string]*peerUpState{"10.0.0.1": {
			peer:      PeerHeader{PeerType: PeerTypeGlobal, PeerAS: 65001},
			localPort: 179, sentOpen: sentOpen, recvOpen: recvOpen,
		}},
	}
	ss.onPrimed = func() { bp.primeSender(ss) }

	// A producer racing the connect, exactly as the delivery goroutine does.
	stop := make(chan struct{})
	var producer sync.WaitGroup
	producer.Go(func() {
		peer := PeerHeader{PeerType: PeerTypeGlobal, PeerAS: 65001}
		body := []byte{0x00, 0x00, 0x00, 0x00}
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body)
		}
	})

	var wg sync.WaitGroup
	wg.Go(ss.run)
	t.Cleanup(func() {
		close(stop)
		producer.Wait()
		ss.stop()
		wg.Wait()
	})

	conn := acceptOne(t, ln)
	defer closeLog(conn, "collector")
	if derr := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); derr != nil {
		t.Fatalf("set read deadline: %v", derr)
	}

	// Read until the first Peer Up. Nothing but Initiation may precede it.
	for i := range 50 {
		msg, rerr := readBMPFromPipe(conn)
		if rerr != nil {
			t.Fatalf("read message %d: %v", i, rerr)
		}
		switch msg.(type) {
		case *PeerUp:
			return // priming won, as it must
		case *Initiation:
			if i != 0 {
				t.Fatalf("Initiation arrived as message %d, want first", i)
			}
		case *RouteMonitoring:
			t.Fatalf("message %d is Route Monitoring: a producer got in front of the session's Peer Up", i)
		default:
			t.Fatalf("unexpected message type %T at index %d", msg, i)
		}
	}
	t.Fatal("no Peer Up in the first 50 messages of the session")
}

// RFC requirement: RFC7854-x-16 positive -- a Peer Up is sent for every
// established BGP peer on each new BMP session, including the peers that came
// up while the collector was disconnected.
// RFC requirement: RFC7854-x-17 positive -- the initial RIB dump (Route
// Monitoring) follows those Peer Ups on the same session.
func TestConcurrentDumpsStayAddressedToTheirOwnCollector(t *testing.T) {
	// VALIDATES: two collectors requesting a dump at the same moment each get
	// their own, and neither gets the other's.
	// PREVENTS: the interleaved-scope bug. requestLocRIBDump runs on each
	// session's own run() goroutine and publishes a process-wide scope; without
	// serialization, A's synchronously-delivered replay batches read B's scope,
	// so A's dump and its End-of-RIB went to B, A got nothing, and A's retract
	// then made B's dump fan out to everyone.
	dumpBus(t, func() *ribevents.BestChangeBatch { return locRIBBatch(1, true) })

	connA, connB := newRecordingConn(), newRecordingConn()
	ssA := newTestSession(t, "collector-a", connA)
	ssB := newTestSession(t, "collector-b", connB)

	bp := &BMPPlugin{
		senders:   []*senderSession{ssA, ssB},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
		identity:  &localIdentity{asn: 65000, routerID: 0x01020305},
	}
	bp.startLocRIB()
	t.Cleanup(bp.stopLocRIB)
	waitQueueDrained(t, ssA)
	waitQueueDrained(t, ssB)
	connA.reset()
	connB.reset()

	var wg sync.WaitGroup
	wg.Go(func() { bp.requestLocRIBDump(ssA) })
	wg.Go(func() { bp.requestLocRIBDump(ssB) })
	wg.Wait()
	waitQueueDrained(t, ssA)
	waitQueueDrained(t, ssB)

	// Each collector must see exactly its own dump: one route, and one
	// End-of-RIB per family the dump owes a marker for. More of either means it
	// received the other collector's as well, which is what this test exists to
	// catch; the per-collector counts are what isolate the dumps, not the
	// absolute marker count.
	const wantEORs = 2 // IPv4 (closed by the batch) + IPv6 (empty, closed anyway)
	for name, conn := range map[string]*recordingConn{"collector-a": connA, "collector-b": connB} {
		var routes, eors int
		for _, m := range decodeBMPStream(t, conn.written()) {
			if rm, ok := m.(*RouteMonitoring); ok {
				if isEndOfRIB(rm) {
					eors++
				} else {
					routes++
				}
			}
		}
		if routes != 1 {
			t.Errorf("%s: %d routes, want exactly its own dump's 1", name, routes)
		}
		if eors != wantEORs {
			t.Errorf("%s: %d End-of-RIB markers, want exactly %d (one per family the dump owes)", name, eors, wantEORs)
		}
	}
}

func TestSenderReconnectReplaysInitiationPeerUpAndDump(t *testing.T) {
	// VALIDATES: every connection a sender makes is a complete BMP session --
	// Initiation, a Peer Up for each established BGP peer, the RFC 9069 Loc-RIB
	// Peer Up, a fresh full-table dump and its End-of-RIB. Asserted on the
	// SECOND connection, after the collector drops the first one.
	// PREVENTS: the reconnect gap. startLocRIB emitted its replay-request once
	// at subscribe time and the reconnect loop sent only Initiation, so a
	// collector that came back saw Route Monitoring for peers it had never seen
	// come up, and never received the table it missed.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer closeLog(ln, "test-listener")
	addr, _ := ln.Addr().(*net.TCPAddr)

	// A stand-in RIB: every replay-request produces one full-table batch.
	bus := newLocRIBTestBus()
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })

	dumped := netip.MustParsePrefix("198.51.100.0/24")
	unsubRIB := ribevents.ReplayRequest.Subscribe(bus, func(*replay.Request) {
		batch := &ribevents.BestChangeBatch{
			Protocol: "bgp",
			Family:   family.IPv4Unicast,
			ReplayID: replay.Broadcast,
			Changes: []ribevents.BestChangeEntry{{
				Action:  ribevents.BestChangeAdd,
				Prefix:  dumped,
				NextHop: netip.MustParseAddr("192.0.2.1"),
				ASPath:  []uint32{65001},
			}},
		}
		if _, err := ribevents.BestChange.Emit(bus, batch); err != nil {
			t.Errorf("stand-in RIB failed to emit the replay batch: %v", err)
		}
	})
	defer unsubRIB()

	sentOpen := makeBGPOpen(65000, 0x0A000064)
	recvOpen := makeBGPOpen(65001, 0x0A000001)
	established := &peerUpState{
		peer:       PeerHeader{PeerType: PeerTypeGlobal, PeerAS: 65001},
		localPort:  179,
		sentOpen:   sentOpen,
		recvOpen:   recvOpen,
		remotePort: 0,
	}

	ss := newSenderSession("collector", collectorConfig{
		Address: "127.0.0.1",
		Port:    strconv.Itoa(addr.Port),
	})
	// Milliseconds instead of the production reconnectMin: this test drives a
	// full disconnect -> reconnect cycle, which is the behavior under test.
	ss.retryWait = 20 * time.Millisecond

	bp := &BMPPlugin{
		senders:   []*senderSession{ss},
		openCache: map[string]*openPair{"10.0.0.1": {sent: sentOpen, received: recvOpen}},
		peerUps:   map[string]*peerUpState{"10.0.0.1": established},
		identity:  &localIdentity{asn: 65000, routerID: 0x0A000064},
	}
	ss.onPrimed = func() { bp.primeSender(ss) }
	ss.onConnected = func() { bp.requestLocRIBDump(ss) }

	bp.startLocRIB() // subscribes handleBestChange; nothing is connected yet
	t.Cleanup(bp.stopLocRIB)

	var wg sync.WaitGroup
	wg.Go(ss.run)
	t.Cleanup(func() {
		ss.stop()
		wg.Wait()
	})

	// Connection 1: prove the session comes up, then take it away.
	first := acceptOne(t, ln)
	readSessionOpening(t, first, dumped, "first connection")
	closeLog(first, "collector-drops-first-connection")

	// Connection 2: the collector knows nothing; it must be told everything.
	second := acceptOne(t, ln)
	defer closeLog(second, "collector-second")
	readSessionOpening(t, second, dumped, "reconnection")
}

func TestSyncSendersWiresSessionPriming(t *testing.T) {
	// VALIDATES: the sessions syncSenders creates are primed -- a collector that
	// connects to a session built by the production entry point receives
	// Initiation and then the Peer Up of every established peer.
	// PREVENTS: the priming existing but never being wired. Every other test in
	// this file installs the hooks itself, so without this one deleting the
	// assignment in syncSenders would leave the whole suite green.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer closeLog(ln, "test-listener")
	addr, _ := ln.Addr().(*net.TCPAddr)

	sentOpen := makeBGPOpen(65000, 0x0A000064)
	recvOpen := makeBGPOpen(65001, 0x0A000001)
	bp := &BMPPlugin{
		peerUps: map[string]*peerUpState{"10.0.0.1": {
			peer:      PeerHeader{PeerType: PeerTypeGlobal, PeerAS: 65001},
			localPort: 179, sentOpen: sentOpen, recvOpen: recvOpen,
		}},
	}
	t.Cleanup(func() {
		bp.stopSenders()
		bp.sessions.Wait()
	})

	bp.syncSenders(&senderConfig{Collectors: map[string]collectorConfig{
		"c1": {Address: "127.0.0.1", Port: strconv.Itoa(addr.Port)},
	}})

	conn := acceptOne(t, ln)
	defer closeLog(conn, "collector")
	if derr := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); derr != nil {
		t.Fatalf("set read deadline: %v", derr)
	}

	first, err := readBMPFromPipe(conn)
	if err != nil {
		t.Fatalf("read Initiation: %v", err)
	}
	if _, ok := first.(*Initiation); !ok {
		t.Fatalf("first message = %T, want *Initiation", first)
	}

	second, err := readBMPFromPipe(conn)
	if err != nil {
		t.Fatalf("read Peer Up: %v (the session was never primed)", err)
	}
	pu, ok := second.(*PeerUp)
	if !ok {
		t.Fatalf("second message = %T, want *PeerUp for the established peer", second)
	}
	if pu.Peer.PeerAS != 65001 {
		t.Errorf("Peer Up PeerAS = %d, want 65001", pu.Peer.PeerAS)
	}
}

// acceptOne accepts one connection with a bounded wait.
func acceptOne(t *testing.T, ln net.Listener) net.Conn {
	t.Helper()
	type accepted struct {
		c   net.Conn
		err error
	}
	ch := make(chan accepted, 1)
	go func() {
		c, err := ln.Accept()
		ch <- accepted{c, err}
	}()
	select {
	case a := <-ch:
		if a.err != nil {
			t.Fatalf("accept: %v", a.err)
		}
		return a.c
	case <-time.After(10 * time.Second):
		t.Fatal("sender did not connect within 10s")
		return nil
	}
}

// readSessionOpening asserts that a freshly accepted BMP session opens with
// Initiation, the Peer Up of every established BGP peer, the Loc-RIB Peer Up,
// the dumped prefix and the End-of-RIB that closes the dump.
func readSessionOpening(t *testing.T, conn net.Conn, dumped netip.Prefix, what string) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		t.Fatalf("%s: set read deadline: %v", what, err)
	}

	var (
		globalPeerUps int
		locRIBPeerUps int
		sawDump       bool
		sawEOR        bool
	)
	// Initiation + global Peer Up + Loc-RIB Peer Up + Route Monitoring + EoR.
	const want = 5
	for i := range want {
		msg, err := readBMPFromPipe(conn)
		if err != nil {
			t.Fatalf("%s: read message %d: %v (got %d global Peer Up, %d Loc-RIB Peer Up, dump=%v, eor=%v)",
				what, i, err, globalPeerUps, locRIBPeerUps, sawDump, sawEOR)
		}
		switch m := msg.(type) {
		case *Initiation:
			if i != 0 {
				t.Errorf("%s: Initiation arrived as message %d, RFC 7854 Section 4.3 requires it first", what, i)
			}
		case *PeerUp:
			if m.Peer.PeerType == PeerTypeLocRIB {
				locRIBPeerUps++
			} else {
				globalPeerUps++
			}
		case *RouteMonitoring:
			switch {
			case isEndOfRIB(m):
				sawEOR = true
			case bytes.Contains(m.BGPUpdate, dumped.Addr().AsSlice()[:3]):
				sawDump = true
			}
		default:
			t.Errorf("%s: unexpected message type %T at index %d", what, msg, i)
		}
	}

	if globalPeerUps != 1 {
		t.Errorf("%s: %d Peer Up for established BGP peers, want 1", what, globalPeerUps)
	}
	if locRIBPeerUps != 1 {
		t.Errorf("%s: %d Loc-RIB Peer Up, want 1 (RFC 9069: one per instance per session)", what, locRIBPeerUps)
	}
	if !sawDump {
		t.Errorf("%s: the dumped prefix never arrived as Route Monitoring", what)
	}
	if !sawEOR {
		t.Errorf("%s: the dump was not closed with an End-of-RIB marker", what)
	}
}

func TestEmptyLocRIBDumpStillEndsWithEndOfRIB(t *testing.T) {
	// VALIDATES: a dump the RIB answers with NO batches at all is still closed
	// with End-of-RIB markers, preceded by the Loc-RIB Peer Up.
	// PREVENTS: the collector waiting forever for a dump that already finished.
	// replayBestPaths publishes a family only when its change list is non-empty
	// (internal/component/bgp/plugins/rib/rib_bestchange.go), so an empty
	// Loc-RIB produces no batch, handleBestChange never runs, and the marker
	// that exists to say "the table is empty" was the one thing never sent.
	conn := newRecordingConn()
	ss := newTestSession(t, "empty-table", conn)
	bp := &BMPPlugin{
		senders:   []*senderSession{ss},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
		identity:  &localIdentity{asn: 65000, routerID: 0x01020305},
	}

	// A stand-in RIB with an empty table: it receives the replay-request and
	// emits nothing, exactly as replayBestPaths does with no best paths.
	bus := newLocRIBTestBus()
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })
	unsub := ribevents.ReplayRequest.Subscribe(bus, func(*replay.Request) {})
	t.Cleanup(unsub)

	bp.startLocRIB()
	t.Cleanup(bp.stopLocRIB)
	waitQueueDrained(t, ss)

	var peerUps, routes, eors int
	for _, m := range decodeBMPStream(t, conn.written()) {
		switch v := m.(type) {
		case *PeerUp:
			peerUps++
		case *RouteMonitoring:
			if isEndOfRIB(v) {
				eors++
			} else {
				routes++
			}
		default:
			t.Errorf("unexpected message type %T on an empty dump", m)
		}
	}

	if eors == 0 {
		t.Error("an empty Loc-RIB dump sent no End-of-RIB marker: the collector cannot tell " +
			"'the table is empty' from 'the dump is still arriving'")
	}
	if routes != 0 {
		t.Errorf("an empty dump produced %d Route Monitoring messages, want 0", routes)
	}
	if peerUps != 1 {
		t.Errorf("Loc-RIB Peer Up count = %d, want exactly 1: RFC 9069 requires it to precede "+
			"Route Monitoring, and an End-of-RIB marker is a Route Monitoring message", peerUps)
	}
}

func TestStopPublishesTheDisconnect(t *testing.T) {
	// VALIDATES: stop() clears the session's conn, so a producer that arrives
	// afterwards is told the truth.
	// PREVENTS: enqueueLocked's nil check passing on a socket stop() has already
	// closed. The producer was told its message was queued, onto a queue whose
	// drain has exited and will never write it -- a silent drop reported as
	// success (ai/rules/evidence.md).
	conn := newRecordingConn()
	ss := newTestSession(t, "stopped", conn)

	peer := testPeerHeader()
	if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, []byte{0, 0, 0, 0}); err != nil {
		t.Fatalf("pre-stop write: %v", err)
	}

	ss.stop()

	ss.connMu.Lock()
	c := ss.conn
	ss.connMu.Unlock()
	if c != nil {
		t.Error("stop() left the session pointing at a closed connection")
	}
	if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, []byte{0, 0, 0, 0}); !errors.Is(err, errNotConnected) {
		t.Errorf("a producer after stop() got %v, want errNotConnected", err)
	}
}
