// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- Child SA policy ownership
// RFC: rfc/short/rfc4301.md -- SPD entries are keyed by their selector
//
// The kernel gives a policy no per-peer identity: it tells one policy from
// another by the whole selector and by nothing else. Two site-to-site peers that
// both negotiate 0.0.0.0/0 therefore describe ONE policy, and the upsert a Child
// SA rekey needs would otherwise let the second peer to establish capture the
// first peer's traffic. dataplane.SPParams.Owner is the identity that separates
// "my own rekey re-claiming my selector" from "a different peer taking it over".
//
// These tests drive that identity from the ENTRY POINTS that produce it,
// createFirstChildSA and the CREATE_CHILD_SA rekey path, rather than from the
// dataplane registry that consumes it (dataplane/policy_owner_test.go owns that
// half). The distinction is load-bearing: a registry test compares whatever it is
// handed, so with both owners empty it compares "" against "" and passes while
// the guard admits every takeover it exists to refuse.

package engine

import (
	"errors"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ownedKey is a policy's identity as the kernel computes it, mirroring the fields
// xfrmPolicyFromParams writes into the netlink selector.
//
// The template is deliberately absent. Tunnel endpoints, reqid and priority do
// not participate in that identity, so two peers whose templates differ still
// collide -- and that collision is the whole subject here.
type ownedKey struct {
	src, dst   string
	dir        dataplane.SADir
	upperProto uint8
	srcPort    dataplane.PortMatch
	dstPort    dataplane.PortMatch
	ifIndex    int
	ifID       uint32
}

func ownedKeyOf(p dataplane.SPParams) ownedKey {
	k := ownedKey{
		dir:        p.Dir,
		upperProto: p.UpperProto,
		srcPort:    p.SrcPort,
		dstPort:    p.DstPort,
		ifIndex:    p.IfIndex,
		ifID:       p.IfID,
	}
	if p.Src != nil {
		k.src = p.Src.String()
	}
	if p.Dst != nil {
		k.dst = p.Dst.String()
	}
	return k
}

// errForeignPolicy stands for dataplane.PolicyOwnedError, which the real backend
// returns when a selector already belongs to somebody else.
var errForeignPolicy = errors.New("policy selector is installed for a different owner")

// ownedDP models the kernel's SPD together with the ownership contract
// dataplane.SPParams.Owner documents: InstallPolicy UPSERTS on the selector, and
// a selector a DIFFERENT owner holds is refused rather than overwritten.
//
// It records the owner it was HANDED, so a test can tell an empty owner from a
// real one. That matters more than the refusal itself: with the owner dropped at
// the producer every claim is "", every comparison succeeds, and a fake that only
// re-derived its own expectation would report the collapse as agreement.
type ownedDP struct {
	policies map[ownedKey]dataplane.SPParams
	states   map[uint32]bool
}

func newOwnedDP() *ownedDP {
	return &ownedDP{
		policies: make(map[ownedKey]dataplane.SPParams),
		states:   make(map[uint32]bool),
	}
}

func (d *ownedDP) InstallSA(p dataplane.SAParams) error {
	d.states[p.SPI] = true
	return nil
}

func (d *ownedDP) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	delete(d.states, spi)
	return nil
}

func (d *ownedDP) InstallPolicy(p dataplane.SPParams) error {
	if held, ok := d.policies[ownedKeyOf(p)]; ok && held.Owner != p.Owner {
		return errForeignPolicy
	}
	d.policies[ownedKeyOf(p)] = p
	return nil
}

// RemovePolicy is the ownerless three-argument route, modeled on what it asks the
// KERNEL for rather than on what it forgets in the registry.
//
// `xfrmBackend.RemovePolicy` (dataplane/xfrm_linux.go) builds a netlink policy from
// the three arguments alone, so every other selector field is zero -- and the
// vendored netlink attaches XFRMA_IF_ID only when if_id is non-zero
// (vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go, XfrmPolicyDel). The
// kernel therefore looks up the policy whose if_id is 0, and a Child SA policy
// bound to if_id 7 is NOT it. The delete succeeds against nothing and the policy
// stays installed.
//
// Modeling the registry half instead (releaseBySelector, which forgets every record
// matching source, destination and direction) made this route look capable of
// removing a policy it cannot name, and that made testChildIfID inert for the
// purpose its own comment gives it.
func (d *ownedDP) RemovePolicy(src, dst *net.IPNet, dir dataplane.SADir) error {
	// The only key the three arguments can name: their own three fields, and zero
	// for every field the route does not carry.
	delete(d.policies, ownedKey{src: cidrText(src), dst: cidrText(dst), dir: dir})
	return nil
}

