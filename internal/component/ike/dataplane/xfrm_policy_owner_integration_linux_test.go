// VALIDATES: the two policy-ownership fixes against a real Linux XFRM stack, read
// from /proc/net/xfrm_stat rather than from a syscall result. A PROTECT policy with
// no matching state raises XfrmOutNoStates for a packet that MATCHES it, so that
// counter is a positive detector of "a policy claimed this flow": it moves when the
// traffic was handed to IPsec, and it stays still when the traffic left IN THE CLEAR.
//
// Three measurements, one file:
//   - a selector re-installed by its own owner survives the retired Child SA's
//     teardown, and still claims traffic (E1);
//   - a second peer can neither take a live peer's selector nor blackhole it on the
//     way out of its own rollback (E2);
//   - the rejected per-peer MARK alternative forwards nothing, which is why SPParams
//     has no Mark field (E3).
//
// PREVENTS: reasoning about XFRM's packet-matching predicate instead of measuring it.
// sendto() returns success whether XFRM protected the datagram, dropped it, or never
// looked at it, so a test that reads a write result measures nothing. Each experiment
// below therefore carries its own positive control: an assertion that the counter DOES
// move for the case that must match. Without it a broken fixture would read as "the
// policy was correctly absent" and every assertion here would pass vacuously
// (ai/rules/fail-closed-guards.md).

//go:build integration && linux

package dataplane

import (
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

const (
	// ownerStatOutNoStates rises once per outbound datagram that matched a PROTECT
	// policy whose template resolved to no state (net/xfrm/xfrm_policy.c, the -EAGAIN
	// arm of xfrm_lookup_with_ifid). Every policy below is installed WITHOUT a state,
	// so this counter answers "did a policy claim that packet".
	ownerStatOutNoStates = "XfrmOutNoStates"

	ownerLocalCIDR  = "10.210.0.0/24"
	ownerRemoteCIDR = "10.211.0.0/24"
	ownerLocalAddr  = "10.210.0.1"
	ownerRemoteAddr = "10.211.0.1"
	ownerProbePort  = 9

	// ownerEgressDev is the probe's egress device. It must not be loopback; see
	// ownerNetns for the measurement behind that.
	ownerEgressDev = "ze-eg0"

	ownerSiteA = "site-a"
	ownerSiteB = "site-b"
)

// ownerSPParams builds the outbound Child SA policy one peer installs.
//
// IfID is 0, and that is load-bearing rather than incidental. __xfrm_policy_match
// requires pol->if_id == the flow's if_id, and ordinary traffic carries 0, so a policy
// installed with a non-zero IfID binds to an XFRM interface and claims nothing else.
// The sibling rekey probe uses a non-zero IfID because it never sends a packet; these
// experiments read the packet path, so they must use the value ordinary traffic meets.
//
// TunnelDst varies with the owner so a takeover would be VISIBLE in the kernel's
// template, not merely inferred from an error. policyKey deliberately excludes the
// template, so the two still collide on one selector.
func ownerSPParams(owner string) SPParams {
	_, local, _ := net.ParseCIDR(ownerLocalCIDR)
	_, remote, _ := net.ParseCIDR(ownerRemoteCIDR)
	p := SPParams{
		Src:       local,
		Dst:       remote,
		Dir:       SADirOut,
		Proto:     50, // ESP
		Mode:      ModeTunnel,
		ReqID:     0x0e21,
		Priority:  PriorityChildSA,
		Owner:     owner,
		TunnelSrc: net.ParseIP("192.0.2.10"),
		TunnelDst: net.ParseIP("192.0.2.20"),
	}
	if owner == ownerSiteB {
		p.ReqID = 0x0e22
		p.TunnelDst = net.ParseIP("192.0.2.30")
	}
	return p
}

// ownerNetns puts this probe in a namespace of its own and gives it somewhere to send.
//
// encapNetns unshares and brings loopback up. What this adds is a real egress device
// carrying an address inside the policy's SOURCE prefix, plus a route to its
// DESTINATION prefix, because XFRM's outbound hook sits in route resolution and a
// datagram with no route never reaches it.
//
// The egress device is a DUMMY, and loopback will not do. MEASURED in this VM's kernel,
// same namespace, same policy, same datagram, varying only the egress device:
//
//	dev lo      -> sendto returned 5, and NO xfrm counter moved at all
//	dev dummy   -> sendto returned 5, and XfrmOutNoStates went 0 -> 1
//
// So a loopback egress bypasses the outbound hook, and every experiment in this file
// would have read "no policy claimed the datagram" for a policy the kernel was holding.
// The two readings also settle why the counter is the instrument rather than the write
// result: sendto reported the SAME success in both, including the one where XFRM took
// the packet away.
//
// It fails rather than skips when the namespace is unusable, and it asserts the XFRM
// tables start empty, so a counter reading can never be attributed to a policy this
// probe did not install.
func ownerNetns(t *testing.T) {
	t.Helper()
	encapNetns(t)
	encapNetnsUsable(t)

	egress := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: ownerEgressDev}}
	if err := netlink.LinkAdd(egress); err != nil {
		t.Fatalf("add dummy %s: %v; without a non-loopback egress device the outbound XFRM hook is never reached and every reading below would be blind", ownerEgressDev, err)
	}
	link, err := netlink.LinkByName(ownerEgressDev)
	if err != nil {
		t.Fatalf("%s lookup: %v", ownerEgressDev, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("%s up: %v", ownerEgressDev, err)
	}
	addr, err := netlink.ParseAddr(ownerLocalAddr + "/24")
	if err != nil {
		t.Fatalf("parse %s/24: %v", ownerLocalAddr, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("add %s to %s: %v", addr, ownerEgressDev, err)
	}
	_, remote, err := net.ParseCIDR(ownerRemoteCIDR)
	if err != nil {
		t.Fatalf("parse %s: %v", ownerRemoteCIDR, err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       remote,
		Scope:     netlink.SCOPE_LINK,
	}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("route %s dev %s: %v", ownerRemoteCIDR, ownerEgressDev, err)
	}

	if got := ownerListPolicies(t); len(got) != 0 {
		t.Fatalf("the fresh namespace already holds %d XFRM policies (%v); every counter reading below would be unattributable", len(got), got)
	}
	t.Cleanup(func() {
		if left := ownerListPolicies(t); len(left) != 0 {
			t.Errorf("the probe left %d XFRM policies behind: %v", len(left), left)
		}
	})
}

// ownerListPolicies returns every policy in this namespace, rendered.
func ownerListPolicies(t *testing.T) []string {
	t.Helper()
	pols, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("xfrm policy list: %v", err)
	}
	out := make([]string, 0, len(pols))
	for i := range pols {
		out = append(out, pols[i].String())
	}
	return out
}

