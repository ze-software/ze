// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- inbound tunnel forwarding policies.
// Related: child_forward_policy_linux_test.go -- actual decrypted packet forwarding.

package engine

import (
	"errors"
	"maps"
	"net"
	"reflect"
	"runtime"
	"slices"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

type forwardPolicyDP struct {
	*spdDP
	reject dataplane.SADir
}

func (d *forwardPolicyDP) RequiresForwardPolicy() bool { return true }

var errForwardPolicyInstall = errors.New("refused policy installation")

func (d *forwardPolicyDP) InstallPolicy(p dataplane.SPParams) error {
	if p.Dir == d.reject {
		return errForwardPolicyInstall
	}
	return d.spdDP.InstallPolicy(p)
}

// TestChildForwardPolicySurvivesRekey proves that the installed transit policy
// has the inbound selector and SA, survives the old Child's retirement, and is
// removed with the last Child. It inspects the backend's live policy set.
func TestChildForwardPolicySurvivesRekey(t *testing.T) {
	dp := &forwardPolicyDP{spdDP: newSPDDP()}
	log := slogutil.DiscardLogger()
	group := testESPGroup()
	child, err := createFirstChildSA(wideSA("forward-peer", "10.0.0.2"), group,
		"10.0.0.1", "10.0.0.2", testChildIfID, dp, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Clear)
	if len(dp.policies) != 3 {
		t.Fatalf("installed %d policies, want in/fwd/out", len(dp.policies))
	}
	inbound := dp.policies[spdKeyOf(childPolicies(child, dataplane.SADirIn)[0])]
	forward, ok := dp.policies[spdKeyOf(childPolicies(child, dataplane.SADirFwd)[0])]
	if !ok {
		t.Fatal("the backend has no policy protecting forwarded tunnel traffic")
	}
	inbound.Dir = dataplane.SADirFwd
	if !reflect.DeepEqual(inbound, forward) {
		t.Fatal("forward policy differs from the inbound selectors, template or owner")
	}

	replacement := newRekeyedChild(child, 0x71920001, 0x71920002, child.Keys, true, nil, child.Mode)
	if err := installChildSA(replacement, group.Proposals[0], dp, log); err != nil {
		t.Fatal(err)
	}
	if replacement.installPrevious != nil {
		t.Fatal("successful install retained its predecessor")
	}
	removeChildSAExcept(child, replacement, dp, log)
	if len(dp.policies) != 3 {
		t.Fatalf("retiring the old Child left %d policies, want all three", len(dp.policies))
	}
	if !dp.hasPolicy(replacement, dataplane.SADirFwd) {
		t.Fatal("retiring the old Child removed the replacement's forward policy")
	}
	removeChildSA(replacement, dp, log)
	if len(dp.policies) != 0 {
		t.Fatalf("last Child teardown stranded %d policies", len(dp.policies))
	}
	if len(dp.states) != 0 {
		t.Fatalf("last Child teardown stranded %d states", len(dp.states))
	}
}

// TestChildForwardPolicyInstallRollback proves that refusal on either the new
// receive path or the final outbound path leaves no partially installed tunnel.
func TestChildForwardPolicyInstallRollback(t *testing.T) {
	for _, direction := range []dataplane.SADir{dataplane.SADirFwd, dataplane.SADirOut} {
		t.Run(dirLabel(direction), func(t *testing.T) {
			dp := &forwardPolicyDP{spdDP: newSPDDP(), reject: direction}
			_, err := createFirstChildSA(wideSA("forward-peer", "10.0.0.2"), testESPGroup(),
				"10.0.0.1", "10.0.0.2", testChildIfID, dp, slogutil.DiscardLogger())
			if !errors.Is(err, errForwardPolicyInstall) {
				t.Fatalf("install error = %v, want the policy refusal", err)
			}
			if len(dp.policies) != 0 {
				t.Fatalf("failed install stranded %d policies", len(dp.policies))
			}
			if len(dp.states) != 0 {
				t.Fatalf("failed install stranded %d states", len(dp.states))
			}
		})
	}
}

// TestTransportChildNeedsNoForwardPolicy keeps transport-mode policy semantics
// unchanged even on a backend with a separate tunnel forwarding lookup.
func TestTransportChildNeedsNoForwardPolicy(t *testing.T) {
	dp := &forwardPolicyDP{spdDP: newSPDDP()}
	sa := testSA()
	sa.UseTransportMode = true
	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2",
		testChildIfID, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Clear)
	if len(dp.policies) != 2 {
		t.Fatalf("transport Child installed %d policies, want in/out only", len(dp.policies))
	}
	if dp.hasPolicy(child, dataplane.SADirFwd) {
		t.Fatal("transport Child installed a tunnel forwarding policy")
	}
}