func cidrText(n *net.IPNet) string {
	if n == nil {
		return ""
	}
	return n.String()
}

func (d *ownedDP) RemovePolicyParams(p dataplane.SPParams) error {
	if held, ok := d.policies[ownedKeyOf(p)]; ok && held.Owner != p.Owner {
		return errForeignPolicy
	}
	delete(d.policies, ownedKeyOf(p))
	return nil
}

func (d *ownedDP) ListSAs(_ uint32) ([]dataplane.SAInfo, error)  { return nil, nil }
func (d *ownedDP) ListPolicies() ([]dataplane.PolicyInfo, error) { return nil, nil }
func (d *ownedDP) Close() error                                  { return nil }

// ownerOf returns the owner recorded against a Child SA's policy for one
// direction, and whether a policy is installed at all.
func (d *ownedDP) ownerOf(child *ChildSA, dir dataplane.SADir) (string, bool) {
	p, ok := d.policies[ownedKeyOf(childPolicyParams(child, dir))]
	return p.Owner, ok
}

// wideSA builds an SA for a peer whose negotiated selector is the whole address
// space, which is the ordinary site-to-site answer and the case that makes two
// peers collide on one kernel policy.
func wideSA(peerName, remote string) *SA {
	sa := testSA()
	sa.PeerName = peerName
	sa.PeerCfg = ipsec.SiteToSitePeer{RemoteAddress: remote}
	_, any4, _ := net.ParseCIDR("0.0.0.0/0")
	sa.NegotiatedTSi, sa.NegotiatedTSr = any4, any4
	return sa
}

// bothDirections is the pair every Child SA installs.
var bothDirections = []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut}

// testChildIfID is the XFRM if_id every Child SA below is bound to.
//
// It is deliberately NOT zero. if_id is part of the selector the kernel matches
// on, so a zero one makes the fully-zeroed key the three-argument RemovePolicy
// builds coincide with the real one, and a removal that cannot name the record
// then removes it anyway. A fixture that hides a route's imprecision cannot
// measure it.
const testChildIfID = 7

// TestFirstChildSAPolicyCarriesTheConfiguredPeerName drives the owner from the
// entry point that sets it to the parameter the dataplane refuses takeovers on.
//
// VALIDATES: createFirstChildSA puts the configured peer name on the Child SA,
// and childPolicyParams carries it into both installed policies as a NON-EMPTY
// owner.
// PREVENTS: the owner being dropped anywhere along that path. An empty owner is
// not a weaker identity, it is no identity: policyOwners.claim refuses only on
// `held != p.Owner`, so once every policy is owned by "" every foreign claim
// matches and the guard admits the takeover it exists to refuse.
func TestFirstChildSAPolicyCarriesTheConfiguredPeerName(t *testing.T) {
	sa := wideSA("site-a", "10.0.0.2")
	dp := newOwnedDP()

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", testChildIfID, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	if sa.PeerName == "" {
		t.Fatal("this test needs a named peer; with an empty one every assertion below is vacuous")
	}
	if child.Owner != sa.PeerName {
		t.Errorf("the Child SA carries owner %q, want the configured peer %q", child.Owner, sa.PeerName)
	}
	for _, dir := range bothDirections {
		owner, ok := dp.ownerOf(child, dir)
		if !ok {
			t.Errorf("no %s policy was installed at all", dirName(dir))
			continue
		}
		if owner == "" {
			t.Errorf("the %s policy was installed with NO owner: the dataplane cannot tell this "+
				"peer's re-install of the selector from a different peer's takeover of it", dirName(dir))
		}
		if owner != sa.PeerName {
			t.Errorf("the %s policy carries owner %q, want %q", dirName(dir), owner, sa.PeerName)
		}
	}
}

