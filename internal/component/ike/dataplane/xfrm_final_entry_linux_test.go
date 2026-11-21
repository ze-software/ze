// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Policy ownership, a PROTECT policy on the catch-all's selector
// Related: xfrm_linux.go -- InstallPolicy, RemovePolicyParams and ListPolicies, the entry points under test
// Related: policy_final.go -- the ledger methods they call
// Related: policy_final_test.go -- the same sequences on the ledger alone, on every OS
//
// No kernel is touched: the xfrmPolicyUpdate, xfrmPolicyDel and xfrmPolicyList seams are
// swapped for a map that keeps the kernel's own rule, one policy per selector, direction
// and if_id, replaced by an update whatever the priority (xfrm_policy_insert_list). So
// the tests need no netns and no CAP_NET_ADMIN, and they can read, after every call,
// what each selector holds, which a real kernel shows only between calls.

//go:build linux

package dataplane

import (
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"
)

// finalFakeKernel is the SPD of a kernel that holds one policy per identity. updates
// and deletes count the requests that reached it; a delete empties its selector, which
// is the instant a packet would meet that lookup with no entry on it.
type finalFakeKernel struct {
	policies map[policySelectorKey]netlink.XfrmPolicy
	updates  int
	deletes  int
}

// finalFakeKernelKey is the identity the fake keeps a policy under: the fields
// policyInfoFromKernel reads back into the ledger's key.
func finalFakeKernelKey(p *netlink.XfrmPolicy) policySelectorKey {
	dir, _ := policyDirFromKernel(p.Dir)
	return policyKey(SPParams{
		Src: p.Src, Dst: p.Dst, Dir: dir, UpperProto: uint8(p.Proto),
		SrcPort: PortMatch{Port: uint16(p.SrcPort), Mask: p.SrcPortMask},
		DstPort: PortMatch{Port: uint16(p.DstPort), Mask: p.DstPortMask},
		IfIndex: p.Ifindex, IfID: uint32(p.Ifid),
	})
}

// newFinalFakeKernel swaps the three kernel calls for the map, for one test.
func newFinalFakeKernel(t *testing.T) *finalFakeKernel {
	t.Helper()
	k := &finalFakeKernel{policies: map[policySelectorKey]netlink.XfrmPolicy{}}
	savedUpdate, savedDel, savedList := xfrmPolicyUpdate, xfrmPolicyDel, xfrmPolicyList
	xfrmPolicyUpdate = func(p *netlink.XfrmPolicy) error {
		k.updates++
		k.policies[finalFakeKernelKey(p)] = *p
		return nil
	}
	xfrmPolicyDel = func(p *netlink.XfrmPolicy) error {
		key := finalFakeKernelKey(p)
		if _, held := k.policies[key]; !held {
			return syscall.ENOENT
		}
		k.deletes++
		delete(k.policies, key)
		return nil
	}
	xfrmPolicyList = func(int) ([]netlink.XfrmPolicy, error) {
		out := make([]netlink.XfrmPolicy, 0, len(k.policies))
		for key := range k.policies {
			out = append(out, k.policies[key])
		}
		return out, nil
	}
	t.Cleanup(func() { xfrmPolicyUpdate, xfrmPolicyDel, xfrmPolicyList = savedUpdate, savedDel, savedList })
	return k
}

// finalCatchAll builds the IPv4 node-wide catch-all's IN and OUT entries as
// engine/unmatched.go spdCatchAllEntries does. FWD is left out, as installUnmatchedEntries
// leaves it out on this backend: the namespace default closes the forward lookup.
func finalCatchAll(action SPAction) []SPParams {
	wide := &net.IPNet{IP: net.IPv4zero.To4(), Mask: net.CIDRMask(0, 32)}
	out := make([]SPParams, 0, 2)
	for _, dir := range []SADir{SADirIn, SADirOut} {
		out = append(out, SPParams{
			Src: wide, Dst: wide, Dir: dir, Action: action, Role: PolicyRoleCatchAll,
			Priority: PriorityUnmatched, SrcPort: AnyPortMatch(), DstPort: AnyPortMatch(),
		})
	}
	return out
}

