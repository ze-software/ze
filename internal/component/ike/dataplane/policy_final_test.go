// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Policy ownership, a PROTECT policy on the catch-all's selector
// Related: policy_final.go -- yieldFinal and reinstateFinal, the methods under test
// Related: policy_owner.go -- claim, label, annotate and deleteThenRelease, which they extend
// Related: xfrm_final_entry_linux_test.go -- the same sequences through InstallPolicy and RemovePolicyParams

package dataplane

import (
	"errors"
	"syscall"
	"testing"
)

// finalTestPair answers the two entries that share one kernel identity when a peer
// negotiates the IPv4 wildcard at if_id 0: the node-wide catch-all's OUT entry, as
// engine/unmatched.go spdCatchAllEntries builds it, and the peer's OUT PROTECT policy.
func finalTestPair(t *testing.T, action SPAction) (catchAll, protect SPParams) {
	t.Helper()
	wide := overlapTestNet(t, "0.0.0.0/0")
	catchAll = SPParams{
		Src: wide, Dst: wide, Dir: SADirOut, Action: action, Role: PolicyRoleCatchAll,
		Priority: PriorityUnmatched, SrcPort: AnyPortMatch(), DstPort: AnyPortMatch(),
	}
	protect = SPParams{
		Src: wide, Dst: wide, Dir: SADirOut, Action: SPActionProtect, Mode: ModeTunnel,
		Owner: "peer-a", Role: PolicyRoleChildSA, Priority: PriorityChildSA,
		SrcPort: AnyPortMatch(), DstPort: AnyPortMatch(),
	}
	if !PolicySelectorsEqual(catchAll, protect) {
		t.Fatal("the catch-all and the wildcard peer's PROTECT policy are not one kernel policy; the test does not hold the collision")
	}
	return catchAll, protect
}

// finalTestDisplaced answers what the ledger reports for the kernel row on p's selector.
func finalTestDisplaced(o *policyOwners, p SPParams) PolicyInfo {
	info := PolicyInfo{
		Src: p.Src, Dst: p.Dst, Dir: p.Dir, UpperProto: p.UpperProto,
		SrcPort: p.SrcPort, DstPort: p.DstPort, IfIndex: p.IfIndex, IfID: p.IfID,
	}
	o.annotate(&info)
	return info
}

// TestFinalEntryYieldsToALivePeerProtectPolicy proves a reload's re-assertion of the
// catch-all, on the selector a wildcard peer's PROTECT policy holds, is neither written
// nor refused, and that the PROTECT policy's removal puts the catch-all back with one
// install and no delete.
//
// Method: the ledger calls InstallPolicy and RemovePolicyParams make, in the order a
// running daemon produces them: the catch-all at start (claim, label), the peer's
// PROTECT policy over it (claim, label), a reload (yieldFinal), the teardown
// (reinstateFinal). install records every entry it is handed. The catch-all discards.
//
// MUTATION: making yieldFinal answer false sends the reload to claim, which answers
// PolicyOwnedError, the refusal that failed every reload; the "reload" step fails.
// Making reinstateFinal answer (false, nil) sends the teardown to deleteThenRelease,
// which deletes the selector's only policy; the "install ran" step fails.
func TestFinalEntryYieldsToALivePeerProtectPolicy(t *testing.T) {
	catchAll, protect := finalTestPair(t, SPActionDiscard)

	var o policyOwners
	if _, err := o.claim(catchAll); err != nil {
		t.Fatalf("claim(catch-all) at start = %v", err)
	}
	o.label(catchAll)
	if created, err := o.claim(protect); err != nil || !created {
		t.Fatalf("claim(peer-a wildcard PROTECT over the catch-all) = %v, %v; want a created record", created, err)
	}
	o.label(protect)

	// The kernel row is now the PROTECT policy, and it says what it stands in for.
	row := finalTestDisplaced(&o, protect)
	if row.Role != PolicyRoleChildSA || row.Owner != "peer-a" {
		t.Fatalf("the row on the shared selector is role %v owner %q, want peer-a's Child SA policy", row.Role, row.Owner)
	}
	if !row.Displaced.Held() || row.Displaced.Role != PolicyRoleCatchAll || row.Displaced.Action != SPActionDiscard {
		t.Fatalf("the PROTECT row reports displaced %+v, want the catch-all with its discard: the listing holds no catch-all row to explain itself",
			row.Displaced)
	}

	// The reload: the catch-all is asked for again.
	if !o.yieldFinal(catchAll) {
		t.Fatal("reload: yieldFinal(catch-all) = false on a selector peer-a's PROTECT policy holds; " +
			"the catch-all would go to claim and the reload would be refused")
	}
	if owner, known := o.ownerOf(protect); !known || owner != "peer-a" {
		t.Fatalf("reload: the selector's owner is %q (known %v), want peer-a", owner, known)
	}

	// The teardown: one install of the catch-all, the upsert that replaces the policy.
	var installed []SPParams
	handled, err := o.reinstateFinal(protect, func(final SPParams) error {
		installed = append(installed, final)
		return nil
	})
	if !handled || err != nil {
		t.Fatalf("teardown: reinstateFinal(peer-a PROTECT) = %v, %v; want it handled", handled, err)
	}
	if len(installed) != 1 || installed[0].Role != PolicyRoleCatchAll || installed[0].Action != SPActionDiscard ||
		installed[0].Priority != PriorityUnmatched || !PolicySelectorsEqual(installed[0], catchAll) {
		t.Fatalf("teardown: install ran with %+v, want exactly the catch-all discard on the selector: "+
			"without it the lookup is left with neither entry", installed)
	}
	if _, known := o.ownerOf(protect); known {
		t.Fatal("teardown: peer-a still owns the selector after its policy was replaced")
	}
	row = finalTestDisplaced(&o, catchAll)
	if row.Role != PolicyRoleCatchAll || row.OwnerKnown || row.Displaced.Held() {
		t.Fatalf("after the teardown the row is role %v, owner known %v, displaced %+v; want the plain catch-all",
			row.Role, row.OwnerKnown, row.Displaced)
	}

	// With the peer gone the catch-all is an ordinary install again.
	if o.yieldFinal(catchAll) {
		t.Fatal("yieldFinal(catch-all) = true with no owner on the selector: the catch-all would never be written")
	}
	if _, err := o.claim(catchAll); err != nil {
		t.Fatalf("claim(catch-all) once peer-a left = %v", err)
	}
}

