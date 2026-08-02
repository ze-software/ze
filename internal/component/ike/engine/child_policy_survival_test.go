package engine

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// spdDP models the kernel's SPD and SAD as SETS of live entries, which the other fakes
// in this package do not: bypassDP and mockDP record CALLS.
//
// A call log can only answer "was a removal attempted". The question these tests exist
// to ask is "is the policy still installed", and the two answers differ precisely when
// two Child SAs share one selector: the retired pair's teardown must issue no removal,
// and the survivor's policy must still be there afterwards.
//
// InstallPolicy UPSERTS, exactly as xfrmBackend.InstallPolicy does, so a second Child SA
// on the same selector replaces the entry rather than adding a second one. That is the
// kernel behavior the whole defect turns on.
type spdDP struct {
	policies map[spdKey]dataplane.SPParams
	states   map[uint32]bool

	// everInstalled records every SPI InstallSA has seen, including SPIs later removed.
	// states answers "is it installed NOW"; this answers "was it installed at all".
	//
	// A rollback test needs both. Asserting only that a state is absent cannot tell a
	// real removal from an install that never happened, and createFirstChildSA tolerates
	// a dataplane that refuses the install, so the absent-only reading is reachable.
	everInstalled map[uint32]bool
}

// spdKey is a policy's identity: everything the kernel compares, and nothing else. The
// template (tunnel endpoints, reqid, priority) is deliberately absent, because the
// kernel does not use it to tell one policy from another.
type spdKey struct {
	src, dst string
	dir      dataplane.SADir
}

func newSPDDP() *spdDP {
	return &spdDP{
		policies:      make(map[spdKey]dataplane.SPParams),
		states:        make(map[uint32]bool),
		everInstalled: make(map[uint32]bool),
	}
}

func spdKeyOf(p dataplane.SPParams) spdKey {
	key := spdKey{dir: p.Dir}
	if p.Src != nil {
		key.src = p.Src.String()
	}
	if p.Dst != nil {
		key.dst = p.Dst.String()
	}
	return key
}

func (d *spdDP) InstallSA(p dataplane.SAParams) error {
	d.states[p.SPI] = true
	d.everInstalled[p.SPI] = true
	return nil
}

func (d *spdDP) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	delete(d.states, spi)
	return nil
}

func (d *spdDP) InstallPolicy(p dataplane.SPParams) error {
	d.policies[spdKeyOf(p)] = p
	return nil
}

func (d *spdDP) RemovePolicy(src, dst *net.IPNet, dir dataplane.SADir) error {
	delete(d.policies, spdKey{src: src.String(), dst: dst.String(), dir: dir})
	return nil
}

func (d *spdDP) RemovePolicyParams(p dataplane.SPParams) error {
	delete(d.policies, spdKeyOf(p))
	return nil
}

func (d *spdDP) ListSAs(_ uint32) ([]dataplane.SAInfo, error) { return nil, nil }
func (d *spdDP) Close() error                                 { return nil }

// dirName names a direction for a failure message. dataplane.SADir is a bare uint8, so
// %v prints a number a reader then has to look up.
func dirName(dir dataplane.SADir) string {
	if dir == dataplane.SADirIn {
		return "inbound"
	}
	return "outbound"
}

// hasPolicy reports whether a policy for this Child SA's direction is installed.
func (d *spdDP) hasPolicy(child *ChildSA, dir dataplane.SADir) bool {
	_, ok := d.policies[spdKeyOf(childPolicyParams(child, dir))]
	return ok
}

// installTestChild puts one Child SA's two states and two policies into the fake, the
// way installChildSA does, without needing negotiated keys or a proposal.
func installTestChild(t *testing.T, d *spdDP, child *ChildSA) {
	t.Helper()
	if err := d.InstallSA(dataplane.SAParams{SPI: child.InboundSPI}); err != nil {
		t.Fatalf("install inbound state: %v", err)
	}
	if err := d.InstallSA(dataplane.SAParams{SPI: child.OutboundSPI}); err != nil {
		t.Fatalf("install outbound state: %v", err)
	}
	for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
		if err := d.InstallPolicy(childPolicyParams(child, dir)); err != nil {
			t.Fatalf("install %s policy: %v", dirName(dir), err)
		}
	}
}