type forwardOwnedDP struct{ *ownedDP }

func (d *forwardOwnedDP) RequiresForwardPolicy() bool { return true }

// TestChildForwardPolicyRefusesForeignOwner reaches the forward install after
// the inbound install succeeded. Its refusal must roll back only the new
// inbound policy, never the transit policy another peer still owns.
func TestChildForwardPolicyRefusesForeignOwner(t *testing.T) {
	dp := &forwardOwnedDP{ownedDP: newOwnedDP()}
	probe := testChildOnSelector(0x71930001, 0x71930002)
	probe.IfID = testChildIfID
	occupied := childPolicies(probe, dataplane.SADirFwd)[0]
	if err := dp.InstallPolicy(occupied); err != nil {
		t.Fatal(err)
	}
	_, err := createFirstChildSA(wideSA("site-b", "10.0.0.2"), testESPGroup(),
		"10.0.0.1", "10.0.0.2", testChildIfID, dp, slogutil.DiscardLogger())
	if !errors.Is(err, errForeignPolicy) {
		t.Fatalf("install error=%v, want foreign forward-policy ownership refusal", err)
	}
	if len(dp.policies) != 1 {
		t.Fatalf("failed install left %d policies, want only the foreign forward policy", len(dp.policies))
	}
	held, ok := dp.policies[ownedKeyOf(occupied)]
	if !ok {
		t.Fatal("rollback removed another peer's forward policy")
	}
	if !reflect.DeepEqual(held, occupied) {
		t.Fatal("refused install changed another peer's forward policy")
	}
	if len(dp.states) != 0 {
		t.Fatalf("failed install stranded %d states", len(dp.states))
	}
}

// TestChildFailedRekeyPreservesForwardPolicies rejects the replacement's OUT
// policy after IN and FWD were upserted. The old Child must retain its complete
// policy/state set and owners; failure must remove only the replacement states.
func TestChildFailedRekeyPreservesForwardPolicies(t *testing.T) {
	dp := &forwardPolicyDP{spdDP: newSPDDP()}
	log := slogutil.DiscardLogger()
	group := testESPGroup()
	child, err := createFirstChildSA(wideSA("forward-peer", "10.0.0.2"), group,
		"10.0.0.1", "10.0.0.2", testChildIfID, dp, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Clear)
	policies := maps.Clone(dp.policies)
	states := maps.Clone(dp.states)
	replacement := newRekeyedChild(child, 0x71940001, 0x71940002, child.Keys, true, nil, child.Mode)
	dp.reject = dataplane.SADirOut
	replacement.RemoteAddr = net.ParseIP("10.0.0.3")
	err = installChildSA(replacement, group.Proposals[0], dp, log)
	if !errors.Is(err, errForwardPolicyInstall) {
		t.Fatalf("replacement error=%v, want outbound-policy refusal", err)
	}
	if replacement.installPrevious != nil {
		t.Fatal("refused install retained its predecessor")
	}
	if !reflect.DeepEqual(dp.policies, policies) {
		t.Errorf("failed rekey changed the old Child's policies or owners: have %d, want %d",
			len(dp.policies), len(policies))
	}
	if !reflect.DeepEqual(dp.states, states) {
		t.Errorf("failed rekey changed the old Child's states: have %v, want %v", dp.states, states)
	}
	for _, direction := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirFwd, dataplane.SADirOut} {
		if !dp.hasPolicy(child, direction) {
			t.Errorf("old Child lost its %s traffic path after replacement refusal", dirLabel(direction))
		}
	}
}

