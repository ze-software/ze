package bmp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// replayPlugin observes the OPEN exchange and Established event with no
// collector attached. Subsequent UPDATEs use the production event handler.
func replayPlugin(t *testing.T) *BMPPlugin {
	t.Helper()
	bp := &BMPPlugin{
		routeMonitorPolicy: policyPrePolicy,
		openCache:          make(map[string]*openPair),
		dedupState:         make(map[string]map[uint64]struct{}),
		dedupCount:         make(map[string]uint32),
	}
	open := fabricateLocRIBOpen(localIdentity{asn: 65001, routerID: 0xc0000201})
	for _, direction := range []rpc.MessageDirection{rpc.DirectionSent, rpc.DirectionReceived} {
		bp.handleStructuredEvent(&rpc.StructuredEvent{
			PeerAddress: "192.0.2.1", PeerAS: 65001, Direction: direction,
			EventType:  rpc.EventKindOpen,
			RawMessage: &bgptypes.RawMessage{Type: msgtype.TypeOPEN, RawBytes: open[message.HeaderLen:]},
		})
	}
	bp.handleStructuredEvent(&rpc.StructuredEvent{
		PeerAddress: "192.0.2.1", PeerAS: 65001, RemoteRouterID: 0xc0000201,
		LocalAddress: "192.0.2.2", LocalPort: 41793, RemotePort: 179,
		EventType: rpc.EventKindState, State: rpc.SessionStateUp,
	})
	return bp
}

func replayUpdate(bp *BMPPlugin, direction rpc.MessageDirection, body []byte, at time.Time, ctxID bgpctx.ContextID) {
	bp.handleStructuredEvent(&rpc.StructuredEvent{
		PeerAddress: "192.0.2.1", PeerAS: 65001, RemoteRouterID: 0xc0000201,
		Direction: direction, EventType: rpc.EventKindUpdate,
		RawMessage: &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: body,
			Timestamp: at, WireUpdate: wireu.NewWireUpdate(body, ctxID)},
	})
}

func primeReplayCollector(t *testing.T, bp *BMPPlugin) (*senderSession, *recordingConn) {
	t.Helper()
	conn := newRecordingConn()
	ss := newTestSession(t, "replay", conn)
	bp.eventMu.Lock()
	ss.writeMu.Lock()
	bp.primeSender(ss)
	ss.writeMu.Unlock()
	bp.eventMu.Unlock()
	waitQueueDrained(t, ss)
	return ss, conn
}

// RFC requirement: RFC7854-3.3-1 positive -- after reconnect, Peer Up precedes the current per-peer snapshot and IPv4 and empty IPv6 each receive EOR after its routes.
// The fixture receives two advertisements and a withdrawal while disconnected;
// the collector must learn only the surviving route before completion.
func TestRFC7854AdjReplayEndsEachFamily(t *testing.T) {
	bp := replayPlugin(t)
	replayUpdate(bp, rpc.DirectionReceived, announceBody(1), time.Time{}, 0)
	replayUpdate(bp, rpc.DirectionReceived, announceBody(2), time.Time{}, 0)
	replayUpdate(bp, rpc.DirectionReceived, withdrawBody(1), time.Time{}, 0)
	_, conn := primeReplayCollector(t, bp)
	msgs := decodeBMPStream(t, conn.written())
	if len(msgs) != 4 {
		t.Fatalf("snapshot messages = %d, want Peer Up, route, IPv4 EOR, IPv6 EOR", len(msgs))
	}
	if _, ok := msgs[0].(*PeerUp); !ok {
		t.Fatalf("first message = %T, want Peer Up", msgs[0])
	}
	route, ok := msgs[1].(*RouteMonitoring)
	if !ok || !bytes.Equal(route.BGPUpdate[message.HeaderLen:], announceBody(2)) {
		t.Fatalf("snapshot retained a withdrawn route or lost the surviving route: %#v", msgs[1])
	}
	for index, fam := range []family.Family{family.IPv4Unicast, family.IPv6Unicast} {
		eor, ok := msgs[index+2].(*RouteMonitoring)
		if !ok || !bytes.Equal(eor.BGPUpdate[message.HeaderLen:], buildEndOfRIBBody(fam)) {
			t.Fatalf("message %d does not close family %v: %#v", index+2, fam, msgs[index+2])
		}
	}
}

