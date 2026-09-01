// RFC 8671 conformance tests: Adj-RIB-Out support in BMP.
//
// Related: header.go -- the O flag and the per-peer header codec
// Related: bmp_events.go -- peerHeaderFromEvent, the only sender-side flag producer
// Related: msg.go -- decodePeerUp, which carries the Peer Up Information TLVs

package bmp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// peerFlagsReserved is the set of per-peer header Flags bits RFC 8671 Section 4
// reserves for future use. It is derived from the four defined flags, so a new
// flag constant moves the mask with it.
const peerFlagsReserved = ^(PeerFlagV | PeerFlagL | PeerFlagA | PeerFlagO)

// adminLabelTLV is Peer Up Information TLV type 4, assigned by RFC 8671
// Section 9.3 for the Admin Label defined in Section 6.3.1.
const adminLabelTLV uint16 = 4

// newPipeSender builds a plugin whose single collector session writes into conn.
// Route Mirroring is on so a test can read both the Route Monitoring message and
// the Route Mirroring message that one UPDATE event produces.
func newPipeSender(conn net.Conn, mirroring bool) *BMPPlugin {
	return &BMPPlugin{
		state:          newBMPState(),
		openCache:      make(map[string]*openPair),
		dedupState:     make(map[string]map[uint64]struct{}),
		routeMirroring: mirroring,
		stopCh:         make(chan struct{}),
		senders: []*senderSession{{
			name:   "test",
			conn:   conn,
			stopCh: make(chan struct{}),
		}},
	}
}

// updateEvent builds a structured UPDATE event for one direction.
func updateEvent(direction rpc.MessageDirection, body []byte) *rpc.StructuredEvent {
	return &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindUpdate,
		Direction:   direction,
		RawMessage:  &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: body},
	}
}

// RFC requirement: RFC8671-4-1 positive -- the per-peer header bits RFC 8671 Section 4
// reserves for future use are transmitted as 0. Both producers of a per-peer header
// (peerHeaderFromEvent for a BGP peer, locRIBPeerHeader for RFC 9069 Loc-RIB) leave the
// reserved bits clear, and writePeerHeader puts the byte on the wire unchanged.
func TestRFC8671ReservedPeerFlagsTransmittedAsZero(t *testing.T) {
	cases := []struct {
		name      string
		address   string
		direction rpc.MessageDirection
	}{
		{"ipv4 adj-rib-in", "10.0.0.1", rpc.DirectionReceived},
		{"ipv4 adj-rib-out", "10.0.0.1", rpc.DirectionSent},
		{"ipv6 adj-rib-in", "2001:db8::1", rpc.DirectionReceived},
		{"ipv6 adj-rib-out", "2001:db8::1", rpc.DirectionSent},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			se := &rpc.StructuredEvent{
				PeerAddress: one.address,
				PeerAS:      65001,
				Direction:   one.direction,
			}
			ph := peerHeaderFromEvent(se)
			if ph.Flags&peerFlagsReserved != 0 {
				t.Errorf("reserved bits set: flags = %#x, reserved mask = %#x", ph.Flags, peerFlagsReserved)
			}

			// The encoder is what actually transmits, so read the byte back off the wire.
			buf := make([]byte, PeerHeaderSize)
			writePeerHeader(buf, 0, ph)
			if buf[1]&peerFlagsReserved != 0 {
				t.Errorf("transmitted flags byte %#x carries a reserved bit", buf[1])
			}
		})
	}

	locRIB := locRIBPeerHeader(localIdentity{asn: 65001, routerID: 0x01020304}, time.Time{})
	if flags := locRIB.Flags; flags&peerFlagsReserved != 0 {
		t.Errorf("loc-rib flags = %#x, reserved bits must be transmitted as 0", flags)
	}
}

// RFC requirement: RFC8671-4-1 negative -- a received per-peer header whose reserved bits
// are all set is not rejected and does not change the meaning of the four defined flags.
// A receiver that read a reserved bit, or refused a header carrying one, would fail here.
func TestRFC8671ReservedPeerFlagsIgnoredOnReceipt(t *testing.T) {
	// Two base headers, because ignoring a reserved bit fails in two directions. A
	// decoder that masked a defined flag away loses it from the all-set base; a decoder
	// that let a reserved bit reach a defined flag INVENTS one on the V|O base, where L
	// and A start clear. Either base alone passes against half of that.
	bases := []struct {
		name  string
		flags uint8
	}{
		{"every defined flag set", PeerFlagV | PeerFlagL | PeerFlagA | PeerFlagO},
		{"only V and O set", PeerFlagV | PeerFlagO},
	}

	for _, base := range bases {
		t.Run(base.name, func(t *testing.T) {
			clear := testPeerHeader()
			clear.Flags = base.flags

			set := clear
			set.Flags = clear.Flags | peerFlagsReserved

			buf := make([]byte, PeerHeaderSize)
			writePeerHeader(buf, 0, set)
			decoded, n, err := decodePeerHeader(buf, 0)
			if err != nil {
				t.Fatalf("a header with every reserved bit set must decode: %v", err)
			}
			if n != PeerHeaderSize {
				t.Fatalf("consumed %d bytes, want %d", n, PeerHeaderSize)
			}
			if decoded.IsIPv6() != clear.IsIPv6() {
				t.Error("reserved bits changed the V flag reading")
			}
			if decoded.isPostPolicy() != clear.isPostPolicy() {
				t.Error("reserved bits changed the L flag reading")
			}
			if decoded.is2ByteAS() != clear.is2ByteAS() {
				t.Error("reserved bits changed the A flag reading")
			}
			if decoded.isAdjRIBOut() != clear.isAdjRIBOut() {
				t.Error("reserved bits changed the O flag reading")
			}

			// The same header inside a whole message: DecodeMsg must not refuse it either.
			msgBuf := make([]byte, 1024)
			rmLen := writeRouteMonitoring(msgBuf, 0, &RouteMonitoring{Peer: set, BGPUpdate: makeBGPOpen(65001, 0x01020304)})
			if _, err := DecodeMsg(msgBuf[:rmLen]); err != nil {
				t.Fatalf("a Route Monitoring message with reserved bits set must decode: %v", err)
			}
		})
	}
}

