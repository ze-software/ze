//go:build linux

// VALIDATES: spec-ike-padded-path-probe, the half of it that needs the datagram to
// leave the host with the DF bit set. The exchange: one INFORMATIONAL request
// carrying one status Notify pads the datagram to the requested size, the DF-clear
// repeat is the same bytes, the answer is correlated by the probe's own id, and an
// unanswered probe fails the SA through the unchanged request window.
// PREVENTS: a request answered off the owner goroutine; a probe response credited to
// Dead Peer Detection; a window released without a response; a datagram sized wrong.
//
// Linux is the only platform that carries IP_MTU_DISCOVER and IP_RECVERR, so
// UDPTransport.SendDF answers probe.ErrDFUnsupported and writes nothing everywhere
// else (transport/udp_other.go). Every test below asks the engine to send a probe,
// so off Linux each one would assert a send the platform declares absent. The two
// cases that need no send stay in probe_test.go and run on every platform.
package engine

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/ikeprobe"
	"github.com/ze-software/ze/internal/core/probe"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestPaddedInformationalReachesTheOwnerLoop proves the whole path from the leaf to
// the owner loop and back: ikeprobe.Probe finds the session, the request rides
// ps.probeRequests, maintainSA takes it on its select, builds and sends the padded
// request, reads the peer's answer off its inbound channel, correlates it and answers
// fits on the reply channel, and the loop keeps running afterwards.
//
// MUTATION: removing the probeRequests case from maintainSA's select
// (established.go) makes Probe wait until the context ends.
func TestPaddedInformationalReachesTheOwnerLoop(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)

	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	ps.inbound = make(chan transport.Packet, 4)
	ps.probeRequests = make(chan probeRequest)
	// The loop is what owns the SA (maintainSA's caller stores it, established.go);
	// this test stands in for that caller.
	ps.ownedSA.Store(ini)
	t.Cleanup(func() { ps.ownedSA.Store(nil) })
	SetActivePeersForTest(map[string]*PeerSession{ps.peerName: ps})
	t.Cleanup(func() { SetActivePeersForTest(nil) })

	dpd := &dpdState{interval: time.Hour, timeout: time.Hour, lastSent: time.Now()}
	done := make(chan struct{})
	go func() {
		_ = ps.maintainSA(ini, dpd, nil, nil,
			testIKEGroup(), NewSATable(), &rkyDP{}, myTr, nil, log)
		close(done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type answer struct {
		result ikeprobe.Result
		err    error
	}
	answered := make(chan answer, 1)
	request := probeRequestFor(ps.peerName)
	request.WireOctets = probeGridSize(t, ini, false)
	go func() {
		result, err := ikeprobe.Probe(ctx, request)
		answered <- answer{result, err}
	}()

	// The peer reads the padded request and answers it; the answer reaches the loop
	// the way the dispatcher delivers it, on ps.inbound.
	raw := rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the owner loop sent no padded request")
	}
	if got := probeSentOctets(raw); got != int(request.WireOctets) {
		t.Fatalf("the datagram is %d octets, want %d", got, request.WireOctets)
	}
	probeID := parseMsg(t, raw).Header.MessageID
	ps.inbound <- transport.Packet{Data: winInformationalAnswer(t, peer, probeID)}

	var got answer
	select {
	case got = <-answered:
	case <-time.After(10 * time.Second):
		t.Fatal("the answered request never came back from the owner loop")
	}
	if got.err != nil {
		t.Fatalf("the request never came back from the owner loop: %v", got.err)
	}
	if got.result.Outcome != ikeprobe.OutcomeFits {
		t.Fatalf("the owner loop answered %+v, want fits", got.result)
	}

	select {
	case <-done:
		t.Fatal("the owner loop exited after answering a probe request")
	default:
	}
	close(ps.stopCh)
	<-done
}

// probeGridSize answers the largest wire size at or below probeGridWant that sa's suite can
// produce exactly on the given send path. Under CBC the sizes sit on a 16-octet grid
// (RFC 7296 Section 3.14); under AEAD every size is reachable. The producer under test
// picks the INPUT here; every assertion below reads the bytes the peer received.
//
// probeGridWant is the size every exchange test asks for: one a loopback path carries
// whole, so the answer comes on the DF copy.
const probeGridWant = 1400

func probeGridSize(t *testing.T, sa *SA, natT bool) uint16 {
	t.Helper()
	_, sent, ok := probeNotifyOctets(sa, natT, probeGridWant)
	if !ok {
		t.Fatalf("no wire size at or below %d is reachable", probeGridWant)
	}
	return uint16(sent)
}

// probeAsk hands one request to the owner loop's handler on this goroutine, as
// maintainSA's select arm does, and returns the request so the test reads the reply.
// The kernel's path cache is honored, the mode a plain show mtu run asks for.
func probeAsk(ps *PeerSession, sa *SA, tr *transport.UDPTransport, octets uint16) probeRequest {
	request := probeRequest{
		req:   ikeprobe.Request{Peer: ps.peerName, WireOctets: octets, DF: probe.DFHonorCache},
		reply: make(chan ikeprobe.Result, 1),
	}
	ps.answerProbeRequest(sa, tr, request, slogutil.DiscardLogger())
	return request
}

