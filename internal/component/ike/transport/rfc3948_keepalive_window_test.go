// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT keepalive send-window tests

package transport

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// RFC requirement: RFC3948-4-1 positive -- "A peer SHOULD send a NAT-keepalive packet if
// a need for it is detected according to [RFC3947] and if no other packet to the peer has
// been sent in M seconds." (rfc/full/rfc3948.txt, Section 4). Once started, Keepalive.Run
// never lets M pass without a packet to the peer: the first keepalive arrives within M of
// the start and each next one within M of the previous, so no idle window longer than M
// exists. The start condition (a detected NAT) is the engine's, not this unit's.
//
// Method: M = 80ms on loopback; three keepalives are read and every gap is held to M plus
// a scheduling allowance well below a second M.
func TestRFC3948KeepaliveIdleWindowNeverExceedsM(t *testing.T) {
	const intervalM = 80 * time.Millisecond
	const allowance = 50 * time.Millisecond

	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	remote, ok := peer.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}

	log := slogutil.DiscardLogger()
	client, err := NewNATTTransport("127.0.0.1:0", log)
	if err != nil {
		t.Fatalf("NewNATTTransport: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	ka := NewKeepalive(client, nil, remote, intervalM, log)
	last := time.Now()
	go ka.Run()
	t.Cleanup(ka.Stop)

	buf := make([]byte, 16)
	for i := range 3 {
		if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		n, _, err := peer.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("keepalive %d: %v (no packet to the peer for a second, M is %v)", i+1, err, intervalM)
		}
		now := time.Now()
		if n != 1 || buf[0] != keepaliveByte {
			t.Fatalf("keepalive %d: got %x, want one 0xFF octet", i+1, buf[:n])
		}
		if gap := now.Sub(last); gap > intervalM+allowance {
			t.Errorf("keepalive %d came %v after the previous packet; M is %v", i+1, gap, intervalM)
		}
		last = now
	}
}

// RFC requirement: RFC3948-4-1 negative -- "A peer SHOULD send a NAT-keepalive packet if
// a need for it is detected according to [RFC3947] and if no other packet to the peer has
// been sent in M seconds." (rfc/full/rfc3948.txt, Section 4). An unset or negative interval
// is the input that would leave the peer with no keepalive (time.NewTicker panics on any
// interval that is not positive, so Run would die unsent); NewKeepalive refuses it and uses
// DefaultKeepaliveInterval, which is the section's default M of 20 seconds.
//
// Method: NewKeepalive with 0 and -1s; the interval it will tick at is read back.
func TestRFC3948UnsetKeepaliveIntervalFallsBackToM(t *testing.T) {
	if DefaultKeepaliveInterval != 20*time.Second {
		t.Fatalf("DefaultKeepaliveInterval = %v; RFC 3948 Section 4 gives M a default of 20 seconds", DefaultKeepaliveInterval)
	}
	for _, input := range []time.Duration{0, -time.Second} {
		ka := NewKeepalive(nil, nil, nil, input, slogutil.DiscardLogger())
		if ka.interval != DefaultKeepaliveInterval {
			t.Errorf("NewKeepalive(interval %v) ticks at %v; want M = %v", input, ka.interval, DefaultKeepaliveInterval)
		}
	}
}
