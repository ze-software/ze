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

	replacement := newRekeyedChild(child, 0x71920001, 0x71920002, child.Keys, true, nil)
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
	replacement := newRekeyedChild(child, 0x71940001, 0x71940002, child.Keys, true, nil)
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
	if len(dp.policies) != 6 {
		t.Fatalf("two negotiated pairs installed %d policies, want six", len(dp.policies))
	}
	want := map[string]bool{"10.1.2.3/32": false, "10.1.2.4/32": false}
	for _, policy := range dp.policies {
		if policy.Dir != dataplane.SADirFwd {
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

func (d *forwardStrandedDP) RemovePolicyParams(policy dataplane.SPParams) error {
	if policy.Dir == dataplane.SADirFwd {
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
	if len(dp.policies) != 3 || !dp.hasPolicy(next, dataplane.SADirFwd) {
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
			replacement := newRekeyedChild(old, 0x71960001, 0x71960002, old.Keys, true, nil)
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
			if !old.ESPInstalled || len(dp.policies) != 3 || len(dp.states) != 2 {
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
	replacement := newRekeyedChild(old, 0x71970001, 0x71970002, old.Keys, true, nil)
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
	first := newRekeyedChild(old, 0x71980001, 0x71980002, old.Keys, true, nil)
	refusing := &forwardRestoreRefusalDP{forwardPolicyDP: dp, oldSPI: old.InboundSPI}
	err = installChildTolerant(first, testESPGroup().Proposals[0], refusing, slogutil.DiscardLogger())
	if !errors.Is(err, errChildPolicyRollback) {
		t.Fatalf("first replacement error=%v, want restoration failure", err)
	}
	next := newRekeyedChild(old, 0x71980003, 0x71980004, old.Keys, true, nil)
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
	replacement := newRekeyedChild(old, 0x71990001, 0x71990002, old.Keys, true, nil)
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
