// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- IPCP peer-selected IPv4 address (RFC 2865 Section 5.8)
// Related: ncp.go -- requestIPCPAddresses, evalIPCPRequest, buildNakOrReject, copyRejectedIPCPOptions
//
// VALIDATES: when IPResponseArgs.PeerNegotiated is true, Ze proposes its local
// gateway IP in IPCP, accepts a valid unicast IPv4 address proposed by the
// peer (programming P2P and emitting EventSessionIPAssigned with that address),
// Configure-Rejects unusable peer proposals (0.0.0.0, broadcast, multicast,
// loopback, link-local, or local IP), and Configure-Naks an omitted IP-Address
// option with 0.0.0.0 per RFC 1332 Section 3.3.
// PREVENTS: rejecting valid peer-proposed IPv4 addresses or accepting unusable
// addresses when RADIUS returned Framed-IP-Address == 0xFFFFFFFF.

package ppp

import (
	"bytes"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

// acceptPeerNegotiatedIP returns an IP responder that authorizes peer-selected
// IPv4 negotiation (RADIUS Framed-IP-Address == 0xFFFFFFFF) with the given
// local gateway and optional DNS servers.
func acceptPeerNegotiatedIP(local, primary, secondary netip.Addr) func(*Driver) {
	return func(d *Driver) {
		for ev := range d.IPEventsOut() {
			req, ok := ev.(EventIPRequest)
			if !ok {
				continue
			}
			args := IPResponseArgs{Accept: true, Family: req.Family}
			switch req.Family {
			case AddressFamilyIPv4:
				args.Local = local
				args.PeerNegotiated = true
				args.DNSPrimary = primary
				args.DNSSecondary = secondary
			case AddressFamilyIPv6:
			default:
				panic("BUG: unknown NCP request address family")
			}
			_ = d.IPResponse(req, args) //nolint:errcheck // ignore teardown race
		}
	}
}

// startNegotiatedIPCPPeer opens a session with PeerNegotiated=true, verifies
// Ze's initial Configure-Request proposes local, and Acks it.
func startNegotiatedIPCPPeer(t *testing.T, local, primary, secondary netip.Addr) *ncpTestDriver {
	t.Helper()
	td := newNCPTestDriverIP(t, &StartSession{DisableIPv6CP: true}, acceptPeerNegotiatedIP(local, primary, secondary))
	cr := td.readPeerNCPPacket(t, ProtoIPCP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("got code %d, want Ze's Configure-Request", cr.Code)
	}
	opts, err := ParseIPCPOptions(cr.Data)
	if err != nil {
		t.Fatalf("ParseIPCPOptions(Ze CR): %v", err)
	}
	if !opts.HasIPAddress || opts.IPAddress != local {
		t.Fatalf("Ze CR IP-Address = %v (has=%v), want local %v", opts.IPAddress, opts.HasIPAddress, local)
	}
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureAck, cr.Identifier, cr.Data)
	return td
}

// TestIPCPPeerNegotiatedAcceptsValidUnicastAddress verifies that a valid
// unicast IPv4 address proposed by the peer is Configure-Acked, installed on
// pppN as the P2P peer address, and emitted on EventSessionIPAssigned.
func TestIPCPPeerNegotiatedAcceptsValidUnicastAddress(t *testing.T) {
	td := startNegotiatedIPCPPeer(t, ipcpTestLocal, ipcpTestDNS1, ipcpTestDNS2)
	defer td.cleanup()

	chosen := netip.MustParseAddr("198.51.100.42")
	peerCR := ipcpOpt(IPCPOptIPAddress, chosen)
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x30, peerCR)

	ack := readIPCPReply(t, td)
	if ack.Code != LCPConfigureAck {
		t.Fatalf("reply code = %d (% x), want Configure-Ack", ack.Code, ack.Data)
	}
	if ack.Identifier != 0x30 || !bytes.Equal(ack.Data, peerCR) {
		t.Fatalf("Configure-Ack = (id=%d, data=% x), want (id=0x30, data=% x)", ack.Identifier, ack.Data, peerCR)
	}

	assigned, ok := waitForEventOfType[EventSessionIPAssigned](t, td.driver.EventsOut(), 2*time.Second)
	if !ok {
		t.Fatal("no EventSessionIPAssigned after peer-selected IPCP Opened")
	}
	if assigned.Family != AddressFamilyIPv4 {
		t.Errorf("family = %s, want ipv4", assigned.Family)
	}
	if assigned.Local != ipcpTestLocal || assigned.Peer != chosen {
		t.Errorf("assigned = (%v, %v), want (%v, %v)", assigned.Local, assigned.Peer, ipcpTestLocal, chosen)
	}

	p2p := td.backend.P2PCalls()
	if len(p2p) != 1 {
		t.Fatalf("AddAddressP2P calls = %d, want 1", len(p2p))
	}
	if p2p[0].local != "10.0.0.1/32" || p2p[0].peer != "198.51.100.42/32" {
		t.Errorf("p2p[0] = %+v, want local=10.0.0.1/32 peer=198.51.100.42/32", p2p[0])
	}
}