// testPolicyOwner is the configured peer name every Child SA in this file belongs to.
// One PeerSession serves one peer, so every child a single session installs shares it;
// the two-owner collision is a dataplane concern and is proven there
// (dataplane/policy_owner_test.go).
const testPolicyOwner = "site-a"

// testChildOnSelector builds a Child SA carrying the ordinary site-to-site selector,
// the one two peers and two parallel handshakes all land on.
func testChildOnSelector(inSPI, outSPI uint32) *ChildSA {
	_, any4, _ := net.ParseCIDR("0.0.0.0/0")
	return &ChildSA{
		InboundSPI:  inSPI,
		OutboundSPI: outSPI,
		LocalAddr:   net.ParseIP("10.0.0.1"),
		RemoteAddr:  net.ParseIP("10.0.0.2"),
		TSLocal:     any4,
		TSRemote:    any4,
		Owner:       testPolicyOwner,
		Mode:        modeTunnel,
		ReqID:       defaultReqID,
	}
}

// VALIDATES: after a peer re-initiates while the IKE SA is up, the Child SA that
// resolvePendingAfterOwnerLoop promotes still has BOTH of its kernel policies.
// PREVENTS: the promoted Child SA coming up with states and no policy, which sends
// outbound traffic in the clear and makes the kernel drop inbound ESP.
//
// The parallel handshake's child is installed by finishResponderEstablish BEFORE the
// supersede token reaches maintainSA, and its policies upsert the selector the old
// owner's child already holds. So cleanupChild runs while one policy per direction is
// shared, and it must not take the survivor's policy with it.
func TestCleanupChildKeepsPolicyForParallelPendingChild(t *testing.T) {
	log := slogutil.DiscardLogger()
	dp := newSPDDP()
	ps := &PeerSession{peerName: "site-a"}

	live := testChildOnSelector(0x1001, 0x1002)
	installTestChild(t, dp, live)
	ps.setChildSA(live)

	// The parallel handshake authenticated and installed its child in the pending slot.
	pending := testChildOnSelector(0x2001, 0x2002)
	installTestChild(t, dp, pending)
	ps.setPendingChild(pending)

	ps.cleanupChild(dp, nil, log)

	for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
		if !dp.hasPolicy(pending, dir) {
			t.Errorf("the promoted Child SA has no %s policy: its traffic would leave unprotected", dirName(dir))
		}
	}
	if dp.states[live.InboundSPI] || dp.states[live.OutboundSPI] {
		t.Error("the superseded Child SA's states were not removed")
	}
	if !dp.states[pending.InboundSPI] || !dp.states[pending.OutboundSPI] {
		t.Error("the promoted Child SA's states were removed")
	}
}

// VALIDATES: cleanupChild DOES remove the policies when no other Child SA answers to
// them, so nothing is leaked at an ordinary teardown.
// PREVENTS: the test above passing vacuously because the removal never happens at all.
func TestCleanupChildRemovesPolicyWhenNothingElseSharesIt(t *testing.T) {
	log := slogutil.DiscardLogger()
	dp := newSPDDP()
	ps := &PeerSession{peerName: "site-a"}

	live := testChildOnSelector(0x1001, 0x1002)
	installTestChild(t, dp, live)
	ps.setChildSA(live)

	ps.cleanupChild(dp, nil, log)

	for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
		if dp.hasPolicy(live, dir) {
			t.Errorf("the %s policy outlived the only Child SA that answered to it", dirName(dir))
		}
	}
	if len(dp.states) != 0 {
		t.Errorf("states left installed: %v", dp.states)
	}
}

// VALIDATES: a Child SA whose selector NOBODY else shares still loses its policies even
// when a pending child exists on a different selector.
// PREVENTS: firstSharingSelector degenerating into "keep the policy whenever any other
// child exists", which would leak a policy on every parallel handshake.
func TestCleanupChildRemovesPolicyWhenPendingChildUsesAnotherSelector(t *testing.T) {
	log := slogutil.DiscardLogger()
	dp := newSPDDP()
	ps := &PeerSession{peerName: "site-a"}

	live := testChildOnSelector(0x1001, 0x1002)
	installTestChild(t, dp, live)
	ps.setChildSA(live)

	pending := testChildOnSelector(0x2001, 0x2002)
	_, other, _ := net.ParseCIDR("192.0.2.0/24")
	pending.TSRemote = other
	installTestChild(t, dp, pending)
	ps.setPendingChild(pending)

	ps.cleanupChild(dp, nil, log)

	for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
		if dp.hasPolicy(live, dir) {
			t.Errorf("the %s policy outlived its only Child SA because an unrelated pending child existed", dirName(dir))
		}
		if !dp.hasPolicy(pending, dir) {
			t.Errorf("the pending Child SA lost its own %s policy", dirName(dir))
		}
	}
}

