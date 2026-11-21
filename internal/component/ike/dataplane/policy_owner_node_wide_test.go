// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Policy ownership
// Related: policy_owner.go -- claim and deleteThenRelease, the refusals under test
// Related: policy_owner_test.go -- the owner-against-owner refusals of the same ledger

package dataplane

import (
	"errors"
	"strings"
	"testing"
)

// nodeWideTestPair answers a peer's OUT PROTECT policy and the operator transit gate
// that names the same selector: a template-free allow at PriorityTransitGate, which is
// what engine/spd_policy.go spdTransitPolicies projects for an `in` bypass whose remote
// and local sides are the peer's local and remote ones.
func nodeWideTestPair(t *testing.T) (protect, gate SPParams) {
	t.Helper()
	src, dst := overlapTestNet(t, "10.2.0.0/24"), overlapTestNet(t, "10.1.0.0/24")
	protect = SPParams{
		Src: src, Dst: dst, Dir: SADirOut, Action: SPActionProtect, Mode: ModeTunnel,
		Owner: "peer-a", Role: PolicyRoleChildSA, Priority: PriorityChildSA,
	}
	gate = SPParams{
		Src: src, Dst: dst, Dir: SADirOut, Action: SPActionBypass,
		Role: PolicyRoleOperator, Priority: PriorityTransitGate,
	}
	return protect, gate
}

// TestPolicyOwnersRefuseANodeWideEntryOnAnOwnedSelector proves a node-wide entry is
// never installed over, nor deleted from, a selector a peer's Child SA policy holds.
//
// The kernel holds one policy per selector, direction and if_id, whatever the priority,
// and the backend upserts, so an operator transit gate that names the selector of a
// peer's OUT PROTECT policy would replace it with an allow and the peer's traffic would
// leave in clear; the gate's removal would delete the PROTECT policy the same way.
//
// Method: claim and deleteThenRelease, the calls InstallPolicy and RemovePolicyParams
// make, with a delete that records whether it reached the kernel. The gate is tried at
// a far lower rank than the PROTECT policy, so a refusal keyed on rank would pass it.
// The catch-all role is tried too: the refusal is for every node-wide role. Once the
// peer's policy is gone the gate is accepted, so the refusal is tied to the holder.
//
// MUTATION: restoring the `if !ownedEntry(p) { return false, nil }` opening of claim
// makes the first claim(gate) answer nil, and this test fails there.
func TestPolicyOwnersRefuseANodeWideEntryOnAnOwnedSelector(t *testing.T) {
	protect, gate := nodeWideTestPair(t)

	var o policyOwners
	if created, err := o.claim(protect); err != nil || !created {
		t.Fatalf("claim(peer-a OUT PROTECT) = %v, %v; want a created record", created, err)
	}
	o.label(protect)

	var owned *PolicyOwnedError
	if _, err := o.claim(gate); !errors.As(err, &owned) {
		t.Fatalf("claim(operator gate on peer-a's PROTECT selector) = %v, want PolicyOwnedError: "+
			"the upsert would replace the PROTECT policy with an allow", err)
	}
	if owned.HeldBy != "peer-a" || owned.WantedRole != PolicyRoleOperator || owned.HeldRole != PolicyRoleUnspecified {
		t.Fatalf("the refusal names holder %q (role %v) and wanted role %v; want peer-a and the operator role",
			owned.HeldBy, owned.HeldRole, owned.WantedRole)
	}
	for _, want := range []string{`peer "peer-a"`, "`vpn ipsec policy`", "whatever its priority", "10.2.0.0/24 -> 10.1.0.0/24"} {
		if !strings.Contains(owned.Error(), want) {
			t.Errorf("the refusal %q does not carry %q", owned.Error(), want)
		}
	}

	catchAll := gate
	catchAll.Role, catchAll.Priority = PolicyRoleCatchAll, PriorityUnmatched
	if _, err := o.claim(catchAll); !errors.As(err, &owned) || owned.WantedRole != PolicyRoleCatchAll {
		t.Fatalf("claim(catch-all on peer-a's PROTECT selector) = %v, want PolicyOwnedError for the catch-all role", err)
	}

	ran := false
	if err := o.deleteThenRelease(gate, func() error { ran = true; return nil }); !errors.As(err, &owned) || ran {
		t.Fatalf("delete of the operator gate on peer-a's selector = %v (delete ran %v), want a refusal before the kernel: "+
			"the delete would remove the PROTECT policy", err, ran)
	}
	if owner, known := o.ownerOf(protect); !known || owner != "peer-a" {
		t.Fatalf("the selector's owner is %q (known %v) after the refusals, want peer-a", owner, known)
	}

	if err := o.deleteThenRelease(protect, func() error { return nil }); err != nil {
		t.Fatalf("peer-a delete of its PROTECT policy = %v", err)
	}
	if _, err := o.claim(gate); err != nil {
		t.Fatalf("claim(operator gate) once peer-a left = %v, want it accepted", err)
	}
}

