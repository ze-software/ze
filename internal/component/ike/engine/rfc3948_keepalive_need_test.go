// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT keepalive sender
// Related: mobike_test.go -- the MOBIKE owner fixture; established.go, mobike.go -- the two keepalive gates.
package engine

import (
	"bytes"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// keepaliveNeedInterval is the keepalive period the established-path tests start
// the sender with, short so the positive case observes keepalives within the window.
const keepaliveNeedInterval = 40 * time.Millisecond

// keepaliveNeedWindow is how long a test listens: ten keepalive periods.
const keepaliveNeedWindow = 400 * time.Millisecond

// keepaliveNeedPeer opens a plain UDP socket for the peer. The IKE transport drops
// a lone 0xFF before Recv (RFC 3948 Section 2.3), so the peer reads raw datagrams.
func keepaliveNeedPeer(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("open peer socket: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test teardown
	return conn
}

// keepaliveNeedCount reads every datagram the peer receives within the window and
// counts the NAT-keepalives: one octet, 0xFF (RFC 3948 Section 2.3).
func keepaliveNeedCount(t *testing.T, conn *net.UDPConn) int {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(keepaliveNeedWindow)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	count := 0
	buf := make([]byte, 2048)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return count
		}
		if err != nil {
			t.Fatalf("peer read: %v", err)
		}
		if bytes.Equal(buf[:n], []byte{0xFF}) {
			count++
		}
	}
}

// keepaliveNeedSA returns an established SA floated to a NAT-T socket, whose peer
// endpoint is the raw socket, with NATDetected set to detected. The SA is the
// responder, so it is not the MOBIKE original initiator and the owner tick sends no
// path update of its own beside the keepalive.
func keepaliveNeedSA(t *testing.T, peer *net.UDPConn, detected, mobike bool) *mbFixture {
	t.Helper()
	ini, resp, _ := establishPSK(t)
	f := mbOwner(t, resp, ini, true)
	endpoint, ok := peer.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer socket address is not *net.UDPAddr")
	}
	f.local.peerEndpoint = endpoint
	f.local.NATDetected = detected
	f.local.mobike.enabled = mobike
	return f
}

// RFC requirement: RFC3948-4-1 positive -- "A peer SHOULD send a NAT-keepalive packet if
// a need for it is detected according to [RFC3947] and if no other packet to the peer has
// been sent in M seconds." (rfc/full/rfc3948.txt, Section 4). The need clause, on the
// established path without MOBIKE: an SA whose NAT detection found a NAT starts the
// keepalive (startNATKeepalive returns it running), and the peer receives 0xFF keepalives
// on the SA's NAT-T path.
//
// VALIDATES: a detected NAT starts the keepalive that runEstablished defers the stop of.
// PREVENTS: a gate that never starts the keepalive, leaving the NAT binding to expire.
func TestRFC3948KeepaliveStartsWhenNATDetected(t *testing.T) {
	peer := keepaliveNeedPeer(t)
	f := keepaliveNeedSA(t, peer, true, false)
	ka := f.ps.startNATKeepalive(f.local, f.myTr, keepaliveNeedInterval, slogutil.DiscardLogger())
	if ka == nil {
		t.Fatal("NAT detected, but no keepalive was started")
	}
	defer ka.Stop()
	if got := keepaliveNeedCount(t, peer); got < 2 {
		t.Fatalf("peer received %d NAT-keepalives in %s at a %s interval, want at least 2",
			got, keepaliveNeedWindow, keepaliveNeedInterval)
	}
}

// RFC requirement: RFC3948-4-1 negative -- "A peer SHOULD send a NAT-keepalive packet if
// a need for it is detected according to [RFC3947] and if no other packet to the peer has
// been sent in M seconds." (rfc/full/rfc3948.txt, Section 4). The need clause, on the
// established path without MOBIKE: an SA whose NAT detection found no NAT starts no
// keepalive, and the peer receives no 0xFF datagram.
//
// VALIDATES: the keepalive is conditional on the detected need.
// PREVENTS: an unconditional keepalive, sent to a peer with no NAT between them.
func TestRFC3948NoKeepaliveWithoutDetectedNAT(t *testing.T) {
	peer := keepaliveNeedPeer(t)
	f := keepaliveNeedSA(t, peer, false, false)
	ka := f.ps.startNATKeepalive(f.local, f.myTr, keepaliveNeedInterval, slogutil.DiscardLogger())
	if ka != nil {
		ka.Stop()
		t.Fatal("no NAT detected, but a keepalive was started")
	}
	if got := keepaliveNeedCount(t, peer); got != 0 {
		t.Fatalf("peer received %d NAT-keepalives with no NAT detected, want 0", got)
	}
}

// RFC requirement: RFC3948-4-1 positive -- "A peer SHOULD send a NAT-keepalive packet if
// a need for it is detected according to [RFC3947] and if no other packet to the peer has
// been sent in M seconds." (rfc/full/rfc3948.txt, Section 4). The need clause, on the
// MOBIKE path: an SA whose NAT detection found a NAT, with no keepalive sent yet, sends
// one 0xFF keepalive to the peer from the owner tick (serviceMobike).
//
// VALIDATES: MOBIKE, which owns the keepalive for its SA, sends it when a NAT was detected.
// PREVENTS: MOBIKE taking the keepalive over from startNATKeepalive and never sending it.
func TestRFC3948MobikeKeepaliveWhenNATDetected(t *testing.T) {
	peer := keepaliveNeedPeer(t)
	f := keepaliveNeedSA(t, peer, true, true)
	f.ps.serviceMobike(f.local, f.myTr, time.Now(), slogutil.DiscardLogger())
	if got := keepaliveNeedCount(t, peer); got != 1 {
		t.Fatalf("peer received %d NAT-keepalives from one owner tick, want 1", got)
	}
}

// RFC requirement: RFC3948-4-1 negative -- "A peer SHOULD send a NAT-keepalive packet if
// a need for it is detected according to [RFC3947] and if no other packet to the peer has
// been sent in M seconds." (rfc/full/rfc3948.txt, Section 4). The need clause, on the
// MOBIKE path: an SA whose NAT detection found no NAT sends no keepalive from the owner
// tick, although no keepalive was ever sent.
//
// VALIDATES: the MOBIKE keepalive is conditional on the detected need.
// PREVENTS: an owner tick that sends a keepalive to a peer with no NAT between them.
func TestRFC3948MobikeNoKeepaliveWithoutDetectedNAT(t *testing.T) {
	peer := keepaliveNeedPeer(t)
	f := keepaliveNeedSA(t, peer, false, true)
	f.ps.serviceMobike(f.local, f.myTr, time.Now(), slogutil.DiscardLogger())
	if got := keepaliveNeedCount(t, peer); got != 0 {
		t.Fatalf("peer received %d NAT-keepalives with no NAT detected, want 0", got)
	}
}