// probeReply reads the answer of one request, or fails when none is there. The reply
// channel is buffered 1, so an answered request reads at once.
func probeReply(t *testing.T, request probeRequest, what string) ikeprobe.Result {
	t.Helper()
	select {
	case result := <-request.reply:
		return result
	default:
		t.Fatalf("%s: the request has no answer", what)
		return ikeprobe.Result{}
	}
}

// probeUnanswered fails the test when the request already carries an answer.
func probeUnanswered(t *testing.T, request probeRequest, what string) {
	t.Helper()
	select {
	case result := <-request.reply:
		t.Fatalf("%s: the request was answered %+v", what, result)
	default:
	}
}

// probeDeliver feeds one datagram to the owner loop's inbound handling on this
// goroutine, in the order maintainSA runs it: authenticate and classify, then
// correlate the response id against the path probe.
func probeDeliver(ps *PeerSession, sa *SA, tr *transport.UDPTransport, raw []byte) ownedOutcome {
	log := slogutil.DiscardLogger()
	out := ps.handleOwnedInbound(sa, transport.Packet{Data: raw}, tr, nil, log)
	if out.dpdResp {
		ps.settleProbe(out.dpdRespMsgID, log)
	}
	return out
}

// probeSentOctets is the size on the wire of a datagram whose UDP payload the peer
// read: the IPv4 and UDP headers are the only octets the transport does not hand up.
func probeSentOctets(payload []byte) int {
	return len(payload) + ipv4HeaderOctets + udpHeaderOctets
}

// TestPaddedInformationalReachesRequestedSize proves the datagram the peer reads is
// exactly the size asked for, on the 500 socket and on the 4500 socket where the
// four-octet non-ESP marker counts (RFC 7296 Section 2.23), for a CBC suite and for an
// AEAD suite whose SK overhead differs (RFC 5282).
//
// MUTATION: dropping the marker from the outer overhead in probeNotifyOctets makes the
// 4500 case read four octets long.
func TestPaddedInformationalReachesRequestedSize(t *testing.T) {
	ini, _, ps, peerTr, myTr := dpdProbeLink(t)

	// CBC on the 500 socket.
	size := probeGridSize(t, ini, false)
	request := probeAsk(ps, ini, myTr, size)
	probeUnanswered(t, request, "a probe that just left")
	raw := rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the probe never reached the peer")
	}
	if got := probeSentOctets(raw); got != int(size) {
		t.Fatalf("CBC on 500: the datagram is %d octets, want %d", got, size)
	}
	if raw[0] == 0 && raw[1] == 0 && raw[2] == 0 && raw[3] == 0 {
		t.Fatal("CBC on 500: the datagram begins with a non-ESP marker")
	}
	ps.pendingProbe = nil
	ini.releaseRequestWindow()

	// CBC on the 4500 socket: the SA floated, the marker is inside the size. A
	// floated SA aims at the peer's port 4500 unless an authenticated endpoint is
	// known (remoteUDPAddr, sa.go), so the peer's loopback socket stands in for one.
	ini.localPort = transport.NATTPort
	ini.nattSocket = myTr
	peerAddr, ok := peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}
	ini.peerEndpoint = peerAddr
	size = probeGridSize(t, ini, true)
	request = probeAsk(ps, ini, myTr, size)
	probeUnanswered(t, request, "a probe that just left on 4500")
	raw = rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the 4500 probe never reached the peer")
	}
	if got := probeSentOctets(raw); got != int(size) {
		t.Fatalf("CBC on 4500: the datagram is %d octets, want %d", got, size)
	}
	if !bytes.Equal(raw[:transport.NonESPMarkerLen], make([]byte, transport.NonESPMarkerLen)) {
		t.Fatalf("CBC on 4500: the datagram does not begin with the non-ESP marker: % x", raw[:4])
	}
	ps.pendingProbe = nil
	ini.releaseRequestWindow()
	ini.localPort = transport.IKEPort
	ini.nattSocket = nil
	ini.peerEndpoint = nil

	// AEAD: an 8-octet IV, no block padding, the tag inside the ciphertext.
	aead, _ := establishAEAD(t)
	aead.PeerCfg.RemoteAddress = "127.0.0.1"
	aps := &PeerSession{peerName: "ze"}
	request = probeAsk(aps, aead, myTr, 1400)
	probeUnanswered(t, request, "an AEAD probe that just left")
	raw = rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the AEAD probe never reached the peer")
	}
	if got := probeSentOctets(raw); got != 1400 {
		t.Fatalf("AEAD on 500: the datagram is %d octets, want 1400", got)
	}
}