// RFC requirement: RFC8671-5.1-1 positive -- the post-policy Adj-RIB-Out feed conveys what
// is actually transmitted to the peer. The Route Monitoring message carries the transmitted
// UPDATE body byte for byte, under a per-peer header with O and L set.
func TestRFC8671PostPolicyConveysTransmittedBytes(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := newPipeSender(client, false)
	body := []byte{0x00, 0x00, 0x00, 0x04, 0x40, 0x01, 0x01, 0x00, 0x18, 0x0A, 0x00, 0x01}

	result := asyncRead(server)
	bp.handleStructuredEvent(updateEvent(rpc.DirectionSent, body))
	got := <-result
	if got.err != nil {
		t.Fatalf("read: %v", got.err)
	}
	rm, ok := got.msg.(*RouteMonitoring)
	if !ok {
		t.Fatalf("expected *RouteMonitoring, got %T", got.msg)
	}
	if rm.Peer.Flags&PeerFlagO == 0 || rm.Peer.Flags&PeerFlagL == 0 {
		t.Fatalf("post-policy Adj-RIB-Out needs O and L set, got flags %#x", rm.Peer.Flags)
	}
	conveyed := rm.BGPUpdate[message.HeaderLen:]
	if !bytes.Equal(conveyed, body) {
		t.Errorf("conveyed % x, transmitted % x", conveyed, body)
	}
}

// RFC requirement: RFC8671-5.1-1 negative -- "what is actually transmitted" is not
// "what ze can parse". An UPDATE carrying an attribute type ze has no codec for
// (optional transitive type 250) reaches the collector unchanged, so no normalizing or
// re-encoding path stands between the wire and the feed.
func TestRFC8671PostPolicyConveysUnknownAttributeUnchanged(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := newPipeSender(client, false)
	// Withdrawn length 0, attribute length 7, one optional transitive attribute of
	// type 250 with a 4-octet value, then NLRI 10.0.5.0/24.
	body := []byte{
		0x00, 0x00,
		0x00, 0x07,
		0xC0, 0xFA, 0x04, 0xDE, 0xAD, 0xBE, 0xEF,
		0x18, 0x0A, 0x00, 0x05,
	}

	result := asyncRead(server)
	bp.handleStructuredEvent(updateEvent(rpc.DirectionSent, body))
	got := <-result
	if got.err != nil {
		t.Fatalf("read: %v", got.err)
	}
	rm, ok := got.msg.(*RouteMonitoring)
	if !ok {
		t.Fatalf("expected *RouteMonitoring, got %T", got.msg)
	}
	conveyed := rm.BGPUpdate[message.HeaderLen:]
	if !bytes.Equal(conveyed, body) {
		t.Errorf("conveyed % x, transmitted % x", conveyed, body)
	}
}

// RFC requirement: RFC8671-5.1-1 negative -- "what is actually transmitted" bounds the
// message set from ABOVE as well. An Adj-RIB-Out event carrying no transmitted PDU
// produces no Route Monitoring message at all: ze conveys what went to the peer and never
// synthesizes a route that did not.
func TestRFC8671AdjRIBOutConveysNoUntransmittedUpdate(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := newPipeSender(client, false)

	// No RawMessage: nothing was transmitted, so nothing may be conveyed.
	bp.handleStructuredEvent(&rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindUpdate,
		Direction:   rpc.DirectionSent,
	})

	// A real transmission follows. It is the FIRST message on the wire when the event
	// above produced none, which is what this test asserts: net.Pipe is unbuffered, so
	// an extra message would be read here instead.
	body := []byte{0x00, 0x00, 0x00, 0x04, 0x40, 0x01, 0x01, 0x00, 0x18, 0x0A, 0x00, 0x02}
	result := asyncRead(server)
	bp.handleStructuredEvent(updateEvent(rpc.DirectionSent, body))
	got := <-result
	if got.err != nil {
		t.Fatalf("read: %v", got.err)
	}
	rm, ok := got.msg.(*RouteMonitoring)
	if !ok {
		t.Fatalf("expected *RouteMonitoring, got %T", got.msg)
	}
	conveyed := rm.BGPUpdate[message.HeaderLen:]
	if !bytes.Equal(conveyed, body) {
		t.Errorf("first message body % x, want the transmitted update % x", conveyed, body)
	}
}

// RFC requirement: RFC8671-6.1-1 positive -- the O flag is set on both message types that
// convey a RIB. A sent UPDATE produces a Route Monitoring message and a Route Mirroring
// message, and each carries O=1 to say the content is Adj-RIB-Out.
func TestRFC8671OFlagSetOnAdjRIBOutMessages(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := newPipeSender(client, true)
	body := []byte{0x00, 0x00, 0x00, 0x00, 0x00}

	go bp.handleStructuredEvent(updateEvent(rpc.DirectionSent, body))

	monitoring, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("Route Monitoring read: %v", err)
	}
	rm, ok := monitoring.(*RouteMonitoring)
	if !ok {
		t.Fatalf("expected *RouteMonitoring, got %T", monitoring)
	}
	if rm.Peer.Flags&PeerFlagO == 0 {
		t.Errorf("Route Monitoring flags %#x, O flag must be set for Adj-RIB-Out", rm.Peer.Flags)
	}

	mirroring, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("Route Mirroring read: %v", err)
	}
	mirror, ok := mirroring.(*routeMirroring)
	if !ok {
		t.Fatalf("expected *routeMirroring, got %T", mirroring)
	}
	if mirror.Peer.Flags&PeerFlagO == 0 {
		t.Errorf("Route Mirroring flags %#x, O flag must be set for Adj-RIB-Out", mirror.Peer.Flags)
	}
}

// RFC requirement: RFC8671-6.1-1 negative -- "accordingly" cuts both ways. A received
// UPDATE is Adj-RIB-In, so its Route Monitoring and Route Mirroring messages carry O=0.
// An implementation that set the O flag unconditionally would pass the positive case.
func TestRFC8671OFlagClearOnAdjRIBInMessages(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := newPipeSender(client, true)
	body := []byte{0x00, 0x00, 0x00, 0x00, 0x00}

	go bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, body))

	monitoring, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("Route Monitoring read: %v", err)
	}
	rm, ok := monitoring.(*RouteMonitoring)
	if !ok {
		t.Fatalf("expected *RouteMonitoring, got %T", monitoring)
	}
	if rm.Peer.Flags&PeerFlagO != 0 {
		t.Errorf("Route Monitoring flags %#x, O flag must be clear for Adj-RIB-In", rm.Peer.Flags)
	}

	mirroring, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("Route Mirroring read: %v", err)
	}
	mirror, ok := mirroring.(*routeMirroring)
	if !ok {
		t.Fatalf("expected *routeMirroring, got %T", mirroring)
	}
	if mirror.Peer.Flags&PeerFlagO != 0 {
		t.Errorf("Route Mirroring flags %#x, O flag must be clear for Adj-RIB-In", mirror.Peer.Flags)
	}
}

