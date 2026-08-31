// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: child.go -- childPolicyParams, the producer of every IKE-installed SPD entry
// RFC: rfc/short/rfc3948.md -- tunnel mode decapsulation NAT procedure (Section 3.1.1)
package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: the inbound Security Policy a Child SA install programs carries the
// NEGOTIATED remote traffic selector as its SOURCE selector, and this node's own
// negotiated selector as its destination. That source selector IS the valid source
// address space Section 3.1.1 asks the policy to define for the peer's decapsulated
// packets, and the kernel matches every decapsulated inner packet against it.
// PREVENTS: an inbound policy whose source selector came from this node's own half of
// the negotiation. Such an entry accepts inner packets sourced from the local network
// and rejects the peer's own traffic, which is the address-spoofing hole Section 3.1.1
// exists to close, and no test over the outbound direction alone can see it.
//
// The whole install path runs: createFirstChildSA resolves the negotiated selectors,
// installChildSA calls dp.InstallPolicy(childPolicyParams(child, SADirIn)), and the
// assertions below read what the dataplane was handed.
//
// RFC requirement: RFC3948-3.1.1-1 positive -- "If a valid source IP address space has
// been defined in the policy for the encapsulated packets from the peer, check that the
// source IP address of the inner packet is valid according to the policy" (RFC 3948
// Section 3.1.1, rfc/full/rfc3948.txt). This test checks that the inbound PROTECT policy
// ze installs defines that space: its source selector is the negotiated remote traffic
// selector and its destination selector is the local one, so an inner source address
// outside the negotiated remote selector is outside the installed policy's source space.
// The per-packet check itself is the kernel's, and this test does not exercise it.
func TestChildInboundPolicyDefinesTheValidInnerSourceSpace(t *testing.T) {
	const (
		localCIDR  = "10.1.0.0/24"
		remoteCIDR = "10.2.0.0/24"
	)

	_, localTS, err := net.ParseCIDR(localCIDR)
	if err != nil {
		t.Fatalf("parse %s: %v", localCIDR, err)
	}
	_, remoteTS, err := net.ParseCIDR(remoteCIDR)
	if err != nil {
		t.Fatalf("parse %s: %v", remoteCIDR, err)
	}

	sa := testSA()
	sa.IsInitiator = true
	sa.NegotiatedTSi, sa.NegotiatedTSr = localTS, remoteTS

	dp := &mockDP{}
	if _, err = createFirstChildSA(sa, testESPGroup(), "192.0.2.10", "192.0.2.20", 0, dp, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	var inbound *dataplane.SPParams
	for i := range dp.policies {
		if dp.policies[i].Dir == dataplane.SADirIn {
			inbound = &dp.policies[i]
			break
		}
	}
	if inbound == nil {
		t.Fatalf("the install programmed no inbound policy (%d policies); nothing defines the valid source space for the peer's decapsulated packets", len(dp.policies))
	}

	if inbound.Action != dataplane.SPActionProtect {
		t.Errorf("the inbound policy action is %v, want SPActionProtect (%v); a policy that does not protect states no source space the kernel enforces", inbound.Action, dataplane.SPActionProtect)
	}
	if got := inbound.Src.String(); got != remoteCIDR {
		t.Errorf("the inbound policy source selector is %s, want the negotiated remote selector %s; the valid source space names the wrong network", got, remoteCIDR)
	}
	if got := inbound.Dst.String(); got != localCIDR {
		t.Errorf("the inbound policy destination selector is %s, want the negotiated local selector %s", got, localCIDR)
	}

	// The two probes state what the selector means for an inner packet. Containment is
	// what the kernel resolves a decapsulated packet against, so an address the peer
	// negotiated is inside the installed space and one it did not is outside it.
	inside := net.ParseIP("10.2.0.7")
	outside := net.ParseIP("10.3.0.7")
	if !inbound.Src.Contains(inside) {
		t.Errorf("the inbound policy source selector %s excludes %s, which the peer negotiated; the peer's own traffic would not match the policy", inbound.Src, inside)
	}
	if inbound.Src.Contains(outside) {
		t.Errorf("the inbound policy source selector %s admits %s, which is outside the negotiated remote selector %s; a spoofed inner source would match the policy", inbound.Src, outside, remoteCIDR)
	}
}
