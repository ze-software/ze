// Design: bfd.go -- pluginService.EnsureSession refuses an IPv6 peer with no IPv6 socket
//
// The goal is that a runtime client (OSPFv3, a BGP peer over IPv6) asking for an
// IPv6 session on a loop that holds only the IPv4 socket is told why, rather
// than handed a session whose every Control packet fails to send. Each case
// applies the operator's bfd container the way the plugin lifecycle does and
// asks for a multi-hop IPv6 session the way a client does.
//
// VALIDATES: EnsureSession refuses an IPv6 peer when bind-v6 is false, naming the fix.
// PREVENTS: an IPv6 session that sits Down because every send goes to the IPv4 socket.
package bfd

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/engine"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
)

// ipv6PeerRequest is the request a client with an IPv6 neighbor produces.
var ipv6PeerRequest = api.SessionRequest{
	Peer:  netip.MustParseAddr("2001:db8::1"),
	Local: netip.MustParseAddr("2001:db8::2"),
	Mode:  api.MultiHop,
}

// serviceForIPv6Config applies the operator's bfd container and pre-populates
// the multi-hop loop, so a build that ignores bind-v6 hands out a session
// rather than failing to bind a socket: the case fails on bind-v6 alone.
func serviceForIPv6Config(t *testing.T, data string) *pluginService {
	t.Helper()

	svc := serviceForConfig(t, data)
	mb, _ := transport.Pair(api.MultiHop, ipv6PeerRequest.Local, ipv6PeerRequest.Peer)
	svc.state.loops[loopKey{vrf: api.DefaultVRF, mode: api.MultiHop}] = engine.NewLoop(mb, clock.RealClock{})
	return svc
}

// TestIPv6SessionRefusedWithoutBindV6 covers bind-v6 false, the default: the
// loop owns no IPv6 socket, so the request is refused naming the leaf that
// fixes it.
func TestIPv6SessionRefusedWithoutBindV6(t *testing.T) {
	svc := serviceForIPv6Config(t, `{"bfd":{"enabled":"true"}}`)

	handle, err := svc.EnsureSession(ipv6PeerRequest)
	if handle != nil {
		t.Fatalf("EnsureSession returned a handle for an IPv6 peer with bind-v6 false")
	}
	if !errors.Is(err, errBfdIPv6NotBound) {
		t.Fatalf("EnsureSession error = %v, want %v", err, errBfdIPv6NotBound)
	}
	if !strings.Contains(err.Error(), "bind-v6 true") {
		t.Fatalf("EnsureSession error %q does not name the fix `bind-v6 true`", err)
	}
}

// TestIPv6SessionAcceptedWithBindV6 covers bind-v6 true, the other polarity.
// Without it a build that refused every IPv6 request would pass the case above.
func TestIPv6SessionAcceptedWithBindV6(t *testing.T) {
	svc := serviceForIPv6Config(t, `{"bfd":{"enabled":"true","bind-v6":"true"}}`)

	handle, err := svc.EnsureSession(ipv6PeerRequest)
	if err != nil {
		t.Fatalf("EnsureSession with bind-v6 true: %v", err)
	}
	if handle == nil {
		t.Fatalf("EnsureSession returned no handle with bind-v6 true")
	}
}

// TestIPv4MappedPeerNeedsNoBindV6 covers an IPv4-mapped IPv6 address, which the
// IPv4 socket carries: it MUST NOT be refused for want of an IPv6 socket.
func TestIPv4MappedPeerNeedsNoBindV6(t *testing.T) {
	svc := serviceForConfig(t, `{"bfd":{"enabled":"true"}}`)
	req := peerRequest
	req.Mode = api.MultiHop
	req.Peer = netip.AddrFrom16(peerRequest.Peer.As16())

	if _, err := svc.EnsureSession(req); errors.Is(err, errBfdIPv6NotBound) {
		t.Fatalf("EnsureSession refused an IPv4-mapped peer for want of bind-v6: %v", err)
	}
}
