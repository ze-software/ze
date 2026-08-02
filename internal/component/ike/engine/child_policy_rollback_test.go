package engine

import (
	"net"
	"strings"
	"sync"
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

// -- the three buildAuthResponse rollbacks -------------------------------------------

// spdBackendName is the registered name of the fake backend the rollback tests load, so
// buildAuthResponse's dataplane.Get() reaches a dataplane whose SPD can be read back.
const spdBackendName = "ike-engine-test-spd"

var (
	spdBackendOnce sync.Once
	spdBackendErr  error
	// spdBackendActive is the fake the registered factory hands out. Tests in a package
	// run sequentially, and each installs its own before loading.
	spdBackendActive *spdDP
)

// useSPDDataplane makes dataplane.Get() answer with a fresh SPD-modeling fake for the
// duration of one test.
//
// buildAuthResponse reads the process-wide dataplane rather than an injected one, so the
// three rollback arms cannot be reached any other way.
func useSPDDataplane(t *testing.T) *spdDP {
	t.Helper()
	spdBackendOnce.Do(func() {
		spdBackendErr = dataplane.Register(spdBackendName, func() (dataplane.Dataplane, error) {
			return spdBackendActive, nil
		})
	})
	if spdBackendErr != nil {
		t.Fatalf("register the fake dataplane: %v", spdBackendErr)
	}
	dp := newSPDDP()
	spdBackendActive = dp
	if err := dataplane.Load(spdBackendName); err != nil {
		t.Fatalf("load the fake dataplane: %v", err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close the fake dataplane: %v", err)
		}
		spdBackendActive = nil
	})
	return dp
}

// rollbackFixture is one established responder session whose live Child SA is installed in
// the fake SPD, ready for a SECOND buildAuthResponse that will fail.
type rollbackFixture struct {
	ps   *PeerSession
	resp *SA
	dp   *spdDP
	live *ChildSA
}

// newRollbackFixture runs a real PSK handshake against the SPD fake, so the live Child SA
// and its two policies are installed exactly as production installs them.
//
// The parallel re-initiation this models reaches buildAuthResponse a second time while the
// first Child SA is still carrying traffic. The second install upserts the same selector,
// so the kernel holds ONE policy per direction shared by both pairs.
func newRollbackFixture(t *testing.T) rollbackFixture {
	t.Helper()
	dp := useSPDDataplane(t)
	_, resp, ps := establishPSK(t)

	live := ps.getChildSA()
	if live == nil {
		t.Fatal("the handshake installed no Child SA, so there is no survivor to protect")
	}
	for _, dir := range spdPolicyDirs {
		if !dp.hasPolicy(live, dir) {
			t.Fatalf("the handshake left no %s policy, so the fixture cannot show one surviving", dirName(dir))
		}
	}
	// A re-initiation carries the initiator's own fresh ESP SPI. Distinct SPIs keep the
	// rolled-back pair's states apart from the live pair's in the fake SAD, so the
	// rollback cannot be credited with removing a state it never installed.
	resp.ChildOutboundSPI = 0x77770001
	return rollbackFixture{ps: ps, resp: resp, dp: dp, live: live}
}

// assertRollbackKeptPolicy checks the one invariant all three rollback arms share.
func (f rollbackFixture) assertRollbackKeptPolicy(t *testing.T, err error, wantErr string) {
	t.Helper()
	if err == nil {
		t.Fatal("buildAuthResponse succeeded, so no rollback ran and the test proves nothing")
	}
	if !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("buildAuthResponse failed with %v, want a failure naming %q: a different arm rolled back", err, wantErr)
	}
	for _, dir := range spdPolicyDirs {
		if !f.dp.hasPolicy(f.live, dir) {
			t.Errorf("the rollback removed the %s policy the live tunnel still answers to: its traffic now leaves in the clear",
				dirName(dir))
		}
	}
	if !f.dp.states[f.live.InboundSPI] || !f.dp.states[f.live.OutboundSPI] {
		t.Error("the rollback removed the live Child SA's states")
	}
	// Presence THEN absence, never absence alone. createFirstChildSA tolerates a
	// dataplane that refuses the install, so "no state for the abandoned SPI" would
	// otherwise read the same whether the rollback removed it or nothing was ever
	// installed -- and that reading would make the policy assertion above vacuous,
	// because the live tunnel's own policy would still be there untouched.
	if !f.dp.everInstalled[f.resp.ChildOutboundSPI] {
		t.Fatalf("the abandoned Child SA's outbound state (spi %#x) was never installed, so no rollback ran and this test proves nothing",
			f.resp.ChildOutboundSPI)
	}
	if f.dp.states[f.resp.ChildOutboundSPI] {
		t.Error("the rollback left the abandoned Child SA's outbound state installed")
	}
}

