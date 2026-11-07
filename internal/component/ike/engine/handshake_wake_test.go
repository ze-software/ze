package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// hswPeerName is the session name these tests register, unique so no other test's
// session in activePeersMap is replaced or found.
const hswPeerName = "handshake-wake-peer"

// hswRegister makes ps the running session for hswPeerName, as reconcile does, and
// removes it when the test ends.
func hswRegister(t *testing.T, ps *PeerSession) {
	t.Helper()
	peersMu.Lock()
	if activePeersMap == nil {
		activePeersMap = make(map[string]*PeerSession)
	}
	activePeersMap[hswPeerName] = ps
	peersMu.Unlock()
	t.Cleanup(func() {
		peersMu.Lock()
		delete(activePeersMap, hswPeerName)
		peersMu.Unlock()
	})
}

// VALIDATES: the dispatch goroutine wakes the initiator's handshake wait after it
// handled a message of that handshake.
// PREVENTS: the initiator acting on an IKE_AUTH response only at its next retransmit
// time, a window in which `show vpn ipsec sa` listed the IKE SA established with no
// Child SA, no XFRM state and no route, and the peer's traffic was dropped.
func TestHandleInboundWakesInitiatorHandshake(t *testing.T) {
	group, esp := testIKEGroup(), testESPGroup()
	iniPeer, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "wake-psk")
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	ini, err := newInitiatorSA(hswPeerName, iniPeer, group, esp)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeSAInitRequest(ini, group)
	if err != nil {
		t.Fatal(err)
	}
	ini.State = StateSAInitSent

	ps := &PeerSession{peerName: hswPeerName, handshakeWake: make(chan struct{}, 1)}
	hswRegister(t, ps)

	select {
	case <-ps.handshakeWake:
		t.Fatal("the wake was pending before any message was handled")
	default:
	}

	handleInbound(ini, transport.Packet{Data: raw}, NewSATable(), nil, slogutil.DiscardLogger())

	select {
	case <-ps.handshakeWake:
	default:
		t.Fatal("handling a message of the initiator's handshake did not wake its wait")
	}
}

// VALIDATES: awaitHandshake returns on a wake long before its retransmit timeout, and
// answers errStopped once the session stops.
// PREVENTS: a wake that the handshake loop never reads, which leaves it on the timer.
func TestAwaitHandshakeReturnsOnWake(t *testing.T) {
	old := afterFunc
	t.Cleanup(func() { afterFunc = old })
	never := make(chan time.Time)
	afterFunc = func(_ time.Duration) <-chan time.Time { return never }

	ps := &PeerSession{stopCh: make(chan struct{}), handshakeWake: make(chan struct{}, 1)}
	ps.wakeHandshake()
	// A second token is dropped, never blocks the dispatch goroutine.
	ps.wakeHandshake()

	result := make(chan error, 1)
	go func() { result <- ps.awaitHandshake(time.Hour) }()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("awaitHandshake after a wake = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("awaitHandshake did not return on a pending wake")
	}

	close(ps.stopCh)
	if err := ps.awaitHandshake(time.Hour); !errors.Is(err, errStopped) {
		t.Fatalf("awaitHandshake on a stopped session = %v, want errStopped", err)
	}
}