// TestProbeCarriesOneStatusNotifyOnly proves the peer decrypts the request to exactly
// one payload: a Notify of the private-use status type with an empty SPI, whose data
// is the padding, and nothing else (RFC 7296 Section 3.10.1).
func TestProbeCarriesOneStatusNotifyOnly(t *testing.T) {
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
	size := probeGridSize(t, ini, false)
	want, _, _ := probeNotifyOctets(ini, false, int(size))

	probeAsk(ps, ini, myTr, size)
	raw := rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the probe never reached the peer")
	}
	msg := parseMsg(t, raw)
	if msg.Header.ExchangeType != wire.ExchangeInformational {
		t.Fatalf("exchange type %d, want INFORMATIONAL", msg.Header.ExchangeType)
	}
	if msg.Header.Flags&wire.FlagResponse != 0 {
		t.Fatal("the probe carries the Response flag")
	}
	inner := lcyDecrypt(t, peer, raw)
	if len(inner) != 1 {
		t.Fatalf("the probe decrypts to %d payloads, want 1", len(inner))
	}
	notify, ok := inner[0].Payload.(*wire.PayloadNotify)
	if !ok {
		t.Fatalf("the one payload is %T, want a Notify", inner[0].Payload)
	}
	if notify.NotifyMsgType != wire.NotifyZePathProbePadding {
		t.Fatalf("notify type %d, want %d", notify.NotifyMsgType, wire.NotifyZePathProbePadding)
	}
	if notify.NotifyMsgType < 40960 {
		t.Fatalf("notify type %d is outside the private-use range", notify.NotifyMsgType)
	}
	if notify.SPISize != 0 {
		t.Fatalf("SPI size %d, want 0", notify.SPISize)
	}
	if len(notify.NotificationData) != want {
		t.Fatalf("notification data is %d octets, want %d", len(notify.NotificationData), want)
	}
}

// TestProbeNeverTouchesDPDState proves a full probe exchange changes no dpdState
// field, and that a response at another id (the earlier DPD's) answers no probe.
//
// MUTATION: settling the probe on any dpdResp regardless of id (settleProbe) makes the
// replayed DPD answer end the second probe.
func TestProbeNeverTouchesDPDState(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
	size := probeGridSize(t, ini, false)

	// A DPD round trip first, so a stale DPD answer exists to replay.
	dpd := winDueDPD()
	_, dpdID := dpdSendProbe(t, ini, myTr, peerTr, dpd)
	dpdAnswer := winInformationalAnswer(t, peer, dpdID)
	out := probeDeliver(ps, ini, myTr, dpdAnswer)
	if !dpd.matchesProbe(out.dpdRespMsgID) {
		t.Fatal("the DPD answer did not correlate")
	}
	handleDPDResponse(dpd, log, "ze")
	before := *dpd

	request := probeAsk(ps, ini, myTr, size)
	raw := rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the probe never reached the peer")
	}
	probeID := parseMsg(t, raw).Header.MessageID
	if probeID == dpdID {
		t.Fatalf("the probe reused the DPD's message id %d", dpdID)
	}

	// The stale DPD answer is not the probe's answer.
	probeDeliver(ps, ini, myTr, dpdAnswer)
	probeUnanswered(t, request, "after a replayed DPD answer")
	if !ini.requestOutstanding {
		t.Fatal("a replayed DPD answer freed the probe's window")
	}

	answer := winInformationalAnswer(t, peer, probeID)
	out = probeDeliver(ps, ini, myTr, answer)
	if dpd.matchesProbe(out.dpdRespMsgID) {
		t.Fatal("the probe's answer correlates as a DPD answer")
	}
	if out.peerAlive {
		t.Fatal("the probe's answer was read as an in-window liveness proof")
	}
	result := probeReply(t, request, "after the peer answered")
	if result.Outcome != ikeprobe.OutcomeFits {
		t.Fatalf("outcome %v, want fits", result.Outcome)
	}
	if ini.requestOutstanding {
		t.Fatal("the answered probe still holds the window")
	}
	dpdAssertUnchanged(t, before, *dpd)
}

// dpdAssertUnchanged fails when any dpdState field differs between the two snapshots.
// The struct holds a slice, so it is compared field by field.
func dpdAssertUnchanged(t *testing.T, before, after dpdState) {
	t.Helper()
	same := before.interval == after.interval && before.timeout == after.timeout &&
		before.action == after.action && before.awaitReply == after.awaitReply &&
		before.probeMsgID == after.probeMsgID && before.retries == after.retries &&
		before.lastSent.Equal(after.lastSent) && before.sentAt.Equal(after.sentAt) &&
		before.lastAttempt.Equal(after.lastAttempt) && bytes.Equal(before.probeMsg, after.probeMsg)
	if !same {
		t.Fatalf("dpdState changed across a probe exchange:\nbefore %+v\nafter  %+v", before, after)
	}
}

