package engine

import (
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The two Child SA removals a PEER can trigger, and the policies they must not take with
// them. child_policy_survival_test.go covers the removals ze reaches on its own; these are
// the ones an incoming message drives.
//
// Both share one shape. Two Child SAs answer to ONE pair of kernel policies whenever they
// negotiated the same selector, which the ordinary 0.0.0.0/0 site-to-site selector always
// does, so a removal that drops the policies unconditionally blackholes the survivor.

// spdPolicyDirs is the pair every assertion below walks. A policy that survives in one
// direction and not the other is still a broken tunnel.
var spdPolicyDirs = []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut}

// VALIDATES: a peer's ESP Delete naming the LIVE Child SA leaves the policies installed
// while a parallel re-initiation's pending Child SA still answers to them.
// PREVENTS: the defect a peer triggers with an ordinary Delete. closeDesignatedChildSAs
// removed the live pair with no survivor named, so both shared policies went; the owner
// loop's reestablish exit then ran cleanupChild, which reinstalls nothing, and
// resolvePendingAfterOwnerLoop promoted a Child SA with states and NO policy. Outbound
// traffic left the box in the clear.
func TestPeerDeleteOfLiveChildKeepsPolicyForPendingChild(t *testing.T) {
	log := slogutil.DiscardLogger()
	dp := newSPDDP()
	ps := &PeerSession{peerName: testPolicyOwner}

	live := testChildOnSelector(0x1001, 0x1002)
	installTestChild(t, dp, live)
	ps.setChildSA(live)

	// finishResponderEstablish (responder.go) installed this one before the supersede
	// token reached the owner loop. Its policies upserted the live pair's selector.
	pending := testChildOnSelector(0x2001, 0x2002)
	installTestChild(t, dp, pending)
	ps.setPendingChild(pending)

	// RFC 7296 Section 1.4.1: the peer names the SPI it expects in the headers of its
	// own inbound packets, which is this node's OUTBOUND half.
	paired, down := ps.closeDesignatedChildSAs(espDeletePayload([]uint32{live.OutboundSPI}), dp, log)

	if !down {
		t.Fatal("the peer's Delete of the live Child SA did not report the session child down")
	}
	if len(paired) != 1 || paired[0] != live.InboundSPI {
		t.Fatalf("the response names %v, want the closed pair's inbound SPI %#x", paired, live.InboundSPI)
	}
	for _, dir := range spdPolicyDirs {
		if !dp.hasPolicy(pending, dir) {
			t.Errorf("the peer's Delete removed the %s policy the parallel handshake's Child SA still answers to: the promoted tunnel sends in the clear",
				dirName(dir))
		}
	}
	if dp.states[live.InboundSPI] || dp.states[live.OutboundSPI] {
		t.Error("the deleted pair's states were left installed")
	}
	if !dp.states[pending.InboundSPI] || !dp.states[pending.OutboundSPI] {
		t.Error("the pending Child SA's states were removed by a Delete that did not name them")
	}
}

// VALIDATES: the same Delete leaves the policies installed while the make-before-break
// supersededChild still answers to them.
// PREVENTS: the rekey survivor losing its policies when the peer deletes the replacement
// rather than the retired pair. supersededChild is installed at that moment, so it is a
// survivor exactly as the pending child is.
func TestPeerDeleteOfLiveChildKeepsPolicyForSupersededChild(t *testing.T) {
	log := slogutil.DiscardLogger()
	dp := newSPDDP()
	ps := &PeerSession{peerName: testPolicyOwner}

	retired := testChildOnSelector(0x3001, 0x3002)
	installTestChild(t, dp, retired)
	ps.supersededChild = retired

	live := testChildOnSelector(0x1001, 0x1002)
	installTestChild(t, dp, live)
	ps.setChildSA(live)

	if _, down := ps.closeDesignatedChildSAs(espDeletePayload([]uint32{live.OutboundSPI}), dp, log); !down {
		t.Fatal("the peer's Delete of the live Child SA did not report the session child down")
	}

	for _, dir := range spdPolicyDirs {
		if !dp.hasPolicy(retired, dir) {
			t.Errorf("the peer's Delete removed the %s policy the retired pair still answers to", dirName(dir))
		}
	}
	if !dp.states[retired.InboundSPI] || !dp.states[retired.OutboundSPI] {
		t.Error("the retired pair's states were removed by a Delete that did not name them")
	}
}

