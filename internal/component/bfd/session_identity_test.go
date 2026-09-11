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
	"github.com/ze-software/ze/internal/component/bfd/engine"
	ifcomp "github.com/ze-software/ze/internal/component/iface"
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
		VRF:  api.DefaultVRF,
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
	if got := bare.toSessionRequest(nil).Canonical(api.Topology{Links: links}).Key(); got != want {
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
	if got := full.toSessionRequest(nil).Canonical(api.Topology{Links: links}).Key(); got != want {
		t.Errorf("pinned entry naming every leaf: key = %+v, want %+v", got, want)
	}
}

// TestNoRuntimeClientBindsTheSharedSocket is the round-6 regression and the
// round-7 correction to it.
//
// A loop owns one socket and serves every session of its (vrf, mode) pair, and
// loopFor hands back an existing loop with the device the FIRST caller gave it.
// Round 6 stopped a DERIVED interface reaching SO_BINDTODEVICE; round 7 found
// that an operator-written one is no better, because the peer it binds the
// socket for is not the only peer that will use it. A peer on eth1 joining a
// loop bound to eth0 transmits from eth0, its BFD session stays Down, and a
// strict-mode BGP peer waits in OpenSent forever.
//
// So no runtime request binds the socket. The pinned path still does, from
// resolveLoopDevices, which sees every session that will share the loop before
// any of them exists.
func TestNoRuntimeClientBindsTheSharedSocket(t *testing.T) {
	peer := netip.MustParseAddr("172.30.0.10")
	links := []api.Link{{
		Name: "eth0",
		VRF:  api.DefaultVRF,
		Addrs: []api.LinkAddress{{
			Addr:   netip.MustParseAddr("172.30.0.2"),
			Prefix: netip.MustParsePrefix("172.30.0.0/24"),
		}},
	}}

	for name, req := range map[string]api.SessionRequest{
		"a peer that wrote no interface leaf, so Canonical derives one": {Peer: peer, Mode: api.SingleHop},
		"a peer whose operator wrote `bfd interface eth0`":              {Peer: peer, Interface: "eth0", Local: netip.MustParseAddr("172.30.0.2"), Mode: api.SingleHop},
	} {
		normalized := req.Canonical(api.Topology{Links: links})
		if normalized.Interface != "eth0" {
			t.Fatalf("setup, %s: key interface = %q, want eth0", name, normalized.Interface)
		}
		if got := loopDeviceFor(normalized); got != "" {
			t.Errorf("%s: device = %q, want empty: this socket also carries every later session in the VRF", name, got)
		}
	}

	// A named VRF is the one device decided per request: Linux binds the
	// socket to the VRF master, which is what every session in that VRF wants.
	inVRF := api.SessionRequest{Peer: peer, VRF: "red", Interface: "eth0", Mode: api.SingleHop}
	if got := loopDeviceFor(inVRF.Canonical(api.Topology{Links: links})); got != "red" {
		t.Errorf("device = %q in VRF red, want red", got)
	}
}

// TestLoopForKeepsTheFirstCallersDevice states the behavior the test above
// exists for: a second caller's device argument is dropped, because the loop
// and its socket already exist. It asserts the engine's own rule, not a defect:
// one socket cannot be re-bound, which is precisely why no runtime client may
// choose what it binds to. Pre-populated rather than started, so no transport
// binds the RFC 5881 port in a unit test.
func TestLoopForKeepsTheFirstCallersDevice(t *testing.T) {
	state := newRuntimeState()
	key := loopKey{vrf: api.DefaultVRF, mode: api.SingleHop}
	first := &engine.Loop{}
	state.loops[key] = first
	state.loopDevices[key] = "eth0"

	// A runtime client asks for no device, and that MUST NOT be reported as an
	// unapplied rebind: it is the healthy path, on every BGP peer, for a loop
	// applyPinned bound correctly.
	if _, err := state.loopFor(key, ""); err != nil {
		t.Fatalf("loopFor with no device: %v", err)
	}
	if state.loopDevices[key] != "eth0" {
		t.Fatalf("loopDevices = %q after a deviceless request, want eth0", state.loopDevices[key])
	}

	got, err := state.loopFor(key, "eth1")
	if err != nil {
		t.Fatalf("loopFor: %v", err)
	}
	if got != first {
		t.Fatalf("loopFor returned a new loop; the existing one and its bound device are what every later session uses")
	}
	// The device the loop is on is unchanged, and that is the point: a reload
	// that moves a pinned session to eth1 does not move the socket, so the
	// record must still say eth0 for the warning to be true.
	if state.loopDevices[key] != "eth0" {
		t.Fatalf("loopDevices = %q after a rebind request, want eth0: the socket cannot be rebound", state.loopDevices[key])
	}
}

// TestVRFMembershipReadsTheMasterChain covers the link table's VRF column,
// which decides whether a link is a candidate for a request at all. A VRF is a
// master device, and the member that carries the addresses can sit under a
// bridge under the VRF, so the walk climbs until it meets a device of type
// "vrf".
func TestVRFMembershipReadsTheMasterChain(t *testing.T) {
	infos := []ifcomp.InterfaceInfo{
		{Name: "red", Index: 10, Type: vrfLinkType},
		{Name: "br0", Index: 11, Type: "bridge", MasterIndex: 10},
		{Name: "eth1", Index: 12, Type: "device", MasterIndex: 11},
		{Name: "eth0", Index: 13, Type: "device"},
		{Name: "br1", Index: 14, Type: "bridge"},
		{Name: "eth2", Index: 15, Type: "device", MasterIndex: 14},
	}
	got := vrfMembership(infos)
	// A chain that cannot terminate on its own: two devices naming each other
	// as master. The kernel does not build one; the bound in vrfMembership is
	// what stops it hanging if anything ever does.
	cyclic := []ifcomp.InterfaceInfo{
		{Name: "a", Index: 20, Type: "device", MasterIndex: 21},
		{Name: "b", Index: 21, Type: "device", MasterIndex: 20},
		// A master this table does not carry, which is what a link whose VRF
		// was deleted between the two reads looks like.
		{Name: "orphan", Index: 22, Type: "device", MasterIndex: 99},
	}
	for index, want := range map[int]string{20: api.DefaultVRF, 21: api.DefaultVRF, 22: api.DefaultVRF} {
		if got := vrfMembership(cyclic)[index]; got != want {
			t.Errorf("index %d in a broken chain: VRF = %q, want %q", index, got, want)
		}
	}

	for _, row := range []struct {
		index int
		want  string
		why   string
	}{
		{10, "red", "a VRF device is in itself"},
		{11, "red", "a bridge enslaved to the VRF"},
		{12, "red", "a member two steps below the VRF"},
		{13, api.DefaultVRF, "an interface under no master"},
		{15, api.DefaultVRF, "a bridge member whose bridge is in no VRF"},
	} {
		if got[row.index] != row.want {
			t.Errorf("index %d: VRF = %q, want %q (%s)", row.index, got[row.index], row.want, row.why)
		}
	}
}