// TestProbeUnansweredFailsTheSALikeDPD proves a probe nobody answers takes the exit
// every unanswered request takes (RFC 7296 Section 2.1): the ordinary schedule repeats
// it with DF clear, serviceRequestWindow deems the SA failed once the bound passes,
// no window is released without a response before that, the message id is not
// rewound, and the loop's StateDead arm tears the SA down and answers sa-failed.
//
// MUTATION: a probe-aware branch in serviceRequestWindow that releases the window and
// leaves the state alone makes the StateDead assertion fail.
func TestProbeUnansweredFailsTheSALikeDPD(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, _, ps, peerTr, myTr := dpdProbeLink(t)
	size := probeGridSize(t, ini, false)
	dpd := &dpdState{interval: time.Hour, timeout: time.Hour, lastSent: time.Now()}

	idBefore := ini.NextMsgID
	request := probeAsk(ps, ini, myTr, size)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the probe never reached the peer")
	}
	if ini.NextMsgID != idBefore+1 {
		t.Fatalf("NextMsgID %d after the probe, want %d", ini.NextMsgID, idBefore+1)
	}

	now := time.Now()
	copies := 0
	for _, wait := range []time.Duration{time.Second, 3 * time.Second, 10 * time.Second, 20 * time.Second} {
		ps.serviceRequestRetransmit(ini, dpd, myTr, now.Add(wait), log)
		ps.serviceRequestWindow(ini, dpd, now.Add(wait), log)
		if raw := rtxRecv(t, peerTr); raw != nil {
			copies++
			if !bytes.Equal(raw, first) {
				t.Fatal("a repeat differs from the first copy")
			}
		}
	}
	if copies != maxRequestRetransmits {
		t.Fatalf("%d repeats went out, want %d", copies, maxRequestRetransmits)
	}
	if ini.State == StateDead {
		t.Fatal("the SA was failed before the request window's bound passed")
	}
	if !ini.requestOutstanding {
		t.Fatal("the window was released without a response")
	}
	probeUnanswered(t, request, "inside the bound")

	ps.serviceRequestWindow(ini, dpd, now.Add(requestWindowTimeout+time.Second), log)
	if ini.State != StateDead {
		t.Fatalf("past the bound the SA is %v, want StateDead", ini.State)
	}
	if ini.NextMsgID != idBefore+1 {
		t.Fatalf("NextMsgID %d after the failure, want %d (never rewound)", ini.NextMsgID, idBefore+1)
	}
	probeUnanswered(t, request, "before the loop tore the SA down")

	// The loop's StateDead arm tears the SA down on its next tick and answers on the
	// way out, as it does after a DPD verdict.
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	ps.inbound = make(chan transport.Packet, 4)
	ps.probeRequests = make(chan probeRequest)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ps.maintainSA(ini, dpd, nil, nil, testIKEGroup(), NewSATable(), &rkyDP{}, myTr, nil, log)
	}()
	select {
	case err := <-errCh:
		if !errors.Is(err, errSADeletedByPeer) {
			t.Fatalf("the loop returned %v, want errSADeletedByPeer", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the loop did not tear the dead SA down")
	}
	result := probeReply(t, request, "after the teardown")
	if result.Outcome != ikeprobe.OutcomeSAFailed {
		t.Fatalf("outcome %v, want sa-failed", result.Outcome)
	}
	if ps.pendingProbe != nil {
		t.Fatal("the failed probe is still pending")
	}
}

// TestProbeAnsweredOnDFClearCopyDoesNotFailTheSA proves a probe whose DF copy draws
// nothing and whose DF-clear repeat is answered frees the window, reads too-big,
// leaves the SA established and dpdState untouched, and lets the next DPD go out and
// be answered.
func TestProbeAnsweredOnDFClearCopyDoesNotFailTheSA(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
	size := probeGridSize(t, ini, false)
	dpd := &dpdState{interval: time.Hour, timeout: time.Hour, lastSent: time.Now()}
	before := *dpd

	request := probeAsk(ps, ini, myTr, size)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the probe never reached the peer")
	}
	if ps.pendingProbe.dfCleared {
		t.Fatal("the first copy is recorded as DF clear")
	}
	// The DF copy is lost (the peer read it and did nothing). The schedule repeats it.
	ps.serviceRequestRetransmit(ini, dpd, myTr, time.Now().Add(time.Second), log)
	second := rtxRecv(t, peerTr)
	if second == nil {
		t.Fatal("the DF-clear repeat never reached the peer")
	}
	if !ps.pendingProbe.dfCleared {
		t.Fatal("the repeat is not recorded as DF clear")
	}
	probeID := parseMsg(t, second).Header.MessageID
	answer := winInformationalAnswer(t, peer, probeID)
	probeDeliver(ps, ini, myTr, answer)
	result := probeReply(t, request, "after the DF-clear copy was answered")
	if result.Outcome != ikeprobe.OutcomeTooBig {
		t.Fatalf("outcome %v, want too-big", result.Outcome)
	}
	if ini.State != StateEstablished {
		t.Fatalf("the SA is %v, want established", ini.State)
	}
	if ini.requestOutstanding {
		t.Fatal("the answered probe still holds the window")
	}
	dpdAssertUnchanged(t, before, *dpd)

	// The next DPD leaves at the next id and is answered.
	dpd.lastSent = time.Now().Add(-2 * time.Hour)
	_, dpdID := dpdSendProbe(t, ini, myTr, peerTr, dpd)
	if dpdID != probeID+1 {
		t.Fatalf("the DPD after the probe carries id %d, want %d", dpdID, probeID+1)
	}
	out := probeDeliver(ps, ini, myTr, winInformationalAnswer(t, peer, dpdID))
	if !dpd.matchesProbe(out.dpdRespMsgID) {
		t.Fatal("the DPD after the probe was not answered")
	}
}