// ownerPoliciesFor returns the rendered policies whose selector and direction match p.
// Rendering rather than counting is deliberate: E2 must show that the surviving policy
// is UNCHANGED, and a count of one cannot show that.
func ownerPoliciesFor(t *testing.T, p SPParams) []string {
	t.Helper()
	pols, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("xfrm policy list: %v", err)
	}
	var out []string
	for i := range pols {
		pol := &pols[i]
		if pol.Dir != netlink.Dir(p.Dir-1) {
			continue
		}
		if pol.Src == nil || pol.Dst == nil {
			continue
		}
		if pol.Src.String() != p.Src.String() || pol.Dst.String() != p.Dst.String() {
			continue
		}
		out = append(out, pol.String())
	}
	return out
}

// ownerOutDelta sends ONE datagram through the selector and reports how far
// XfrmOutNoStates moved.
//
// The write error is read and returned, never fatal. When a PROTECT policy matches and
// resolves to no state the kernel answers the sender EAGAIN, so a failed write is one
// of the EXPECTED outcomes of a match; and when nothing matches, the write succeeds and
// the datagram leaves unprotected. Neither result is the measurement. The counter is.
func ownerOutDelta(t *testing.T) (int, error) {
	t.Helper()
	before := encapStat(t, ownerStatOutNoStates)

	// A fresh, unconnected socket per probe. A connected socket caches its route, and
	// a cached bundle would answer the second send without consulting the policy.
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(t.Context(), "udp", net.JoinHostPort(ownerLocalAddr, "0"))
	if err != nil {
		t.Fatalf("bind udp on %s: %v", ownerLocalAddr, err)
	}
	_, writeErr := pc.WriteTo([]byte("ze-policy-owner-probe"),
		&net.UDPAddr{IP: net.ParseIP(ownerRemoteAddr), Port: ownerProbePort})
	if cerr := pc.Close(); cerr != nil {
		t.Errorf("close probe socket: %v", cerr)
	}

	return encapStat(t, ownerStatOutNoStates) - before, writeErr
}

// ownerAssertClaimed is the positive reading: a policy matched the probe datagram.
func ownerAssertClaimed(t *testing.T, what string) {
	t.Helper()
	delta, writeErr := ownerOutDelta(t)
	if delta < 1 {
		t.Fatalf("%s: %s did not move (delta=%d, write=%v); no policy claimed the datagram, so the traffic left IN THE CLEAR",
			what, ownerStatOutNoStates, delta, writeErr)
	}
	t.Logf("%s: %s +%d (write=%v) -- the flow was handed to IPsec", what, ownerStatOutNoStates, delta, writeErr)
}

