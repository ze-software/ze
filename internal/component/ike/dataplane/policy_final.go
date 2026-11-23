// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Policy ownership, a PROTECT policy on the catch-all's selector
// RFC: rfc/short/rfc4301.md -- Section 4.4.1, the final entry of an SPD; Section 5, an unmatched packet
// Related: policy_owner.go -- the ledger these methods extend (policyOwners, claim, deleteThenRelease)
// Related: xfrm_linux.go -- InstallPolicy and RemovePolicyParams, the two callers

package dataplane

import (
	"fmt"
	"syscall"
)

// A peer that negotiates the wildcard of a family (0.0.0.0/0 to 0.0.0.0/0, any protocol,
// any port) installs PROTECT policies whose selector, direction and if_id are those of
// the final entry of the SPD they sit in: the node-wide catch-all at if_id 0, or the
// catch-all of the XFRM interface the peer is bound to. The kernel holds one policy per
// such identity, whatever the priority (xfrm_policy_insert_list), so the two cannot both
// be installed.
//
// RFC 4301 Section 4.4.1: "Every SPD SHOULD have a nominal, final entry that matches
// anything that is otherwise unmatched, and discards it."
// RFC 4301 Section 5: "If no policy is found in the SPD that matches a packet (for
// either inbound or outbound traffic), the packet MUST be discarded."
//
// While the PROTECT policy is live it is that lookup's final entry: it matches every
// packet the catch-all would, so no packet of that direction and family is unmatched,
// and none passes in clear, because a packet it matches leaves through the SA or
// arrives through it, or is dropped. That holds whether the catch-all discards or
// bypasses: under `unmatched bypass` the catch-all would pass only what no other entry
// matched, and the PROTECT policy leaves nothing for it. So the ledger:
//
//   - keeps the final entry it was asked for (policyOwners.finals) and leaves the
//     PROTECT policy in the kernel when the final entry is installed again
//     (yieldFinal), so a reload neither replaces the PROTECT policy nor fails;
//   - puts the final entry back when the owner removes its PROTECT policy
//     (reinstateFinal), as ONE kernel upsert that replaces the PROTECT policy, so at
//     no instant does the identity hold neither;
//   - reports the PROTECT policy's row as standing in for the final entry
//     (PolicyInfo.Displaced), so the missing catch-all row is explained.
//
// strongSwan reaches the same kernel state by another route: its kernel-netlink plugin
// keeps every policy requested for one selector and direction in a list sorted by
// priority and installs the first, and installs the next one when the first is deleted
// (src/libcharon/plugins/kernel_netlink/kernel_netlink_ipsec.c, add_policy and
// del_policy over policy_entry_t.used_by).

// DisplacedFinalEntry names the final entry of an SPD that a peer's PROTECT policy or an
// operator's wildcard entry stands in for: the entry Ze was asked to hold on the policy's
// own selector, which the kernel cannot hold beside it. Role is PolicyRoleUnspecified
// when the policy stands in for none, and Action is then meaningless.
type DisplacedFinalEntry struct {
	Role   PolicyRole
	Action SPAction
}

// Held reports whether the policy stands in for a final entry.
func (d DisplacedFinalEntry) Held() bool {
	return d.Role != PolicyRoleUnspecified
}

// errFinalEntryHolds answers a peer's or operator's delete at a selector where the kernel
// holds the SPD's final entry and neither a peer's policy nor an operator's entry does.
// It wraps ENOENT, which is the fact the kernel would report for a policy that is already
// gone, so a caller reads it as it reads a kernel delete of a missing policy.
var errFinalEntryHolds = fmt.Errorf("the SPD's final entry holds this selector and no peer's or operator's policy does: %w", syscall.ENOENT)

