// VALIDATES: a cross-peer inner-selector record a failed policy removal stranded is
// retried and released (retryStranded), and a removal answer that leaves no policy
// releases the record at once (policyGone).
// PREVENTS: one transient netlink failure refusing an overlapping peer for the
// daemon's life, and a degraded Child SA leaking its record.

package engine

import (
	"errors"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// flakyRemoveDP is a dataplane whose policy removal answers err for the first fails
// calls and succeeds after, counting every call.
type flakyRemoveDP struct {
	mockDP
	err   error
	fails int
	calls int
}

func (f *flakyRemoveDP) RemovePolicyParams(dataplane.SPParams) error {
	f.calls++
	if f.fails > 0 {
		f.fails--
		return f.err
	}
	return nil
}

// unsupportedDP is a dataplane that refuses every install and removal with err, the
// answer of a kernel that offers no XFRM.
type unsupportedDP struct {
	mockDP
	err error
}

func (u *unsupportedDP) InstallSA(dataplane.SAParams) error          { return u.err }
func (u *unsupportedDP) InstallPolicy(dataplane.SPParams) error      { return u.err }
func (u *unsupportedDP) RemovePolicyParams(dataplane.SPParams) error { return u.err }

// clearLedgerAfter drops child's policies from the process-wide SPD ledger when the test
// ends, so a removal a test made fail does not reach the next test.
func clearLedgerAfter(t *testing.T, child *ChildSA) {
	t.Helper()
	leftover := append(childPolicies(child, dataplane.SADirOut), childPolicies(child, dataplane.SADirIn)...)
	t.Cleanup(func() {
		for i := range leftover {
			spdLeft.record(&leftover[i], nil)
		}
	})
}

// ledgerNames reports whether the SPD ledger still names any policy of child.
func ledgerNames(child *ChildSA) bool {
	left := spdLeft.describe()
	for _, p := range append(childPolicies(child, dataplane.SADirOut), childPolicies(child, dataplane.SADirIn)...) {
		if slices.Contains(left, dataplane.XFRMPolicyDeleteCommand(&p)) {
			return true
		}
	}
	return false
}

// TestInnerRecordReleasedWhenRemovalLeavesNoPolicy: every removal answer policyGone
// accepts releases the record. Method: peer a installs 10.1.2.3/32, removeChildSA ends it
// on a dataplane whose removal answers err, and peer b then proposes the same address and
// must be served, with no SPD ledger entry left for a.
func TestInnerRecordReleasedWhenRemovalLeavesNoPolicy(t *testing.T) {
	log := slogutil.DiscardLogger()
	cases := []struct {
		name string
		err  error
	}{
		{name: "dataplane holds no policies", err: dataplane.ErrNotSupported},
		{name: "another peer owns the selector", err: &dataplane.PolicyOwnedError{HeldBy: "x", Wanted: "a"}},
		{name: "kernel offers no XFRM", err: syscall.EPROTONOSUPPORT},
		{name: "kernel lacks the protocol option", err: syscall.ENOPROTOOPT},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			dp := &failingRemoveDP{err: tc.err}
			child := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
			clearLedgerAfter(t, child)
			removeChildSA(child, dp, log)
			if got := reg.holdings(); got != 0 {
				t.Fatalf("registry holds %d Child SAs after a removal answered %v, want 0", got, tc.err)
			}
			if _, owned := errors.AsType[*dataplane.PolicyOwnedError](tc.err); !owned && ledgerNames(child) {
				t.Fatalf("SPD ledger still names a's policy after a removal answered %v", tc.err)
			}
			if _, err := innerAuthChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"}); err != nil {
				t.Fatalf("peer b after a removal answered %v: %v", tc.err, err)
			}
		})
	}
}

// TestInnerRecordDegradedInstallReleased: a Child SA kept degraded because the kernel
// offers no XFRM keeps its record, and its removal on the same dataplane releases it.
// Method: the dataplane answers EPROTONOSUPPORT to every install and removal; peer a's
// Child SA must install degraded and be recorded, and after removeChildSA the registry
// must be empty and peer b served.
func TestInnerRecordDegradedInstallReleased(t *testing.T) {
	log := slogutil.DiscardLogger()
	reg := newInnerSelectorRegistry()
	dp := &unsupportedDP{err: syscall.EPROTONOSUPPORT}
	child := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	clearLedgerAfter(t, child)
	if child.ESPInstalled {
		t.Fatal("setup: a Child SA the dataplane refused reports its ESP installed")
	}
	if got := reg.holdings(); got != 1 {
		t.Fatalf("registry holds %d Child SAs after a degraded install, want 1", got)
	}
	removeChildSA(child, dp, log)
	if got := reg.holdings(); got != 0 {
		t.Fatalf("registry holds %d Child SAs after the degraded Child SA was removed, want 0", got)
	}
	if ledgerNames(child) {
		t.Fatal("SPD ledger names a policy the dataplane never held")
	}
	if _, err := innerAuthChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"}); err != nil {
		t.Fatalf("peer b after the degraded Child SA was removed: %v", err)
	}
}