// TestChildSARekeyReclaimsItsOwnPolicySelector drives a real CREATE_CHILD_SA
// rekey against a dataplane that refuses a foreign owner.
//
// VALIDATES: the replacement newRekeyedChild builds claims the retired pair's
// selector under the SAME owner, so its install is an upsert and succeeds.
// PREVENTS: the rekey failing at its policy install. The replacement inherits
// every selector field from the retired pair, so it describes the same kernel
// policy; a replacement that dropped the owner would present "" against a
// selector the peer already holds and be refused as a takeover -- a tunnel that
// dies at the moment its rekey succeeds.
func TestChildSARekeyReclaimsItsOwnPolicySelector(t *testing.T) {
	sa := wideSA("site-a", "10.0.0.2")
	dp := newOwnedDP()
	log := slogutil.DiscardLogger()

	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", testChildIfID, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	pending := &pendingRekey{
		kind:          rekeyChild,
		localNonce:    make([]byte, nonceLen),
		newInboundSPI: 0x55667788,
		oldChild:      old,
	}
	inner := []wire.PayloadEntry{
		{Payload: espSAPayload(0x99AABBCC)},
		{Payload: &wire.PayloadNonce{NonceData: make([]byte, nonceLen)}},
	}

	replacement, err := applyChildRekeyResponse(sa, pending, inner, dp, log)
	if err != nil {
		t.Fatalf("the rekey was refused at its policy install (%v): the replacement did not "+
			"claim the retired pair's selector under the same owner", err)
	}

	if replacement.Owner == "" {
		t.Fatal("the replacement Child SA carries no owner, so its policy is unowned from the rekey onward")
	}
	if replacement.Owner != old.Owner {
		t.Errorf("the replacement carries owner %q, the retired pair %q", replacement.Owner, old.Owner)
	}
	if !samePolicySelector(old, replacement) {
		t.Fatal("the replacement does not share the retired pair's selector; this test's premise " +
			"is gone and the upsert it guards no longer happens")
	}
	for _, dir := range bothDirections {
		owner, ok := dp.ownerOf(replacement, dir)
		if !ok {
			t.Errorf("the replacement has no %s policy", dirName(dir))
			continue
		}
		if owner != sa.PeerName {
			t.Errorf("after the rekey the %s policy is owned by %q, want %q", dirName(dir), owner, sa.PeerName)
		}
	}
}

// TestSecondPeerCannotTakeOverAnotherPeersPolicySelector is the negative half,
// driven end to end from the two peers' own Child SA creation.
//
// VALIDATES: two DIFFERENT configured peers negotiating the same 0.0.0.0/0
// selector present DIFFERENT owners, so the second peer's install is refused and
// the first peer's policies survive untouched.
// PREVENTS: the takeover dataplane.SPParams.Owner exists to refuse. With the
// owner dropped at the producer both peers present "", the refusal never fires,
// and the second peer to establish silently captures the first peer's traffic --
// with nothing in the logs, because from the kernel's side it is an ordinary
// upsert of a selector that was already there.
func TestSecondPeerCannotTakeOverAnotherPeersPolicySelector(t *testing.T) {
	dp := newOwnedDP()
	log := slogutil.DiscardLogger()

	first := wideSA("site-a", "10.0.0.2")
	firstChild, err := createFirstChildSA(first, testESPGroup(), "10.0.0.1", "10.0.0.2", testChildIfID, dp, log)
	if err != nil {
		t.Fatalf("first peer: createFirstChildSA: %v", err)
	}

	// A genuinely different peer, over a different tunnel, whose negotiated
	// selector is the same one. The tunnel endpoints differ and do not matter: the
	// kernel identifies a policy by the selector alone.
	second := wideSA("site-b", "10.0.0.3")
	secondChild, err := createFirstChildSA(second, testESPGroup(), "10.0.0.1", "10.0.0.3", testChildIfID, dp, log)

	// A refused install returns no Child SA, so secondChild is read only on the
	// branch where the install SUCCEEDED -- which is the branch the dropped-owner
	// mutation takes, and the one whose message must name both owners.
	if err == nil {
		t.Fatalf("the second peer installed over the first peer's selector unopposed "+
			"(owners %q and %q): its traffic now answers to a policy the first peer's tunnel owns",
			firstChild.Owner, secondChild.Owner)
	}
	if !errors.Is(err, errForeignPolicy) {
		t.Fatalf("the second peer was refused for the wrong reason: %v", err)
	}
	if first.PeerName == second.PeerName {
		t.Fatal("this test needs two DIFFERENTLY named peers; with one name the refusal above " +
			"proves nothing about ownership")
	}

	// The refused peer must not have taken the survivor's policies down on its way
	// out. Here the refusal lands on the FIRST install, so no rollback runs at all;
	// TestRefusedOutboundInstallRollsBackOnlyItsOwnPolicy below drives that arm.
	for _, dir := range bothDirections {
		owner, ok := dp.ownerOf(firstChild, dir)
		if !ok {
			t.Errorf("the first peer's %s policy is gone: the refused peer removed it", dirName(dir))
			continue
		}
		if owner != first.PeerName {
			t.Errorf("the first peer's %s policy is now owned by %q", dirName(dir), owner)
		}
	}
}

