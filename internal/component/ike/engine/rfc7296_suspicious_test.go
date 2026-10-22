// RFC 7296 Section 2.21.4: a suspicious message from the address of an IKE SA draws an
// IKE Notify in an INFORMATIONAL exchange over that SA.
//
// Helpers here start with `sus`, so they cannot collide with the sibling RFC files in
// this package. This file reuses the `osr` session, the `imi` drain and notify readers,
// and the `rtx` loopback helpers.

package engine

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// susOutOfSA builds the header-only datagram a stale or forged peer sends: a request
// under an SPI pair no SA holds.
func susOutOfSA(t *testing.T, flags, exchange uint8) []byte {
	t.Helper()
	msg := wire.Message{Header: wire.Header{
		InitiatorSPI: [8]byte{0xde, 0xad, 0xbe, 0xef, 1, 2, 3, 4},
		ResponderSPI: [8]byte{0xca, 0xfe, 0xf0, 0x0d, 5, 6, 7, 8},
		MajorVersion: 2,
		ExchangeType: exchange,
		Flags:        flags,
		MessageID:    7,
	}}
	buf := make([]byte, wire.HeaderLen)
	n, err := msg.CheckedWriteTo(buf, 0)
	if err != nil {
		t.Fatalf("build the out-of-SA datagram: %v", err)
	}
	return buf[:n]
}

// susRegister makes ps the running session for the SA's peer address, as reconcile
// does, so noteSuspicious can find it, and removes it when the test ends.
func susRegister(t *testing.T, ps *PeerSession, ini *SA) {
	t.Helper()
	ps.peerName = "sus-peer"
	ps.peerCfg.RemoteAddress = "127.0.0.1"
	ps.suspicious = make(chan transport.Packet, suspiciousQueueDepth)
	ps.ownedSA.Store(ini)
	peersMu.Lock()
	if activePeersMap == nil {
		activePeersMap = make(map[string]*PeerSession)
	}
	activePeersMap[ps.peerName] = ps
	peersMu.Unlock()
	t.Cleanup(func() {
		peersMu.Lock()
		delete(activePeersMap, ps.peerName)
		peersMu.Unlock()
	})
}

// susDeliver runs the two halves of the path: the dispatch goroutine's noteSuspicious,
// then the owner loop's arm, which takes what was queued and answers it.
func susDeliver(t *testing.T, ps *PeerSession, ini *SA, myTr *transport.UDPTransport, pkt transport.Packet) {
	t.Helper()
	noteSuspicious(pkt)
	select {
	case queued := <-ps.suspicious:
		ps.informSuspicious(ini, queued, myTr, slogutil.DiscardLogger())
	default:
	}
}

// VALIDATES: a request naming an unknown IKE SA, sent from the address of an
// established IKE SA, draws one protected INFORMATIONAL request over that SA carrying
// INVALID_IKE_SPI, and the SA's own state is unchanged apart from the request it raised.
// PREVENTS: the silence Section 2.21.4 asks a node to break, where only the unprotected
// INVALID_IKE_SPI goes out and the peer cannot trust it.
//
// RFC requirement: RFC7296-2.21.4-8 positive -- an out-of-SA request from the address of an established IKE SA draws exactly one INFORMATIONAL request over that SA, at the SA's next Message ID with the Response flag clear, carrying one INVALID_IKE_SPI Notify that the peer authenticates under the SA's keys; the SA stays established.
func TestSusOutOfSARequestFromSAPeerDrawsProtectedNotify(t *testing.T) {
	ini, peer, ps, peerTr, myTr := osrSession(t)
	susRegister(t, ps, ini)
	remote := ini.remoteUDPAddr()
	wantID := ini.NextMsgID
	state := ini.State

	susDeliver(t, ps, ini, myTr, transport.Packet{
		Data:       susOutOfSA(t, wire.FlagInitiator, wire.ExchangeInformational),
		RemoteAddr: remote,
	})

	got := imiDrain(t, peerTr, myTr, remote, "an out-of-SA request from the SA's peer")
	if len(got) != 1 {
		t.Fatalf("%d datagrams went out over the SA, want exactly 1", len(got))
	}
	msg := parseMsg(t, got[0])
	if msg.Header.ExchangeType != wire.ExchangeInformational {
		t.Errorf("exchange %d, want INFORMATIONAL", msg.Header.ExchangeType)
	}
	if msg.Header.Flags&wire.FlagResponse != 0 {
		t.Error("the notification is marked as a response; Section 2.21.4 asks for an exchange of its own")
	}
	if msg.Header.MessageID != wantID {
		t.Errorf("message id %d, want the SA's next request id %d", msg.Header.MessageID, wantID)
	}
	if msg.Header.InitiatorSPI != ini.InitiatorSPI || msg.Header.ResponderSPI != ini.ResponderSPI {
		t.Error("the notification does not travel over the SA the peer's address holds")
	}
	inner, err := decryptAndParse(peer, msg, got[0])
	if err != nil {
		t.Fatalf("the peer could not authenticate the notification: %v", err)
	}
	if len(inner) != 1 {
		t.Fatalf("the notification carries %d payloads, want exactly 1 Notify", len(inner))
	}
	notify, ok := inner[0].Payload.(*wire.PayloadNotify)
	if !ok {
		t.Fatalf("the payload is %T, want a Notify", inner[0].Payload)
	}
	if notify.NotifyMsgType != wire.NotifyInvalidIKESPI {
		t.Errorf("Notify type %d, want INVALID_IKE_SPI (%d)", notify.NotifyMsgType, wire.NotifyInvalidIKESPI)
	}
	if len(notify.NotificationData) != 0 {
		t.Errorf("INVALID_IKE_SPI carries %d octets of data, want none", len(notify.NotificationData))
	}
	if ini.State != state {
		t.Errorf("SA state moved from %v to %v", state, ini.State)
	}
}