// RFC requirement: RFC7854-3.3-1 negative -- reconnecting one collector does not close another collector's dump, and an incremental UPDATE produces no EOR.
// Each collector reads the bytes produced on its own connection.
func TestRFC7854AdjReplayCompletionIsSessionScoped(t *testing.T) {
	bp := replayPlugin(t)
	replayUpdate(bp, rpc.DirectionReceived, announceBody(1), time.Time{}, 0)
	first, conn := primeReplayCollector(t, bp)
	bp.senders = []*senderSession{first}
	conn.reset()
	_, second := primeReplayCollector(t, bp)
	if len(second.written()) == 0 {
		t.Fatal("new collector received no snapshot")
	}
	if len(conn.written()) != 0 {
		t.Fatal("settled collector received another session's snapshot")
	}
	replayUpdate(bp, rpc.DirectionReceived, announceBody(2), time.Time{}, 0)
	waitQueueDrained(t, first)
	msgs := decodeBMPStream(t, conn.written())
	if len(msgs) != 1 {
		t.Fatalf("incremental messages = %d, want one UPDATE without EOR", len(msgs))
	}
	mon, ok := msgs[0].(*RouteMonitoring)
	if !ok || !bytes.Equal(mon.BGPUpdate[message.HeaderLen:], announceBody(2)) {
		t.Fatalf("incremental message = %#v", msgs[0])
	}
}

// RFC requirement: RFC7854-5-2 positive -- replayed routes with unavailable receipt time and their completion markers have both timestamp fields zero.
// The peer's connection time cannot be substituted for an unavailable route time.
func TestRFC7854UnknownRouteTimeIsZero(t *testing.T) {
	bp := replayPlugin(t)
	replayUpdate(bp, rpc.DirectionReceived, announceBody(1), time.Time{}, 0)
	_, conn := primeReplayCollector(t, bp)
	var routes int
	for _, msg := range decodeBMPStream(t, conn.written()) {
		if mon, ok := msg.(*RouteMonitoring); ok {
			routes++
			if mon.Peer.TimestampSec != 0 || mon.Peer.TimestampUsec != 0 {
				t.Fatalf("unknown timestamp = %d.%06d", mon.Peer.TimestampSec, mon.Peer.TimestampUsec)
			}
		}
	}
	if routes != 3 {
		t.Fatalf("route and two EORs = %d messages, want 3", routes)
	}
}

// RFC requirement: RFC7854-5-2 negative -- a known receipt timestamp survives replay exactly, rather than becoming zero or the collector's connection time.
func TestRFC7854KnownRouteTimeSurvivesReplay(t *testing.T) {
	bp := replayPlugin(t)
	at := time.Unix(1700000000, 123456000)
	replayUpdate(bp, rpc.DirectionReceived, announceBody(1), at, 0)
	_, conn := primeReplayCollector(t, bp)
	msgs := decodeBMPStream(t, conn.written())
	mon, ok := msgs[1].(*RouteMonitoring)
	if !ok {
		t.Fatalf("second message = %T, want Route Monitoring", msgs[1])
	}
	if mon.Peer.TimestampSec != 1700000000 || mon.Peer.TimestampUsec != 123456 {
		t.Fatalf("receipt timestamp changed to %d.%06d", mon.Peer.TimestampSec, mon.Peer.TimestampUsec)
	}
}