// TestChildForwardPolicyCoversEverySelector exercises negotiated multi-selector
// installation and final removal, rather than only enumerating builder output.
func TestChildForwardPolicyCoversEverySelector(t *testing.T) {
	dp := &forwardPolicyDP{spdDP: newSPDDP()}
	reg := newInnerSelectorRegistry()
	child := mustInnerChild(t, innerPeerSA(t, reg, "forward-peer", "198.18.0.7"), dp,
		[]string{"10.1.2.3/32", "10.1.2.4/32"}, []string{"10.2.0.0/24"})
	t.Cleanup(child.Clear)
	if len(dp.policies) != 10 {
		t.Fatalf("two negotiated pairs installed %d policies, want ten (in, fwd, out, outbound fwd and delivery each)", len(dp.policies))
	}
	want := map[string]bool{"10.1.2.3/32": false, "10.1.2.4/32": false}
	wantOutbound := map[string]bool{"10.1.2.3/32": false, "10.1.2.4/32": false}
	for _, policy := range dp.policies {
		if policy.Dir != dataplane.SADirFwd {
			continue
		}
		if policy.Action == dataplane.SPActionBypass {
			// The outbound forward entry of a pair: local to remote (RFC 4301 Section 5.1).
			destination := policy.Dst.String()
			if _, ok := wantOutbound[destination]; !ok || policy.Src.String() != "10.2.0.0/24" {
				t.Fatalf("unexpected outbound forward selector %s -> %s", policy.Src, policy.Dst)
			}
			wantOutbound[destination] = true
			continue
		}
		source := policy.Src.String()
		if _, ok := want[source]; !ok {
			t.Fatalf("unexpected forward selector %s", source)
		}
		want[source] = true
		if policy.Dst.String() != "10.2.0.0/24" || policy.Owner != "forward-peer" {
			t.Fatalf("forward policy lost its local selector or owner: %s, %q", policy.Dst, policy.Owner)
		}
	}
	for source, installed := range want {
		if !installed {
			t.Errorf("no forward policy installed for negotiated selector %s", source)
		}
	}
	for destination, installed := range wantOutbound {
		if !installed {
			t.Errorf("no outbound forward entry installed for negotiated selector %s", destination)
		}
	}
	removeChildSA(child, dp, slogutil.DiscardLogger())
	if len(dp.policies) != 0 || len(dp.states) != 0 || reg.holdings() != 0 {
		t.Fatal("multi-selector Child teardown left installed or claimed resources")
	}
}

type forwardStrandedDP struct {
	*forwardPolicyDP
	failForward     bool
	forwardRemovals int
}

// RemovePolicyParams fails the removal of the inbound transit policy, the FWD entry
// carrying the SA's template. The template-free outbound forward entry is removed
// normally, so a stranding leaves exactly the policy the registry adopts.
func (d *forwardStrandedDP) RemovePolicyParams(policy dataplane.SPParams) error {
	if policy.Dir == dataplane.SADirFwd && policy.Action == dataplane.SPActionProtect {
		d.forwardRemovals++
		if d.failForward {
			return syscall.EIO
		}
	}
	return d.forwardPolicyDP.RemovePolicyParams(policy)
}