// TestStrandedRecordRetryReleases: a record a failed removal stranded is released by the
// retry the next proposal runs. Method: peer a's two policy removals fail with EIO once,
// so its record stays; peer b then proposes the same address, the retry succeeds, and b
// must be served, a's policies removed again and dropped from the SPD ledger.
func TestStrandedRecordRetryReleases(t *testing.T) {
	log := slogutil.DiscardLogger()
	reg := newInnerSelectorRegistry()
	dp := &flakyRemoveDP{err: syscall.EIO, fails: 2}
	child := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	clearLedgerAfter(t, child)
	removeChildSA(child, dp, log)
	if got := reg.holdings(); got != 1 {
		t.Fatalf("setup: registry holds %d Child SAs after both removals failed, want 1", got)
	}
	dpB := &mockDP{}
	if _, err := innerAuthChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), dpB, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"}); err != nil {
		t.Fatalf("peer b after the retry could remove a's policies: %v", err)
	}
	if dp.calls != 4 {
		t.Fatalf("a's dataplane saw %d policy removals, want 4 (two failed, two retried)", dp.calls)
	}
	if len(dpB.policies) == 0 {
		t.Fatal("peer b was served but installed no policy")
	}
	if got := reg.holdings(); got != 1 {
		t.Fatalf("registry holds %d Child SAs, want 1 (peer b's)", got)
	}
	if ledgerNames(child) {
		t.Fatal("SPD ledger still names a's policies after the retry removed them")
	}
}

// TestStrandedRecordRefusalNamesItsFix: while the retry keeps failing, the refusal names
// the commands that remove the stranded policies and no configuration of the peer that
// held them. Method: peer a's removals always fail; peer b's proposal must be refused
// with errInnerSelectorStranded, carry `ip xfrm policy delete dir out`, not tell the
// operator to configure peer a, and map to the stranded fixes on both roles.
func TestStrandedRecordRefusalNamesItsFix(t *testing.T) {
	log := slogutil.DiscardLogger()
	reg := newInnerSelectorRegistry()
	dp := &xfrmCommandDP{err: syscall.EIO}
	child := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	clearLedgerAfter(t, child)
	removeChildSA(child, dp, log)
	_, err := innerAuthChild(t, innerPeerSA(t, reg, "b", "198.18.0.9"), &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	mustInnerRefused(t, err, "peer b while a's policy cannot be removed")
	if !errors.Is(err, errInnerSelectorStranded) {
		t.Fatalf("refusal %v does not wrap errInnerSelectorStranded", err)
	}
	if !strings.Contains(err.Error(), "ip xfrm policy delete dir out") {
		t.Fatalf("refusal names no command that removes the stranded policy: %v", err)
	}
	if strings.Contains(err.Error(), "site-to-site peer a ") {
		t.Fatalf("refusal tells the operator to configure peer a, which may be gone: %v", err)
	}
	if got := childRefusedResponderFix(err); got != childRefusedFixResponderStranded {
		t.Fatalf("responder fix %q, want the stranded fix", got)
	}
	if got := childResponseRefusedFix(err); got != childResponseFixStranded {
		t.Fatalf("initiator fix %q, want the stranded fix", got)
	}
}

// TestStrandedRecordRetriedOnReconcile: a configuration commit retries a stranded record.
// Method: peer a's removals fail once on the engine's own registry; reconcilePeers with a
// configuration naming no peer must leave the registry empty.
func TestStrandedRecordRetriedOnReconcile(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()
	dp := &flakyRemoveDP{err: syscall.EIO, fails: 2}
	child := mustInnerChild(t, innerPeerSA(t, table.innerSelectors, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	clearLedgerAfter(t, child)
	removeChildSA(child, dp, log)
	if got := table.innerSelectors.holdings(); got != 1 {
		t.Fatalf("setup: registry holds %d Child SAs after both removals failed, want 1", got)
	}
	reconcilePeers(&ipsec.IPsecConfig{}, map[string]*PeerSession{}, table, nil, nil, nil, log)
	if got := table.innerSelectors.holdings(); got != 0 {
		t.Fatalf("registry holds %d Child SAs after a commit retried the removal, want 0", got)
	}
}

// TestStrandedRecordAdoptedBySameOwner: when the peer that stranded a record installs the
// same selectors again, the live Child SA owns the kernel entry and no retry removes it.
// Method: peer a's removals fail; a re-establishes the same tunnel on the same
// dataplane; the registry must hold only the new Child SA, and a later retry, with
// removals now succeeding, must make no removal call.
func TestStrandedRecordAdoptedBySameOwner(t *testing.T) {
	log := slogutil.DiscardLogger()
	reg := newInnerSelectorRegistry()
	dp := &flakyRemoveDP{err: syscall.EIO, fails: 1000}
	first := mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	clearLedgerAfter(t, first)
	removeChildSA(first, dp, log)
	// The retries at narrowing and at the claim fail too, then the claim adopts the record.
	mustInnerChild(t, innerPeerSA(t, reg, "a", "198.18.0.7"), dp, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	if got := reg.holdings(); got != 1 {
		t.Fatalf("registry holds %d Child SAs after peer a re-installed its tunnel, want 1", got)
	}
	dp.fails = 0
	before := dp.calls
	reg.retryStranded()
	if dp.calls != before {
		t.Fatalf("a retry made %d removal calls after the live Child SA adopted the selectors, want 0", dp.calls-before)
	}
}