// TestIPCPPeerNegotiatedWithDNSNegotiation verifies that a peer proposing a
// valid IPv4 address alongside DNS=0.0.0.0 receives a Configure-Nak carrying
// only the DNS options (never overwriting the peer's chosen address), and then
// opens cleanly with the peer's chosen address.
func TestIPCPPeerNegotiatedWithDNSNegotiation(t *testing.T) {
	td := startNegotiatedIPCPPeer(t, ipcpTestLocal, ipcpTestDNS1, ipcpTestDNS2)
	defer td.cleanup()

	chosen := netip.MustParseAddr("203.0.113.77")
	zero := netip.IPv4Unspecified()
	firstCR := bytes.Join([][]byte{
		ipcpOpt(IPCPOptIPAddress, chosen),
		ipcpOpt(IPCPOptPrimaryDNS, zero),
	}, nil)
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x31, firstCR)

	nak := readIPCPReply(t, td)
	if nak.Code != LCPConfigureNak {
		t.Fatalf("reply code = %d (% x), want Configure-Nak for DNS", nak.Code, nak.Data)
	}
	nakOpts, err := ParseIPCPOptions(nak.Data)
	if err != nil {
		t.Fatalf("ParseIPCPOptions(Nak): %v", err)
	}
	if nakOpts.HasIPAddress {
		t.Errorf("Nak unexpectedly included IP-Address %v for a valid peer-chosen address", nakOpts.IPAddress)
	}
	if !nakOpts.HasPrimary || nakOpts.PrimaryDNS != ipcpTestDNS1 {
		t.Errorf("Nak PrimaryDNS = %v (has=%v), want %v", nakOpts.PrimaryDNS, nakOpts.HasPrimary, ipcpTestDNS1)
	}

	secondCR := bytes.Join([][]byte{
		ipcpOpt(IPCPOptIPAddress, chosen),
		ipcpOpt(IPCPOptPrimaryDNS, ipcpTestDNS1),
	}, nil)
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x32, secondCR)

	ack := readIPCPReply(t, td)
	if ack.Code != LCPConfigureAck {
		t.Fatalf("second reply code = %d (% x), want Configure-Ack", ack.Code, ack.Data)
	}
	assigned, ok := waitForEventOfType[EventSessionIPAssigned](t, td.driver.EventsOut(), 2*time.Second)
	if !ok {
		t.Fatal("no EventSessionIPAssigned")
	}
	if assigned.Peer != chosen {
		t.Errorf("assigned.Peer = %v, want %v", assigned.Peer, chosen)
	}
}