// The cache stores route identity including ADD-PATH, rather than complete
// UPDATE bodies. Replacing one path and withdrawing another cannot resurrect it.
func TestRFC7854AdjReplayPreservesOnlyCurrentAddPaths(t *testing.T) {
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true,
		map[family.Family]bool{family.IPv4Unicast: true}))
	if err != nil {
		t.Fatal(err)
	}
	bp := replayPlugin(t)
	body := func(path uint32, origin byte, withdraw bool) []byte {
		nlri := []byte{0, 0, 0, 0, 24, 10, 20, 30}
		binary.BigEndian.PutUint32(nlri[:4], path)
		if withdraw {
			return assembleUpdateBody(nlri, nil, nil)
		}
		return assembleUpdateBody(nil, []byte{0x40, 1, 1, origin}, nlri)
	}
	replayUpdate(bp, rpc.DirectionReceived, body(7, 0, false), time.Time{}, ctxID)
	replayUpdate(bp, rpc.DirectionReceived, body(8, 0, false), time.Time{}, ctxID)
	replayUpdate(bp, rpc.DirectionReceived, body(7, 2, false), time.Time{}, ctxID)
	replayUpdate(bp, rpc.DirectionReceived, body(8, 0, true), time.Time{}, ctxID)
	_, conn := primeReplayCollector(t, bp)
	var routes int
	for _, msg := range decodeBMPStream(t, conn.written()) {
		mon, ok := msg.(*RouteMonitoring)
		if !ok || isEndOfRIB(mon) {
			continue
		}
		routes++
		if !bytes.Equal(mon.BGPUpdate[message.HeaderLen:], body(7, 2, false)) {
			t.Fatalf("replayed stale attributes or withdrawn ADD-PATH: %x", mon.BGPUpdate)
		}
	}
	if routes != 1 {
		t.Fatalf("replayed %d paths, want surviving path 7 only", routes)
	}
}

// An incomplete snapshot must close the collector rather than ending a partial
// dump with EOR. The opposite-polarity carrier proves the normal completed dump.
// RFC requirement: RFC7854-3.3-1 negative -- an unavailable snapshot never emits EOR claiming completion.
func TestRFC7854IncompleteAdjReplayCannotClaimCompletion(t *testing.T) {
	bp := replayPlugin(t)
	bp.peerUps["192.0.2.1"].replayErr = errAdjReplayLimit
	_, conn := primeReplayCollector(t, bp)
	for _, msg := range decodeBMPStream(t, conn.written()) {
		if mon, ok := msg.(*RouteMonitoring); ok && isEndOfRIB(mon) {
			t.Fatal("incomplete snapshot emitted EOR")
		}
	}
	select {
	case <-conn.closed:
	default:
		t.Fatal("incomplete snapshot left collector connected")
	}
	if !errors.Is(bp.peerUps["192.0.2.1"].replayErr, errAdjReplayLimit) {
		t.Fatal("reconnect cleared a storage failure without rebuilding the table")
	}
}

func TestRFC7854AdjReplaySeparatesDirections(t *testing.T) {
	bp := replayPlugin(t)
	bp.routeMonitorPolicy = policyAll
	replayUpdate(bp, rpc.DirectionReceived, announceBody(1), time.Time{}, 0)
	replayUpdate(bp, rpc.DirectionSent, announceBody(2), time.Time{}, 0)
	_, conn := primeReplayCollector(t, bp)
	var received, sent int
	for _, msg := range decodeBMPStream(t, conn.written()) {
		mon, ok := msg.(*RouteMonitoring)
		if !ok || isEndOfRIB(mon) {
			continue
		}
		if mon.Peer.isAdjRIBOut() {
			sent++
			if !mon.Peer.isPostPolicy() || !bytes.Equal(mon.BGPUpdate[message.HeaderLen:], announceBody(2)) {
				t.Fatalf("sent snapshot has wrong policy flags or route: %#v", mon)
			}
		} else {
			received++
			if mon.Peer.isPostPolicy() || !bytes.Equal(mon.BGPUpdate[message.HeaderLen:], announceBody(1)) {
				t.Fatalf("received snapshot has wrong policy flags or route: %#v", mon)
			}
		}
	}
	if received != 1 || sent != 1 {
		t.Fatalf("snapshot route counts pre=%d post=%d, want one each", received, sent)
	}
}

