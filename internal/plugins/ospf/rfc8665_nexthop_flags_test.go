// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing forwarding.
// Related: sr_install.go -- srInstaller.installRoutes and forwarding, the producer this drives.
//
// VALIDATES: RFC 8665 section 5 on the outgoing label toward a next-hop that advertised the
// Prefix-SID: its E-Flag selects Explicit NULL, and its M-Flag makes NP and E ignored.
// PREVENTS: an installer that reads only the NP-Flag of the advertising next-hop.
package ospf

import (
	"net/netip"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc8665PushTowardOriginator installs 10.0.0.9/32 whose only next-hop is its originator
// 10.0.0.9, advertising index 9 with flags, and returns the out-label pushed toward it, or
// false when no label is pushed.
func rfc8665PushTowardOriginator(t *testing.T, flags sr.SIDFlags) (uint32, bool) {
	t.Helper()
	bus := &srCaptureBus{}
	inst := newTestInstaller(bus)
	orig := types.RouterID{10, 0, 0, 9}
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nhOrig := netip.MustParseAddr("10.0.9.9")
	sids := map[netip.Prefix]srRemotePrefixSID{fec: {Originator: orig, SID: sr.PrefixSID{Flags: flags, Index: 9}}}
	caps := map[types.RouterID]sr.SRGB{orig: sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})}
	algos := map[types.RouterID][]uint8{orig: {0}}
	routes := []srRoute{{Prefix: fec, Origin: orig, NextHops: []srNextHop{{Addr: nhOrig, Router: orig}}}}
	inst.installRoutes(routes, sids, caps, algos, sr.NewSRGB([]sr.LabelRange{{Base: 18000, Size: 100}}))
	for _, e := range bus.entries {
		if e.Op == mplsfibevents.OpPush && e.Action == mplsfibevents.ActionAdd && e.NextHop == nhOrig {
			if len(e.OutLabels) != 1 {
				t.Fatalf("push toward the originator carries %d labels, want 1: %+v", len(e.OutLabels), e)
			}
			return e.OutLabels[0], true
		}
	}
	return 0, false
}

// RFC requirement: RFC8665-5-7 positive -- toward the next-hop router that advertised the SID,
// its E-Flag and M-Flag are applied: NP=1 and E=0 pushes the SRGB label 16009, NP=1 and E=1
// pushes IPv4 Explicit NULL (0) instead, and M=1 ignores NP and E, so NP=0 (which alone means
// PHP) and NP=1 with E=1 (which alone means Explicit NULL) both push 16009.
func TestRFC8665NextHopEAndMFlagsApplied(t *testing.T) {
	// Goal: the installer reads E and M, not only NP. Method: one originator next-hop, one
	// flag set per case, the pushed label read from the mpls-fib entries.
	cases := []struct {
		name  string
		flags sr.SIDFlags
		want  uint32
	}{
		{"np-set", sr.SIDFlags{NP: true}, 16009},
		{"np-set-e-set", sr.SIDFlags{NP: true, E: true}, sr.ExplicitNullV4},
		{"m-set-np-clear", sr.SIDFlags{M: true}, 16009},
		{"m-set-np-set-e-set", sr.SIDFlags{M: true, NP: true, E: true}, 16009},
	}
	for _, c := range cases {
		label, ok := rfc8665PushTowardOriginator(t, c.flags)
		if !ok {
			t.Fatalf("%s: no label pushed toward the originator, want %d", c.name, c.want)
		}
		if label != c.want {
			t.Fatalf("%s: pushed label %d, want %d", c.name, label, c.want)
		}
	}
}