// TestProbeRetransmitIsBitwiseIdenticalWithDFClear proves the repeat is sa.requestMsg
// byte for byte, so nothing rebuilt, re-encrypted or re-padded it, and that it is the
// copy the probe records as sent with DF clear. The DF bit itself is read off the wire
// by the transport's integration test (udp_df_integration_linux_test.go) and by the
// interop scenario; this harness sees the UDP payload only.
func TestProbeRetransmitIsBitwiseIdenticalWithDFClear(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, _, ps, peerTr, myTr := dpdProbeLink(t)
	size := probeGridSize(t, ini, false)

	probeAsk(ps, ini, myTr, size)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the probe never reached the peer")
	}
	if !bytes.Equal(first, ini.requestMsg) {
		t.Fatal("the first copy is not the datagram armed for retransmission")
	}
	if ps.pendingProbe.dfCleared {
		t.Fatal("the first copy is recorded as DF clear")
	}
	ps.serviceRequestRetransmit(ini, nil, myTr, time.Now().Add(time.Second), log)
	second := rtxRecv(t, peerTr)
	if second == nil {
		t.Fatal("the repeat never reached the peer")
	}
	if !bytes.Equal(first, second) {
		t.Fatal("the repeat differs from the first copy from the IKE header on")
	}
	if !ps.pendingProbe.dfCleared {
		t.Fatal("the repeat is not recorded as the DF-clear copy")
	}
	if ini.requestAttempts != 1 {
		t.Fatalf("requestAttempts %d after one repeat, want 1", ini.requestAttempts)
	}
}

// TestProbeSizeCeiling proves the boundary rows: a size above the 3000-octet ceiling
// (RFC 7296 Section 2) is refused by name before anything is built, the ceiling itself
// is sent, the smallest datagram the suite produces is sent, and one octet below it is
// refused. The refusal spends no message id and takes no window.
func TestProbeSizeCeiling(t *testing.T) {
	_, _, _, peerTr, myTr := dpdProbeLink(t)
	// AEAD reaches every size, so the ceiling is exact rather than on a grid.
	aead, _ := establishAEAD(t)
	aead.PeerCfg.RemoteAddress = "127.0.0.1"
	ps := &PeerSession{peerName: "ze"}

	refuse := func(octets uint16, what string) {
		t.Helper()
		idBefore := aead.NextMsgID
		result := probeReply(t, probeAsk(ps, aead, myTr, octets), what)
		if result.Outcome != ikeprobe.OutcomeRefused || result.Refusal != ikeprobe.RefusalSize {
			t.Fatalf("%s: answered %+v, want refused size", what, result)
		}
		if aead.NextMsgID != idBefore {
			t.Fatalf("%s: the refusal spent a message id", what)
		}
		if aead.requestOutstanding {
			t.Fatalf("%s: the refusal holds the window", what)
		}
		if raw := rtxRecv(t, peerTr); raw != nil {
			t.Fatalf("%s: a refused probe reached the peer (%d octets)", what, probeSentOctets(raw))
		}
	}
	send := func(octets uint16, what string) {
		t.Helper()
		request := probeAsk(ps, aead, myTr, octets)
		probeUnanswered(t, request, what)
		raw := rtxRecv(t, peerTr)
		if raw == nil {
			t.Fatalf("%s: the probe never reached the peer", what)
		}
		if got := probeSentOctets(raw); got != int(octets) {
			t.Fatalf("%s: %d octets on the wire, want %d", what, got, octets)
		}
		ps.pendingProbe = nil
		aead.releaseRequestWindow()
	}

	refuse(probeWireCeiling+1, "one above the ceiling")
	send(probeWireCeiling, "the ceiling")
	smallest := 0
	for size := 1; size <= probeWireCeiling; size++ {
		if _, _, ok := probeNotifyOctets(aead, false, size); ok {
			smallest = size
			break
		}
	}
	if smallest == 0 {
		t.Fatal("no size is reachable")
	}
	refuse(uint16(smallest-1), "one below the smallest datagram")
	send(uint16(smallest), "the smallest datagram")
}

