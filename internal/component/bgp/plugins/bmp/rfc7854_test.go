package bmp

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Design: the RFC 7854 obligations the 2026-09-21 extraction walk added, each
// proven at the boundary Ze owns. A sender obligation is read off the collector
// end of a pipe; a receiver obligation is proven by whether the session survives
// the message, which net.Pipe makes deterministic: a write after the receiver
// closed its end returns io.ErrClosedPipe, and a write the receiver consumed
// returns nil. Every producer runs on the test goroutine so a discrimination
// break reddens a test go test can name.

// RFC requirement: RFC7854-x-18 positive — the Initiation the sender opens a session with carries a sysDescr TLV (type 1) with a non-empty value
// The collector end of a pipe reads the Initiation sendInitiation writes.
func TestRFC7854InitiationCarriesSysDescr(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	ss := &senderSession{name: "test", stopCh: make(chan struct{})}
	result := asyncRead(server)
	if err := ss.sendInitiation(client); err != nil {
		t.Fatalf("sendInitiation: %v", err)
	}
	res := <-result
	if res.err != nil {
		t.Fatalf("read initiation: %v", res.err)
	}
	init, ok := res.msg.(*Initiation)
	if !ok {
		t.Fatalf("first message = %T, want *Initiation", res.msg)
	}
	found := false
	for _, tlv := range init.TLVs {
		if tlv.Type != InitTLVSysDescr {
			continue
		}
		found = true
		if len(tlv.Value) == 0 {
			t.Error("sysDescr TLV carries an empty value")
		}
	}
	if !found {
		t.Errorf("Initiation TLVs %v carry no sysDescr TLV (type %d)", init.TLVs, InitTLVSysDescr)
	}
}

// RFC requirement: RFC7854-3.2-1 positive — each failed dial doubles the wait before the next retry, and the wait is capped at reconnectMax
// nextReconnectWait is the arithmetic run() applies between two refused dials.
func TestRFC7854RetryBackoffDoubles(t *testing.T) {
	cases := []struct {
		current time.Duration
		want    time.Duration
	}{
		{30 * time.Second, 60 * time.Second},
		{60 * time.Second, 120 * time.Second},
		{360 * time.Second, 720 * time.Second},
		{500 * time.Second, reconnectMax},
		{reconnectMax, reconnectMax},
	}
	for _, one := range cases {
		if got := nextReconnectWait(one.current); got != one.want {
			t.Errorf("nextReconnectWait(%s) = %s, want %s", one.current, got, one.want)
		}
	}
}

// loopbackPort is the port a loopback listener was given.
func loopbackPort(t *testing.T, ln net.Listener) int {
	t.Helper()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address %T is not a TCP address", ln.Addr())
	}
	return addr.Port
}

// acceptDeadline bounds how long the next Accept on ln can block.
func acceptDeadline(ln net.Listener, d time.Duration) error {
	tcp, ok := ln.(*net.TCPListener)
	if !ok {
		return fmt.Errorf("listener %T carries no deadline", ln)
	}
	return tcp.SetDeadline(time.Now().Add(d))
}

// listenLoopback opens a TCP listener on the given loopback port (0 for any).
func listenLoopback(t *testing.T, port int) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln
}

// RFC requirement: RFC7854-3.2-1 negative — a refused dial is not retried before the base wait: the collector port comes up a quarter of the way into that wait and the sender still arrives no sooner than the wait
// run() executes on the test goroutine and returns once the collector side stops it.
func TestRFC7854RetryNotBeforeBackoff(t *testing.T) {
	// Reserve a port, then close it so the sender's first dial is refused.
	ln := listenLoopback(t, 0)
	port := loopbackPort(t, ln)
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	const base = 400 * time.Millisecond
	ss := newSenderSession("test", collectorConfig{Address: "127.0.0.1", Port: strconv.Itoa(port)})
	ss.retryWait = base

	type arrival struct {
		at  time.Duration
		err error
	}
	arrived := make(chan arrival, 1)
	start := time.Now()
	go func() {
		defer ss.stop()
		// The port comes back a quarter of the way into the base wait, well
		// after the first (refused) dial, so a sender that retried at once
		// would connect long before the wait elapsed.
		reopen := time.NewTimer(base / 4)
		<-reopen.C
		var lc net.ListenConfig
		again, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			arrived <- arrival{err: fmt.Errorf("relisten: %w", err)}
			return
		}
		defer closeLog(again, "relisten")
		if err := acceptDeadline(again, 10*time.Second); err != nil {
			arrived <- arrival{err: err}
			return
		}
		c, err := again.Accept()
		if err != nil {
			arrived <- arrival{err: fmt.Errorf("accept: %w", err)}
			return
		}
		arrived <- arrival{at: time.Since(start)}
		closeLog(c, "accepted")
	}()

	// The producer runs here: run() returns once stop() closes stopCh.
	ss.run()

	got := <-arrived
	if got.err != nil {
		t.Fatalf("collector side: %v", got.err)
	}
	if got.at < base {
		t.Errorf("sender reconnected %s after the refused dial, before the %s backoff elapsed", got.at, base)
	}
}

