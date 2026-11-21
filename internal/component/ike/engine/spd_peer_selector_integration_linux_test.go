//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions, transit traffic
// Related: spd_policy.go -- spdTransitPolicies, the producer of the gate, and reconcileSPDPolicies
// Related: ../dataplane/policy_owner.go -- the ownership ledger whose refusal this observes in the kernel
// Related: rfc4301_espform_policy_integration_linux_test.go -- newEspPolLab, espPolSPD and espPolChild, the lab
// Related: spd_peer_selector_test.go -- the commit refusal of the same collision

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// peerSelectorKernelPolicy answers the policy the kernel holds on want's selector,
// direction and if_id, read back through ListPolicies, the listing
// `show vpn ipsec dataplane policy` prints. It fails the test when the kernel holds none.
func peerSelectorKernelPolicy(t *testing.T, want *dataplane.SPParams) dataplane.PolicyInfo {
	t.Helper()
	policies, err := dataplane.Get().ListPolicies()
	if err != nil {
		t.Fatalf("read back the SPD: %v", err)
	}
	for i := range policies {
		got := &policies[i]
		if got.PerSocket {
			continue
		}
		held := dataplane.SPParams{
			Src: got.Src, Dst: got.Dst, Dir: got.Dir, UpperProto: got.UpperProto,
			SrcPort: got.SrcPort, DstPort: got.DstPort, IfIndex: got.IfIndex, IfID: got.IfID,
		}
		if dataplane.PolicySelectorsEqual(held, *want) {
			return *got
		}
	}
	t.Fatalf("the kernel holds no %s policy %v to %v", dirLabel(want.Dir), want.Src, want.Dst)
	return dataplane.PolicyInfo{}
}

// peerSelectorWantProtect fails the test unless the kernel still holds the Child SA's
// OUT PROTECT policy on its selector: the PROTECT action, the Child SA's rank, the
// Child SA role and the Child SA's owner.
func peerSelectorWantProtect(t *testing.T, step string, protect *dataplane.SPParams) {
	t.Helper()
	got := peerSelectorKernelPolicy(t, protect)
	if got.Action != dataplane.SPActionProtect || got.Priority != protect.Priority {
		t.Fatalf("%s: the kernel holds action %v at rank %d on the Child SA's OUT selector, want PROTECT at rank %d: "+
			"the traffic the peer protects would leave in clear", step, got.Action, got.Priority, protect.Priority)
	}
	if got.Role != dataplane.PolicyRoleChildSA || !got.OwnerKnown || got.Owner != protect.Owner {
		t.Fatalf("%s: the policy on the Child SA's OUT selector is listed with role %v and owner %q (known %v), want the Child SA of %q",
			step, got.Role, got.Owner, got.OwnerKnown, protect.Owner)
	}
}