// ownerAssertUnclaimed is the negative reading, and it is only meaningful after a
// positive one in the same namespace. Every caller below takes one first.
func ownerAssertUnclaimed(t *testing.T, what string) {
	t.Helper()
	delta, writeErr := ownerOutDelta(t)
	if delta != 0 {
		t.Fatalf("%s: %s moved by %d (write=%v); a policy still claimed the datagram",
			what, ownerStatOutNoStates, delta, writeErr)
	}
	t.Logf("%s: %s +0 (write=%v) -- the flow left unprotected", what, ownerStatOutNoStates, writeErr)
}

// TestXFRMPolicyOwnerSharedSelectorSurvivesRetiredChild is E1.
//
// A make-before-break rekey and a parallel re-initiation both install ONE selector
// twice under ONE owner, and the backend upserts, so the kernel then holds a single
// policy that two Child SAs believe they own. removeChildSAExcept answers that by not
// removing at all when a survivor shares the selector, and this measures what the
// kernel does with that decision: the policy stays, and it still claims traffic.
//
// The second half is the control. Removing the policy for real must STOP the counter,
// otherwise the first half is reading something other than this policy.
func TestXFRMPolicyOwnerSharedSelectorSurvivesRetiredChild(t *testing.T) {
	// One namespace probe per PROCESS. See encapOwnProcess: a second unshare in one
	// binary reads a namespace whose datagrams never reach XFRM, and every counter
	// then stays still -- which is this file's negative reading, so a probe that
	// skipped this would manufacture its own false pass.
	if !encapOwnProcess(t) {
		return
	}
	ownerNetns(t)

	b := &xfrmBackend{}
	p := ownerSPParams(ownerSiteA)

	// The retired Child SA's policy.
	if err := b.InstallPolicy(p); err != nil {
		skipWithoutPolicyPermission(t, err)
		t.Fatalf("installing the retired pair's policy: %v", err)
	}
	// The replacement pair's policy: identical selector, identical owner. This is the
	// upsert, and the kernel is left holding exactly one policy for two Child SAs.
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("the replacement pair's install was refused (%v); every make-before-break rekey fails here", err)
	}

	held := ownerPoliciesFor(t, p)
	if len(held) != 1 {
		t.Fatalf("after two installs of one selector the kernel holds %d policies, want 1: %v", len(held), held)
	}

	// The retired half is now torn down, and removeChildSAExcept removes NOTHING
	// because the replacement shares the selector. Measure what the survivor has.
	ownerAssertClaimed(t, "after the retired Child SA's teardown")

	if after := ownerPoliciesFor(t, p); len(after) != 1 || after[0] != held[0] {
		t.Fatalf("the teardown changed the surviving policy:\nbefore %v\nafter  %v", held, after)
	}

	// The control. A real removal must silence the counter; if it does not, the
	// reading above was never about this policy.
	if err := b.RemovePolicyParams(p); err != nil {
		t.Fatalf("removing the surviving policy: %v", err)
	}
	if left := ownerPoliciesFor(t, p); len(left) != 0 {
		t.Fatalf("the policy survived its own removal: %v", left)
	}
	ownerAssertUnclaimed(t, "after removing the surviving policy")
}