// TestProbeRoundsDownToTheCipherGrid proves a CBC SA asked for a size off its
// 16-octet grid (RFC 7296 Section 3.14) sends the largest datagram on the grid at or
// below it, never above, and that the answer names the size sent: asked 1400 on a
// suite whose grid reaches 1392, the peer reads 1392 octets and the result carries
// WireOctets 1392 with the fit. A size on the grid is sent as asked. An AEAD SA
// reaches every size, so its answer names the size asked.
//
// MUTATION: rounding encrypted UP to the next block in probeNotifyOctets sends 1408
// and the wire assertion fails; dropping WireOctets from settleProbe's result fails
// the result assertion.
func TestProbeRoundsDownToTheCipherGrid(t *testing.T) {
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
	onGrid := probeGridSize(t, ini, false)
	if onGrid == probeGridWant {
		t.Fatalf("the CBC suite reaches %d exactly; the grid case needs an off-grid ask", probeGridWant)
	}
	if probeGridWant-onGrid >= cbcBlockOctets {
		t.Fatalf("the grid size %d is more than one block below %d", onGrid, probeGridWant)
	}

	request := probeAsk(ps, ini, myTr, probeGridWant)
	probeUnanswered(t, request, "asked off the grid")
	raw := rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the probe never reached the peer")
	}
	if got := probeSentOctets(raw); got != int(onGrid) {
		t.Fatalf("%d octets on the wire for an ask of %d, want the grid size %d", got, probeGridWant, onGrid)
	}
	if got := ps.pendingProbe.octets; got != onGrid {
		t.Fatalf("the pending probe records %d octets, want the size sent %d", got, onGrid)
	}
	probeDeliver(ps, ini, myTr, winInformationalAnswer(t, peer, ps.pendingProbe.msgID))
	result := probeReply(t, request, "answered on the DF copy")
	if result.Outcome != ikeprobe.OutcomeFits {
		t.Fatalf("answered %+v, want fits", result)
	}
	if result.WireOctets != onGrid {
		t.Fatalf("the result names %d octets, want the size sent %d", result.WireOctets, onGrid)
	}

	// On the grid the size is sent as asked.
	request = probeAsk(ps, ini, myTr, onGrid)
	raw = rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the on-grid probe never reached the peer")
	}
	if got := probeSentOctets(raw); got != int(onGrid) {
		t.Fatalf("%d octets on the wire for an on-grid ask of %d", got, onGrid)
	}
	probeDeliver(ps, ini, myTr, winInformationalAnswer(t, peer, ps.pendingProbe.msgID))
	if result = probeReply(t, request, "on the grid"); result.WireOctets != onGrid {
		t.Fatalf("the on-grid result names %d octets, want %d", result.WireOctets, onGrid)
	}
}

// TestProbeAdvancesMsgIDLikeDPD proves a probe spends exactly one message id, in the
// order sendDPD uses (reserve, build, send, arm, advance), so the next DPD carries the
// probe's id plus one and the peer answers it (RFC 7296 Section 2.2).
func TestProbeAdvancesMsgIDLikeDPD(t *testing.T) {
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
	size := probeGridSize(t, ini, false)
	idBefore := ini.NextMsgID

	request := probeAsk(ps, ini, myTr, size)
	raw := rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the probe never reached the peer")
	}
	probeID := parseMsg(t, raw).Header.MessageID
	if probeID != idBefore {
		t.Fatalf("the probe carries id %d, want %d", probeID, idBefore)
	}
	if ini.NextMsgID != idBefore+1 {
		t.Fatalf("NextMsgID %d, want %d", ini.NextMsgID, idBefore+1)
	}
	if !ini.requestOutstanding || ini.requestMsgID != probeID {
		t.Fatal("the probe does not hold the window at its own id")
	}
	if !bytes.Equal(ini.requestMsg, raw) {
		t.Fatal("the retransmit slot does not hold the probe")
	}
	probeDeliver(ps, ini, myTr, winInformationalAnswer(t, peer, probeID))
	if probeReply(t, request, "answered").Outcome != ikeprobe.OutcomeFits {
		t.Fatal("the answered probe is not fits")
	}

	dpd := winDueDPD()
	_, dpdID := dpdSendProbe(t, ini, myTr, peerTr, dpd)
	if dpdID != probeID+1 {
		t.Fatalf("the DPD after the probe carries id %d, want %d", dpdID, probeID+1)
	}
	out := probeDeliver(ps, ini, myTr, winInformationalAnswer(t, peer, dpdID))
	if !dpd.matchesProbe(out.dpdRespMsgID) {
		t.Fatal("the peer's answer to the DPD after the probe did not correlate")
	}
}