// VALIDATES: the same request draws nothing over the SA when it is marked as a
// response, when it starts an IKE SA, or when it comes from another address; and, once
// the SA runs NAT traversal, from the peer's address at another port.
// PREVENTS: an emitter keyed on anything but the endpoint the SA talks to, which lets
// any host spend the SA's request window, and an answer to a response.
//
// RFC requirement: RFC7296-2.21.4-8 negative -- no INFORMATIONAL goes out over the SA for an out-of-SA message from another address (including the configured address once the SA's endpoint has moved), for one from the peer's address at another port once the SA has floated to port 4500, for one marked as a response, or for an IKE_SA_INIT, while the control request from the SA's endpoint does draw one.
func TestSusOnlyTheSAEndpointDrawsTheNotify(t *testing.T) {
	ini, _, ps, peerTr, myTr := osrSession(t)
	susRegister(t, ps, ini)
	remote := ini.remoteUDPAddr()
	request := susOutOfSA(t, wire.FlagInitiator, wire.ExchangeInformational)

	other := &net.UDPAddr{IP: net.ParseIP("127.0.0.2"), Port: remote.Port}
	susDeliver(t, ps, ini, myTr, transport.Packet{Data: request, RemoteAddr: other})
	susDeliver(t, ps, ini, myTr, transport.Packet{
		Data: susOutOfSA(t, wire.FlagInitiator|wire.FlagResponse, wire.ExchangeInformational), RemoteAddr: remote,
	})
	susDeliver(t, ps, ini, myTr, transport.Packet{
		Data: susOutOfSA(t, wire.FlagInitiator, wire.ExchangeIKESAInit), RemoteAddr: remote,
	})
	if got := imiDrain(t, peerTr, myTr, remote, "three messages that are not the trigger"); len(got) != 0 {
		t.Fatalf("%d datagrams went out over the SA, want none", len(got))
	}

	// The configured address is only the dispatch goroutine's first filter. Once the
	// SA's endpoint has moved (MOBIKE), the owner loop compares against the endpoint
	// the SA talks to, so a message from the configured address draws nothing.
	nextID := ini.NextMsgID
	ini.peerEndpoint = &net.UDPAddr{IP: net.ParseIP("127.0.0.3"), Port: remote.Port}
	susDeliver(t, ps, ini, myTr, transport.Packet{Data: request, RemoteAddr: remote})
	ini.peerEndpoint = nil
	if ini.NextMsgID != nextID {
		t.Fatalf("a message from the configured address raised a request over an SA whose endpoint moved (next id %d, was %d)",
			ini.NextMsgID, nextID)
	}

	// NAT traversal: the port identifies the peer too. The stored endpoint is the
	// peer link itself, so the control below still reaches the test's peer socket.
	ini.peerEndpoint = copyUDPAddr(remote)
	ini.localPort = transport.NATTPort
	ini.nattSocket = myTr
	otherPort := &net.UDPAddr{IP: remote.IP, Port: remote.Port + 1}
	susDeliver(t, ps, ini, myTr, transport.Packet{Data: request, RemoteAddr: otherPort})
	if got := imiDrain(t, peerTr, myTr, remote, "the peer's address at another port"); len(got) != 0 {
		t.Fatalf("%d datagrams went out over a NAT-T SA for another port, want none", len(got))
	}

	// Control: the emitter is not mute, so the silence above is the endpoint test.
	susDeliver(t, ps, ini, myTr, transport.Packet{Data: request, RemoteAddr: remote})
	if got := imiDrain(t, peerTr, myTr, remote, "the SA's own endpoint"); len(got) != 1 {
		t.Fatalf("the control drew %d datagrams, want 1", len(got))
	}
}