// finalWildcardPeer builds the IN and OUT PROTECT policies of a tunnel-mode peer that
// negotiated 0.0.0.0/0 to 0.0.0.0/0 at if_id 0, as engine/child.go childPairPolicy does.
func finalWildcardPeer(owner string) []SPParams {
	wide := &net.IPNet{IP: net.IPv4zero.To4(), Mask: net.CIDRMask(0, 32)}
	local, remote := net.ParseIP("192.0.2.10"), net.ParseIP("192.0.2.20")
	return []SPParams{
		{
			Src: wide, Dst: wide, Dir: SADirIn, Action: SPActionProtect, Owner: owner, Role: PolicyRoleChildSA,
			Proto: ProtoESP, Mode: ModeTunnel, ReqID: 0x0d31, Priority: PriorityChildSA,
			TunnelSrc: remote, TunnelDst: local, SrcPort: AnyPortMatch(), DstPort: AnyPortMatch(),
		},
		{
			Src: wide, Dst: wide, Dir: SADirOut, Action: SPActionProtect, Owner: owner, Role: PolicyRoleChildSA,
			Proto: ProtoESP, Mode: ModeTunnel, ReqID: 0x0d31, Priority: PriorityChildSA,
			TunnelSrc: local, TunnelDst: remote, SrcPort: AnyPortMatch(), DstPort: AnyPortMatch(),
		},
	}
}

// finalWantRow fails the test unless the listing, read through ListPolicies as
// `show vpn ipsec dataplane policy` reads it, holds exactly one row on want's selector
// and that row has the action, role and displaced final entry given.
func finalWantRow(t *testing.T, b *xfrmBackend, step string, want *SPParams, action SPAction, role PolicyRole, displaced DisplacedFinalEntry) {
	t.Helper()
	rows, err := b.ListPolicies()
	if err != nil {
		t.Fatalf("%s: list the SPD: %v", step, err)
	}
	found := 0
	for i := range rows {
		row := &rows[i]
		held := SPParams{
			Src: row.Src, Dst: row.Dst, Dir: row.Dir, UpperProto: row.UpperProto,
			SrcPort: row.SrcPort, DstPort: row.DstPort, IfIndex: row.IfIndex, IfID: row.IfID,
		}
		if !PolicySelectorsEqual(held, *want) {
			continue
		}
		found++
		if row.Action != action || row.Role != role || row.Displaced != displaced {
			t.Fatalf("%s: direction %d holds action %v, role %v, displaced %+v; want action %v, role %v, displaced %+v",
				step, want.Dir, row.Action, row.Role, row.Displaced, action, role, displaced)
		}
	}
	if found != 1 {
		t.Fatalf("%s: direction %d holds %d policies on the wildcard selector, want 1: "+
			"with none, a packet no other entry matches passes in the clear", step, want.Dir, found)
	}
}