// TestFinalEntryAskedForUnderALivePeerComesBackAsLastAsked proves the other order, the
// PROTECT policy first and the catch-all asked for afterwards, and that the entry put
// back is the one the LAST reload asked for.
//
// Method: the peer claims the wildcard with no catch-all recorded; the catch-all is
// then asked for as a discard and, on a second reload, as a bypass (`unmatched`
// changed). The row must name the bypass, and the teardown must install the bypass.
//
// MUTATION: recording the final entry only when none is recorded yet (dropping the
// overwrite in yieldFinal) puts the discard back, and the last step fails.
func TestFinalEntryAskedForUnderALivePeerComesBackAsLastAsked(t *testing.T) {
	discard, protect := finalTestPair(t, SPActionDiscard)
	bypass, _ := finalTestPair(t, SPActionBypass)

	var o policyOwners
	if created, err := o.claim(protect); err != nil || !created {
		t.Fatalf("claim(peer-a wildcard PROTECT) = %v, %v; want a created record", created, err)
	}
	o.label(protect)
	if row := finalTestDisplaced(&o, protect); row.Displaced.Held() {
		t.Fatalf("the PROTECT row reports displaced %+v before any catch-all was asked for", row.Displaced)
	}

	if !o.yieldFinal(discard) {
		t.Fatal("yieldFinal(catch-all discard) = false under peer-a's PROTECT policy")
	}
	if !o.yieldFinal(bypass) {
		t.Fatal("yieldFinal(catch-all bypass) = false under peer-a's PROTECT policy")
	}
	if row := finalTestDisplaced(&o, protect); row.Displaced.Role != PolicyRoleCatchAll || row.Displaced.Action != SPActionBypass {
		t.Fatalf("the PROTECT row reports displaced %+v, want the catch-all as last asked for, a bypass", row.Displaced)
	}

	var installed []SPParams
	handled, err := o.reinstateFinal(protect, func(final SPParams) error {
		installed = append(installed, final)
		return nil
	})
	if !handled || err != nil {
		t.Fatalf("reinstateFinal(peer-a PROTECT) = %v, %v; want it handled", handled, err)
	}
	if len(installed) != 1 || installed[0].Action != SPActionBypass {
		t.Fatalf("install ran with %+v, want the bypass the last reload asked for", installed)
	}
}

