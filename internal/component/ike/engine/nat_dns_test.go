package engine

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// natDetectionOf returns the NAT_DETECTION_SOURCE_IP and DESTINATION_IP data of an
// encoded IKE_SA_INIT request, nil for each one absent.
func natDetectionOf(t *testing.T, msg []byte) (source, destination []byte) {
	t.Helper()
	var h wire.Header
	if err := h.ReadFrom(msg); err != nil {
		t.Fatalf("header: %v", err)
	}
	entries, err := wire.ParsePayloadChain(msg[wire.HeaderLen:], h.NextPayload)
	if err != nil {
		t.Fatalf("payloads: %v", err)
	}
	for _, e := range entries {
		n, ok := e.Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		switch n.NotifyMsgType {
		case wire.NotifyNATDetectionSourceIP:
			source = n.NotificationData
		case wire.NotifyNATDetectionDestIP:
			destination = n.NotificationData
		}
	}
	return source, destination
}

// cycleSA builds the initiator SA one connect cycle of peer would build.
func cycleSA(t *testing.T, peer ipsec.SiteToSitePeer) *SA {
	t.Helper()
	pair, err := resolveCyclePair(peer.Name, peer)
	if err != nil {
		t.Fatalf("resolveCyclePair: %v", err)
	}
	sa, err := newInitiatorSA(peer.Name, peer, testIKEGroup(), ipsec.ESPGroup{})
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	sa.cycleRemote = pair.remote.IP
	sa.cycleLocal = pair.local
	return sa
}

// VALIDATES: AC-10, RFC 7296 Section 2.23: an initiator whose remote-address is a DNS name
// sends both NAT_DETECTION payloads, hashed over the address the cycle resolved.
//
// RED before the fix: the payloads were built from net.ParseIP(remote-address), which is
// nil for a name, so the IKE_SA_INIT carried none.
//
// RFC requirement: RFC7296-2.23-7 positive -- an initiator whose remote-address is a DNS
// name includes NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP in its
// IKE_SA_INIT, hashed over its own address and the address the name resolved to.
func TestDNSRemoteSendsNATDetection(t *testing.T) {
	previous := resolveRemoteFn
	resolveRemoteFn = func(host string) (*net.UDPAddr, error) {
		if host != "vpn.example.net" {
			t.Fatalf("resolved %q, want the configured name", host)
		}
		return &net.UDPAddr{IP: net.IPv4(198, 51, 100, 7), Port: 500}, nil
	}
	t.Cleanup(func() { resolveRemoteFn = previous })

	peer := testPeer()
	peer.RemoteAddress = "vpn.example.net"
	peer.LocalAddress = "203.0.113.1"
	sa := cycleSA(t, peer)

	msg, err := encodeSAInitRequest(sa, testIKEGroup())
	if err != nil {
		t.Fatalf("encodeSAInitRequest: %v", err)
	}
	source, destination := natDetectionOf(t, msg)
	wantSource := transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, net.ParseIP("203.0.113.1"), transport.IKEPort)
	wantDestination := transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, net.ParseIP("198.51.100.7"), transport.IKEPort)
	if !bytes.Equal(source, wantSource) {
		t.Fatalf("NAT_DETECTION_SOURCE_IP %x, want the hash over 203.0.113.1", source)
	}
	if !bytes.Equal(destination, wantDestination) {
		t.Fatalf("NAT_DETECTION_DESTINATION_IP %x, want the hash over the resolved 198.51.100.7", destination)
	}
}

// VALIDATES: AC-6: a peer with no `interface` and no local-address sends from the source
// the routing table picks toward the resolved remote. NAT_DETECTION_SOURCE_IP is hashed
// over it, and the transport host pair is built on it.
//
// RED before the fix: the empty local-address reached every reader as nil, so no NAT
// detection was sent and newChildSA refused the Child SA.
func TestRouteChosenLocalAddressBuildsChildSA(t *testing.T) {
	previous := routeSourceFn
	routeSourceFn = func(remote net.IP) (net.IP, error) {
		if !remote.Equal(net.ParseIP("192.0.2.1")) {
			t.Fatalf("route lookup toward %s, want the remote", remote)
		}
		return net.ParseIP("203.0.113.1"), nil
	}
	t.Cleanup(func() { routeSourceFn = previous })

	sa := cycleSA(t, testPeer())
	if !cycleLocalAddress(sa).Equal(net.ParseIP("203.0.113.1")) {
		t.Fatalf("local address %v, want the route's source", cycleLocalAddress(sa))
	}
	msg, err := encodeSAInitRequest(sa, testIKEGroup())
	if err != nil {
		t.Fatalf("encodeSAInitRequest: %v", err)
	}
	source, _ := natDetectionOf(t, msg)
	want := transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, net.ParseIP("203.0.113.1"), transport.IKEPort)
	if !bytes.Equal(source, want) {
		t.Fatalf("NAT_DETECTION_SOURCE_IP %x, want the hash over the route's source", source)
	}
	host := configuredHostPair(sa)
	if host == nil || !host.local.IP.Equal(net.ParseIP("203.0.113.1")) {
		t.Fatalf("host pair %+v, want local 203.0.113.1", host)
	}
}