// RFC requirement: RFC7854-3.2-2 positive — after a collector accepts and closes a session, the sender establishes a new one
// RFC requirement: RFC7854-3.2-2 negative — the second session is not established before the base wait has elapsed since the first one ended
// run() executes on the test goroutine; the collector goroutine accepts twice and closes each session at once.
func TestRFC7854SessionEstablishmentRateLimited(t *testing.T) {
	ln := listenLoopback(t, 0)
	defer closeLog(ln, "listener")
	port := loopbackPort(t, ln)

	const base = 300 * time.Millisecond
	ss := newSenderSession("test", collectorConfig{Address: "127.0.0.1", Port: strconv.Itoa(port)})
	ss.retryWait = base

	type accepted struct {
		at  time.Time
		err error
	}
	sessions := make(chan accepted, 2)
	go func() {
		defer ss.stop()
		for i := range 2 {
			if err := acceptDeadline(ln, 10*time.Second); err != nil {
				sessions <- accepted{err: err}
				return
			}
			c, err := ln.Accept()
			if err != nil {
				sessions <- accepted{err: fmt.Errorf("accept %d: %w", i+1, err)}
				return
			}
			sessions <- accepted{at: time.Now()}
			// The collector ends the session at once: the rate limit is
			// what stands between this close and the next dial.
			closeLog(c, "accepted")
		}
	}()

	ss.run()

	first := <-sessions
	if first.err != nil {
		t.Fatalf("first session: %v", first.err)
	}
	second := <-sessions
	if second.err != nil {
		t.Fatalf("no second session after the collector closed the first: %v", second.err)
	}
	if gap := second.at.Sub(first.at); gap < base {
		t.Errorf("second session established %s after the first, before the %s wait", gap, base)
	}
}

// receiverSurvives runs the receiver over a pipe, writes first and then a
// Termination from the router end, and returns the Termination write's error:
// nil when the receiver consumed it, io.ErrClosedPipe when the receiver had
// already ended the session on first.
func receiverSurvives(t *testing.T, first []byte) error {
	t.Helper()
	bp := &BMPPlugin{state: newBMPState(), stopCh: make(chan struct{})}
	server, client := net.Pipe()
	defer closeLog(client, "router")

	var buf [64]byte
	term := &Termination{TLVs: []TLV{{Type: TermTLVReason, Length: 2, Value: []byte{0, 0}}}}
	n := writeTermination(buf[:], 0, term)

	done := make(chan error, 1)
	go func() {
		if _, err := client.Write(first); err != nil {
			done <- fmt.Errorf("write first message: %w", err)
			return
		}
		_, err := client.Write(buf[:n])
		done <- err
	}()

	bp.handleSession(server)
	return <-done
}

// RFC requirement: RFC7854-4.1-1 positive — a message of an unrecognized type (200) is skipped and the session goes on to consume the Termination that follows it
// The receiver runs on the test goroutine; the Termination write succeeding is the proof the session survived.
func TestRFC7854UnrecognizedMessageTypeIgnored(t *testing.T) {
	unknown := []byte{Version, 0, 0, 0, 10, 200, 0xde, 0xad, 0xbe, 0xef}
	if err := receiverSurvives(t, unknown); err != nil {
		t.Fatalf("receiver ended the session on an unrecognized message type: %v", err)
	}
}

// RFC requirement: RFC7854-4.1-1 negative — only the TYPE is forgiven: a recognized type (Peer Up) whose body does not decode still ends the session, so the Termination after it is refused
// The Termination write fails with io.ErrClosedPipe because the receiver closed its end first.
func TestRFC7854MalformedKnownTypeStillEndsSession(t *testing.T) {
	short := []byte{Version, 0, 0, 0, 11, MsgPeerUpNotify, 1, 2, 3, 4, 5}
	err := receiverSurvives(t, short)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Termination write after a malformed Peer Up returned %v, want %v (session ended)", err, io.ErrClosedPipe)
	}
}