// TestFinalEntryIsNeverLostToAFailedOrForeignRemoval proves the three removals that
// must leave the selector's policy in the kernel.
//
// Method: one ledger per case, each with the catch-all recorded.
//   - The kernel refuses the catch-all's install: the PROTECT policy is still there, so
//     the owner record stays and the error is returned, and a retry succeeds.
//   - Another peer asks: reinstateFinal does not apply, and deleteThenRelease refuses
//     before the kernel.
//   - The catch-all itself holds the selector (no owner): install is not called and the
//     answer is ENOENT, where a kernel delete would have removed the catch-all.
//
// MUTATION: dropping the owner record before install in reinstateFinal fails the first
// case at "still owns". Removing the `!existed` arm of reinstateFinal sends the third
// case to deleteThenRelease, whose delete runs.
func TestFinalEntryIsNeverLostToAFailedOrForeignRemoval(t *testing.T) {
	catchAll, protect := finalTestPair(t, SPActionDiscard)

	t.Run("the kernel refuses the catch-all", func(t *testing.T) {
		var o policyOwners
		if _, err := o.claim(protect); err != nil {
			t.Fatalf("claim: %v", err)
		}
		o.label(protect)
		if !o.yieldFinal(catchAll) {
			t.Fatal("yieldFinal(catch-all) = false")
		}
		handled, err := o.reinstateFinal(protect, func(SPParams) error { return syscall.EPERM })
		if !handled || !errors.Is(err, syscall.EPERM) {
			t.Fatalf("reinstateFinal with a refusing kernel = %v, %v; want handled with the kernel's EPERM", handled, err)
		}
		if owner, known := o.ownerOf(protect); !known || owner != "peer-a" {
			t.Fatalf("owner after the refused install = %q (known %v), want peer-a still owns: the kernel still holds its policy", owner, known)
		}
		if row := finalTestDisplaced(&o, protect); row.Role != PolicyRoleChildSA || !row.Displaced.Held() {
			t.Fatalf("the row after the refused install is role %v displaced %+v, want the PROTECT policy still standing in", row.Role, row.Displaced)
		}
		handled, err = o.reinstateFinal(protect, func(SPParams) error { return nil })
		if !handled || err != nil {
			t.Fatalf("the retry = %v, %v; want it handled", handled, err)
		}
	})

	t.Run("another peer asks", func(t *testing.T) {
		var o policyOwners
		if _, err := o.claim(protect); err != nil {
			t.Fatalf("claim: %v", err)
		}
		o.label(protect)
		if !o.yieldFinal(catchAll) {
			t.Fatal("yieldFinal(catch-all) = false")
		}
		foreign := protect
		foreign.Owner = "peer-b"
		ran := false
		handled, err := o.reinstateFinal(foreign, func(SPParams) error { ran = true; return nil })
		if handled || err != nil || ran {
			t.Fatalf("reinstateFinal(peer-b on peer-a's selector) = %v, %v (install ran %v); want it left to deleteThenRelease", handled, err, ran)
		}
		var owned *PolicyOwnedError
		if err := o.deleteThenRelease(foreign, func() error { ran = true; return nil }); !errors.As(err, &owned) || ran {
			t.Fatalf("peer-b delete on peer-a's selector = %v (delete ran %v), want a refusal before the kernel", err, ran)
		}
	})

	t.Run("the catch-all itself holds the selector", func(t *testing.T) {
		var o policyOwners
		if _, err := o.claim(catchAll); err != nil {
			t.Fatalf("claim: %v", err)
		}
		o.label(catchAll)
		ran := false
		handled, err := o.reinstateFinal(protect, func(SPParams) error { ran = true; return nil })
		if !handled || !errors.Is(err, syscall.ENOENT) || ran {
			t.Fatalf("peer-a delete where only the catch-all is installed = %v, %v (install ran %v); "+
				"want handled with ENOENT and no kernel call: a delete would remove the catch-all", handled, err, ran)
		}
		if row := finalTestDisplaced(&o, catchAll); row.Role != PolicyRoleCatchAll {
			t.Fatalf("the row is role %v after the refused delete, want the catch-all", row.Role)
		}
	})
}

