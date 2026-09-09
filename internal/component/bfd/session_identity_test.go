// VALIDATES: RFC 5882 sec 4.4 -- a pinned `single-hop-session` entry and a
// protocol client for the same neighbor reach one api.Key, so the operator's
// configured session is the session BGP and OSPF join.
// PREVENTS: the pinned path drifting away from the protocol path. The pinned
// producer normalizes its VRF at parse time and the protocol producers do not,
// so before Canonical the two sides differed on a field the operator never
// wrote.
package bfd

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// RFC requirement: RFC5882-4.4-1 positive -- "If multiple control protocols
// wish to establish BFD sessions with the same remote system for the same data
// protocol, all MUST share a single BFD session" (RFC 5882 sec 4.4). A pinned
// session is a client like any other (RFC 5882 sec 5, configuration-driven
// bootstrapping), so applyPinned canonicalizes its request through the same
// api.SessionRequest.Canonical that pluginService.EnsureSession applies to
// every protocol client. This drives the real parser, parseSingleHopSession,
// and the real request builder, toSessionRequest. The two protocol halves are
// TestStrictPeerRequestReachesTheSharedKey
// (internal/component/bgp/reactor/config_bfd_strict_test.go) and
// TestOSPFNeighborRequestReachesTheSharedKey (internal/plugins/ospf), which
// assert this same key.
func TestPinnedSessionReachesTheSharedKey(t *testing.T) {
	links := []api.Link{{
		Name: "eth0",
		Addrs: []api.LinkAddress{{
			Addr:   netip.MustParseAddr("172.30.0.2"),
			Prefix: netip.MustParsePrefix("172.30.0.0/24"),
		}},
	}}
	want := api.Key{
		Peer:      netip.MustParseAddr("172.30.0.10"),
		Local:     netip.MustParseAddr("172.30.0.2"),
		Interface: "eth0",
		VRF:       api.DefaultVRF,
		Mode:      api.SingleHop,
	}

	// The operator writes the peer and nothing else: no interface, no local
	// address, no VRF. This is the entry a `bfd { single-hop-session
	// 172.30.0.10 { } }` stanza parses to.
	bare, err := parseSingleHopSession("172.30.0.10", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("parseSingleHopSession: %v", err)
	}
	if got := bare.toSessionRequest(nil).Canonical(links).Key(); got != want {
		t.Errorf("pinned entry naming only the peer: key = %+v, want %+v", got, want)
	}

	// The operator writes every leaf. The same key, or writing them costs a
	// second session.
	full, err := parseSingleHopSession("172.30.0.10", map[string]any{
		"local":     "172.30.0.2",
		"interface": "eth0",
		"vrf":       "default",
	}, nil)
	if err != nil {
		t.Fatalf("parseSingleHopSession (all leaves): %v", err)
	}
	if got := full.toSessionRequest(nil).Canonical(links).Key(); got != want {
		t.Errorf("pinned entry naming every leaf: key = %+v, want %+v", got, want)
	}
}