// TestReloadAndTeardownWithAWildcardPeerKeepAFinalEntry proves the lifetime of a peer
// that negotiates 0.0.0.0/0 to 0.0.0.0/0 beside the catch-all, through the backend's
// entry points: the catch-all at start, the peer's PROTECT policies over it, two reloads
// that re-assert the catch-all (the second with `unmatched` changed to bypass), and the
// peer's teardown.
//
// RFC 4301 Section 5: "If no policy is found in the SPD that matches a packet (for
// either inbound or outbound traffic), the packet MUST be discarded." The kernel passes
// a packet no policy matches, so each of IN and OUT must hold a policy on the wildcard
// selector after every call: the catch-all, or the PROTECT policy standing in for it.
//
// Method: the fake kernel above. After each step the listing is read for both
// directions. The reloads must succeed and send the kernel nothing. The teardown must
// send no delete at all, since a delete followed by an install leaves the selector empty
// between the two, and must leave the catch-all with the disposition the last reload
// asked for.
//
// MUTATION: removing the `finalEntry(p) && b.policies.yieldFinal(p)` branch of
// InstallPolicy makes the first reload answer PolicyOwnedError, the failure every reload
// had while such a peer was up. Removing the reinstateFinal call of RemovePolicyParams
// makes the teardown delete the PROTECT policies and leave both directions empty: the
// delete count is 2 and the last read finds no policy.
func TestReloadAndTeardownWithAWildcardPeerKeepAFinalEntry(t *testing.T) {
	kernel := newFinalFakeKernel(t)
	b := &xfrmBackend{}
	discard, bypass := finalCatchAll(SPActionDiscard), finalCatchAll(SPActionBypass)
	peer := finalWildcardPeer("site-a")
	none := DisplacedFinalEntry{}

	for i := range discard {
		if err := b.InstallPolicy(discard[i]); err != nil {
			t.Fatalf("start: install the catch-all (direction %d): %v", discard[i].Dir, err)
		}
		finalWantRow(t, b, "start", &discard[i], SPActionDiscard, PolicyRoleCatchAll, none)
	}

	for i := range peer {
		if err := b.InstallPolicy(peer[i]); err != nil {
			t.Fatalf("peer up: install the PROTECT policy (direction %d): %v", peer[i].Dir, err)
		}
	}
	for i := range peer {
		finalWantRow(t, b, "peer up", &peer[i], SPActionProtect, PolicyRoleChildSA,
			DisplacedFinalEntry{Role: PolicyRoleCatchAll, Action: SPActionDiscard})
	}

	// The first reload re-asserts the catch-all unchanged.
	updates := kernel.updates
	for i := range discard {
		if err := b.InstallPolicy(discard[i]); err != nil {
			t.Fatalf("reload: the catch-all (direction %d) answered %v; want success, because the peer's PROTECT policy is that lookup's final entry",
				discard[i].Dir, err)
		}
	}
	if kernel.updates != updates {
		t.Fatalf("reload: %d policy updates reached the kernel, want 0: an update would replace the PROTECT policy with the catch-all",
			kernel.updates-updates)
	}
	for i := range peer {
		finalWantRow(t, b, "reload", &peer[i], SPActionProtect, PolicyRoleChildSA,
			DisplacedFinalEntry{Role: PolicyRoleCatchAll, Action: SPActionDiscard})
	}

	// The second reload changes `unmatched` to bypass.
	for i := range bypass {
		if err := b.InstallPolicy(bypass[i]); err != nil {
			t.Fatalf("reload to bypass: the catch-all (direction %d) answered %v", bypass[i].Dir, err)
		}
	}
	for i := range peer {
		finalWantRow(t, b, "reload to bypass", &peer[i], SPActionProtect, PolicyRoleChildSA,
			DisplacedFinalEntry{Role: PolicyRoleCatchAll, Action: SPActionBypass})
	}

	// The teardown, one direction at a time, reading both after each.
	for i := range peer {
		if err := b.RemovePolicyParams(peer[i]); err != nil {
			t.Fatalf("teardown: remove the PROTECT policy (direction %d): %v", peer[i].Dir, err)
		}
		finalWantRow(t, b, "teardown", &peer[i], SPActionBypass, PolicyRoleCatchAll, none)
	}
	if kernel.deletes != 0 {
		t.Fatalf("teardown: %d policy deletes reached the kernel, want 0: the catch-all replaces the PROTECT policy in one update, "+
			"and a delete leaves the selector with no policy until the next install", kernel.deletes)
	}
	if _, known := b.policies.ownerOf(peer[0]); known {
		t.Fatal("teardown: the peer still owns the inbound wildcard selector")
	}

	// The catch-all is an ordinary entry again: a reload writes it.
	updates = kernel.updates
	for i := range discard {
		if err := b.InstallPolicy(discard[i]); err != nil {
			t.Fatalf("reload after the teardown: the catch-all (direction %d): %v", discard[i].Dir, err)
		}
		finalWantRow(t, b, "reload after the teardown", &discard[i], SPActionDiscard, PolicyRoleCatchAll, none)
	}
	if kernel.updates != updates+len(discard) {
		t.Fatalf("reload after the teardown: %d updates reached the kernel, want %d", kernel.updates-updates, len(discard))
	}
}