// TestChildForwardPolicyAdoptsStrandedForward leaves only FWD behind, then
// re-establishes the same peer. A later successful retry must not delete the
// transit policy the new Child adopted.
func TestChildForwardPolicyAdoptsStrandedForward(t *testing.T) {
	dp := &forwardStrandedDP{forwardPolicyDP: &forwardPolicyDP{spdDP: newSPDDP()}, failForward: true}
	log := slogutil.DiscardLogger()
	reg := newInnerSelectorRegistry()
	first := mustInnerChild(t, innerPeerSA(t, reg, "forward-peer", "198.18.0.7"), dp,
		[]string{"10.1.2.3/32"}, []string{"10.2.0.0/24"})
	t.Cleanup(first.Clear)
	policies := childInstalledPolicies(first)
	t.Cleanup(func() {
		for i := range policies {
			spdLeft.record(&policies[i], nil)
		}
	})
	removeChildSA(first, dp, log)
	if len(dp.policies) != 1 || !dp.hasPolicy(first, dataplane.SADirFwd) {
		t.Fatal("setup did not strand exactly the forward policy")
	}
	if reg.holdings() != 1 {
		t.Fatal("failed forward-policy removal lost its selector claim")
	}
	next := mustInnerChild(t, innerPeerSA(t, reg, "forward-peer", "198.18.0.7"), dp,
		[]string{"10.1.2.3/32"}, []string{"10.2.0.0/24"})
	t.Cleanup(next.Clear)
	if reg.holdings() != 1 {
		t.Fatal("re-establishment did not adopt the forward-only stranded record")
	}
	removals := dp.forwardRemovals
	dp.failForward = false
	reg.retryStranded()
	if dp.forwardRemovals != removals {
		t.Fatal("retry tried to remove the live Child's adopted forward policy")
	}
	if len(dp.policies) != 5 || !dp.hasPolicy(next, dataplane.SADirFwd) {
		t.Fatal("retry removed the live Child's transit protection")
	}
	removeChildSA(next, dp, log)
	if len(dp.policies) != 0 || len(dp.states) != 0 || reg.holdings() != 0 {
		t.Fatal("adopted Child teardown left installed or claimed resources")
	}
}