// TestIPCPPeerNegotiatedRejectsUnusableAddresses verifies that unusable IPv4
// proposals draw a Configure-Reject carrying that IP-Address option and never
// open IPCP, while a subsequent valid unicast proposal succeeds.
func TestIPCPPeerNegotiatedRejectsUnusableAddresses(t *testing.T) {
	cases := []struct {
		name string
		addr netip.Addr
	}{
		{"unspecified", netip.MustParseAddr("0.0.0.0")},
		{"broadcast", netip.MustParseAddr("255.255.255.255")},
		{"loopback", netip.MustParseAddr("127.0.0.1")},
		{"multicast", netip.MustParseAddr("224.0.0.1")},
		{"link-local", netip.MustParseAddr("169.254.1.1")},
		{"equal-to-local", ipcpTestLocal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td := startNegotiatedIPCPPeer(t, ipcpTestLocal, ipcpTestDNS1, netip.Addr{})
			defer td.cleanup()

			badOpt := ipcpOpt(IPCPOptIPAddress, tc.addr)
			td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x40, badOpt)

			reply := readIPCPReply(t, td)
			if reply.Code != LCPConfigureReject {
				t.Fatalf("reply code = %d (% x), want Configure-Reject for %s", reply.Code, reply.Data, tc.addr)
			}
			if !bytes.Equal(reply.Data, badOpt) {
				t.Fatalf("Configure-Reject Data = % x, want % x", reply.Data, badOpt)
			}
			if calls := td.backend.P2PCalls(); len(calls) != 0 {
				t.Fatalf("AddAddressP2P called after rejected proposal: %+v", calls)
			}

			// A valid retry on the same session still converges.
			good := netip.MustParseAddr("198.51.100.88")
			goodOpt := ipcpOpt(IPCPOptIPAddress, good)
			td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x41, goodOpt)
			ack := readIPCPReply(t, td)
			if ack.Code != LCPConfigureAck {
				t.Fatalf("retry reply code = %d (% x), want Configure-Ack", ack.Code, ack.Data)
			}
			assigned, ok := waitForEventOfType[EventSessionIPAssigned](t, td.driver.EventsOut(), 2*time.Second)
			if !ok {
				t.Fatal("no EventSessionIPAssigned after valid retry")
			}
			if assigned.Peer != good {
				t.Errorf("assigned.Peer = %v, want %v", assigned.Peer, good)
			}
		})
	}
}

// TestIPCPPeerNegotiatedOmittedIPAddressDrawsNakZero verifies that a peer
// omitting the IP-Address option in peer-negotiated mode receives a
// Configure-Nak with IP-Address=0.0.0.0 (RFC 1332 Section 3.3 request for the
// peer to provide its address) and does not open with an unset peer IP.
func TestIPCPPeerNegotiatedOmittedIPAddressDrawsNakZero(t *testing.T) {
	td := startNegotiatedIPCPPeer(t, ipcpTestLocal, netip.Addr{}, netip.Addr{})
	defer td.cleanup()

	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x50, nil)

	reply := readIPCPReply(t, td)
	if reply.Code != LCPConfigureNak {
		t.Fatalf("reply code = %d (% x), want Configure-Nak", reply.Code, reply.Data)
	}
	opts, err := ParseIPCPOptions(reply.Data)
	if err != nil {
		t.Fatalf("ParseIPCPOptions(Nak): %v", err)
	}
	if !opts.HasIPAddress || opts.IPAddress != netip.IPv4Unspecified() {
		t.Fatalf("Nak IP-Address = %v (has=%v), want 0.0.0.0", opts.IPAddress, opts.HasIPAddress)
	}
	if calls := td.backend.P2PCalls(); len(calls) != 0 {
		t.Fatalf("AddAddressP2P called without a negotiated peer address: %+v", calls)
	}
}

// TestIPCPNonNegotiatedMissingPeerStillFailsSession verifies that omitting Peer
// when PeerNegotiated is false still fails the session rather than entering
// IPCP without an address.
func TestIPCPNonNegotiatedMissingPeerStillFailsSession(t *testing.T) {
	ipFn := func(d *Driver) {
		for ev := range d.IPEventsOut() {
			req, ok := ev.(EventIPRequest)
			if !ok {
				continue
			}
			args := IPResponseArgs{
				Accept:         true,
				Family:         req.Family,
				Local:          ipcpTestLocal,
				PeerNegotiated: false,
			}
			_ = d.IPResponse(req, args) //nolint:errcheck // ignore teardown race
		}
	}
	td := newNCPTestDriverIP(t, &StartSession{DisableIPv6CP: true}, ipFn)
	defer td.cleanup()

	down, ok := waitForEventOfType[EventSessionDown](t, td.driver.EventsOut(), 2*time.Second)
	if !ok {
		t.Fatal("expected EventSessionDown when Peer is missing and PeerNegotiated is false")
	}
	if down.Reason == "" {
		t.Error("expected non-empty teardown reason")
	}
}