// TestRFC8671StatisticsReportClearsTheOFlag checks the ENCODER only: given an
// Adj-RIB-Out per-peer header, writeStatisticsReport puts the O flag on the wire
// as zero.
//
// It carries no RFC requirement tag, and it is not evidence that ze conforms to
// RFC 8671 Section 6.2. Nothing in production calls writeStatisticsReport, so ze
// transmits no Statistics Report and the obligation is never exercised.
// RFC8671-6.2-1 is a {gap} in rfc/short/rfc8671.md and the missing piece is the
// timer behind the statistics-timeout leaf (Thomas, 2026-08-31: an emission path
// added later that produced a non-zero O flag would leave this test green).
func TestRFC8671StatisticsReportClearsTheOFlag(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	peer := testPeerHeader()
	peer.Flags |= PeerFlagO | PeerFlagL

	result := asyncRead(server)
	if err := ss.writeStatisticsReport(peer, []StatEntry{makeStatGauge(StatPrefixesRejected, 42)}); err != nil {
		t.Fatalf("writeStatisticsReport: %v", err)
	}
	got := <-result
	if got.err != nil {
		t.Fatalf("read: %v", got.err)
	}
	sr, ok := got.msg.(*statisticsReport)
	if !ok {
		t.Fatalf("expected *statisticsReport, got %T", got.msg)
	}
	if sr.Peer.Flags&PeerFlagO != 0 {
		t.Errorf("statistics report flags = %#x, the O flag must be zero", sr.Peer.Flags)
	}
}

// TestRFC8671RouteMonitoringKeepsTheOFlag is the companion of the test above: the
// same Adj-RIB-Out header on a Route Monitoring message keeps the O flag, so
// clearing it in writeStatisticsReport is not a sender-wide loss of the direction.
//
// It carries no RFC requirement tag either. RFC8671-6.1-1 is proven at the wire
// level by TestRFC8671OFlagSetOnAdjRIBOutMessages and
// TestRFC8671OFlagClearOnAdjRIBInMessages, which drive the event path rather than
// the encoder.
func TestRFC8671RouteMonitoringKeepsTheOFlag(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	peer := testPeerHeader()
	peer.Flags |= PeerFlagO | PeerFlagL

	body := []byte{0x00, 0x00, 0x00, 0x04, 0x40, 0x01, 0x01, 0x00, 0x18, 0x0A, 0x00, 0x05}
	result := asyncRead(server)
	if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body); err != nil {
		t.Fatalf("writeRouteMonitoring: %v", err)
	}
	got := <-result
	if got.err != nil {
		t.Fatalf("read: %v", got.err)
	}
	rm, ok := got.msg.(*RouteMonitoring)
	if !ok {
		t.Fatalf("expected *RouteMonitoring, got %T", got.msg)
	}
	if rm.Peer.Flags&PeerFlagO == 0 {
		t.Errorf("route monitoring flags = %#x, the O flag must survive", rm.Peer.Flags)
	}
}

// peerUpWithAdminLabels encodes a Peer Up notification carrying labels as Admin Label
// TLVs, in the order given, and decodes it through the receiver's entry point.
func peerUpWithAdminLabels(t *testing.T, labels ...string) *PeerUp {
	t.Helper()

	tlvs := make([]TLV, 0, len(labels))
	for _, label := range labels {
		tlvs = append(tlvs, makeStringTLV(adminLabelTLV, label))
	}
	pu := &PeerUp{
		Peer:            testPeerHeader(),
		LocalPort:       179,
		RemotePort:      54321,
		SentOpenMsg:     makeBGPOpen(65001, 0x01020304),
		ReceivedOpenMsg: makeBGPOpen(65002, 0x05060708),
		InfoTLVs:        tlvs,
	}

	buf := make([]byte, 1024)
	n := writePeerUp(buf, 0, pu)
	msg, err := DecodeMsg(buf[:n])
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	decoded, ok := msg.(*PeerUp)
	if !ok {
		t.Fatalf("expected *PeerUp, got %T", msg)
	}
	return decoded
}

// adminLabels returns the values of the Admin Label TLVs of a decoded Peer Up, in the
// order the decoder produced them.
func adminLabels(pu *PeerUp) []string {
	var labels []string
	for _, tlv := range pu.InfoTLVs {
		if tlv.Type == adminLabelTLV {
			labels = append(labels, string(tlv.Value))
		}
	}
	return labels
}