// statsReportBytes encodes one Stats Report for the test peer.
func statsReportBytes(stats []StatEntry) []byte {
	buf := make([]byte, 256)
	n := writeStatisticsReport(buf, 0, &statisticsReport{Peer: testPeerHeader(), Stats: stats})
	return buf[:n]
}

// RFC requirement: RFC7854-4.8-1 positive — a Stats Report carrying an unrecognized stat type (60000) and a known type with unexpected Stat Data (3 bytes for a 4-byte counter) is consumed and the session goes on
// The receiver runs on the test goroutine; the Termination write succeeding is the proof the session survived.
func TestRFC7854UnrecognizedStatTypeIgnored(t *testing.T) {
	sr := statsReportBytes([]StatEntry{
		{Type: 60000, Value: []byte{1, 2, 3, 4}},
		{Type: StatPrefixesRejected, Value: []byte{1, 2, 3}},
	})
	if err := receiverSurvives(t, sr); err != nil {
		t.Fatalf("receiver ended the session on an unrecognized stat: %v", err)
	}
}

// RFC requirement: RFC7854-4.8-1 negative — ignoring is per stat, not per message: a stat whose length runs past the end of the report is a malformed message and ends the session
// The Termination write fails with io.ErrClosedPipe because the receiver closed its end first.
func TestRFC7854TruncatedStatEndsSession(t *testing.T) {
	sr := statsReportBytes([]StatEntry{{Type: StatPrefixesRejected, Value: []byte{1, 2, 3, 4}}})
	// The stat's length field sits after the common header, the per-peer
	// header, the 4-byte count and the 2-byte type.
	lengthOff := CommonHeaderSize + PeerHeaderSize + 4 + 2
	binary.BigEndian.PutUint16(sr[lengthOff:], 100)
	err := receiverSurvives(t, sr)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Termination write after a truncated stat returned %v, want %v (session ended)", err, io.ErrClosedPipe)
	}
}

// announceBody is a minimal UPDATE body announcing 10.20.30.0/24 with no path
// attributes; the BMP sender wraps the body without validating it.
func announceBody(seed byte) []byte {
	return []byte{0, 0, 0, 0, 24, 10, 20, seed}
}

// withdrawBody is a minimal UPDATE body withdrawing 10.20.30.0/24.
func withdrawBody(seed byte) []byte {
	return []byte{0, 4, 24, 10, 20, seed, 0, 0}
}

// routeMonitoringFor streams one UPDATE event and returns the Route Monitoring
// the collector reads.
func routeMonitoringFor(t *testing.T, direction rpc.MessageDirection, body []byte) *RouteMonitoring {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() {
		closeLog(server, "server")
		closeLog(client, "client")
	})
	bp := newPipeSender(client, false)
	bp.handleSenderUpdate(updateEvent(direction, body), bp.senders, true, false)
	msg, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("read route monitoring: %v", err)
	}
	mon, ok := msg.(*RouteMonitoring)
	if !ok {
		t.Fatalf("message = %T, want *RouteMonitoring", msg)
	}
	return mon
}

// RFC requirement: RFC7854-4.2-1 positive — the per-peer header of every transmitted Route Monitoring, pre-policy and post-policy, IPv4 and IPv6, carries its reserved flag bits as 0
// The flags come off the wire through the collector end of a pipe.
func TestRFC7854ReservedFlagsTransmittedAsZero(t *testing.T) {
	cases := []struct {
		name      string
		direction rpc.MessageDirection
	}{
		{"received", rpc.DirectionReceived},
		{"sent", rpc.DirectionSent},
	}
	for i, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			mon := routeMonitoringFor(t, one.direction, announceBody(byte(i)))
			if mon.Peer.Flags&peerFlagsReserved != 0 {
				t.Errorf("transmitted flags %#x carry a reserved bit (mask %#x)", mon.Peer.Flags, peerFlagsReserved)
			}
		})
	}
}