// finalEntry reports whether p is the final entry of an SPD: the node-wide catch-all or
// an XFRM interface's catch-all. The role decides, as it does for the ICMP yield
// (catchAllYieldsToICMPErrorCheck): an operator may write a wildcard entry of their
// own, and that entry is an order of theirs, never the nominal final entry.
func finalEntry(p SPParams) bool {
	if ownedEntry(p) {
		return false
	}
	return p.Role == PolicyRoleCatchAll || p.Role == PolicyRoleInterfaceCatchAll
}

// yieldFinal reports whether the final entry p must NOT be written to the kernel
// because an owner's PROTECT policy or an operator's wildcard entry holds p's selector,
// and records p as the entry to put back when that policy goes. The caller makes no
// kernel call and answers success: the lookup has its final entry, which is the holding
// policy.
//
// It answers false for every other case, and the caller then installs p through claim:
// no owner or operator entry holds the selector, or the holder is a template-free allow
// of a peer, which cannot stand in for a final entry and which claim refuses.
//
// The record replaces an earlier one, so a reload that changes `unmatched` while the
// holding policy is live changes the entry that is put back.
func (o *policyOwners) yieldFinal(p SPParams) bool {
	key := policyKey(p)

	o.mu.Lock()
	defer o.mu.Unlock()
	if held, existed := o.owners[key]; existed {
		if !held.protects {
			return false
		}
	} else if !o.operatorHeldLocked(key) {
		return false
	}
	if o.finals == nil {
		o.finals = make(map[policySelectorKey]SPParams)
	}
	o.finals[key] = p
	return true
}

// reinstateFinal handles an owner's or operator's delete of p at a selector Ze holds a
// final entry for. handled is false when it does not apply, and the caller then runs
// deleteThenRelease: no final entry is kept at p's selector, p is neither an owned entry
// nor an operator entry, a different owner holds it, or an operator entry holds what an
// owner tried to delete (deleteThenRelease refuses the last two).
//
// When p's owner (or an operator deleting its own wildcard entry) holds the selector,
// install is called with the kept final entry, and MUST upsert it: the kernel replaces
// the removed policy with the final entry in one request, so the lookup always holds one
// of the two. The owner record goes, and the label becomes the final entry's, only once
// install succeeded. On a failure the kernel still holds the previous policy, the record
// stays, and the error is returned: the caller keeps the policy as not removed and
// retries.
//
// When NO owner and NO operator entry holds the selector for an owned or operator delete,
// the kernel holds the final entry itself, so the requested policy is not there to
// delete, and a kernel delete would remove the final entry. install is not called, and
// errFinalEntryHolds (ENOENT) is returned.
//
// The lock spans install for the reason deleteThenRelease gives for del: check, kernel
// request and record are one step.
func (o *policyOwners) reinstateFinal(p SPParams, install func(final SPParams) error) (handled bool, err error) {
	owned := ownedEntry(p)
	if !owned && p.Role != PolicyRoleOperator {
		return false, nil
	}
	key := policyKey(p)

	o.mu.Lock()
	defer o.mu.Unlock()
	final, kept := o.finals[key]
	if !kept {
		return false, nil
	}
	held, existed := o.owners[key]
	if !owned {
		if existed {
			return false, nil
		}
		if !o.operatorHeldLocked(key) {
			return true, errFinalEntryHolds
		}
		if installErr := install(final); installErr != nil {
			return true, installErr
		}
		if o.labels == nil {
			o.labels = make(map[policySelectorKey]policyLabel)
		}
		o.labels[key] = policyLabel{role: final.Role, spd: final.SPD}
		return true, nil
	}
	if !existed {
		if o.operatorHeldLocked(key) {
			return false, nil
		}
		return true, errFinalEntryHolds
	}
	if held.owner != p.Owner {
		return false, nil
	}
	if installErr := install(final); installErr != nil {
		return true, installErr
	}
	delete(o.owners, key)
	if o.labels == nil {
		o.labels = make(map[policySelectorKey]policyLabel)
	}
	o.labels[key] = policyLabel{role: final.Role, spd: final.SPD}
	return true, nil
}