// VALIDATES: AC-13, AC-14 and assumption A-3: a remote-address name that does not resolve
// fails the cycle, the session retries on the reconnect backoff (1 s, 2 s, 4 s ...) and
// never gives up, and the first cycle after the name answers builds its SA on the address.
func TestUnresolvableRemoteRetriesWithBackoff(t *testing.T) {
	var resolves atomic.Int32
	previousResolve := resolveRemoteFn
	resolveRemoteFn = func(string) (*net.UDPAddr, error) {
		if resolves.Add(1) <= 4 {
			return nil, errors.New("no such host")
		}
		return &net.UDPAddr{IP: net.IPv4(198, 51, 100, 7), Port: 500}, nil
	}
	var mu sync.Mutex
	var delays []time.Duration
	previousAfter := afterFunc
	afterFunc = func(d time.Duration) <-chan time.Time {
		mu.Lock()
		delays = append(delays, d)
		mu.Unlock()
		fired := make(chan time.Time, 1)
		fired <- time.Now()
		return fired
	}
	t.Cleanup(func() {
		resolveRemoteFn = previousResolve
		afterFunc = previousAfter
	})

	peer := testPeer()
	peer.RemoteAddress = "vpn.example.net"
	peer.LocalAddress = "203.0.113.1"
	ps := startPeerSession(peer.Name, peer, testIKEGroup(), testESPGroup(), NewSATable(), nil, nil, nil, slog.Default())
	t.Cleanup(func() {
		ps.Stop()
		<-ps.done
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if sa := ps.getSA(); sa != nil && sa.cycleRemote != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	sa := ps.getSA()
	if sa == nil || !sa.cycleRemote.Equal(net.ParseIP("198.51.100.7")) {
		t.Fatal("no cycle reached the resolved address once the name answered")
	}
	mu.Lock()
	defer mu.Unlock()
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	if len(delays) < len(want) {
		t.Fatalf("delays %v, want at least %v", delays, want)
	}
	for i, d := range want {
		if delays[i] != d {
			t.Fatalf("retry %d waited %v, want %v (all: %v)", i+1, delays[i], d, delays)
		}
	}
}

// VALIDATES: AC-11, RFC 7296 Section 2.23: an initiator whose remote-address is a DNS name
// checks the responder's NAT_DETECTION_SOURCE_IP against the address the cycle resolved.
// A mismatch records that the peer is behind a NAT and floats the SA to port 4500, so
// every later message and the Child SA's encapsulation (newChildSA reads NATDetected)
// use UDP port 4500. A hash over the resolved address floats nothing, which proves the
// comparison is made against that address and is not a float on every DNS peer.
//
// RED before the fix: the check hashed net.ParseIP(remote-address), nil for a name, so
// it was skipped and a NAT between ze and a peer named by DNS was never detected.
//
// RFC requirement: RFC7296-2.23-1 positive -- an initiator whose remote-address is a DNS
// name checks the responder's NAT_DETECTION_SOURCE_IP against the resolved address and
// records the peer behind a NAT on a mismatch.
// RFC requirement: RFC7296-2.23-8 positive -- an initiator that discovers a NAT toward a
// peer named by DNS moves the IKE SA to port 4500; one whose hash matches the resolved
// address stays on port 500.
func TestDNSRemoteNATMismatchFloats(t *testing.T) {
	log := slogutil.DiscardLogger()
	resolved := net.ParseIP("198.51.100.7")
	local := net.ParseIP("203.0.113.1")

	cases := []struct {
		name    string
		srcHash []byte
		want    ndVerdict
		floated bool
	}{{
		name:    "responder translated by a NAT",
		srcHash: ndTranslatedHash(),
		want:    ndVerdict{detected: true, peerBehindNAT: true},
		floated: true,
	}, {
		name:    "responder at the resolved address",
		srcHash: transport.NATDetectionHash(ndSPIi, ndSPIr, resolved, transport.IKEPort),
		want:    ndVerdict{},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			peer := testPeer()
			peer.RemoteAddress = "vpn.example.net"
			peer.LocalAddress = "203.0.113.1"
			sa := &SA{
				PeerCfg:      peer,
				InitiatorSPI: ndSPIi,
				ResponderSPI: ndSPIr,
				State:        StateSAInitSent,
				cycleRemote:  resolved,
				cycleLocal:   local,
			}
			table := NewSATable()
			table.Insert(sa)
			cleanDst := transport.NATDetectionHash(ndSPIi, ndSPIr, local, transport.IKEPort)
			raw := ndSAInitResponse(c.srcHash, cleanDst)
			handleSAInitResponse(sa, parseMsg(t, raw), raw, table, nil, nil, log)

			if got := ndVerdictOf(sa); got != c.want {
				t.Fatalf("verdict = %+v, want %+v", got, c.want)
			}
			if floated := sa.localPort == transport.NATTPort; floated != c.floated {
				t.Fatalf("floated to port 4500 = %v, want %v", floated, c.floated)
			}
		})
	}
}