// VALIDATES: the peer's Delete DOES remove the policies when no other Child SA answers to
// them, so an ordinary teardown leaks nothing.
// PREVENTS: the two tests above passing vacuously because the removal never happens at all.
func TestPeerDeleteOfLiveChildRemovesPolicyWhenNothingElseSharesIt(t *testing.T) {
	log := slogutil.DiscardLogger()
	dp := newSPDDP()
	ps := &PeerSession{peerName: testPolicyOwner}

	live := testChildOnSelector(0x1001, 0x1002)
	installTestChild(t, dp, live)
	ps.setChildSA(live)

	if _, down := ps.closeDesignatedChildSAs(espDeletePayload([]uint32{live.OutboundSPI}), dp, log); !down {
		t.Fatal("the peer's Delete of the live Child SA did not report the session child down")
	}

	for _, dir := range spdPolicyDirs {
		if dp.hasPolicy(live, dir) {
			t.Errorf("the %s policy outlived the only Child SA that answered to it", dirName(dir))
		}
	}
	if len(dp.states) != 0 {
		t.Errorf("states left installed: %v", dp.states)
	}
}

// VALIDATES: a pending Child SA on a DIFFERENT selector does not keep the deleted pair's
// policies alive.
// PREVENTS: the survivor lookup degenerating into "keep the policy whenever any other child
// exists", which would leak a policy on every parallel handshake that narrowed differently.
func TestPeerDeleteOfLiveChildRemovesPolicyWhenPendingChildUsesAnotherSelector(t *testing.T) {
	log := slogutil.DiscardLogger()
	dp := newSPDDP()
	ps := &PeerSession{peerName: testPolicyOwner}

	live := testChildOnSelector(0x1001, 0x1002)
	installTestChild(t, dp, live)
	ps.setChildSA(live)

	pending := testChildOnSelector(0x2001, 0x2002)
	pending.TSRemote = mustCIDR(t, "192.0.2.0/24")
	installTestChild(t, dp, pending)
	ps.setPendingChild(pending)

	if _, down := ps.closeDesignatedChildSAs(espDeletePayload([]uint32{live.OutboundSPI}), dp, log); !down {
		t.Fatal("the peer's Delete of the live Child SA did not report the session child down")
	}

	for _, dir := range spdPolicyDirs {
		if dp.hasPolicy(live, dir) {
			t.Errorf("the deleted pair's %s policy survived because an unrelated pending child existed", dirName(dir))
		}
		if !dp.hasPolicy(pending, dir) {
			t.Errorf("the pending Child SA lost its own %s policy", dirName(dir))
		}
	}
}

// Authentication response failures must leave an existing, migrated Child intact.
// The SPD model retains both templates and state endpoints: merely retaining a
// selector is not enough when a parallel handshake would upsert the original tuple.
type authFailureFixture struct {
	ps   *PeerSession
	resp *SA
	dp   *mbHandoffDP
	live *ChildSA
}