// TestWildcardPeerBeforeTheCatchAllKeepsAFinalEntry proves the other order: the peer's
// PROTECT policies are in the kernel before the catch-all is first asked for, as when
// an address event or a reload brings the catch-all after the peer established.
//
// Method: the fake kernel. The peer installs first, then the catch-all discard is
// asked for and must be kept without reaching the kernel; a second peer's install and
// removal on the selector are refused; the peer's own teardown leaves the discard, and
// a removal it repeats finds the catch-all and must not delete it.
//
// MUTATION: the same two as TestReloadAndTeardownWithAWildcardPeerKeepAFinalEntry: with
// no yield the catch-all's install answers PolicyOwnedError, and with no reinstatement
// the teardown leaves both directions empty.
func TestWildcardPeerBeforeTheCatchAllKeepsAFinalEntry(t *testing.T) {
	kernel := newFinalFakeKernel(t)
	b := &xfrmBackend{}
	discard := finalCatchAll(SPActionDiscard)
	peer := finalWildcardPeer("site-a")
	none := DisplacedFinalEntry{}

	for i := range peer {
		if err := b.InstallPolicy(peer[i]); err != nil {
			t.Fatalf("peer up: install the PROTECT policy (direction %d): %v", peer[i].Dir, err)
		}
		finalWantRow(t, b, "peer up", &peer[i], SPActionProtect, PolicyRoleChildSA, none)
	}

	updates := kernel.updates
	for i := range discard {
		if err := b.InstallPolicy(discard[i]); err != nil {
			t.Fatalf("the catch-all (direction %d) under a live wildcard peer answered %v, want success", discard[i].Dir, err)
		}
	}
	if kernel.updates != updates {
		t.Fatalf("%d policy updates reached the kernel for the catch-all, want 0", kernel.updates-updates)
	}
	for i := range peer {
		finalWantRow(t, b, "catch-all asked for", &peer[i], SPActionProtect, PolicyRoleChildSA,
			DisplacedFinalEntry{Role: PolicyRoleCatchAll, Action: SPActionDiscard})
	}

	// A second peer on the same selector is still refused, install and removal both.
	foreign := finalWildcardPeer("site-b")
	var owned *PolicyOwnedError
	if err := b.InstallPolicy(foreign[1]); !errors.As(err, &owned) {
		t.Fatalf("a second peer's install on the wildcard answered %v, want PolicyOwnedError", err)
	}
	if err := b.RemovePolicyParams(foreign[1]); !errors.As(err, &owned) {
		t.Fatalf("a second peer's removal on the wildcard answered %v, want PolicyOwnedError", err)
	}
	finalWantRow(t, b, "after the second peer was refused", &peer[1], SPActionProtect, PolicyRoleChildSA,
		DisplacedFinalEntry{Role: PolicyRoleCatchAll, Action: SPActionDiscard})

	for i := range peer {
		if err := b.RemovePolicyParams(peer[i]); err != nil {
			t.Fatalf("teardown: remove the PROTECT policy (direction %d): %v", peer[i].Dir, err)
		}
		finalWantRow(t, b, "teardown", &peer[i], SPActionDiscard, PolicyRoleCatchAll, none)
	}
	if kernel.deletes != 0 {
		t.Fatalf("teardown: %d policy deletes reached the kernel, want 0", kernel.deletes)
	}

	// A removal the peer repeats finds the catch-all, not its policy, and must not delete it.
	err := b.RemovePolicyParams(peer[1])
	if !errors.Is(err, syscall.ENOENT) {
		t.Fatalf("the peer's repeated removal answered %v, want ENOENT: its policy is gone and the catch-all holds the selector", err)
	}
	finalWantRow(t, b, "after the repeated removal", &peer[1], SPActionDiscard, PolicyRoleCatchAll, none)
	if kernel.deletes != 0 {
		t.Fatalf("the repeated removal sent %d deletes, want 0: it would remove the catch-all", kernel.deletes)
	}
}
