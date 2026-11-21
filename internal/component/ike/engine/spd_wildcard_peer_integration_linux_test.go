//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Policy ownership, a PROTECT policy on the catch-all's selector
// Related: unmatched.go -- installUnmatched, the catch-all a reload re-asserts
// Related: child.go -- createFirstChildSA and removeChildSA, the peer's install and teardown
// Related: ../dataplane/policy_final.go -- the ledger methods whose effect this reads in the kernel
// Related: rfc4301_espform_policy_integration_linux_test.go -- newEspPolLab and espPolSPD, the lab
// Related: spd_peer_selector_integration_linux_test.go -- peerSelectorKernelPolicy, the kernel read

package engine

import (
	"net"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// wildcardPeerChild installs one tunnel-mode Child SA whose negotiated pair is the IPv4
// wildcard on both sides, any protocol and any port, at if_id 0: the Child SA of a
// policy-based peer that carries all traffic. It is espPolChild with that pair, over the
// XFRM backend espPolSPD loaded.
func wildcardPeerChild(t *testing.T) *ChildSA {
	t.Helper()
	natt := mbTransport(t, net.JoinHostPort(espPolZeOuter, strconv.Itoa(transport.NATTPort)), true)
	if err := transport.EnableESPInUDP(natt.Conn()); err != nil {
		t.Fatalf("set UDP_ENCAP_ESPINUDP on the NAT-T socket: %v", err)
	}

	sa := testSA()
	sa.IsInitiator = true
	sa.bindSockets(nil, natt)
	sa.floatToNATTPort()
	sa.NATDetected = false
	sa.peerEndpoint = &net.UDPAddr{IP: net.ParseIP(espPolPeerOuter), Port: transport.NATTPort}
	wide := mustCIDRNet(t, "0.0.0.0/0")
	sa.NegotiatedTSi, sa.NegotiatedTSr = wide, wide
	sa.NegotiatedPairs = []tsPair{{
		I: tsSelector{Net: wide, Port: ipsec.AnyPort()},
		R: tsSelector{Net: wide, Port: ipsec.AnyPort()},
	}}
	group := ipsec.ESPGroup{
		Name:      "esp-gcm",
		Lifetime:  3600,
		PFS:       ipsec.PFSDisable,
		Proposals: []ipsec.ESPProposal{{Number: 1, Encryption: ipsec.EncryptionAES256GCM}},
	}
	child, err := createFirstChildSA(sa, group, espPolZeOuter, espPolPeerOuter, 0, dataplane.Get(), slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA for the wildcard pair: %v", err)
	}
	t.Cleanup(child.Clear)
	if !child.ESPInstalled {
		t.Fatal("the wildcard Child SA was not installed into XFRM")
	}
	return child
}

// wildcardWantCatchAll fails the test unless the kernel holds the node-wide catch-all
// itself on want's selector: a DISCARD at the catch-all's rank, listed with the
// catch-all role, no owner and no displaced final entry, and on the inbound entry the
// ICMP yield spdCatchAllEntries' discard carries.
func wildcardWantCatchAll(t *testing.T, step string, want *dataplane.SPParams) {
	t.Helper()
	got := peerSelectorKernelPolicy(t, want)
	if got.Action != dataplane.SPActionDiscard || got.Priority != dataplane.PriorityUnmatched {
		t.Fatalf("%s: the kernel holds action %v at rank %d on the %s wildcard selector, want the catch-all DISCARD at rank %d",
			step, got.Action, got.Priority, dirLabel(want.Dir), dataplane.PriorityUnmatched)
	}
	if got.Role != dataplane.PolicyRoleCatchAll || got.OwnerKnown || got.Displaced.Held() {
		t.Fatalf("%s: the %s wildcard policy is listed with role %v, owner known %v, displaced %+v; want the plain catch-all",
			step, dirLabel(want.Dir), got.Role, got.OwnerKnown, got.Displaced)
	}
	if want.Dir == dataplane.SADirIn && !got.ICMPErrorYield {
		t.Fatalf("%s: the inbound catch-all does not carry the ICMP yield, so it is not the entry installUnmatched builds", step)
	}
}

// wildcardWantProtect fails the test unless the kernel holds the peer's PROTECT policy
// on want's selector, listed as standing in for the catch-all with the disposition
// given.
func wildcardWantProtect(t *testing.T, step string, want *dataplane.SPParams, displaced dataplane.SPAction) {
	t.Helper()
	got := peerSelectorKernelPolicy(t, want)
	if got.Action != dataplane.SPActionProtect || got.Priority != want.Priority {
		t.Fatalf("%s: the kernel holds action %v at rank %d on the %s wildcard selector, want the peer's PROTECT policy at rank %d: "+
			"the traffic the peer protects would be dropped or leave in clear", step, got.Action, got.Priority, dirLabel(want.Dir), want.Priority)
	}
	if got.Role != dataplane.PolicyRoleChildSA || !got.OwnerKnown || got.Owner != want.Owner {
		t.Fatalf("%s: the %s wildcard policy is listed with role %v and owner %q (known %v), want the Child SA of %q",
			step, dirLabel(want.Dir), got.Role, got.Owner, got.OwnerKnown, want.Owner)
	}
	if got.Displaced.Role != dataplane.PolicyRoleCatchAll || got.Displaced.Action != displaced {
		t.Fatalf("%s: the %s PROTECT row reports displaced %+v, want the catch-all with action %v: "+
			"the listing holds no catch-all row there and nothing else says why", step, dirLabel(want.Dir), got.Displaced, displaced)
	}
}

// TestWildcardPeerStandsInForTheCatchAllAndGivesItBack proves, in the kernel, the
// lifetime of a peer that negotiates 0.0.0.0/0 to 0.0.0.0/0 beside the node-wide
// catch-all: the peer's PROTECT policies take the catch-all's inbound and outbound
// selectors, every reload still succeeds and leaves them, and the peer's teardown
// leaves the catch-all on both.
//
// RFC 4301 Section 4.4.1: "Every SPD SHOULD have a nominal, final entry that matches
// anything that is otherwise unmatched, and discards it." Linux identifies a policy by
// selector, direction and if_id (xfrm_policy_insert_list), so the wildcard peer's
// PROTECT policy and the catch-all are one kernel policy: the kernel holds one of them.
//
// Method: the netns lab with the real XFRM backend and the catch-all `unmatched
// discard` (espPolSPD, through installUnmatched). A wildcard Child SA is installed
// through createFirstChildSA, and its IN and OUT policies are checked to carry the
// catch-all's kernel identity before they are used. Then, each followed by a read of
// the kernel through ListPolicies: a reload (installUnmatched with discard), a reload
// that changes `unmatched` to bypass and one that changes it back, the teardown
// (removeChildSA), and a last reload.
//
// MUTATION: removing the `finalEntry(p) && b.policies.yieldFinal(p)` branch of
// xfrmBackend.InstallPolicy (dataplane/xfrm_linux.go) makes the first reload answer the
// ledger's PolicyOwnedError. Removing the reinstateFinal call of RemovePolicyParams
// makes the teardown delete the PROTECT policies, and the read after it finds no
// policy on the wildcard selector.
//
//ze:guest-root
func TestWildcardPeerStandsInForTheCatchAllAndGivesItBack(t *testing.T) {
	newEspPolLab(t)
	espPolSPD(t)
	dp := dataplane.Get()
	log := slogutil.DiscardLogger()

	built, err := unmatchedPolicies(net.IPv4zero, dataplane.SPActionDiscard)
	if err != nil {
		t.Fatalf("build the IPv4 catch-all: %v", err)
	}
	catchAll := make([]dataplane.SPParams, 0, len(built))
	for i := range built {
		if built[i].Dir != dataplane.SADirFwd {
			catchAll = append(catchAll, built[i])
		}
	}
	for i := range catchAll {
		wildcardWantCatchAll(t, "before the peer", &catchAll[i])
	}

	child := wildcardPeerChild(t)
	protect := []dataplane.SPParams{
		childPolicies(child, dataplane.SADirIn)[0],
		childPolicies(child, dataplane.SADirOut)[0],
	}
	for i := range protect {
		if protect[i].Action != dataplane.SPActionProtect || protect[i].Owner == "" {
			t.Fatalf("the Child SA's %s policy is action %v for owner %q, not an owned PROTECT policy",
				dirLabel(protect[i].Dir), protect[i].Action, protect[i].Owner)
		}
		if protect[i].Dir != catchAll[i].Dir || !dataplane.PolicySelectorsEqual(protect[i], catchAll[i]) {
			t.Fatalf("the Child SA's %s policy %v to %v does not carry the kernel identity of the %s catch-all entry; "+
				"the test does not hold the collision", dirLabel(protect[i].Dir), protect[i].Src, protect[i].Dst, dirLabel(catchAll[i].Dir))
		}
		wildcardWantProtect(t, "after the peer established", &protect[i], dataplane.SPActionDiscard)
	}

	if err := installUnmatched(dp, dataplane.SPActionDiscard, log); err != nil {
		t.Fatalf("the reload's catch-all answered %v with a wildcard peer up; want success: every reload failed here", err)
	}
	for i := range protect {
		wildcardWantProtect(t, "after the reload", &protect[i], dataplane.SPActionDiscard)
	}

	if err := installUnmatched(dp, dataplane.SPActionBypass, log); err != nil {
		t.Fatalf("the reload to `unmatched bypass` answered %v with a wildcard peer up", err)
	}
	for i := range protect {
		wildcardWantProtect(t, "after the reload to bypass", &protect[i], dataplane.SPActionBypass)
	}
	if err := installUnmatched(dp, dataplane.SPActionDiscard, log); err != nil {
		t.Fatalf("the reload back to `unmatched discard` answered %v with a wildcard peer up", err)
	}
	for i := range protect {
		wildcardWantProtect(t, "after the reload back to discard", &protect[i], dataplane.SPActionDiscard)
	}

	removeChildSA(child, dp, log)
	for i := range catchAll {
		wildcardWantCatchAll(t, "after the peer's teardown", &catchAll[i])
	}

	if err := installUnmatched(dp, dataplane.SPActionDiscard, log); err != nil {
		t.Fatalf("the reload after the teardown answered %v", err)
	}
	for i := range catchAll {
		wildcardWantCatchAll(t, "after the last reload", &catchAll[i])
	}
}