// TestFinalEntryYieldsOnlyToAProtectPolicy proves the yield is tied to its condition: a
// final entry gives way to a PROTECT policy and to nothing else, and only a final entry
// gives way.
//
// Method: three cases just outside the yield.
//   - A peer's template-free allow on a selector a final entry holds is refused: it
//     would replace a discard of everything with a bypass of everything.
//   - A peer's template-free allow that holds the selector first does not make the
//     catch-all yield, and claim refuses the catch-all, as it does every node-wide entry.
//   - An operator entry on the wildcard is not a final entry (finalEntry), so the
//     backend sends it to claim, which refuses it at an owner's selector.
//
// MUTATION: removing the `held.protects` test of yieldFinal makes the second case
// yield to the allow, and that step fails.
func TestFinalEntryYieldsOnlyToAProtectPolicy(t *testing.T) {
	catchAll, protect := finalTestPair(t, SPActionDiscard)
	allow := protect
	allow.Action, allow.Mode = SPActionBypass, 0

	var o policyOwners
	if _, err := o.claim(catchAll); err != nil {
		t.Fatalf("claim(catch-all) = %v", err)
	}
	o.label(catchAll)
	var owned *PolicyOwnedError
	if created, err := o.claim(allow); !errors.As(err, &owned) || created {
		t.Fatalf("claim(peer-a template-free allow on the catch-all's selector) = %v, %v; want PolicyOwnedError", created, err)
	}
	if owned.HeldRole != PolicyRoleCatchAll || owned.Wanted != "peer-a" {
		t.Fatalf("the refusal names holder role %v and wanted %q, want the catch-all and peer-a", owned.HeldRole, owned.Wanted)
	}

	var first policyOwners
	if created, err := first.claim(allow); err != nil || !created {
		t.Fatalf("claim(peer-a template-free allow, no final entry recorded) = %v, %v; want a created record", created, err)
	}
	if first.yieldFinal(catchAll) {
		t.Fatal("yieldFinal(catch-all) = true under a template-free allow: the lookup would have a bypass of everything as its final entry")
	}
	if _, err := first.claim(catchAll); !errors.As(err, &owned) {
		t.Fatalf("claim(catch-all on the allow's selector) = %v, want PolicyOwnedError", err)
	}

	operator := catchAll
	operator.Role, operator.Priority = PolicyRoleOperator, 500
	if finalEntry(operator) {
		t.Fatal("an operator entry on the wildcard is read as an SPD's final entry; it would yield where it must be refused")
	}
	if !finalEntry(catchAll) {
		t.Fatal("the node-wide catch-all is not read as a final entry")
	}
	interfaceCatchAll := catchAll
	interfaceCatchAll.Role, interfaceCatchAll.IfID = PolicyRoleInterfaceCatchAll, 7
	if !finalEntry(interfaceCatchAll) {
		t.Fatal("an XFRM interface's catch-all is not read as a final entry")
	}
	if finalEntry(protect) {
		t.Fatal("a PROTECT policy is read as a final entry")
	}
}

// TestFinalEntryRemovedUnderALivePeerIsNotPutBack proves the catch-all's own removal,
// while a PROTECT policy stands in for it, sends no kernel delete (it would remove the
// PROTECT policy) and ends the stand-in: the peer's later removal is a plain delete.
//
// Method: PROTECT policy, catch-all asked for, catch-all removed, peer removed. The
// delete callback of each step records whether it ran.
//
// MUTATION: keeping the finals record in the catch-all's deleteThenRelease makes the
// peer's removal reinstate a catch-all nobody holds any more, and the last step fails.
func TestFinalEntryRemovedUnderALivePeerIsNotPutBack(t *testing.T) {
	catchAll, protect := finalTestPair(t, SPActionDiscard)

	var o policyOwners
	if _, err := o.claim(protect); err != nil {
		t.Fatalf("claim: %v", err)
	}
	o.label(protect)
	if !o.yieldFinal(catchAll) {
		t.Fatal("yieldFinal(catch-all) = false")
	}

	ran := false
	if err := o.deleteThenRelease(catchAll, func() error { ran = true; return nil }); err != nil || ran {
		t.Fatalf("delete of the displaced catch-all = %v (delete ran %v), want success with no kernel call: "+
			"the kernel holds the PROTECT policy on that selector", err, ran)
	}
	if row := finalTestDisplaced(&o, protect); row.Displaced.Held() {
		t.Fatalf("the PROTECT row still reports displaced %+v after the catch-all was removed", row.Displaced)
	}

	handled, err := o.reinstateFinal(protect, func(SPParams) error { return nil })
	if handled || err != nil {
		t.Fatalf("reinstateFinal(peer-a PROTECT) once the catch-all was removed = %v, %v; want it left to a plain delete", handled, err)
	}
	if err := o.deleteThenRelease(protect, func() error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("peer-a delete of its PROTECT policy = %v (delete ran %v), want the kernel delete", err, ran)
	}
}