// VALIDATES: a flood of out-of-SA requests from the peer's address draws no more
// notifications than the token bucket admits.
// PREVENTS: an amplifier, where every unauthenticated datagram costs one protected
// request.
//
// RFC requirement: RFC7296-2.21.4-8 negative -- twelve out-of-SA requests from the SA's endpoint, the request window freed between them, draw at least one and at most suspiciousNotifyBurst plus the tokens refilled during the run.
func TestSusRateLimitCapsTheNotify(t *testing.T) {
	ini, _, ps, peerTr, myTr := osrSession(t)
	susRegister(t, ps, ini)
	remote := ini.remoteUDPAddr()
	request := susOutOfSA(t, wire.FlagInitiator, wire.ExchangeInformational)

	const requests = 12
	start := time.Now()
	for range requests {
		susDeliver(t, ps, ini, myTr, transport.Packet{Data: request, RemoteAddr: remote})
		ini.releaseRequestWindow()
	}
	elapsed := time.Since(start)

	sent := len(imiDrain(t, peerTr, myTr, remote, "a burst of out-of-SA requests"))
	if sent == 0 {
		t.Fatal("no notification went out at all, so the cap proves nothing")
	}
	bound := suspiciousNotifyBurst + int(elapsed.Seconds()*suspiciousNotifyRate) + 1
	if sent > bound {
		t.Errorf("%d notifications went out, above the bucket bound of %d over %v", sent, bound, elapsed)
	}
}

// VALIDATES: once the SA's endpoint is no longer its configured remote (NAT rebinding or
// a MOBIKE address update), an out-of-SA request from the endpoint the SA now uses still
// draws the protected INVALID_IKE_SPI notification over that SA.
// PREVENTS: the dispatch goroutine finding the session by its configured remote address
// alone, which let every suspicious message from a moved peer go unanswered.
//
// RFC requirement: RFC7296-2.21.4-8 positive -- an out-of-SA request from the endpoint an established IKE SA moved to, which is not the session's configured remote, draws exactly one INFORMATIONAL request over that SA carrying INVALID_IKE_SPI.
func TestSusMovedSAEndpointDrawsTheNotify(t *testing.T) {
	ini, peer, ps, peerTr, myTr := osrSession(t)
	susRegister(t, ps, ini)
	remote := ini.remoteUDPAddr()
	// The peer now talks from remote, and the configuration still names another host.
	ps.peerCfg.RemoteAddress = "127.0.0.9"
	ini.peerEndpoint = copyUDPAddr(remote)
	ps.publishSAEndpoint(ini)

	susDeliver(t, ps, ini, myTr, transport.Packet{
		Data:       susOutOfSA(t, wire.FlagInitiator, wire.ExchangeInformational),
		RemoteAddr: remote,
	})

	got := imiDrain(t, peerTr, myTr, remote, "an out-of-SA request from the SA's moved endpoint")
	if len(got) != 1 {
		t.Fatalf("%d datagrams went out over the SA, want exactly 1", len(got))
	}
	msg := parseMsg(t, got[0])
	inner, err := decryptAndParse(peer, msg, got[0])
	if err != nil {
		t.Fatalf("the peer could not authenticate the notification: %v", err)
	}
	if len(inner) != 1 {
		t.Fatalf("the notification carries %d payloads, want exactly 1 Notify", len(inner))
	}
	notify, ok := inner[0].Payload.(*wire.PayloadNotify)
	if !ok {
		t.Fatalf("the payload is %T, want a Notify", inner[0].Payload)
	}
	if notify.NotifyMsgType != wire.NotifyInvalidIKESPI {
		t.Errorf("Notify type %d, want INVALID_IKE_SPI (%d)", notify.NotifyMsgType, wire.NotifyInvalidIKESPI)
	}
}

