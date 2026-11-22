// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: predicate.go -- MPLSTransitMTUInUse, SRv6InUse

package kernelcap

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// VALIDATES: the transit MTU capability is in use exactly when RSVP-TE runs on
// the kernel FIB, the one configuration that installs a path MTU on an AF_MPLS
// route.
// PREVENTS: an LDP-only or labeled-BGP router warned about an MTU it never
// installs, and an RSVP-TE router on another backend probed for a kernel route
// attribute it does not use.
func TestMPLSTransitMTUInUse(t *testing.T) {
	withKernelFIB := func() *config.Tree {
		tree := config.NewTree()
		tree.GetOrCreateContainer("fib").GetOrCreateContainer("kernel")
		return tree
	}

	rsvp := withKernelFIB()
	rsvp.GetOrCreateContainer("rsvp-te")
	if !MPLSTransitMTUInUse(rsvp) {
		t.Error("RSVP-TE on the kernel FIB installs transit path MTUs, but the predicate says not in use")
	}

	ldp := withKernelFIB()
	ldp.GetOrCreateContainer("ldp")
	if MPLSTransitMTUInUse(ldp) {
		t.Error("LDP installs no path MTU, but the predicate says in use")
	}

	otherBackend := config.NewTree()
	otherBackend.GetOrCreateContainer("rsvp-te")
	if MPLSTransitMTUInUse(otherBackend) {
		t.Error("RSVP-TE without the kernel FIB programs no AF_MPLS route, but the predicate says in use")
	}

	if MPLSTransitMTUInUse(nil) {
		t.Error("a nil tree is in use")
	}
}

// VALIDATES: SRv6 kernel capabilities are in use when the kernel FIB is
// configured with at least one BGP peer or group, and never without the kernel
// FIB or without any BGP peer/group.
// PREVENTS: probing or warning on non-BGP or non-kernel-FIB routers, and
// missing SRv6 checks on BGP routers whose peers live under a peer group.
func TestSRv6InUse(t *testing.T) {
	withKernelFIB := func() *config.Tree {
		tree := config.NewTree()
		tree.GetOrCreateContainer("fib").GetOrCreateContainer("kernel")
		return tree
	}

	withPeer := withKernelFIB()
	withPeer.GetOrCreateContainer("bgp").AddListEntry("peer", "p1", config.NewTree())
	if !SRv6InUse(withPeer) {
		t.Error("kernel FIB + BGP peer must be in use, but SRv6InUse returned false")
	}

	withGroup := withKernelFIB()
	withGroup.GetOrCreateContainer("bgp").AddListEntry("group", "g1", config.NewTree())
	if !SRv6InUse(withGroup) {
		t.Error("kernel FIB + BGP group must be in use, but SRv6InUse returned false")
	}

	noFIB := config.NewTree()
	noFIB.GetOrCreateContainer("bgp").AddListEntry("peer", "p1", config.NewTree())
	if SRv6InUse(noFIB) {
		t.Error("BGP peer without fib block must not be in use")
	}

	fibNoKernel := config.NewTree()
	fibNoKernel.GetOrCreateContainer("fib")
	fibNoKernel.GetOrCreateContainer("bgp").AddListEntry("peer", "p1", config.NewTree())
	if SRv6InUse(fibNoKernel) {
		t.Error("empty fib block without kernel must not be in use")
	}

	vppFIB := config.NewTree()
	vppFIB.GetOrCreateContainer("fib").GetOrCreateContainer("vpp")
	vppFIB.GetOrCreateContainer("bgp").AddListEntry("peer", "p1", config.NewTree())
	if SRv6InUse(vppFIB) {
		t.Error("BGP peer on VPP FIB must not trigger kernel SRv6 checks")
	}

	noBGP := withKernelFIB()
	if SRv6InUse(noBGP) {
		t.Error("kernel FIB without bgp block must not be in use")
	}

	emptyBGP := withKernelFIB()
	emptyBGP.GetOrCreateContainer("bgp")
	if SRv6InUse(emptyBGP) {
		t.Error("kernel FIB with empty bgp block (no peer or group) must not be in use")
	}

	if SRv6InUse(nil) {
		t.Error("a nil tree must not be in use")
	}
}