// RFC requirement: RFC7854-4.2-1 negative — a received Peer Up whose reserved flag bits are all set is neither refused (the session consumes the Termination after it) nor read differently: the four defined flags decode as they were sent
// The session check goes through the receiver loop and the flag check through DecodeMsg.
func TestRFC7854ReservedFlagsIgnoredOnReceipt(t *testing.T) {
	peer := testPeerHeader()
	peer.Flags = PeerFlagV | PeerFlagL | peerFlagsReserved
	open := makeBGPOpen(65001, 0x01020304)
	buf := make([]byte, 256)
	n := writePeerUp(buf, 0, &PeerUp{Peer: peer, LocalPort: 179, RemotePort: 40000, SentOpenMsg: open, ReceivedOpenMsg: open})

	if err := receiverSurvives(t, buf[:n]); err != nil {
		t.Fatalf("receiver ended the session on reserved flag bits: %v", err)
	}

	decoded, err := DecodeMsg(buf[:n])
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	up, ok := decoded.(*PeerUp)
	if !ok {
		t.Fatalf("decoded %T, want *PeerUp", decoded)
	}
	if !up.Peer.IsIPv6() || !up.Peer.isPostPolicy() || up.Peer.is2ByteAS() || up.Peer.isAdjRIBOut() {
		t.Errorf("defined flags read back V=%v L=%v A=%v O=%v, want V L set and A O clear",
			up.Peer.IsIPv6(), up.Peer.isPostPolicy(), up.Peer.is2ByteAS(), up.Peer.isAdjRIBOut())
	}
}

// RFC requirement: RFC7854-4.5-1 positive — after the Termination the collector reads end-of-stream: the router closed the TCP session and wrote nothing after it
// RFC requirement: RFC7854-4.5-1 negative — a Peer Up produced after the Termination is refused with errNotConnected, so no message can follow it
// stop() runs on the test goroutine; the collector end of a pipe reads the Termination, then EOF.
func TestRFC7854TerminationThenCloseAndSilence(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	result := asyncRead(server)
	ss.stop()

	res := <-result
	if res.err != nil {
		t.Fatalf("read termination: %v", res.err)
	}
	if _, ok := res.msg.(*Termination); !ok {
		t.Fatalf("message after stop = %T, want *Termination", res.msg)
	}

	// A pipe whose far end is closed answers EOF at once and refuses a
	// deadline, so the read carries none.
	var trailing [1]byte
	got, err := server.Read(trailing[:])
	if got != 0 {
		t.Fatalf("read %d byte(s) after the termination, want none", got)
	}
	if !errors.Is(err, io.EOF) {
		t.Fatalf("read after termination = %v, want %v (session closed, nothing further sent)", err, io.EOF)
	}

	var local [16]byte
	open := makeBGPOpen(65001, 0x01020304)
	if err := ss.writePeerUp(testPeerHeader(), local, 179, 40000, open, open); !errors.Is(err, errNotConnected) {
		t.Errorf("Peer Up after termination returned %v, want %v", err, errNotConnected)
	}
}

// RFC requirement: RFC7854-4.7-1 positive — the BGP Message TLV is the last TLV of a transmitted Route Mirroring, and its value is the mirrored PDU starting at the BGP marker
// The Route Mirroring comes off the collector end of a pipe.
func TestRFC7854RouteMirroringBGPMessageTLVLast(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := newPipeSender(client, true)
	bp.handleSenderMirror(updateEvent(rpc.DirectionReceived, announceBody(7)), bp.senders)

	msg, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("read route mirroring: %v", err)
	}
	mirror, ok := msg.(*routeMirroring)
	if !ok {
		t.Fatalf("message = %T, want *routeMirroring", msg)
	}
	if len(mirror.TLVs) == 0 {
		t.Fatal("route mirroring carries no TLV")
	}
	last := mirror.TLVs[len(mirror.TLVs)-1]
	if last.Type != MirrorTLVBGPMsg {
		t.Fatalf("last TLV type = %d, want %d (BGP Message)", last.Type, MirrorTLVBGPMsg)
	}
	if len(last.Value) < message.HeaderLen || !bytes.Equal(last.Value[:message.MarkerLen], message.Marker[:]) {
		t.Errorf("BGP Message TLV value %x does not start with the BGP marker", last.Value)
	}
}

// RFC requirement: RFC7854-4.8-2 positive — a transmitted Stats Report carries exactly one statistic, the duplicate-updates counter (stat type 13)
// sendStatisticsReports runs on the test goroutine over one established peer.
func TestRFC7854StatsReportCarriesAStatistic(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := newPipeSender(client, false)
	bp.peerUps = map[string]*peerUpState{"10.0.0.1": establishedPeer("10.0.0.1", 65001)}
	bp.sendStatisticsReports()

	msg, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("read stats report: %v", err)
	}
	sr, ok := msg.(*statisticsReport)
	if !ok {
		t.Fatalf("message = %T, want *statisticsReport", msg)
	}
	if len(sr.Stats) != 1 {
		t.Fatalf("stats report carries %d statistics, want 1", len(sr.Stats))
	}
	if sr.Stats[0].Type != statTypeDuplicateUpdates {
		t.Errorf("stat type = %d, want %d", sr.Stats[0].Type, statTypeDuplicateUpdates)
	}
}