func TestRFC7854ChangedAttributesCanReturnToEarlierValue(t *testing.T) {
	bp := replayPlugin(t)
	ss, conn := primeReplayCollector(t, bp)
	bp.senders = []*senderSession{ss}
	conn.reset()
	for _, origin := range []byte{0, 2, 0} {
		body := assembleUpdateBody(nil, []byte{0x40, 1, 1, origin}, []byte{24, 10, 20, 30})
		replayUpdate(bp, rpc.DirectionReceived, body, time.Time{}, 0)
	}
	waitQueueDrained(t, ss)
	msgs := decodeBMPStream(t, conn.written())
	if len(msgs) != 3 {
		t.Fatalf("A, B, A attribute transitions produced %d messages, want 3", len(msgs))
	}
	last, ok := msgs[2].(*RouteMonitoring)
	want := assembleUpdateBody(nil, []byte{0x40, 1, 1, 0}, []byte{24, 10, 20, 30})
	if !ok || !bytes.Equal(last.BGPUpdate[message.HeaderLen:], want) {
		t.Fatalf("collector remained on superseded attributes: %#v", msgs[2])
	}
}

func TestRFC7854MultiprotocolWithdrawalDoesNotReappear(t *testing.T) {
	bp := replayPlugin(t)
	nlri := []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0, 1, 0, 0}
	reach := []byte{0x80, 14, 30, 0, 2, 1, 16}
	reach = append(reach, make([]byte, 16)...)
	reach = append(reach, 0)
	reach = append(reach, nlri...)
	replayUpdate(bp, rpc.DirectionReceived, assembleUpdateBody(nil, reach, nil), time.Time{}, 0)
	_, first := primeReplayCollector(t, bp)
	var routes int
	for _, msg := range decodeBMPStream(t, first.written()) {
		mon, ok := msg.(*RouteMonitoring)
		if !ok || isEndOfRIB(mon) {
			continue
		}
		routes++
		wu := wireu.NewWireUpdate(mon.BGPUpdate[message.HeaderLen:], 0)
		mp, err := wu.MPReach()
		if err != nil || mp == nil || mp.Family() != family.IPv6Unicast || !bytes.Equal(mp.NLRIBytes(), nlri) {
			t.Fatalf("replayed IPv6 route changed: %#v, %v", mp, err)
		}
	}
	if routes != 1 {
		t.Fatalf("initial IPv6 replay contains %d routes, want 1", routes)
	}
	unreach := append([]byte{0x80, 15, 12, 0, 2, 1}, nlri...)
	replayUpdate(bp, rpc.DirectionReceived, assembleUpdateBody(nil, unreach, nil), time.Time{}, 0)
	_, second := primeReplayCollector(t, bp)
	var eors int
	for _, msg := range decodeBMPStream(t, second.written()) {
		if mon, ok := msg.(*RouteMonitoring); ok {
			if !isEndOfRIB(mon) {
				t.Fatalf("withdrawn IPv6 route reappeared: %x", mon.BGPUpdate)
			}
			eors++
		}
	}
	if eors != 2 {
		t.Fatalf("empty replay completed %d families, want 2", eors)
	}
}

func TestBMPFramingPreservesMaximumExtendedBGPMessage(t *testing.T) {
	conn := newRecordingConn()
	ss := newTestSession(t, "extended", conn)
	body := make([]byte, message.ExtMsgLen-message.HeaderLen)
	binary.BigEndian.PutUint16(body[2:4], uint16(len(body)-4))
	body[4], body[5] = 0xd0, 99 // Optional transitive, extended-length opaque attribute.
	binary.BigEndian.PutUint16(body[6:8], uint16(len(body)-8))
	if err := ss.writeRouteMonitoring(testPeerHeader(), msgtype.TypeUPDATE, body); err != nil {
		t.Fatalf("maximum BGP UPDATE monitoring: %v", err)
	}
	if err := ss.writeRouteMirroring(testPeerHeader(), msgtype.TypeUPDATE, body); err != nil {
		t.Fatalf("maximum BGP UPDATE mirroring: %v", err)
	}
	tooLarge := append(bytes.Clone(body), 0)
	if err := ss.writeRouteMonitoring(testPeerHeader(), msgtype.TypeUPDATE, tooLarge); err == nil {
		t.Fatal("monitoring accepted an overflowing BGP length")
	}
	if err := ss.writeRouteMirroring(testPeerHeader(), msgtype.TypeUPDATE, tooLarge); err == nil {
		t.Fatal("mirroring accepted an overflowing BGP length")
	}
	waitQueueDrained(t, ss)
	msgs := decodeBMPStream(t, conn.written())
	if len(msgs) != 2 {
		t.Fatalf("collector received %d frames, want both valid maximum-size PDUs only", len(msgs))
	}
	monitor, ok := msgs[0].(*RouteMonitoring)
	if !ok || len(monitor.BGPUpdate) != message.ExtMsgLen || !bytes.Equal(monitor.BGPUpdate[message.HeaderLen:], body) {
		t.Fatalf("monitoring lost extended BGP data: %T", msgs[0])
	}
	mirror, ok := msgs[1].(*routeMirroring)
	if !ok || len(mirror.TLVs) != 1 || !bytes.Equal(mirror.TLVs[0].Value, monitor.BGPUpdate) {
		t.Fatalf("mirroring lost extended BGP data: %T", msgs[1])
	}
	if got := binary.BigEndian.Uint16(monitor.BGPUpdate[message.MarkerLen:]); got != message.ExtMsgLen {
		t.Fatalf("BGP length wrapped to %d", got)
	}
}