// TestPolicyOwnersRefuseAnOwnedEntryOnAnOperatorSelector proves the reverse: a peer's
// Child SA policy is never installed over, nor deleted from, a selector an operator
// entry holds in the kernel.
//
// The Child SA install would replace the operator's disposition, and the Child SA's
// teardown would delete it, the operator gate with it.
//
// Method: the operator gate is claimed and labeled, as InstallPolicy does once the
// kernel accepted it; then the peer's claim and delete are tried. Two cases just
// outside the refusal are accepted. A catch-all on the selector does not refuse the
// peer: a peer that negotiates the wildcard installs its PROTECT policy on the
// catch-all's selector, and matches everything the catch-all would. An operator entry
// the kernel never accepted (claimed, never labeled) holds nothing. Once the operator
// entry is removed the peer is accepted.
//
// MUTATION: removing the operatorHeldLocked branch of claim makes claim(protect) answer
// a created record, and this test fails there.
func TestPolicyOwnersRefuseAnOwnedEntryOnAnOperatorSelector(t *testing.T) {
	protect, gate := nodeWideTestPair(t)

	var o policyOwners
	if created, err := o.claim(gate); err != nil || created {
		t.Fatalf("claim(operator gate) = %v, %v; want it accepted with no owner record", created, err)
	}
	o.label(gate)

	var owned *PolicyOwnedError
	created, err := o.claim(protect)
	if !errors.As(err, &owned) || created {
		t.Fatalf("claim(peer-a OUT PROTECT on the operator gate's selector) = %v, %v; want PolicyOwnedError", created, err)
	}
	if owned.HeldRole != PolicyRoleOperator || owned.Wanted != "peer-a" || owned.WantedRole != PolicyRoleUnspecified {
		t.Fatalf("the refusal names holder role %v and wanted %q (role %v); want the operator role and peer-a",
			owned.HeldRole, owned.Wanted, owned.WantedRole)
	}
	if !strings.Contains(owned.Error(), "delete or narrow the `vpn ipsec policy` entry") {
		t.Errorf("the refusal %q does not say what removes the collision", owned.Error())
	}
	if _, known := o.ownerOf(protect); known {
		t.Fatal("the refused claim left an owner record on the operator gate's selector")
	}

	ran := false
	if err := o.deleteThenRelease(protect, func() error { ran = true; return nil }); !errors.As(err, &owned) || ran {
		t.Fatalf("peer-a delete on the operator gate's selector = %v (delete ran %v), want a refusal before the kernel: "+
			"the delete would remove the operator gate", err, ran)
	}

	// A catch-all yields to a peer's PROTECT policy on its own selector.
	wide := overlapTestNet(t, "0.0.0.0/0")
	catchAll := SPParams{Src: wide, Dst: wide, Dir: SADirOut, Action: SPActionDiscard, Role: PolicyRoleCatchAll, Priority: PriorityUnmatched}
	if _, err := o.claim(catchAll); err != nil {
		t.Fatalf("claim(catch-all) = %v", err)
	}
	o.label(catchAll)
	fullTunnel := protect
	fullTunnel.Src, fullTunnel.Dst = wide, wide
	if created, err := o.claim(fullTunnel); err != nil || !created {
		t.Fatalf("claim(peer-a full-tunnel PROTECT on the catch-all's selector) = %v, %v; want a created record", created, err)
	}

	// An operator entry the kernel refused was never labeled, so it holds nothing.
	other := overlapTestNet(t, "10.3.0.0/24")
	refusedGate, otherProtect := gate, protect
	refusedGate.Src, otherProtect.Src = other, other
	if _, err := o.claim(refusedGate); err != nil {
		t.Fatalf("claim(second operator gate) = %v", err)
	}
	if created, err := o.claim(otherProtect); err != nil || !created {
		t.Fatalf("claim(peer-a PROTECT where the operator gate never reached the kernel) = %v, %v; want a created record", created, err)
	}

	if err := o.deleteThenRelease(gate, func() error { return nil }); err != nil {
		t.Fatalf("delete of the operator gate = %v", err)
	}
	if created, err := o.claim(protect); err != nil || !created {
		t.Fatalf("claim(peer-a OUT PROTECT) once the operator gate left = %v, %v; want a created record", created, err)
	}
}

// TestPolicySelectorsEqualIsTheKernelIdentity proves PolicySelectorsEqual, the key the
// engine's commit check shares with the ledger, compares what the kernel tells policies
// apart by and nothing else. Method: one policy against copies that differ in one field.
// Action, priority, template and owner must not separate two policies; direction,
// protocol, a port and the if_id must. A nil prefix is the wildcard of its family.
func TestPolicySelectorsEqualIsTheKernelIdentity(t *testing.T) {
	protect, gate := nodeWideTestPair(t)
	if !PolicySelectorsEqual(protect, gate) {
		t.Fatal("a PROTECT policy and a gate on one selector, at different priorities, are not one kernel policy")
	}
	for _, tc := range []struct {
		name string
		edit func(*SPParams)
	}{
		{"direction", func(p *SPParams) { p.Dir = SADirFwd }},
		{"protocol", func(p *SPParams) { p.UpperProto = 17 }},
		{"destination port", func(p *SPParams) { p.DstPort = ExactPortMatch(5000) }},
		{"if_id", func(p *SPParams) { p.IfID = 7 }},
		{"source prefix", func(p *SPParams) { p.Src = overlapTestNet(t, "10.2.0.0/25") }},
	} {
		other := gate
		tc.edit(&other)
		if PolicySelectorsEqual(protect, other) {
			t.Errorf("two policies that differ in %s are reported as one kernel policy", tc.name)
		}
	}
	wide := overlapTestNet(t, "0.0.0.0/0")
	if !PolicySelectorsEqual(SPParams{Dir: SADirOut}, SPParams{Src: wide, Dst: wide, Dir: SADirOut}) {
		t.Error("a policy with no prefix and one naming 0.0.0.0/0 on both sides are not one kernel policy")
	}
}