// VALIDATES: buildAuthResponse's AUTH-computation rollback keeps the live tunnel's policies.
// PREVENTS: an IKE_AUTH that fails at the AUTH payload blackholing a tunnel that is up.
func TestBuildAuthResponseAuthRollbackKeepsLivePolicy(t *testing.T) {
	log := slogutil.DiscardLogger()
	f := newRollbackFixture(t)

	// computePSKAuth returns errNoPSK for an empty secret, which is the first arm.
	f.resp.PeerCfg.Auth.PSK = ""

	_, _, err := f.ps.buildAuthResponse(f.resp, 2, nil, nil, nil, false, log)
	f.assertRollbackKeptPolicy(t, err, "pre-shared")
}

// VALIDATES: buildAuthResponse's certificate-payload rollback keeps the live tunnel's policies.
// PREVENTS: the same blackhole reached through the X.509 branch instead of the AUTH one.
func TestBuildAuthResponseCertRollbackKeepsLivePolicy(t *testing.T) {
	log := slogutil.DiscardLogger()
	wpcChain(t, 1)
	f := newRollbackFixture(t)

	// computeX509Auth succeeds from the fixture's PKI entry, so the failure lands on the
	// cert-payload arm below it: hash-and-url with no URL to publish at.
	f.resp.PeerCfg.Auth = wpcAuth()
	f.resp.PeerCfg.Auth.HashAndURL = true
	f.resp.PeerCfg.Auth.CertificateURL = ""

	_, _, err := f.ps.buildAuthResponse(f.resp, 2, nil, nil, nil, false, log)
	f.assertRollbackKeptPolicy(t, err, "certificate-url")
}

// VALIDATES: buildAuthResponse's response-build rollback keeps the live tunnel's policies.
// PREVENTS: the same blackhole reached from the last arm, after both payload halves
// succeeded.
func TestBuildAuthResponseBuildRollbackKeepsLivePolicy(t *testing.T) {
	log := slogutil.DiscardLogger()
	f := newRollbackFixture(t)

	// The responder encrypts with SK_er. A key of a length AES has no cipher for fails
	// inside buildSKMessageCBCWithMsgID, which nothing before it reads: the Child SA keys
	// come from SK_d and the AUTH from the PRF over the PSK.
	f.resp.SKKeys.SK_er = []byte{1, 2, 3}

	_, _, err := f.ps.buildAuthResponse(f.resp, 2, nil, nil, nil, false, log)
	f.assertRollbackKeptPolicy(t, err, "key size")
}

// VALIDATES: the same rollback DOES remove both policies when no other Child SA answers to
// them, which is the ordinary first handshake.
// PREVENTS: the three tests above passing because the rollback stopped removing anything.
func TestBuildAuthResponseRollbackRemovesPolicyWhenNothingElseSharesIt(t *testing.T) {
	log := slogutil.DiscardLogger()
	f := newRollbackFixture(t)

	// No survivor: the owner loop's Child SA is gone, as it is on a first handshake.
	abandoned := f.live
	f.ps.setChildSA(nil)
	f.resp.PeerCfg.Auth.PSK = ""

	if _, _, err := f.ps.buildAuthResponse(f.resp, 2, nil, nil, nil, false, log); err == nil {
		t.Fatal("buildAuthResponse succeeded, so no rollback ran")
	}
	for _, dir := range spdPolicyDirs {
		if f.dp.hasPolicy(abandoned, dir) {
			t.Errorf("the %s policy outlived the only Child SA that answered to it", dirName(dir))
		}
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
