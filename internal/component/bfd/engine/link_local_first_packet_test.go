// VALIDATES: a session to an IPv6 link-local peer is selected by a packet
// that carries no discriminator, and two sessions to the SAME link-local
// address on two links stay apart.
// PREVENTS: the two-forms defect in the first-packet index. The kernel reports
// a zone on every link-local source, so the packet says fe80::1%eth0 while the
// session's key, parsed from a config leaf, says fe80::1. Compared raw, the
// two are never one session and a link-local peer can never be selected by its
// first packet. Reconciling them by dropping the zone instead of reducing it
// into the interface would break the second case, which is what the zone is
// for.
package engine

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/session"
	"github.com/ze-software/ze/internal/core/clock"
)

// linkLocalRequest is a single-hop session to a link-local peer on one link,
// in the shape configuration produces it: the peer address carries no zone,
// because the YANG leaf an operator writes holds an address alone.
func linkLocalRequest(iface string) api.SessionRequest {
	return api.SessionRequest{
		Peer:                  netip.MustParseAddr("fe80::1"),
		Local:                 netip.MustParseAddr("fe80::2"),
		Interface:             iface,
		Mode:                  api.SingleHop,
		DesiredMinTxInterval:  10_000,
		RequiredMinRxInterval: 10_000,
		DetectMult:            3,
	}
}

// linkLocalLoop returns an unstarted loop holding one session per named link.
func linkLocalLoop(t *testing.T, ifaces ...string) (*Loop, []*session.Machine) {
	t.Helper()
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	machines := make([]*session.Machine, 0, len(ifaces))
	for _, iface := range ifaces {
		req := linkLocalRequest(iface)
		if _, err := l.EnsureSession(req); err != nil {
			t.Fatalf("EnsureSession(%s): %v", iface, err)
		}
		machines = append(machines, machineFor(t, l, req.Key()))
	}
	return l, machines
}

// TestFirstPacketSelectsALinkLocalPeer drives the entry point with the address
// forms the two sides really hold: the packet's source carries the zone the
// kernel reported, and the session's key carries the address configuration
// gave it.
func TestFirstPacketSelectsALinkLocalPeer(t *testing.T) {
	l, machines := linkLocalLoop(t, "eth0")

	in := inboundControl(netip.MustParseAddr("fe80::1%eth0"), netip.MustParseAddr("fe80::2"), "eth0", 0)
	l.handleInbound(in)

	if got := machines[0].RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("the link-local session was not selected by its first packet: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}
}

// TestFirstPacketTakesTheLinkFromTheZone covers the reduction in the other
// direction: a transport that reports no ingress interface still names the
// link, in the zone on the source address. The reconciliation moves it into
// the field the key states a link in, so the session on that link is selected
// and a session on another link is not.
func TestFirstPacketTakesTheLinkFromTheZone(t *testing.T) {
	l, machines := linkLocalLoop(t, "eth0", "eth1")

	in := inboundControl(netip.MustParseAddr("fe80::1%eth1"), netip.MustParseAddr("fe80::2"), "", 0)
	l.handleInbound(in)

	if got := machines[0].RemoteDiscriminator(); got != 0 {
		t.Fatalf("a packet zoned to eth1 reached the eth0 session (RemoteDiscriminator = %d)", got)
	}
	if got := machines[1].RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("the eth1 session was not selected: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}
}

// TestTwoLinkLocalSessionsStayApart is what makes dropping the zone unsafe:
// one address, two links, two sessions. RFC 4007 Section 6 names exactly this
// case, "the use of link-local address fe80::1 in two separate physical
// links", and a packet from either link must reach its own session.
func TestTwoLinkLocalSessionsStayApart(t *testing.T) {
	l, machines := linkLocalLoop(t, "eth0", "eth1")

	in := inboundControl(netip.MustParseAddr("fe80::1%eth0"), netip.MustParseAddr("fe80::2"), "eth0", 0)
	l.handleInbound(in)

	if got := machines[0].RemoteDiscriminator(); got != peerMyDiscr {
		t.Fatalf("the eth0 session was not selected: RemoteDiscriminator = %d, want %d", got, peerMyDiscr)
	}
	if got := machines[1].RemoteDiscriminator(); got != 0 {
		t.Fatalf("the packet from eth0 also reached the eth1 session (RemoteDiscriminator = %d)", got)
	}
}