// TestIPCPPeerNegotiatedRefusesAddressRejectedByAdmitCallback verifies that
// when AdmitPeerIPv4 refuses a peer-proposed address (for example, one inside
// a configured pool or already held by another live session), IPCP sends a
// Configure-Reject for that IP-Address option and does not open, while a
// subsequent admitted address opens cleanly and ReleasePeerIPv4 fires on IPCP
// This-Layer-Down.
func TestIPCPPeerNegotiatedRefusesAddressRejectedByAdmitCallback(t *testing.T) {
	inPool := netip.MustParseAddr("10.0.0.10")
	heldByOther := netip.MustParseAddr("203.0.113.5")
	allowed := netip.MustParseAddr("198.51.100.77")

	var released atomic.Int32
	ipFn := func(d *Driver) {
		for ev := range d.IPEventsOut() {
			req, ok := ev.(EventIPRequest)
			if !ok {
				continue
			}
			args := IPResponseArgs{
				Accept:         true,
				Family:         req.Family,
				Local:          ipcpTestLocal,
				PeerNegotiated: true,
				AdmitPeerIPv4: func(addr netip.Addr) bool {
					return addr != inPool && addr != heldByOther
				},
				ReleasePeerIPv4: func() {
					released.Add(1)
				},
			}
			_ = d.IPResponse(req, args) //nolint:errcheck // ignore teardown race
		}
	}

	td := newNCPTestDriverIP(t, &StartSession{DisableIPv6CP: true}, ipFn)
	defer td.cleanup()

	cr := td.readPeerNCPPacket(t, ProtoIPCP)
	if cr.Code != LCPConfigureRequest {
		t.Fatalf("got code %d, want Configure-Request", cr.Code)
	}
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureAck, cr.Identifier, cr.Data)

	for i, bad := range []netip.Addr{inPool, heldByOther} {
		badOpt := ipcpOpt(IPCPOptIPAddress, bad)
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, uint8(0x60+i), badOpt)
		reply := readIPCPReply(t, td)
		if reply.Code != LCPConfigureReject {
			t.Fatalf("reply for %s = code %d (% x), want Configure-Reject", bad, reply.Code, reply.Data)
		}
		if !bytes.Equal(reply.Data, badOpt) {
			t.Fatalf("Configure-Reject Data for %s = % x, want % x", bad, reply.Data, badOpt)
		}
	}
	if calls := td.backend.P2PCalls(); len(calls) != 0 {
		t.Fatalf("AddAddressP2P called after pool-refused proposals: %+v", calls)
	}

	// Retry with an admitted address -> opens cleanly.
	goodOpt := ipcpOpt(IPCPOptIPAddress, allowed)
	td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x62, goodOpt)
	ack := readIPCPReply(t, td)
	if ack.Code != LCPConfigureAck {
		t.Fatalf("reply for %s = code %d (% x), want Configure-Ack", allowed, ack.Code, ack.Data)
	}
	assigned, ok := waitForEventOfType[EventSessionIPAssigned](t, td.driver.EventsOut(), 2*time.Second)
	if !ok {
		t.Fatal("no EventSessionIPAssigned after admitted address")
	}
	if assigned.Peer != allowed {
		t.Errorf("assigned.Peer = %v, want %v", assigned.Peer, allowed)
	}
	if _, ok := waitForEventOfType[EventSessionUp](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionUp")
	}

	// Peer terminates IPCP -> LCPActTLD invokes ReleasePeerIPv4.
	td.writePeerNCPPacket(t, ProtoIPCP, LCPTerminateRequest, 0x63, nil)
	termAck := readIPCPReply(t, td)
	if termAck.Code != LCPTerminateAck {
		t.Fatalf("reply to Terminate-Request = code %d, want Terminate-Ack", termAck.Code)
	}
	if _, ok := waitForEventOfType[EventSessionIPReleased](t, td.driver.EventsOut(), 2*time.Second); !ok {
		t.Fatal("no EventSessionIPReleased after IPCP Terminate-Request")
	}
	if released.Load() == 0 {
		t.Error("ReleasePeerIPv4 was not called on IPCP This-Layer-Down")
	}
}