func newAuthFailureFixture(t *testing.T) authFailureFixture {
	t.Helper()
	dp := mbUseHandoffDataplane(t)
	_, resp, ps := establishPSK(t)
	live := ps.getChildSA()
	if live == nil {
		t.Fatal("the handshake installed no Child SA")
	}
	resp.mobike.local = &net.UDPAddr{IP: net.ParseIP("192.0.2.10"), Port: 4500}
	resp.peerEndpoint = &net.UDPAddr{IP: net.ParseIP("192.0.2.20"), Port: 4500}
	if err := ps.migrateMobikeChild(resp, dp); err != nil {
		t.Fatalf("move the established Child: %v", err)
	}
	dp.assertPolicyResolves(t, live)

	// The candidate exchange is still on the configured tuple. Reuse negotiated
	// key material for the response-builder failure, not the survivor's MOBIKE path.
	next := *resp
	next.mobike = mobikeState{}
	next.ChildOutboundSPI = 0x77770001
	return authFailureFixture{ps: ps, resp: &next, dp: dp, live: live}
}

func (f authFailureFixture) assertLiveChildUnchanged(t *testing.T, err error, wantErr string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("buildAuthResponse error = %v, want the failure naming %q", err, wantErr)
	}
	f.dp.assertPolicyResolves(t, f.live)
	if len(f.dp.states) != 2 || !f.dp.states[f.live.InboundSPI] || !f.dp.states[f.live.OutboundSPI] {
		t.Fatalf("failed authentication changed the live SAD or leaked a new state: %v", f.dp.states)
	}
}

// An AUTH computation error must not replace the moved survivor's policy tuple.
func TestBuildAuthResponseAuthFailureKeepsMovedChild(t *testing.T) {
	f := newAuthFailureFixture(t)
	f.resp.PeerCfg.Auth.PSK = ""
	_, _, err := f.ps.buildAuthResponse(f.resp, 2, nil, nil, nil, false, slogutil.DiscardLogger())
	f.assertLiveChildUnchanged(t, err, "pre-shared")
}

// Certificate encoding can fail after AUTH succeeded, before a reply exists.
func TestBuildAuthResponseCertFailureKeepsMovedChild(t *testing.T) {
	wpcChain(t, 1)
	f := newAuthFailureFixture(t)
	f.resp.PeerCfg.Auth = wpcAuth()
	f.resp.PeerCfg.Auth.HashAndURL = true
	f.resp.PeerCfg.Auth.CertificateURL = ""
	_, _, err := f.ps.buildAuthResponse(f.resp, 2, nil, nil, nil, false, slogutil.DiscardLogger())
	f.assertLiveChildUnchanged(t, err, "certificate-url")
}

// The final encryption error is subject to the same dataplane transaction boundary.
func TestBuildAuthResponseEncryptionFailureKeepsMovedChild(t *testing.T) {
	f := newAuthFailureFixture(t)
	f.resp.SKKeys.SK_er = []byte{1, 2, 3}
	_, _, err := f.ps.buildAuthResponse(f.resp, 2, nil, nil, nil, false, slogutil.DiscardLogger())
	f.assertLiveChildUnchanged(t, err, "key size")
}

// With no survivor, failed authentication must leave neither states nor policies.
func TestBuildAuthResponseFailureLeavesEmptyDataplane(t *testing.T) {
	log := slogutil.DiscardLogger()
	f := newAuthFailureFixture(t)
	removeChildSA(f.live, f.dp, log)
	f.ps.setChildSA(nil)
	if len(f.dp.states) != 0 || len(f.dp.policies) != 0 {
		t.Fatal("fixture teardown did not empty the SAD and SPD")
	}
	f.resp.PeerCfg.Auth.PSK = ""
	_, _, err := f.ps.buildAuthResponse(f.resp, 2, nil, nil, nil, false, log)
	if err == nil || !strings.Contains(err.Error(), "pre-shared") {
		t.Fatalf("authentication error = %v", err)
	}
	if len(f.dp.states) != 0 || len(f.dp.policies) != 0 {
		t.Fatalf("failed authentication leaked SAD/SPD entries: states=%v policies=%v", f.dp.states, f.dp.policies)
	}
}

// mustCIDR parses a prefix a test fixture depends on.
func mustCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, prefix, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse %q: %v", cidr, err)
	}
	return prefix
}