// TestRefusedOutboundInstallRollsBackOnlyItsOwnPolicy drives the rollback arm of
// installChildSA, which the test above cannot reach.
//
// Reaching it needs the OUTBOUND install to be the refused one. Two peers on one
// symmetric selector cannot produce that: the inbound install is the first call
// installChildSA makes, so it is refused and the function returns before the
// outbound arm exists. The fixture therefore seeds the fake with a foreign-owned
// OUTBOUND policy and no inbound one, which is the state a peer whose teardown is
// half done leaves behind.
//
// VALIDATES: the refused peer's rollback removes ITS OWN inbound policy and
// leaves the foreign outbound policy installed and still foreign-owned.
// PREVENTS: a leak. A rollback that ran no removal at all leaves the refused
// peer's inbound policy behind, and the next flow matching that selector is
// captured by a policy resolving to no state. MEASURED: deleting the
// RemovePolicyParams call from installChildSA reddens this test and nothing else
// in the package.
//
// It also pins the ROUTE. Swapping RemovePolicyParams for the three-argument
// RemovePolicy reddens this test, because that route carries no if_id and the
// vendored netlink omits XFRMA_IF_ID when if_id is zero, so the kernel is asked to
// delete an if_id=0 policy while this Child SA's is bound to if_id 7. The delete
// matches nothing and the refused peer's inbound policy is left behind.
//
// WHAT IT DOES NOT PROVE: the OWNER half of RemovePolicyParams. The rollback can
// only ever reach a policy this peer installed moments earlier in the same call --
// an inbound install over a foreign selector is refused, so the function returns
// before an outbound install exists to fail -- so no foreign record is in reach
// here. That half is exercised where the state needing it is reachable, in
// dataplane/policy_owner_test.go's TestPolicyOwnerRefusesAForeignDeleteBeforeTheKernel.
func TestRefusedOutboundInstallRollsBackOnlyItsOwnPolicy(t *testing.T) {
	dp := newOwnedDP()
	log := slogutil.DiscardLogger()

	intruder := wideSA("site-b", "10.0.0.3")

	// The shape of the policy the intruder is about to collide with. Built from
	// its OWN child so the selector matches by construction, then re-owned.
	probe := &ChildSA{
		LocalAddr: net.ParseIP("10.0.0.1"), RemoteAddr: net.ParseIP("10.0.0.3"),
		TSLocal: intruder.NegotiatedTSi, TSRemote: intruder.NegotiatedTSr,
		Owner: "site-a", Mode: modeTunnel, ReqID: defaultReqID, IfID: testChildIfID,
	}
	occupied := childPolicyParams(probe, dataplane.SADirOut)
	if err := dp.InstallPolicy(occupied); err != nil {
		t.Fatalf("seeding the foreign outbound policy: %v", err)
	}

	child, err := createFirstChildSA(intruder, testESPGroup(), "10.0.0.1", "10.0.0.3", testChildIfID, dp, log)
	if err == nil {
		t.Fatalf("the intruder installed over a foreign outbound policy unopposed (child %+v)", child)
	}
	if !errors.Is(err, errForeignPolicy) {
		t.Fatalf("refused for the wrong reason: %v", err)
	}

	// Its own inbound policy went in before the outbound was refused, so the
	// rollback must have taken it back out.
	inbound := childPolicyParams(probe, dataplane.SADirIn)
	if held, ok := dp.policies[ownedKeyOf(inbound)]; ok {
		t.Errorf("the refused peer left its inbound policy installed (owner %q): a flow matching "+
			"that selector is now captured by a policy resolving to no state", held.Owner)
	}

	// And the foreign policy it collided with is untouched.
	held, ok := dp.policies[ownedKeyOf(occupied)]
	if !ok {
		t.Fatal("the refused peer removed the foreign outbound policy on its way out: that is " +
			"the live tunnel it was refused in order to protect")
	}
	if held.Owner != "site-a" {
		t.Errorf("the foreign outbound policy is now owned by %q, want site-a", held.Owner)
	}
}