// RFC requirement: RFC7854-5-1 positive — a received (pre-policy) announcement is transmitted with the L flag clear and a sent (post-policy) one with the L flag set
// RFC requirement: RFC7854-5-3 positive — a withdraw carries the L flag its direction's announcement carried: clear on the received side, set on the sent side
// Each case streams one UPDATE event and reads the Route Monitoring off the collector end of a pipe.
func TestRFC7854LFlagFollowsPolicy(t *testing.T) {
	cases := []struct {
		name       string
		direction  rpc.MessageDirection
		body       []byte
		postPolicy bool
	}{
		{"pre-policy announce", rpc.DirectionReceived, announceBody(1), false},
		{"post-policy announce", rpc.DirectionSent, announceBody(2), true},
		{"pre-policy withdraw", rpc.DirectionReceived, withdrawBody(3), false},
		{"post-policy withdraw", rpc.DirectionSent, withdrawBody(4), true},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			mon := routeMonitoringFor(t, one.direction, one.body)
			if got := mon.Peer.isPostPolicy(); got != one.postPolicy {
				t.Errorf("L flag = %v, want %v (flags %#x)", got, one.postPolicy, mon.Peer.Flags)
			}
		})
	}
}

// RFC requirement: RFC7854-5-1 negative — the L flag is decided by the direction and never by the message: one and the same announcement body streamed on both sides leaves with L clear on the received copy and L set on the sent copy
// RFC requirement: RFC7854-5-3 negative — one and the same withdraw body streamed on both sides leaves with L clear on the received copy and L set on the sent copy, so a withdraw can never carry the flag of the other side's announcement
// Each body is streamed once per direction and both Route Monitorings are read off the collector end of a pipe.
func TestRFC7854LFlagNotDecidedByBody(t *testing.T) {
	bodies := []struct {
		name string
		body []byte
	}{
		{"announce", announceBody(9)},
		{"withdraw", withdrawBody(9)},
	}
	for _, one := range bodies {
		t.Run(one.name, func(t *testing.T) {
			pre := routeMonitoringFor(t, rpc.DirectionReceived, one.body)
			post := routeMonitoringFor(t, rpc.DirectionSent, one.body)
			if pre.Peer.isPostPolicy() {
				t.Errorf("received copy carries L set (flags %#x)", pre.Peer.Flags)
			}
			if !post.Peer.isPostPolicy() {
				t.Errorf("sent copy carries L clear (flags %#x)", post.Peer.Flags)
			}
		})
	}
}

// RFC requirement: RFC7854-8.2-1 positive — the Peer Up of the Loc-RIB instance peer carries Local Port 0 and Remote Port 0
// One best-change batch produces the Loc-RIB Peer Up the collector end of a pipe reads.
func TestRFC7854LocRIBPeerUpPortsZero(t *testing.T) {
	bp, conn := locRIBTestPlugin(t)
	bp.handleBestChange(oneBestChange(0))
	up, _ := readLocRIBPeerUpThenRM(t, conn)
	if up.Peer.PeerType != PeerTypeLocRIB {
		t.Fatalf("peer type = %d, want %d (Loc-RIB)", up.Peer.PeerType, PeerTypeLocRIB)
	}
	if up.LocalPort != 0 || up.RemotePort != 0 {
		t.Errorf("Loc-RIB Peer Up ports = %d/%d, want 0/0", up.LocalPort, up.RemotePort)
	}
}

// RFC requirement: RFC7854-8.2-1 negative — the zero is the rule for a peer with no transport session, not a constant: a BGP peer's Peer Up carries that session's Local Port and Remote Port
// The Peer Up comes off the collector end of a pipe.
func TestRFC7854BGPPeerUpCarriesTransportPorts(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := newPipeSender(client, false)
	var local [16]byte
	open := makeBGPOpen(65001, 0x01020304)
	if err := bp.senders[0].writePeerUp(testPeerHeader(), local, 179, 40000, open, open); err != nil {
		t.Fatalf("writePeerUp: %v", err)
	}
	msg, err := readBMPFromPipe(server)
	if err != nil {
		t.Fatalf("read peer up: %v", err)
	}
	up, ok := msg.(*PeerUp)
	if !ok {
		t.Fatalf("message = %T, want *PeerUp", msg)
	}
	if up.LocalPort != 179 || up.RemotePort != 40000 {
		t.Errorf("BGP peer Peer Up ports = %d/%d, want 179/40000", up.LocalPort, up.RemotePort)
	}
}
