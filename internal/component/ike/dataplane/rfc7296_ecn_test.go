// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- XFRM kernel backend

package dataplane

// RFC naming: untagged -- proves the XFRM mode mapping, not ECN handling; the RFC7296-2.24 tags sit on the kernel-state and decapsulation tests.

import (
	"testing"
)

// The RFC7296-2.24 tags sit on tests that read what the kernel does with the state Ze
// installs: rfc7296_ecn_kernel_state_linux_test.go reads the NOECN bit the kernel stores,
// and engine's TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE reads the ECN field a
// decapsulated packet arrives with. What stays here is the cross-platform half of the mode
// mapping, untagged because on its own it does not prove Section 2.24.

// VALIDATES: the mode mapping that decides whether an installed SA is a tunnel-mode SA at
// all, which is the class RFC 7296 Section 2.24 binds.
//
// PREVENTS: a silent remap of ModeTunnel onto a kernel mode that is not XFRM_MODE_TUNNEL.
// That happened once already: the constants are 1-based here and 0-based in the kernel, and
// passing one through unshifted sent ModeTunnel to the kernel as
// XFRM_MODE_ROUTEOPTIMIZATION (see the comment block above kernelXFRMMode).
//
// This carries no RFC tag. It proves the mode mapping, not the ECN behavior, and a check
// that cannot fail when ECN handling breaks must not be counted as evidence that it works.
func TestEcnModeMappingIdentifiesTunnelMode(t *testing.T) {
	tunnel, ok := kernelXFRMMode(ModeTunnel)
	if !ok {
		t.Fatal("ModeTunnel is not a mode this backend can express")
	}
	if tunnel != kernelModeTunnel {
		t.Errorf("ModeTunnel maps to kernel mode %d, want XFRM_MODE_TUNNEL (%d)", tunnel, kernelModeTunnel)
	}

	transport, ok := kernelXFRMMode(ModeTransport)
	if !ok {
		t.Fatal("ModeTransport is not a mode this backend can express")
	}
	if transport == tunnel {
		t.Error("tunnel and transport map to one kernel mode, so no SA can be identified as tunnel mode")
	}

	if _, ok := kernelXFRMMode(0); ok {
		t.Error("the unset mode was accepted, so an SA of no known mode would reach the kernel")
	}
}