// The collector learns the actual established endpoints, then receives Peer
// Down for the same identity even when teardown has lost negotiated fields.
func TestBMPPeerLifecycleRetainsEstablishedIdentity(t *testing.T) {
	bp := replayPlugin(t)
	replayUpdate(bp, rpc.DirectionReceived, announceBody(1), time.Time{}, 0)
	ss, conn := primeReplayCollector(t, bp)
	bp.senders = []*senderSession{ss}
	messages := decodeBMPStream(t, conn.written())
	up, ok := messages[0].(*PeerUp)
	if !ok {
		t.Fatalf("first message = %T, want Peer Up", messages[0])
	}
	if up.LocalPort != 41793 {
		t.Fatalf("Peer Up local port = %d, want active opener port 41793", up.LocalPort)
	}
	if up.RemotePort != 179 {
		t.Fatalf("Peer Up remote port = %d, want connected port 179", up.RemotePort)
	}
	conn.reset()
	bp.handleStructuredEvent(&rpc.StructuredEvent{
		PeerAddress: "192.0.2.1", PeerAS: 65001,
		EventType: rpc.EventKindState, State: rpc.SessionStateDown,
	})
	waitQueueDrained(t, ss)
	messages = decodeBMPStream(t, conn.written())
	if len(messages) != 1 {
		t.Fatalf("teardown sent %d messages, want one Peer Down", len(messages))
	}
	down, ok := messages[0].(*PeerDown)
	if !ok {
		t.Fatalf("teardown message = %T, want Peer Down", messages[0])
	}
	if down.Peer != up.Peer {
		t.Fatalf("Peer Down identity = %#v, want established peer %#v", down.Peer, up.Peer)
	}
	_, reconnected := primeReplayCollector(t, bp)
	if got := reconnected.written(); len(got) != 0 {
		t.Fatalf("disconnected BGP peer was replayed: %x", got)
	}
}

// A truncated MP next hop cannot become an apparently empty, completed table.
func TestBMPMalformedMPReplayResetsCollector(t *testing.T) {
	bp := replayPlugin(t)
	ss, conn := primeReplayCollector(t, bp)
	bp.senders = []*senderSession{ss}
	conn.reset()
	reach := []byte{0x80, 14, 5, 0, 2, 1, 16, 0}
	replayUpdate(bp, rpc.DirectionReceived, assembleUpdateBody(nil, reach, nil), time.Time{}, 0)
	select {
	case <-conn.closed:
	default:
		t.Fatal("malformed MP next hop left the collector connected")
	}
	_, reconnected := primeReplayCollector(t, bp)
	for _, msg := range decodeBMPStream(t, reconnected.written()) {
		if mon, ok := msg.(*RouteMonitoring); ok {
			if isEndOfRIB(mon) {
				t.Fatal("malformed MP next hop became a completed empty snapshot")
			}
		}
	}
}