// TestChildRekeyPredecessorCleared covers exits before or without policy writes.
// Successful installation and policy refusal are checked by the rekey tests above.
func TestChildRekeyPredecessorCleared(t *testing.T) {
	for _, outcome := range []string{"no-dataplane", "unsupported", "selector-refused"} {
		t.Run(outcome, func(t *testing.T) {
			dp := &forwardPolicyDP{spdDP: newSPDDP()}
			reg := newInnerSelectorRegistry()
			old := mustInnerChild(t, innerPeerSA(t, reg, "forward-peer", "198.18.0.7"), dp,
				[]string{"10.1.2.3/32"}, []string{"10.2.0.0/24"})
			t.Cleanup(old.Clear)
			replacement := newRekeyedChild(old, 0x71960001, 0x71960002, old.Keys, true, nil, old.Mode)
			var backend dataplane.Dataplane
			switch outcome {
			case "unsupported":
				backend = &unsupportedDP{err: dataplane.ErrNotSupported}
			case "selector-refused":
				backend = dp
				replacement.Owner = "foreign-peer"
			}
			err := installChildTolerant(replacement, testESPGroup().Proposals[0], backend, slogutil.DiscardLogger())
			if outcome == "selector-refused" {
				if !errors.Is(err, errTSUnacceptable) {
					t.Fatalf("install error=%v, want selector refusal", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if replacement.installPrevious != nil {
				t.Fatal("installation exit retained its predecessor")
			}
			if !old.ESPInstalled || len(dp.policies) != 5 || len(dp.states) != 2 {
				t.Fatal("installation without policy writes disturbed the old Child")
			}
		})
	}
}

type forwardRestoreRefusalDP struct {
	*forwardPolicyDP
	oldSPI uint32
}

func (d *forwardRestoreRefusalDP) InstallPolicy(policy dataplane.SPParams) error {
	if policy.Dir == dataplane.SADirOut {
		return dataplane.ErrNotSupported
	}
	if policy.Dir == dataplane.SADirFwd && policy.SAID == d.oldSPI {
		return syscall.EIO
	}
	return d.forwardPolicyDP.InstallPolicy(policy)
}

// TestChildFailedRekeyReportsRestoreFailure proves that unsupported installation
// does not swallow a failed restoration, release the old selector claim or delete
// the policy whose restoration failed.
func TestChildFailedRekeyReportsRestoreFailure(t *testing.T) {
	dp := &forwardPolicyDP{spdDP: newSPDDP()}
	reg := newInnerSelectorRegistry()
	old := mustInnerChild(t, innerPeerSA(t, reg, "forward-peer", "198.18.0.7"), dp,
		[]string{"10.1.2.3/32"}, []string{"10.2.0.0/24"})
	t.Cleanup(old.Clear)
	states := maps.Clone(dp.states)
	replacement := newRekeyedChild(old, 0x71970001, 0x71970002, old.Keys, true, nil, old.Mode)
	refusing := &forwardRestoreRefusalDP{forwardPolicyDP: dp, oldSPI: old.InboundSPI}
	err := installChildTolerant(replacement, testESPGroup().Proposals[0], refusing, slogutil.DiscardLogger())
	if !errors.Is(err, dataplane.ErrNotSupported) || !errors.Is(err, syscall.EIO) ||
		!errors.Is(err, errChildPolicyRollback) {
		t.Fatalf("install error=%v, want explicit install and rollback failures", err)
	}
	if espInstalled(map[string]*PeerSession{old.Owner: {childSA: old}}, old.Owner) {
		t.Fatal("failed restoration left the old Child reported healthy")
	}
	if replacement.installPrevious != nil {
		t.Fatal("failed restoration retained its predecessor")
	}
	if !reflect.DeepEqual(dp.states, states) {
		t.Fatal("failed restoration changed the old Child's states")
	}
	if reg.holdings() != 1 {
		t.Fatal("failed restoration released the old selector claim or retained the replacement claim")
	}
	forward := childPolicies(old, dataplane.SADirFwd)[0]
	installed, ok := dp.policies[spdKeyOf(forward)]
	if !ok || installed.Owner != old.Owner {
		t.Fatal("failed restoration deleted or transferred the shared forward policy")
	}
	if installed.SAID != replacement.InboundSPI {
		t.Fatal("restore-refusal fixture did not leave the replacement template installed")
	}
	if slices.Contains(spdLeft.describe(&xfrmCommandDP{}), dataplane.XFRMPolicyDeleteCommand(&forward)) {
		t.Fatal("live predecessor policy was incorrectly scheduled for deletion")
	}
}

// TestChildForwardRollbackStrandsFailedRemoval retains both ownership and the
// existing SPD ledger when rollback cannot remove a genuinely new FWD policy.
func TestChildForwardRollbackStrandsFailedRemoval(t *testing.T) {
	dp := &forwardStrandedDP{forwardPolicyDP: &forwardPolicyDP{
		spdDP: newSPDDP(), reject: dataplane.SADirOut,
	}, failForward: true}
	reg := newInnerSelectorRegistry()
	_, err := innerAuthChild(t, innerPeerSA(t, reg, "forward-peer", "198.18.0.7"), dp,
		[]string{"10.1.2.3/32"}, []string{"10.2.0.0/24"})
	if !errors.Is(err, errForwardPolicyInstall) || !errors.Is(err, errChildPolicyRollback) {
		t.Fatalf("install error=%v, want explicit install and cleanup failures", err)
	}
	if len(dp.policies) != 1 || reg.holdings() != 1 || len(dp.states) != 0 {
		t.Fatal("failed rollback did not retain exactly the stranded forward policy and claim")
	}
	var forward dataplane.SPParams
	for _, policy := range dp.policies {
		forward = policy
	}
	t.Cleanup(func() { spdLeft.record(&forward, nil) })
	if forward.Dir != dataplane.SADirFwd {
		t.Fatal("rollback stranded a policy other than FWD")
	}
	command := dataplane.XFRMPolicyDeleteCommand(&forward)
	if !slices.Contains(spdLeft.describe(&xfrmCommandDP{}), command) {
		t.Fatal("failed rollback omitted the forward policy from the SPD ledger")
	}
	dp.failForward = false
	reg.retryStranded()
	if len(dp.policies) != 0 || reg.holdings() != 0 {
		t.Fatal("successful retry left the rollback policy or its claim behind")
	}
	if slices.Contains(spdLeft.describe(&xfrmCommandDP{}), command) {
		t.Fatal("successful retry left the forward policy in the SPD ledger")
	}
}

// TestChildFailedRekeyAfterRestoreFailure retains ownership even when the
// predecessor's previous policy restoration made its forwarding health degraded.
func TestChildFailedRekeyAfterRestoreFailure(t *testing.T) {
	dp := &forwardPolicyDP{spdDP: newSPDDP()}
	old, err := createFirstChildSA(wideSA("forward-peer", "10.0.0.2"), testESPGroup(),
		"10.0.0.1", "10.0.0.2", testChildIfID, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(old.Clear)
	policies := maps.Clone(dp.policies)
	states := maps.Clone(dp.states)
	first := newRekeyedChild(old, 0x71980001, 0x71980002, old.Keys, true, nil, old.Mode)
	refusing := &forwardRestoreRefusalDP{forwardPolicyDP: dp, oldSPI: old.InboundSPI}
	err = installChildTolerant(first, testESPGroup().Proposals[0], refusing, slogutil.DiscardLogger())
	if !errors.Is(err, errChildPolicyRollback) {
		t.Fatalf("first replacement error=%v, want restoration failure", err)
	}
	next := newRekeyedChild(old, 0x71980003, 0x71980004, old.Keys, true, nil, old.Mode)
	dp.reject = dataplane.SADirOut
	err = installChildSA(next, testESPGroup().Proposals[0], dp, slogutil.DiscardLogger())
	if !errors.Is(err, errForwardPolicyInstall) {
		t.Fatalf("second replacement error=%v, want outbound refusal", err)
	}
	if !reflect.DeepEqual(dp.policies, policies) {
		t.Errorf("second refused rekey did not restore the predecessor's policies: have %d, want %d",
			len(dp.policies), len(policies))
	}
	if !reflect.DeepEqual(dp.states, states) {
		t.Error("second refused rekey changed the predecessor's states")
	}
}

type forwardStateRefusalDP struct{ *forwardStrandedDP }

func (d *forwardStateRefusalDP) InstallSA(dataplane.SAParams) error {
	return syscall.EIO
}

// TestChildForwardAdoptionSurvivesStateFailure refuses a new install before any
// policy write. The same owner's previously stranded FWD must remain reserved
// and retryable rather than being forgotten by the tentative selector claim.
func TestChildForwardAdoptionSurvivesStateFailure(t *testing.T) {
	dp := &forwardStrandedDP{forwardPolicyDP: &forwardPolicyDP{spdDP: newSPDDP()}, failForward: true}
	reg := newInnerSelectorRegistry()
	old := mustInnerChild(t, innerPeerSA(t, reg, "forward-peer", "198.18.0.7"), dp,
		[]string{"10.1.2.3/32"}, []string{"10.2.0.0/24"})
	t.Cleanup(old.Clear)
	forward := childPolicies(old, dataplane.SADirFwd)[0]
	t.Cleanup(func() { spdLeft.record(&forward, nil) })
	removeChildSA(old, dp, slogutil.DiscardLogger())
	if len(dp.policies) != 1 || reg.holdings() != 1 {
		t.Fatal("setup did not strand one forward policy and claim")
	}
	_, err := innerAuthChild(t, innerPeerSA(t, reg, "forward-peer", "198.18.0.7"),
		&forwardStateRefusalDP{forwardStrandedDP: dp},
		[]string{"10.1.2.3/32"}, []string{"10.2.0.0/24"})
	if !errors.Is(err, syscall.EIO) {
		t.Fatalf("new install error=%v, want state refusal", err)
	}
	if reg.holdings() != 1 {
		t.Error("failed same-owner installation lost the stranded forward reservation")
	}
	_, err = innerAuthChild(t, innerPeerSA(t, reg, "foreign-peer", "198.18.0.9"),
		&forwardPolicyDP{spdDP: newSPDDP()},
		[]string{"10.1.2.3/32"}, []string{"10.2.0.0/24"})
	if !errors.Is(err, errInnerSelectorStranded) {
		t.Errorf("foreign overlapping install error=%v, want stranded-policy refusal", err)
	}
	dp.failForward = false
	reg.retryStranded()
	if len(dp.policies) != 0 || reg.holdings() != 0 {
		t.Error("successful removal retry did not clear the preserved forward policy and claim")
	}
	if slices.Contains(spdLeft.describe(&xfrmCommandDP{}), dataplane.XFRMPolicyDeleteCommand(&forward)) {
		t.Error("successful removal retry left the forward policy in the SPD ledger")
	}
}

// TestChildFailedRekeyHealthPublication exercises the published-child metrics
// reader concurrently with a real installation rollback. Run with -race.
func TestChildFailedRekeyHealthPublication(t *testing.T) {
	dp := &forwardPolicyDP{spdDP: newSPDDP()}
	old, err := createFirstChildSA(wideSA("forward-peer", "10.0.0.2"), testESPGroup(),
		"10.0.0.1", "10.0.0.2", testChildIfID, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(old.Clear)
	peers := map[string]*PeerSession{old.Owner: {childSA: old}}
	ready, stop, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		close(ready)
		for {
			select {
			case <-stop:
				return
			default:
				_ = espInstalled(peers, old.Owner)
			}
		}
	}()
	<-ready
	runtime.Gosched()
	replacement := newRekeyedChild(old, 0x71990001, 0x71990002, old.Keys, true, nil, old.Mode)
	err = installChildTolerant(replacement, testESPGroup().Proposals[0],
		&forwardRestoreRefusalDP{forwardPolicyDP: dp, oldSPI: old.InboundSPI}, slogutil.DiscardLogger())
	close(stop)
	<-done
	if !errors.Is(err, errChildPolicyRollback) {
		t.Fatalf("replacement error=%v, want restoration failure", err)
	}
	if espInstalled(peers, old.Owner) {
		t.Fatal("published metrics still report the predecessor healthy after restoration failure")
	}
}

// TestChildOutboundForwardPolicyAdmitsProtectedHostTransit proves that a policy-based
// tunnel Child SA (if_id 0) on a backend with a separate forward lookup installs the
// outbound forward entry, so a packet a protected host sends towards the tunnel reaches
// the OUT lookup that protects it under a blocking FWD default (`unmatched discard`).
//
// RFC 4301 Section 5.1, step 3a: "If there is a match, then process the packet as
// specified by the matching cache entry, i.e., BYPASS, DISCARD, or PROTECT using AH or
// ESP."
//
// Method: the production responder creator on the policy-recording fake. The entry
// must carry the OUT selector (local to remote) in FWD, as a template-free allow (no
// mode, endpoint, reqid or SA) one rank below the Child SA, owned by the peer, come
// after OUT in the install order, and leave with the Child SA.
func TestChildOutboundForwardPolicyAdmitsProtectedHostTransit(t *testing.T) {
	dp := &forwardPolicyDP{spdDP: newSPDDP()}
	log := slogutil.DiscardLogger()
	reg := newInnerSelectorRegistry()
	child := mustInnerChild(t, innerPeerSA(t, reg, "forward-peer", "198.18.0.7"), dp,
		[]string{"10.1.2.3/32"}, []string{"10.2.0.0/24"})
	t.Cleanup(child.Clear)
	out := childPolicies(child, dataplane.SADirOut)[0]
	if out.Src.String() != "10.2.0.0/24" || out.Dst.String() != "10.1.2.3/32" {
		t.Fatalf("fixture: OUT selector %s -> %s, want 10.2.0.0/24 -> 10.1.2.3/32", out.Src, out.Dst)
	}
	want := dataplane.SPParams{
		Src: out.Src, Dst: out.Dst, Dir: dataplane.SADirFwd, Action: dataplane.SPActionBypass,
		Owner: "forward-peer", Role: dataplane.PolicyRoleChildSA, IfID: 0,
		Priority: out.Priority + 1, UpperProto: out.UpperProto, SrcPort: out.SrcPort, DstPort: out.DstPort,
	}
	got, ok := dp.policies[spdKeyOf(want)]
	if !ok {
		t.Fatalf("no outbound forward entry %s -> %s: a protected host's transit meets the FWD default and is dropped", out.Src, out.Dst)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("outbound forward entry = %+v, want %+v", got, want)
	}
	if !dp.hasPolicy(child, dataplane.SADirFwd) {
		t.Fatal("the outbound forward entry replaced the inbound transit policy")
	}
	installed := childInstalledPolicies(child)
	if len(installed) != 5 || !reflect.DeepEqual(installed[3], want) || installed[2].Dir != dataplane.SADirOut {
		t.Fatalf("install order = %d policies, want in, fwd, out, the outbound forward entry, then delivery", len(installed))
	}
	// RFC 4301 Section 5.2: the decrypted packet's own leg towards the protected host.
	inbound := childPolicies(child, dataplane.SADirFwd)[0]
	delivery := dataplane.SPParams{
		Src: inbound.Src, Dst: inbound.Dst, Dir: dataplane.SADirOut, Action: dataplane.SPActionBypass,
		Owner: "forward-peer", Role: dataplane.PolicyRoleChildSA, IfID: 0,
		Priority: inbound.Priority + 1, UpperProto: inbound.UpperProto, SrcPort: inbound.SrcPort, DstPort: inbound.DstPort,
	}
	if got, ok := dp.policies[spdKeyOf(delivery)]; !ok || !reflect.DeepEqual(got, delivery) || !reflect.DeepEqual(installed[4], delivery) {
		t.Fatalf("delivery entry = %+v (installed %v), want %+v last", got, ok, delivery)
	}
	if !dp.hasPolicy(child, dataplane.SADirOut) {
		t.Fatal("the delivery entry replaced the OUT PROTECT policy")
	}
	if migrated := childMigratedPolicies(child); len(migrated) != 3 {
		t.Fatalf("MOBIKE would move %d policies, want the three template-carrying ones", len(migrated))
	}
	removeChildSA(child, dp, log)
	if len(dp.policies) != 0 {
		t.Fatalf("Child teardown left %d policies, the outbound forward and delivery entries included", len(dp.policies))
	}
}

// TestChildOutboundForwardPolicyBuiltOnlyForPolicyBasedTunnels takes the cases just
// outside the entry's condition, where no outbound forward entry may be built: a Child
// SA bound to an XFRM interface (its forwarded traffic is checked at if_id 0, where
// xfrmInterfaceExemptions answers), a Child SA with no separate forward lookup
// (transport mode, or a backend such as VPP), and a pair whose two sides are equal,
// whose entries would carry the kernel selector of the inbound forward policy and of the
// OUT PROTECT policy and replace them. The delivery entry (the inbound selectors in OUT)
// follows the same conditions. The positive row is the same selectors at if_id 0. Method: the policy builder
// over hand-built Child SAs, counting template-free forward entries and checking the
// inbound forward policy is still built where the backend needs it.
func TestChildOutboundForwardPolicyBuiltOnlyForPolicyBasedTunnels(t *testing.T) {
	cidr := func(s string) *net.IPNet {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, tc := range []struct {
		name          string
		local, remote string
		ifID          uint32
		forward       bool
		want          int
	}{
		{"policy-based tunnel at if_id 0", "10.2.0.0/24", "10.1.0.0/24", 0, true, 1},
		{"tunnel bound to an XFRM interface", "10.2.0.0/24", "10.1.0.0/24", testChildIfID, true, 0},
		{"no separate forward lookup", "10.2.0.0/24", "10.1.0.0/24", 0, false, 0},
		{"equal sides keep the inbound forward policy", "0.0.0.0/0", "0.0.0.0/0", 0, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := &ChildSA{
				TSLocal: cidr(tc.local), TSRemote: cidr(tc.remote), IfID: tc.ifID,
				forwardPolicy: tc.forward, Owner: "forward-peer",
			}
			outbound, inbound, delivery := 0, 0, 0
			for _, p := range childInstalledPolicies(child) {
				if p.Dir == dataplane.SADirOut && p.Action == dataplane.SPActionBypass {
					delivery++
					continue
				}
				if p.Dir != dataplane.SADirFwd {
					continue
				}
				if p.Action == dataplane.SPActionBypass {
					outbound++
					continue
				}
				inbound++
			}
			if outbound != tc.want {
				t.Fatalf("built %d outbound forward entries, want %d", outbound, tc.want)
			}
			if delivery != tc.want {
				t.Fatalf("built %d delivery entries, want %d", delivery, tc.want)
			}
			if tc.forward && inbound != 1 {
				t.Fatalf("built %d inbound forward policies, want the one the backend needs", inbound)
			}
		})
	}
}
