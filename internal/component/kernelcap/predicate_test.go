// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: predicate.go -- MPLSTransitMTUInUse

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