// RFC requirement: RFC8671-6.3.1-1 positive -- a Peer Up carrying more than one Admin Label
// decodes with the labels in the order the wire gave them.
func TestRFC8671AdminLabelOrderPreserved(t *testing.T) {
	decoded := peerUpWithAdminLabels(t, "type=wholesale", "region=west", "site=lon1")

	want := []string{"type=wholesale", "region=west", "site=lon1"}
	got := adminLabels(decoded)
	if len(got) != len(want) {
		t.Fatalf("got %d Admin Labels, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("label %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// RFC requirement: RFC8671-6.3.1-1 negative -- the order comes from the wire and from
// nowhere else. The same three labels sent in the opposite order decode in that opposite
// order, so a decoder that sorted the labels, or that returned a fixed order, fails here
// while still passing the positive case.
func TestRFC8671AdminLabelReversedOrderPreserved(t *testing.T) {
	decoded := peerUpWithAdminLabels(t, "site=lon1", "region=west", "type=wholesale")

	want := []string{"site=lon1", "region=west", "type=wholesale"}
	got := adminLabels(decoded)
	if len(got) != len(want) {
		t.Fatalf("got %d Admin Labels, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("label %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// The collector every Section 7.2 test configures. The address and the port are
// what the live pipe session is built with, so a reload naming this collector
// keeps that session and dials nothing.
const (
	testCollectorName = "one"
	testCollectorAddr = "127.0.0.1"
	testCollectorPort = "11019"
)

// testCollectorEntry is the `collector` map of a sender configuration that
// names testCollectorName at the address the live pipe session carries.
func testCollectorEntry() map[string]any {
	return map[string]any{
		testCollectorName: map[string]any{"address": testCollectorAddr, "port": testCollectorPort},
	}
}

// liveCollectorSession installs one connected collector session on bp for
// testCollectorName and tells the plugin, over its own reload rail, that this
// collector is configured with sender.
//
// Every Section 7.2 test starts here, because the obligation is about a session
// that EXISTS under the configuration in force. A session installed on bp alone
// is not that: the plugin would hold a configuration naming no collector, and a
// later reload would read the session as one to stop rather than one to keep.
//
// Nothing reaches the collector while this runs, so the caller does not have to
// be reading the pipe yet. The session is already live and the configuration
// names it, so the reload starts nothing and stops nothing; and a caller that
// passes a sender configuration altering behavior passes one with no peer
// established under it, so the peer bounce writes nothing either.
func liveCollectorSession(t *testing.T, bp *BMPPlugin, engine *reloadEngine, conn net.Conn, sender map[string]any) *senderSession {
	t.Helper()

	ss := newSenderSession(testCollectorName, collectorConfig{Address: testCollectorAddr, Port: testCollectorPort})
	ss.conn = conn
	bp.mu.Lock()
	bp.senders = []*senderSession{ss}
	bp.mu.Unlock()
	t.Cleanup(func() {
		bp.stopSenders()
		bp.sessions.Wait()
	})

	if err := engine.reloadBGP(senderJSON(t, sender)); err != nil {
		t.Fatalf("configure the live collector: %v", err)
	}

	bp.mu.RLock()
	senders := bp.senders
	bp.mu.RUnlock()
	if len(senders) != 1 || senders[0] != ss {
		t.Fatalf("the reload naming the live collector left %d sessions, and not the live one", len(senders))
	}
	return ss
}

// requireCollectorSilent requires that nothing more reaches the collector.
//
// The read runs concurrently with the reload rather than after it, so a session
// the reload ends is seen as the read failing rather than as a write into a pipe
// nobody is reading. The window is short because everything a reload owes the
// collector is enqueued before the reload returns: the only wait is the session
// drain goroutine's wake-up.
func requireCollectorSilent(t *testing.T, result <-chan pipeResult, what string) {
	t.Helper()

	select {
	case got := <-result:
		if got.err != nil {
			t.Fatalf("%s: the collector session was closed: %v", what, got.err)
		}
		t.Fatalf("%s: the collector was sent a %T", what, got.msg)
	case <-time.After(500 * time.Millisecond):
	}
}

// awaitTermination requires that the bounce put a Termination on the collector socket.
// The wait is bounded because a session that is never bounced writes nothing at all: an
// unbounded read would hang the run rather than fail it.
func awaitTermination(t *testing.T, result <-chan pipeResult) {
	t.Helper()

	select {
	case got := <-result:
		if got.err != nil {
			t.Fatalf("the bounce owes the collector a Termination: %v", got.err)
		}
		if _, ok := got.msg.(*Termination); !ok {
			t.Fatalf("expected *Termination, got %T", got.msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no Termination reached the collector: the session was not bounced")
	}
}

// pipeSenderForReload installs one live collector session on bp, writing into conn, and
// registers the cleanup that stops whatever the reload leaves behind.
func pipeSenderForReload(t *testing.T, bp *BMPPlugin, conn net.Conn) *senderSession {
	t.Helper()

	ss := &senderSession{name: "live", conn: conn, stopCh: make(chan struct{})}
	bp.senders = []*senderSession{ss}
	t.Cleanup(func() {
		bp.stopSenders()
		bp.sessions.Wait()
	})
	return ss
}

// reloadEngine drives a config reload at a BMPPlugin the way the plugin engine
// drives one, so a test reaches the sender configuration only through the
// callbacks the plugin files with the SDK.
//
// The engine's reload rail is config-verify followed by config-apply
// (internal/component/plugin/server/config_tx_bridge.go, phaseKind.runRPC).
// Stage-2 configure is served once, by serveOne inside Plugin.Run, so a reload
// never reaches OnConfigure. Calling applySenderConfig from a test instead
// would assert nothing about whether a reload can reach it.
type reloadEngine struct {
	ctx context.Context
	mux *rpc.MuxConn
}

// startReloadEngine registers bp's callbacks on a real SDK plugin, runs the
// plugin over a pipe, and walks the five-stage startup handshake from the
// engine side so the SDK reaches its event loop.
//
// startupBGP is the `bgp` subtree Stage-2 configure delivers, or "" for a
// plugin that boots with no BMP sender configuration. A caller that passes one
// MUST install its collector session after this returns, because configure
// applies the configuration and the bounce would otherwise write a Termination
// into a pipe nobody is reading yet.
func startReloadEngine(t *testing.T, bp *BMPPlugin, startupBGP string) *reloadEngine {
	t.Helper()

	pluginSide, engineSide := net.Pipe()
	plugin := sdk.NewWithConn("bgp-bmp", pluginSide)
	mux := rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))

	bp.plugin = plugin
	bp.registerCallbacks()

	ctx, cancel := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() {
		runResult <- plugin.Run(ctx, sdk.Registration{
			WantsConfig: []string{configRootBGP, configRootEnvironment},
		})
	}()

	e := &reloadEngine{ctx: ctx, mux: mux}
	t.Cleanup(func() {
		cancel()
		closeLog(mux, "engine-mux")
		closeLog(plugin, "plugin")
		select {
		case <-runResult:
		case <-time.After(5 * time.Second):
			t.Error("plugin.Run did not return after the transport closed")
		}
	})

	var startupSections []sdk.ConfigSection
	if startupBGP != "" {
		startupSections = []sdk.ConfigSection{{Root: configRootBGP, Data: startupBGP}}
	}

	e.expectRequest(t, "ze-plugin-engine:declare-registration")
	e.call(t, "ze-plugin-callback:configure", struct {
		Sections []sdk.ConfigSection `json:"sections"`
	}{Sections: startupSections})
	e.expectRequest(t, "ze-plugin-engine:declare-capabilities")
	e.call(t, "ze-plugin-callback:share-registry", struct {
		Commands []sdk.RegistryCommand `json:"commands"`
	}{})
	e.expectRequest(t, "ze-plugin-engine:ready")

	return e
}

// expectRequest reads the next plugin-to-engine request, requires it to be the
// stage the handshake is at, and answers it.
func (e *reloadEngine) expectRequest(t *testing.T, method string) {
	t.Helper()

	select {
	case request := <-e.mux.Requests():
		if request.Method != method {
			t.Fatalf("startup: got request %q, want %q", request.Method, method)
		}
		if err := e.mux.SendOK(e.ctx, request.ID); err != nil {
			t.Fatalf("startup: answer %s: %v", method, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("startup: no %s request arrived", method)
	}
}

// call sends one engine-to-plugin callback and fails the test when the plugin
// answers with an error.
func (e *reloadEngine) call(t *testing.T, method string, params any) {
	t.Helper()

	if _, err := e.mux.CallRPC(e.ctx, method, params); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
}

// reloadSections delivers one config change over the reload rail: the candidate
// subtrees on config-verify, then the same content as a diff on config-apply.
//
// It returns an error rather than failing the test, because the bounce writes a
// Termination into an unbuffered pipe: the caller has to be reading that pipe
// while this runs, so this runs on its own goroutine and only the test
// goroutine may fail the test.
func (e *reloadEngine) reloadSections(sections []rpc.ConfigSection) error {
	if err := e.callConfig("ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{
		Sections: sections,
	}); err != nil {
		return err
	}
	diffs := make([]rpc.ConfigDiffSection, 0, len(sections))
	for _, section := range sections {
		diffs = append(diffs, rpc.ConfigDiffSection{Root: section.Root, Changed: section.Data})
	}
	return e.callConfig("ze-plugin-callback:config-apply", rpc.ConfigApplyInput{Sections: diffs})
}

// reloadBGP delivers one `bgp` root config change. bgpJSON is the whole
// {"bgp": {...}} subtree ExtractConfigSubtree hands a plugin.
func (e *reloadEngine) reloadBGP(bgpJSON string) error {
	return e.reloadSections([]rpc.ConfigSection{{Root: configRootBGP, Data: bgpJSON}})
}

// callConfig sends one reload callback and reports whether the plugin accepted
// it. The SDK answers a rejected verify or apply with a status document rather
// than an RPC error, which the engine reads as an abort, so both are read.
func (e *reloadEngine) callConfig(method string, params any) error {
	result, err := e.mux.CallRPC(e.ctx, method, params)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if len(result) > 0 {
		if err := json.Unmarshal(result, &out); err != nil {
			return fmt.Errorf("%s: decode answer: %w", method, err)
		}
	}
	if out.Status == "error" {
		return fmt.Errorf("%s: plugin rejected the reload: %s", method, out.Error)
	}
	return nil
}

// awaitReload requires the reload running on its own goroutine to have finished
// and been accepted. Bounded, because a reload that never returns would
// otherwise hang the run rather than fail it.
func awaitReload(t *testing.T, done <-chan error) {
	t.Helper()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the reload never completed")
	}
}

// rollback delivers the config-rollback a failed transaction fans out to every
// participant. It returns an error for the same reason reloadSections does: the
// restore bounces the collector session, which writes into an unbuffered pipe.
func (e *reloadEngine) rollback() error {
	return e.callConfig("ze-plugin-callback:config-rollback", struct {
		TransactionID string `json:"transaction-id"`
	}{TransactionID: "tx-test"})
}

// senderJSON builds the `bgp` config subtree for one BMP sender configuration,
// in the shape ExtractConfigSubtree delivers it.
func senderJSON(t *testing.T, sender map[string]any) string {
	t.Helper()

	encoded, err := json.Marshal(map[string]any{
		"bgp": map[string]any{"bmp": map[string]any{"sender": sender}},
	})
	if err != nil {
		t.Fatalf("marshal sender config: %v", err)
	}
	return string(encoded)
}

// establishedPeer builds the Peer Up state of one established BGP peer, as
// recordPeerUp would from that peer's state event and its cached OPEN PDUs.
func establishedPeer(address string, asn uint16) *peerUpState {
	st := &peerUpState{
		peer:      PeerHeader{PeerType: PeerTypeGlobal, PeerAS: uint32(asn)},
		localPort: 179,
		sentOpen:  makeBGPOpen(65000, 0x0A000064),
		recvOpen:  makeBGPOpen(asn, 0x0A000001),
	}
	parseIPInto(address, &st.peer.Address)
	return st
}

// RFC requirement: RFC8671-7.2-1 positive -- "In case of any change that results in the
// alteration of behavior of an existing BMP session (i.e., changes to filtering and
// table names), the session MUST be bounced with a Peer Down/Peer Up sequence." Changing
// the route-monitoring policy under two established peers puts a Peer Down and then a
// Peer Up on the wire for EACH of them, and leaves the BMP session up: the collector
// keeps its connection and re-reads both peers under the new policy. It does not get
// the routes back with them -- no Adj-RIB-In replay follows a Peer Up -- so what is
// asserted below is the Peer Down/Peer Up pair and the session's survival, never a
// route count.
//
// What this must not do is end the session. A Termination and a TCP close discard every
// peer's state on the collector, including the state the change did not touch, and cost
// it a full re-dump. That is a different act from the one the section names, so the
// absence of a Termination is asserted rather than assumed.
//
// The reload is delivered over the engine's own rail (config-verify then config-apply)
// rather than by calling the plugin's apply function, so the test fails if the plugin
// files no reload callback at all.
func TestRFC8671BehaviorChangeBouncesEachPeerAndKeepsTheSession(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{
		stopCh: make(chan struct{}),
		peerUps: map[string]*peerUpState{
			"10.0.0.1": establishedPeer("10.0.0.1", 65001),
			"10.0.0.2": establishedPeer("10.0.0.2", 65002),
		},
		dedupState: map[string]map[uint64]struct{}{
			"10.0.0.1": {7: struct{}{}},
			"10.0.0.2": {9: struct{}{}},
		},
	}
	engine := startReloadEngine(t, bp, "")
	live := liveCollectorSession(t, bp, engine, client, map[string]any{
		"route-monitoring-policy": policyAll,
		"collector":               testCollectorEntry(),
	})

	// The one leaf that moves is the policy. The collector set is byte for byte
	// what the session already runs under, so nothing about which sessions exist
	// can account for what the collector is sent.
	config := senderJSON(t, map[string]any{
		"route-monitoring-policy": policyPostPolicy,
		"collector":               testCollectorEntry(),
	})

	done := make(chan error, 1)
	go func() { done <- engine.reloadBGP(config) }()

	// Two peers, two messages each, in whichever order the peer map yielded.
	downFor := map[string]int{}
	upFor := map[string]int{}
	for range 4 {
		msg, err := readBMPFromPipe(server)
		if err != nil {
			t.Fatalf("read the peer bounce: %v", err)
		}
		switch m := msg.(type) {
		case *PeerDown:
			// RFC 7854 Section 4.9 reason 5: "Information for this peer will no
			// longer be sent to the monitoring station for configuration
			// reasons." The BGP session is untouched, so a reason that reports
			// the peer going down would be false.
			if m.Reason != PeerDownDeconfigured {
				t.Errorf("Peer Down reason = %d, want %d (configuration reasons)", m.Reason, PeerDownDeconfigured)
			}
			downFor[peerAddressString(m.Peer)]++
		case *PeerUp:
			address := peerAddressString(m.Peer)
			if downFor[address] == 0 {
				t.Errorf("peer %s was announced up before it was announced down", address)
			}
			upFor[address]++
		default:
			t.Fatalf("the bounce put a %T on the wire, want a Peer Down or a Peer Up", msg)
		}
	}
	awaitReload(t, done)

	for _, address := range []string{"10.0.0.1", "10.0.0.2"} {
		if downFor[address] != 1 || upFor[address] != 1 {
			t.Errorf("peer %s got %d Peer Down and %d Peer Up, want 1 and 1", address, downFor[address], upFor[address])
		}
	}

	// Nothing else: no Termination, and no second bounce.
	requireCollectorSilent(t, asyncRead(server), "after the peer bounce")

	bp.mu.RLock()
	senders := bp.senders
	policy := bp.routeMonitorPolicy
	dedup := len(bp.dedupState)
	bp.mu.RUnlock()
	if policy != policyPostPolicy {
		t.Errorf("policy = %q, want %q", policy, policyPostPolicy)
	}
	if len(senders) != 1 || senders[0] != live {
		t.Fatalf("the BMP session did not survive the change: got %d sessions, and not the live one", len(senders))
	}
	// A Peer Down implicitly withdraws the peer's routes (RFC 7854 Section 4.9),
	// so a body the collector has just been made to forget must not be
	// suppressed as a duplicate the next time it is announced.
	if dedup != 0 {
		t.Errorf("%d peers kept their Route Monitoring dedup state across the bounce, want 0", dedup)
	}
}

// RFC requirement: RFC8671-7.2-1 negative -- the bounce is owed to a change in BEHAVIOR,
// and a reload that alters none owes the collector nothing. The plugin is handed the
// whole `bgp` root, so it is told about every neighbor the operator adds; a plugin that
// acted on arrival rather than on comparison would bounce every peer on every collector
// each time an unrelated neighbor was configured.
//
// The sender subtree here is byte for byte the one in force. Only a `neighbor` beside it
// is new, which is what makes this the negative of the test above: the same rail, the
// same live session, the same established peer, and no Peer Down, no Peer Up, no
// Termination.
func TestRFC8671UnrelatedBGPChangeBouncesNothing(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{
		stopCh:  make(chan struct{}),
		peerUps: map[string]*peerUpState{"10.0.0.1": establishedPeer("10.0.0.1", 65001)},
	}
	engine := startReloadEngine(t, bp, "")
	sender := map[string]any{
		"route-monitoring-policy": policyAll,
		"collector":               testCollectorEntry(),
	}
	live := liveCollectorSession(t, bp, engine, client, sender)

	encoded, err := json.Marshal(map[string]any{
		"bgp": map[string]any{
			"bmp":      map[string]any{"sender": sender},
			"neighbor": map[string]any{"192.0.2.7": map[string]any{"peer-as": "65007"}},
		},
	})
	if err != nil {
		t.Fatalf("marshal the neighbor config: %v", err)
	}

	result := asyncRead(server)
	done := make(chan error, 1)
	go func() { done <- engine.reloadBGP(string(encoded)) }()
	awaitReload(t, done)
	requireCollectorSilent(t, result, "a reload that added a neighbor")

	bp.mu.RLock()
	senders := bp.senders
	bp.mu.RUnlock()
	if len(senders) != 1 || senders[0] != live {
		t.Fatalf("adding a neighbor replaced the collector session: got %d sessions, and not the live one", len(senders))
	}
}

// RFC requirement: RFC8671-7.2-1 negative -- the bounce is not skipped when the change
// is the removal of every collector. That reload ENDS an existing BMP session rather
// than altering one, so the session is stopped with the RFC 7854 Section 4.5 Termination
// it is owed, rather than left streaming to a collector the configuration no longer
// names. It is the Section 7.2 peer bounce that must not happen here: the section
// governs a session that CONTINUES under altered behavior, and there is no session left
// to announce a peer on.
//
// This is also the regression test for the `len(cfg.Collectors) > 0` guard that used to
// sit in front of the sender reconciliation in applySenderConfig: with the guard
// restored, the live session is never stopped and this test goes red.
func TestRFC8671RemovingEveryCollectorBouncesTheSession(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{stopCh: make(chan struct{})}
	engine := startReloadEngine(t, bp, "")
	liveCollectorSession(t, bp, engine, client, map[string]any{
		"route-monitoring-policy": policyPrePolicy,
		"route-mirroring":         "true",
		"collector":               testCollectorEntry(),
	})

	config := senderJSON(t, map[string]any{"route-monitoring-policy": policyAll})

	result := asyncRead(server)
	done := make(chan error, 1)
	go func() { done <- engine.reloadBGP(config) }()
	awaitTermination(t, result)
	awaitReload(t, done)

	bp.mu.RLock()
	senders := bp.senders
	mirroring := bp.routeMirroring
	bp.mu.RUnlock()
	if len(senders) != 0 {
		t.Errorf("got %d senders after every collector was removed, want 0", len(senders))
	}
	if mirroring {
		t.Error("route mirroring stayed on after a reload that turned it off")
	}
}

// TestBMPReloadWithoutBGPSectionLeavesTheSessionUp
// VALIDATES: a reload that carries no `bgp` section carries no change to the
// BMP sender configuration, so it leaves the live collector session up. The
// method is to drive one reload over the engine's callbacks with an
// `environment` section only, and read the sender list back off the plugin.
// PREVENTS: bouncing every collector on a reload that changed nothing they
// depend on. RFC 8671 Section 7.2 owes the bounce to a change, and a plugin
// that bounced on every reload would cost each collector a full re-dump each
// time an unrelated config root moved.
func TestBMPReloadWithoutBGPSectionLeavesTheSessionUp(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{routeMonitorPolicy: policyPrePolicy, stopCh: make(chan struct{})}
	live := pipeSenderForReload(t, bp, client)
	engine := startReloadEngine(t, bp, "")

	// No collector session is bounced, so nothing writes to the pipe and the
	// reload can run on this goroutine.
	if err := engine.reloadSections([]rpc.ConfigSection{
		{Root: configRootEnvironment, Data: `{"environment":{}}`},
	}); err != nil {
		t.Fatalf("reload: %v", err)
	}

	bp.mu.RLock()
	senders := bp.senders
	bp.mu.RUnlock()
	if len(senders) != 1 || senders[0] != live {
		t.Fatalf("a reload carrying no bgp section must leave the live session in place, got %d senders", len(senders))
	}
}

// TestBMPRolledBackReloadRestoresTheSenderConfiguration
// VALIDATES: a reload whose transaction rolls back leaves the collectors on the
// configuration the router kept, not on the one the commit failed to take.
// The method is to drive verify, apply and rollback over the engine's own
// callbacks and read the policy back off the plugin.
// PREVENTS: the BMP feed describing a configuration that never took, with
// nothing to correct it until the next reload. Registering config-apply is what
// makes that divergence reachable, because before it the SDK's default answered
// every apply without applying anything.
func TestBMPRolledBackReloadRestoresTheSenderConfiguration(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	// The plugin boots through Stage-2 configure, so the configuration the
	// rollback restores is the one the router actually started with. The live
	// session is installed after: configure would otherwise bounce it into a
	// pipe nobody is reading.
	bp := &BMPPlugin{stopCh: make(chan struct{})}
	engine := startReloadEngine(t, bp, senderJSON(t, map[string]any{"route-monitoring-policy": policyPrePolicy}))
	pipeSenderForReload(t, bp, client)

	config := senderJSON(t, map[string]any{"route-monitoring-policy": policyAll})

	// The apply bounces the live session, so it has to run while the test reads.
	result := asyncRead(server)
	done := make(chan error, 1)
	go func() { done <- engine.reloadBGP(config) }()
	awaitTermination(t, result)
	awaitReload(t, done)

	bp.mu.RLock()
	applied := bp.routeMonitorPolicy
	bp.mu.RUnlock()
	if applied != policyAll {
		t.Fatalf("policy after the apply = %q, want %q", applied, policyAll)
	}

	// The rollback starts no session, so nothing writes to the pipe.
	if err := engine.rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	bp.mu.RLock()
	restored := bp.routeMonitorPolicy
	senders := bp.senders
	bp.mu.RUnlock()
	if restored != policyPrePolicy {
		t.Errorf("policy after the rollback = %q, want the pre-reload %q", restored, policyPrePolicy)
	}
	if len(senders) != 0 {
		t.Errorf("got %d senders after the rollback, want the pre-reload 0", len(senders))
	}
}

// TestBMPRollbackWithoutAnApplyLeavesTheSenderConfigurationAlone
// VALIDATES: the rollback of a transaction whose apply never reached this
// plugin restores nothing. The method is to run one reload to completion, then
// drive a second transaction that carries no `bgp` section and roll it back.
// PREVENTS: a stale stash from an earlier committed transaction undoing a
// change the operator did commit.
func TestBMPRollbackWithoutAnApplyLeavesTheSenderConfigurationAlone(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{stopCh: make(chan struct{})}
	engine := startReloadEngine(t, bp, senderJSON(t, map[string]any{"route-monitoring-policy": policyPrePolicy}))
	pipeSenderForReload(t, bp, client)

	result := asyncRead(server)
	done := make(chan error, 1)
	go func() { done <- engine.reloadBGP(senderJSON(t, map[string]any{"route-monitoring-policy": policyAll})) }()
	awaitTermination(t, result)
	awaitReload(t, done)

	// A second transaction that touches no `bgp` section, then rolled back.
	if err := engine.reloadSections([]rpc.ConfigSection{
		{Root: configRootEnvironment, Data: `{"environment":{}}`},
	}); err != nil {
		t.Fatalf("second reload: %v", err)
	}
	if err := engine.rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	bp.mu.RLock()
	policy := bp.routeMonitorPolicy
	bp.mu.RUnlock()
	if policy != policyAll {
		t.Errorf("policy = %q, want the committed %q: the rollback undid a committed change", policy, policyAll)
	}
}

// RFC requirement: RFC9069-5.3-1 positive -- "The Peer Down notification MUST use reason
// code 6." The Loc-RIB Peer Down this reload puts on the wire carries reason 6. What
// follows the reason is the VRF/Table Name TLV every ze Loc-RIB Peer Up carries, which is
// RFC9069-5.2.1-1 rather than this row, and is asserted there.
//
// TestBMPReloadTurningLocRIBOffUnsubscribesAndSaysSo
// VALIDATES: a reload that removes `loc-rib true` stops Loc-RIB monitoring and
// tells the collector, before the bounce replaces the session that carries the
// Loc-RIB Peer Up. The method is to drive the reload over the engine's
// callbacks and read the two messages off the collector socket in order.
// PREVENTS: Loc-RIB Route Monitoring streaming on after the operator turned it
// off. The startup rail can only turn monitoring ON, because nothing is
// subscribed when it runs; the reload rail is the one that has to be able to
// turn it off, and registering config-apply is what makes that reachable.
//
// The plugin is booted WITH `loc-rib true`, because the reload is judged against
// the configuration in force: a plugin told at startup that monitoring was off
// reads this reload as no change at all, and the case would prove nothing.
func TestBMPReloadTurningLocRIBOffUnsubscribesAndSaysSo(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{stopCh: make(chan struct{})}
	engine := startReloadEngine(t, bp, senderJSON(t, map[string]any{"loc-rib": yangTrue}))

	unsubscribed := false
	bp.locRIBUnsub = func() { unsubscribed = true }
	bp.locRIBUp = true
	pipeSenderForReload(t, bp, client)

	// The new configuration names no loc-rib leaf, which is how the operator
	// turns Loc-RIB monitoring off.
	done := make(chan error, 1)
	go func() { done <- engine.reloadBGP(senderJSON(t, map[string]any{"route-monitoring-policy": policyAll})) }()

	first, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("read the loc-rib peer down: %v", err)
	}
	down, ok := first.(*PeerDown)
	if !ok {
		t.Fatalf("expected *PeerDown before the bounce, got %T", first)
	}
	if down.Peer.PeerType != PeerTypeLocRIB {
		t.Errorf("peer type = %d, want the Loc-RIB peer %d", down.Peer.PeerType, PeerTypeLocRIB)
	}
	// RFC 9069 Section 5.3: "The Peer Down notification MUST use reason code 6."
	if down.Reason != PeerDownTLVData {
		t.Errorf("loc-rib Peer Down reason = %d, want %d (RFC 9069 Section 5.3)", down.Reason, PeerDownTLVData)
	}

	second, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("read the termination: %v", err)
	}
	if _, ok := second.(*Termination); !ok {
		t.Fatalf("expected *Termination after the Peer Down, got %T", second)
	}
	awaitReload(t, done)

	bp.mu.RLock()
	unsub := bp.locRIBUnsub
	up := bp.locRIBUp
	bp.mu.RUnlock()
	if !unsubscribed || unsub != nil {
		t.Error("the reload left the plugin subscribed to best-change, so Loc-RIB monitoring is still running")
	}
	if up {
		t.Error("locRIBUp stayed set, so shutdown would send a second Loc-RIB Peer Down")
	}
}

// RFC requirement: RFC9069-x-6 positive, RFC9069-5.2-1 positive -- the Loc-RIB emulated
// peer a collector is actually sent describes this router: RFC 9069 Section 5.1 "Peer
// Autonomous System (AS): Set to the primary router BGP autonomous system number (ASN)",
// and Section 5.2 "Sent OPEN Message: This is a fabricated BGP OPEN message. Capabilities
// MUST include the 4-octet ASN and all necessary capabilities to represent the Loc-RIB
// Route Monitoring messages", repeated into the received OPEN field.
//
// This is the same rail the RFC 8671 Section 7.2 tests use, and it is the rail that
// matters: the operator turns `loc-rib` on with a commit, the commit arrives as
// config-verify then config-apply over a real sdk.Plugin, and what the collector reads
// off its socket is the evidence. Reading the fields off a constructor instead would
// leave a plugin that never reaches the constructor green.
func TestRFC9069ReloadTurningLocRIBOnAnnouncesTheRouterIdentity(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	// The router's own identity, as a cached sent OPEN carrying AS_TRANS in My AS
	// and the real 4-octet ASN in the capability: what ze puts on the wire for an
	// ASN above 65535, and the case a 2-octet read gets wrong.
	sentOpen := fabricateLocRIBOpen(localIdentity{asn: 4200000001, routerID: 0x0a141e01})
	bp := &BMPPlugin{
		stopCh:    make(chan struct{}),
		openCache: map[string]*openPair{"10.0.0.1": {sent: sentOpen}},
	}
	dumpBus(t, func() *ribevents.BestChangeBatch { return locRIBBatch(1, true) })

	engine := startReloadEngine(t, bp, "")
	sender := map[string]any{"collector": testCollectorEntry()}
	liveCollectorSession(t, bp, engine, client, sender)

	// The commit that turns Loc-RIB monitoring on, over the reload rail.
	sender["loc-rib"] = yangTrue
	done := make(chan error, 1)
	go func() { done <- engine.reloadBGP(senderJSON(t, sender)) }()

	// The peer bounce comes first (the behavior changed, and no peer is
	// established), then the Loc-RIB Peer Up the dump owes.
	first, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("read the loc-rib peer up: %v", err)
	}
	up, ok := first.(*PeerUp)
	if !ok {
		t.Fatalf("first message = %T, want the Loc-RIB *PeerUp", first)
	}
	if up.Peer.PeerType != PeerTypeLocRIB {
		t.Fatalf("peer type = %d, want the Loc-RIB peer %d", up.Peer.PeerType, PeerTypeLocRIB)
	}
	if up.Peer.PeerAS != 4200000001 {
		t.Errorf("Peer AS = %d, want the router's own ASN 4200000001 (RFC 9069 Section 5.1)", up.Peer.PeerAS)
	}
	if up.Peer.PeerBGPID != 0x0a141e01 {
		t.Errorf("Peer BGP ID = %#x, want the router-id 0x0a141e01", up.Peer.PeerBGPID)
	}
	if len(up.SentOpenMsg) == 0 || !bytes.Equal(up.SentOpenMsg, up.ReceivedOpenMsg) {
		t.Fatalf("sent OPEN is %d octets and the received OPEN is %d: Section 5.2 requires a fabricated OPEN repeated into both",
			len(up.SentOpenMsg), len(up.ReceivedOpenMsg))
	}

	parsed, err := message.UnpackOpen(up.SentOpenMsg[message.HeaderLen:])
	if err != nil {
		t.Fatalf("the Peer Up's OPEN does not decode: %v", err)
	}
	caps, err := capability.ParseFromOptionalParams(parsed.OptionalParams, parsed.ExtendedParams)
	if err != nil {
		t.Fatalf("the Peer Up's OPEN carries capabilities that do not decode: %v", err)
	}
	var asn uint32
	families := 0
	for _, capa := range caps {
		switch c := capa.(type) {
		case *capability.ASN4:
			asn = c.ASN
		case *capability.Multiprotocol:
			families++
		}
	}
	if asn != 4200000001 {
		t.Errorf("the OPEN's 4-octet ASN capability = %d, want 4200000001", asn)
	}
	if families != len(dumpFamilies) {
		t.Errorf("the OPEN advertises %d address families, want the %d the dump delivers", families, len(dumpFamilies))
	}

	awaitReload(t, done)
}