// TestXFRMPolicyOwnerRefusesTakeoverAndItsRollback is E2.
//
// Two site-to-site peers that negotiate overlapping selectors describe ONE kernel
// policy, because a policy's whole identity there is its selector. The backend upserts,
// so without an ownership record the second peer would silently replace the first
// peer's template and capture its traffic into a different tunnel.
//
// The rollback is the second half of the same defect and is measured separately:
// installChildSA rolls a failed policy install back by removing the OTHER direction's
// policy, so a refused peer that could still delete would blackhole the peer it just
// failed to displace, with that peer's states left installed and nothing to show for it.
func TestXFRMPolicyOwnerRefusesTakeoverAndItsRollback(t *testing.T) {
	if !encapOwnProcess(t) {
		return
	}
	ownerNetns(t)

	b := &xfrmBackend{}
	live := ownerSPParams(ownerSiteA)

	if err := b.InstallPolicy(live); err != nil {
		skipWithoutPolicyPermission(t, err)
		t.Fatalf("installing site-a's policy: %v", err)
	}
	t.Cleanup(func() {
		if err := b.RemovePolicyParams(live); err != nil {
			t.Errorf("cleanup: removing site-a's policy: %v", err)
		}
	})

	installed := ownerPoliciesFor(t, live)
	if len(installed) != 1 {
		t.Fatalf("site-a installed %d policies, want 1: %v", len(installed), installed)
	}
	ownerAssertClaimed(t, "site-a alone")

	// Site-b arrives on the same selector.
	takeover := ownerSPParams(ownerSiteB)
	err := b.InstallPolicy(takeover)
	var owned *PolicyOwnedError
	if !errors.As(err, &owned) {
		t.Fatalf("site-b's install of site-a's selector returned %v, want *PolicyOwnedError; the takeover reached the kernel", err)
	}
	t.Logf("site-b's install refused: %v", err)

	// The kernel, not the error, is the evidence. Site-b's template names a different
	// tunnel endpoint, so a landed takeover would show here.
	if after := ownerPoliciesFor(t, live); len(after) != 1 || after[0] != installed[0] {
		t.Fatalf("site-b's refused install still changed the kernel:\nbefore %v\nafter  %v", installed, after)
	}
	ownerAssertClaimed(t, "site-a after site-b's refused install")

	// Site-b's rollback path, which removes a policy it does not own.
	rollbackErr := b.RemovePolicyParams(takeover)
	if !errors.As(rollbackErr, &owned) {
		t.Fatalf("site-b's rollback delete returned %v, want *PolicyOwnedError; site-b tore down site-a's policy on its way out", rollbackErr)
	}
	t.Logf("site-b's rollback delete refused: %v", rollbackErr)

	if after := ownerPoliciesFor(t, live); len(after) != 1 || after[0] != installed[0] {
		t.Fatalf("site-b's rollback changed site-a's policy:\nbefore %v\nafter  %v", installed, after)
	}
	ownerAssertClaimed(t, "site-a after site-b's rollback")
}

// TestXFRMPolicyOwnerMarkedPolicyClaimsNothing is E3: the REJECTED alternative,
// measured rather than argued.
//
// The design considered giving each peer's policy a per-peer XFRM mark so two peers
// could hold one selector between them. It was rejected on a reading of the kernel's
// packet-matching predicate, (fl->flowi_mark & pol->mark.m) == pol->mark.v
// (__xfrm_policy_match), which no unmarked flow can satisfy against a non-zero value.
// Nothing in ze marks a flow, so a marked policy would forward nothing.
//
// The policies here are built by hand from netlink rather than through SPParams,
// because this probe exists to show why SPParams has no Mark field. Adding one to
// production code to test the case would be the change the measurement argues against.
//
// A shape that DID match would overturn the design decision, so each is reported by
// name whichever way it reads.
func TestXFRMPolicyOwnerMarkedPolicyClaimsNothing(t *testing.T) {
	if !encapOwnProcess(t) {
		return
	}
	ownerNetns(t)

	build := func(t *testing.T, mark *netlink.XfrmMark) *netlink.XfrmPolicy {
		t.Helper()
		pol, err := xfrmPolicyFromParams(ownerSPParams(ownerSiteA))
		if err != nil {
			t.Fatalf("building the policy: %v", err)
		}
		pol.Mark = mark
		return pol
	}

	// Install one policy, take one reading, remove it. Keeping only one policy in the
	// namespace at a time means a reading can name exactly one cause.
	measure := func(t *testing.T, what string, mark *netlink.XfrmMark, wantClaim bool) {
		t.Helper()
		pol := build(t, mark)
		if err := netlink.XfrmPolicyUpdate(pol); err != nil {
			skipWithoutPolicyPermission(t, err)
			t.Fatalf("%s: install: %v", what, err)
		}
		if got := ownerPoliciesFor(t, ownerSPParams(ownerSiteA)); len(got) != 1 {
			t.Fatalf("%s: kernel holds %d matching policies, want 1: %v", what, len(got), got)
		}
		if wantClaim {
			ownerAssertClaimed(t, what)
		} else {
			ownerAssertUnclaimed(t, what)
		}
		if err := netlink.XfrmPolicyDel(pol); err != nil {
			t.Fatalf("%s: remove: %v", what, err)
		}
	}

	// The control, and it runs FIRST. Without it every "did not claim" reading below
	// would also be produced by a namespace that never reaches XFRM at all.
	measure(t, "unmarked policy", nil, true)

	// Value with no mask. (mark & 0) == 0, and the value is not 0, so no flow matches.
	measure(t, "marked policy value=0x1234 mask=0x0", &netlink.XfrmMark{Value: 0x1234}, false)

	// Value with a full mask. An unmarked flow carries 0, and 0 != the value.
	measure(t, "marked policy value=0x1234 mask=0xffffffff", &netlink.XfrmMark{Value: 0x1234, Mask: 0xffffffff}, false)

	// The control again, last. It proves the instrument was still live at the end of
	// the run, so the two negative readings are readings and not a dead namespace.
	measure(t, "unmarked policy, re-measured", nil, true)
}