// VALIDATES: reaping an abandoned parallel handshake leaves the LIVE Child SA's policies
// installed.
// PREVENTS: a peer that starts a second handshake and walks away blackholing the tunnel
// that is still up, one responderHandshakeTimeout later.
func TestReapStalePendingKeepsLiveChildPolicy(t *testing.T) {
	log := slogutil.DiscardLogger()
	dp := newSPDDP()
	table := NewSATable()
	ps := &PeerSession{peerName: "site-a"}
	ps.responderBusy.Store(true)

	live := testChildOnSelector(0x1001, 0x1002)
	installTestChild(t, dp, live)
	ps.setChildSA(live)

	pending := testSA()
	pending.IsInitiator = false
	pending.InitiatorSPI = [8]byte{1, 1, 1, 1, 1, 1, 1, 1}
	pending.ResponderSPI = [8]byte{2, 2, 2, 2, 2, 2, 2, 2}
	pending.CreatedAt = time.Now().Add(-2 * responderHandshakeTimeout)
	pending.State = StateSAInitReceived
	table.Insert(pending)
	ps.setPendingSA(pending)

	pendingChild := testChildOnSelector(0x2001, 0x2002)
	installTestChild(t, dp, pendingChild)
	ps.setPendingChild(pendingChild)

	ps.reapStalePending(time.Now(), table, dp, log)

	for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
		if !dp.hasPolicy(live, dir) {
			t.Errorf("reaping an abandoned handshake removed the live tunnel's %s policy", dirName(dir))
		}
	}
	if dp.states[pendingChild.InboundSPI] || dp.states[pendingChild.OutboundSPI] {
		t.Error("the abandoned handshake's states were not removed")
	}
	if !dp.states[live.InboundSPI] || !dp.states[live.OutboundSPI] {
		t.Error("the live Child SA's states were removed")
	}
}

// VALIDATES: childPolicyParams builds the delete selector from the same fields as the
// install selector, in both directions, and carries the owner.
// PREVENTS: a removal that names a different selector from the install and so either
// misses the policy or matches a wider one.
func TestChildPolicyParamsMirrorsDirection(t *testing.T) {
	child := testChildOnSelector(1, 2)

	in := childPolicyParams(child, dataplane.SADirIn)
	out := childPolicyParams(child, dataplane.SADirOut)

	if in.Src.String() != child.TSRemote.String() || in.Dst.String() != child.TSLocal.String() {
		t.Errorf("inbound selector = %v -> %v, want %v -> %v", in.Src, in.Dst, child.TSRemote, child.TSLocal)
	}
	if out.Src.String() != child.TSLocal.String() || out.Dst.String() != child.TSRemote.String() {
		t.Errorf("outbound selector = %v -> %v, want %v -> %v", out.Src, out.Dst, child.TSLocal, child.TSRemote)
	}
	if !in.TunnelSrc.Equal(child.RemoteAddr) || !in.TunnelDst.Equal(child.LocalAddr) {
		t.Errorf("inbound tunnel endpoints = %v -> %v, want %v -> %v", in.TunnelSrc, in.TunnelDst, child.RemoteAddr, child.LocalAddr)
	}
	if !out.TunnelSrc.Equal(child.LocalAddr) || !out.TunnelDst.Equal(child.RemoteAddr) {
		t.Errorf("outbound tunnel endpoints = %v -> %v, want %v -> %v", out.TunnelSrc, out.TunnelDst, child.LocalAddr, child.RemoteAddr)
	}
	if in.Owner != testPolicyOwner || out.Owner != testPolicyOwner {
		t.Errorf("owner = %q / %q, want site-a on both: an unowned policy cannot be told from another peer's", in.Owner, out.Owner)
	}
}