// TestProbeRefusedWhilstRekeyPending proves the four conditions AC-8 names each
// answer a distinct refusal at once, with nothing built, no id spent and no window
// taken: a pending rekey, a rekey hold, a held window, and an SA below established.
func TestProbeRefusedWhilstRekeyPending(t *testing.T) {
	ini, _, ps, peerTr, myTr := dpdProbeLink(t)
	size := probeGridSize(t, ini, false)

	expect := func(want ikeprobe.Refusal, what string) {
		t.Helper()
		idBefore := ini.NextMsgID
		result := probeReply(t, probeAsk(ps, ini, myTr, size), what)
		if result.Outcome != ikeprobe.OutcomeRefused {
			t.Fatalf("%s: outcome %v, want refused", what, result.Outcome)
		}
		if result.Refusal != want {
			t.Fatalf("%s: refused %v, want %v", what, result.Refusal, want)
		}
		if ini.NextMsgID != idBefore {
			t.Fatalf("%s: the refusal spent a message id", what)
		}
		if ps.pendingProbe != nil {
			t.Fatalf("%s: the refusal left a probe pending", what)
		}
		if raw := rtxRecv(t, peerTr); raw != nil {
			t.Fatalf("%s: a refused probe reached the peer", what)
		}
	}

	ps.pendingRekey = &pendingRekey{kind: rekeyChild, messageID: ini.NextMsgID}
	expect(ikeprobe.RefusalRekeyPending, "a pending rekey")
	ps.pendingRekey = nil

	ps.ikeRekeyHoldUntil = time.Now().Add(time.Hour)
	expect(ikeprobe.RefusalRekeyHeld, "an IKE rekey hold")
	ps.ikeRekeyHoldUntil = time.Time{}
	ps.childRekeyHoldUntil = time.Now().Add(time.Hour)
	expect(ikeprobe.RefusalRekeyHeld, "a Child SA rekey hold")
	ps.childRekeyHoldUntil = time.Time{}

	dpd := winDueDPD()
	dpdSendProbe(t, ini, myTr, peerTr, dpd)
	expect(ikeprobe.RefusalWindowHeld, "a DPD probe holding the window")
	ini.releaseRequestWindow()
	dpd.awaitReply = false

	ini.State = StateAuthSent
	expect(ikeprobe.RefusalSADown, "an SA below established")
	ini.State = StateEstablished

	// The refusals were the whole story: the same request now goes out.
	request := probeAsk(ps, ini, myTr, size)
	probeUnanswered(t, request, "with every condition cleared")
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("with every condition cleared the probe never reached the peer")
	}
	idBefore := ini.NextMsgID
	result := probeReply(t, probeAsk(ps, ini, myTr, size), "a second probe")
	if result.Refusal != ikeprobe.RefusalWindowHeld {
		t.Fatalf("a second probe while the first holds the window was refused %v, want window-held", result.Refusal)
	}
	if ini.NextMsgID != idBefore {
		t.Fatal("the second probe's refusal spent a message id")
	}
	probeUnanswered(t, request, "the first probe, after the second was refused")
}

// TestErrQueueTooBigTriggersDFClearAtOnce proves a router's size refusal for the
// probe's peer sends the DF-clear copy before the retransmit timer (AC-11), that one
// for another peer sends nothing (R-4), and that the answer then reads too-big with the
// router's figure.
func TestErrQueueTooBigTriggersDFClearAtOnce(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
	size := probeGridSize(t, ini, false)
	remote := ini.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}
	peerAddr := netip.AddrPortFrom(remote.AddrPort().Addr().Unmap(), uint16(remote.Port))

	request := probeAsk(ps, ini, myTr, size)
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the probe never reached the peer")
	}

	// A refusal for another peer is dropped.
	other := transport.SizeRefusal{
		Peer:     netip.MustParseAddrPort("192.0.2.9:500"),
		Outcome:  probe.ErrQueueMTUReported,
		MTU:      1200,
		Offender: netip.MustParseAddr("192.0.2.1"),
	}
	ps.handleSizeRefusal(ini, myTr, other, log)
	if raw := rtxRecv(t, peerTr); raw != nil {
		t.Fatal("a refusal for another peer sent a DF-clear copy")
	}
	if ps.pendingProbe.dfCleared {
		t.Fatal("a refusal for another peer marked the probe DF clear")
	}

	// A refusal for this peer sends the DF-clear copy at once.
	mine := transport.SizeRefusal{
		Peer:     peerAddr,
		Outcome:  probe.ErrQueueMTUReported,
		MTU:      1300,
		Offender: netip.MustParseAddr("192.0.2.1"),
	}
	ps.handleSizeRefusal(ini, myTr, mine, log)
	second := rtxRecv(t, peerTr)
	if second == nil {
		t.Fatal("the router's refusal did not send the DF-clear copy at once")
	}
	if !bytes.Equal(first, second) {
		t.Fatal("the DF-clear copy differs from the first copy")
	}
	if !ps.pendingProbe.dfCleared {
		t.Fatal("the copy is not recorded as DF clear")
	}
	// A second refusal for the same probe sends nothing more.
	ps.handleSizeRefusal(ini, myTr, mine, log)
	if raw := rtxRecv(t, peerTr); raw != nil {
		t.Fatal("a repeated refusal sent another copy")
	}

	probeID := parseMsg(t, second).Header.MessageID
	probeDeliver(ps, ini, myTr, winInformationalAnswer(t, peer, probeID))
	result := probeReply(t, request, "after the DF-clear copy was answered")
	if result.Outcome != ikeprobe.OutcomeTooBig {
		t.Fatalf("outcome %v, want too-big", result.Outcome)
	}
	if result.MTU != 1300 {
		t.Fatalf("reported MTU %d, want 1300", result.MTU)
	}
}