// susAwaitPublished waits, bounded by rtxArrive, for the owner loop to publish the
// endpoint its SA talks to, and returns it. It fails the test when nothing is published,
// because then no suspicious message from a moved endpoint can reach the loop at all.
func susAwaitPublished(t *testing.T, ps *PeerSession) *net.UDPAddr {
	t.Helper()
	deadline := time.Now().Add(rtxArrive)
	for time.Now().Before(deadline) {
		if endpoint := ps.saEndpoint.Load(); endpoint != nil {
			return endpoint
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("the owner loop never published the endpoint its SA talks to")
	return nil
}

// VALIDATES: with the real owner loop (maintainSA) running an SA whose endpoint has
// moved away from the configured remote, an out-of-SA request from that moved endpoint
// travels the whole path: noteSuspicious on the dispatch side finds the session by the
// endpoint the loop published, the loop's suspicious arm answers it, and one protected
// INVALID_IKE_SPI notification reaches the peer. Once the loop returns, the endpoint is
// withdrawn and the same message no longer finds the session.
// PREVENTS: the owner loop not publishing the endpoint, which the sibling test cannot
// see because it publishes by hand; and a stale endpoint outliving the loop that
// answered for it.
//
// RFC requirement: RFC7296-2.21.4-8 positive -- with maintainSA running an established IKE SA whose endpoint moved away from the configured remote, an out-of-SA request from that moved endpoint, handed to noteSuspicious, draws exactly one INFORMATIONAL request over that SA, which the peer authenticates under the SA's keys and which carries one INVALID_IKE_SPI Notify; after the loop returns, the same request is queued to no session.
func TestSusOwnerLoopPublishesMovedEndpoint(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := osrSession(t)
	susRegister(t, ps, ini)
	remote := ini.remoteUDPAddr()
	// The peer now talks from remote, and the configuration still names another host,
	// so only the endpoint the owner loop publishes can match the source.
	ps.peerCfg.RemoteAddress = "127.0.0.9"
	ini.peerEndpoint = copyUDPAddr(remote)
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)

	done := make(chan struct{})
	go func() {
		_ = ps.maintainSA(ini, nil, nil, nil,
			testIKEGroup(), NewSATable(), &rkyDP{}, myTr, nil, log)
		close(done)
	}()
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		close(ps.stopCh)
		<-done
	}
	t.Cleanup(stop)

	published := susAwaitPublished(t, ps)
	if !published.IP.Equal(remote.IP) || published.Port != remote.Port {
		t.Fatalf("the owner loop published %v, want the SA's moved endpoint %v", published, remote)
	}

	pkt := transport.Packet{
		Data:       susOutOfSA(t, wire.FlagInitiator, wire.ExchangeInformational),
		RemoteAddr: remote,
	}
	noteSuspicious(pkt)

	raw := rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("no notification reached the peer for an out-of-SA request from the SA's moved endpoint")
	}
	msg := parseMsg(t, raw)
	if msg.Header.ExchangeType != wire.ExchangeInformational {
		t.Errorf("exchange %d, want INFORMATIONAL", msg.Header.ExchangeType)
	}
	if msg.Header.Flags&wire.FlagResponse != 0 {
		t.Error("the notification is marked as a response; Section 2.21.4 asks for an exchange of its own")
	}
	if msg.Header.InitiatorSPI != ini.InitiatorSPI || msg.Header.ResponderSPI != ini.ResponderSPI {
		t.Error("the notification does not travel over the SA the moved endpoint holds")
	}
	inner, err := decryptAndParse(peer, msg, raw)
	if err != nil {
		t.Fatalf("the peer could not authenticate the notification: %v", err)
	}
	if len(inner) != 1 {
		t.Fatalf("the notification carries %d payloads, want exactly 1 Notify", len(inner))
	}
	notify, ok := inner[0].Payload.(*wire.PayloadNotify)
	if !ok {
		t.Fatalf("the payload is %T, want a Notify", inner[0].Payload)
	}
	if notify.NotifyMsgType != wire.NotifyInvalidIKESPI {
		t.Errorf("Notify type %d, want INVALID_IKE_SPI (%d)", notify.NotifyMsgType, wire.NotifyInvalidIKESPI)
	}

	// The loop has returned once stop does, so nothing it could still write is pending:
	// the drain then proves the request drew exactly one datagram.
	stop()
	if extra := imiDrain(t, peerTr, myTr, remote, "after the owner loop returned"); len(extra) != 0 {
		t.Errorf("%d more datagrams went out over the SA, want exactly 1 in total", len(extra))
	}

	if endpoint := ps.saEndpoint.Load(); endpoint != nil {
		t.Errorf("the endpoint %v is still published after the owner loop returned", endpoint)
	}
	noteSuspicious(pkt)
	select {
	case queued := <-ps.suspicious:
		t.Errorf("a request from %v was queued to a session whose owner loop has returned", queued.RemoteAddr)
	default:
	}
}