// TestOperatorGateNeverReplacesALiveChildSAPolicy proves that an operator entry whose
// transit gate names the selector of a live Child SA's OUT PROTECT policy is refused,
// and that the PROTECT policy stays in the kernel.
//
// RFC 4301 Section 4.4.1: "An SPD must discriminate among traffic that is afforded
// IPsec protection and traffic that is allowed to bypass IPsec." Linux identifies a
// policy by selector, direction and if_id and replaces the one it holds whatever the
// priority (xfrm_policy_insert_list), and the XFRM backend upserts, so the gate, a
// template-free allow at dataplane.PriorityTransitGate, would take the PROTECT policy's
// place and the peer's traffic would leave in clear.
//
// Method: the netns lab with the real XFRM backend, the catch-all `unmatched discard`
// and one tunnel-mode Child SA (espPolChild: UDP from local port 5000 of 10.2.0.0/16 to
// 10.1.0.0/16). The operator entry is a bypass, direction in, whose remote side is the
// Child SA's local selector and whose local side is its remote one; the gate its transit
// projection puts in OUT then carries the PROTECT policy's kernel identity, which the
// test checks before it uses it. Three steps, each followed by a read of the kernel:
// the gate handed to the backend alone, the entry handed to the reconciler a reload
// uses, and the gate's removal. Each must be refused with the ledger's
// PolicyOwnedError, and after each the kernel must hold PROTECT at the Child SA's rank
// on that selector. The reload must answer an operator error whose fix removes the
// entry, and must leave no operator policy behind.
//
// MUTATION: restoring the `if !ownedEntry(p) { return false, nil }` opening of
// policyOwners.claim (dataplane/policy_owner.go) lets the first step install the gate,
// and the read after it finds a template-free allow at PriorityTransitGate where the
// PROTECT policy was.
//
//ze:guest-root
func TestOperatorGateNeverReplacesALiveChildSAPolicy(t *testing.T) {
	newEspPolLab(t)
	espPolSPD(t)
	child := espPolChild(t)
	dp := dataplane.Get()
	log := slogutil.DiscardLogger()

	protect := childPolicies(child, dataplane.SADirOut)[0]
	if protect.Action != dataplane.SPActionProtect || protect.Owner == "" {
		t.Fatalf("the Child SA's first OUT policy is action %v for owner %q, not an owned PROTECT policy", protect.Action, protect.Owner)
	}
	peerSelectorWantProtect(t, "before any operator entry", &protect)

	if protect.SrcPort != dataplane.ExactPortMatch(protect.SrcPort.Port) || !protect.DstPort.IsAny() {
		t.Fatalf("the Child SA's OUT selector has ports %+v to %+v; the entry below is written for one local port and any remote port",
			protect.SrcPort, protect.DstPort)
	}
	entry := ipsec.SPDPolicy{
		Name: "hole", Action: dataplane.SPActionBypass, Order: 500,
		Direction: ipsec.SPDDirIn, Protocol: protect.UpperProto,
		LocalPrefix: protect.Dst, LocalPort: ipsec.AnyPort(),
		RemotePrefix: protect.Src, RemotePort: ipsec.PortSelector{Form: ipsec.PortSingle, Port: protect.SrcPort.Port},
	}
	var gate *dataplane.SPParams
	projected := spdEntryPolicies(dp, entry)
	for i := range projected {
		if projected[i].Dir == dataplane.SADirOut && projected[i].Priority == dataplane.PriorityTransitGate {
			gate = &projected[i]
		}
	}
	if gate == nil {
		t.Fatalf("the entry projects no OUT gate (%d policies): a port-scoped bypass gets its transit projection only "+
			"on a kernel that gathers forwarded fragments (capability bit 3), which this test needs", len(projected))
	}
	if !dataplane.PolicySelectorsEqual(*gate, protect) {
		t.Fatalf("the gate %v to %v does not carry the kernel identity of the Child SA's OUT PROTECT policy %v to %v",
			gate.Src, gate.Dst, protect.Src, protect.Dst)
	}

	// The gate alone: the install the defect let through.
	err := dp.InstallPolicy(*gate)
	owned, refused := errors.AsType[*dataplane.PolicyOwnedError](err)
	if !refused {
		t.Errorf("the backend answered %v to the gate on the Child SA's OUT selector, want PolicyOwnedError", err)
	} else if owned.HeldBy != protect.Owner || owned.WantedRole != dataplane.PolicyRoleOperator {
		t.Errorf("the refusal names holder %q and wanted role %v, want %q and the operator role", owned.HeldBy, owned.WantedRole, protect.Owner)
	}
	peerSelectorWantProtect(t, "after the gate was handed to the backend", &protect)

	// The whole entry, through the reconciler a reload uses.
	_, err = reconcileSPDPolicies(dp, nil, map[string]ipsec.SPDPolicy{"hole": entry}, log)
	refusal, isOperatorError := errors.AsType[*opererr.Error](err)
	if !isOperatorError {
		t.Fatalf("the reload that adds the entry answered %v, want an operator error", err)
	}
	if _, wraps := errors.AsType[*dataplane.PolicyOwnedError](err); !wraps || refusal.Fix() != "delete vpn ipsec policy hole" {
		t.Errorf("the reload refusal is %v with fix %q, want the ledger's refusal and `delete vpn ipsec policy hole`", err, refusal.Fix())
	}
	peerSelectorWantProtect(t, "after the refused reload", &protect)
	policies, err := dp.ListPolicies()
	if err != nil {
		t.Fatalf("read back the SPD: %v", err)
	}
	for i := range policies {
		if policies[i].Role == dataplane.PolicyRoleOperator {
			t.Errorf("the refused reload left an operator policy in the kernel: %s %v to %v at rank %d",
				dirLabel(policies[i].Dir), policies[i].Src, policies[i].Dst, policies[i].Priority)
		}
	}

	// The gate's removal: it must not delete the PROTECT policy either.
	err = dp.RemovePolicyParams(*gate)
	if _, refused := errors.AsType[*dataplane.PolicyOwnedError](err); !refused {
		t.Errorf("the backend answered %v to the gate's removal on the Child SA's OUT selector, want PolicyOwnedError", err)
	}
	peerSelectorWantProtect(t, "after the gate's removal was asked", &protect)
}