// TestZeResponderIgnoresThePrivateStatusNotify proves Ze's own responder answers a
// request carrying only the private-use status Notify with an empty INFORMATIONAL
// response and keeps the SA (RFC 7296 Section 3.10.1: an unrecognized status type is
// ignored). The request is built by the initiator's probe path and handled by the
// responder SA as a new in-window request.
func TestZeResponderIgnoresThePrivateStatusNotify(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
	size := probeGridSize(t, ini, false)

	probeAsk(ps, ini, myTr, size)
	raw := rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the probe never reached the peer")
	}
	probeID := parseMsg(t, raw).Header.MessageID
	if peer.ExpectedMsgID != probeID {
		t.Fatalf("the responder expects id %d and the probe carries %d", peer.ExpectedMsgID, probeID)
	}

	// The responder's session answers on its own send path, which the test port
	// seam points at peerTr as well, so the response is the next datagram there.
	responder := &PeerSession{peerName: "ze"}
	out := responder.handleOwnedInbound(peer, transport.Packet{Data: raw}, peerTr, nil, log)
	if out.reestablish || out.newSA != nil || out.newChild != nil {
		t.Fatalf("the responder acted on the padding notify: %+v", out)
	}
	if peer.State != StateEstablished {
		t.Fatalf("the responder's SA is %v after the probe, want established", peer.State)
	}
	response := rtxRecv(t, peerTr)
	if response == nil {
		t.Fatal("the responder sent no answer")
	}
	msg := parseMsg(t, response)
	if msg.Header.Flags&wire.FlagResponse == 0 {
		t.Fatal("the answer is not a response")
	}
	if msg.Header.MessageID != probeID {
		t.Fatalf("the answer carries id %d, want %d", msg.Header.MessageID, probeID)
	}
	if msg.Header.ExchangeType != wire.ExchangeInformational {
		t.Fatalf("the answer is exchange %d, want INFORMATIONAL", msg.Header.ExchangeType)
	}
	inner := lcyDecrypt(t, ini, response)
	if len(inner) != 0 {
		t.Fatalf("the answer carries %d payloads, want an empty response", len(inner))
	}
	if len(response) >= len(raw) {
		t.Fatalf("the answer (%d octets) is not smaller than the request (%d): amplification", len(response), len(raw))
	}
	if peer.ExpectedMsgID != probeID+1 {
		t.Fatalf("the responder expects id %d after the probe, want %d", peer.ExpectedMsgID, probeID+1)
	}
}

// TestProbeRefusesANonIPv4Peer proves a peer whose authenticated endpoint is an IPv6
// address is refused `family` by name before any size arithmetic: the transport is
// udp4 and the size budget counts a 20-octet IPv4 header, so a datagram built for
// that peer would be sized wrong (AC-8). The refusal spends no message id, holds no
// window and sends nothing. The same SA reached over IPv4 sends, which pins the
// refusal to the family rather than to the endpoint route.
//
// MUTATION: dropping the `remote.IP.To4() == nil` check in answerProbeRequest sends
// the probe and the refusal assertion fails.
func TestProbeRefusesANonIPv4Peer(t *testing.T) {
	_, _, _, peerTr, myTr := dpdProbeLink(t)
	aead, _ := establishAEAD(t)
	aead.PeerCfg.RemoteAddress = "127.0.0.1"
	ps := &PeerSession{peerName: "ze"}

	aead.peerEndpoint = &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: transport.IKEPort}
	idBefore := aead.NextMsgID
	result := probeReply(t, probeAsk(ps, aead, myTr, probeGridWant), "an IPv6 peer")
	if result.Outcome != ikeprobe.OutcomeRefused || result.Refusal != ikeprobe.RefusalFamily {
		t.Fatalf("an IPv6 peer: answered %+v, want refused family", result)
	}
	if aead.NextMsgID != idBefore {
		t.Fatal("an IPv6 peer: the refusal spent a message id")
	}
	if aead.requestOutstanding {
		t.Fatal("an IPv6 peer: the refusal holds the window")
	}
	if raw := rtxRecv(t, peerTr); raw != nil {
		t.Fatalf("an IPv6 peer: a refused probe reached the peer (%d octets)", probeSentOctets(raw))
	}

	// Back to the configured IPv4 remote, which the ze.test.ike.port seam aims at peerTr.
	aead.peerEndpoint = nil
	request := probeAsk(ps, aead, myTr, probeGridWant)
	probeUnanswered(t, request, "an IPv4 peer")
	if raw := rtxRecv(t, peerTr); raw == nil {
		t.Fatal("an IPv4 peer: the probe never reached the peer")
	}
}
