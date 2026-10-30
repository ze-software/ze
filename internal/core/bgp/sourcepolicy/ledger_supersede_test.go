// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owners
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection

package sourcepolicy

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestLedgerReprepareReplacesAbandoned proves an open ledger an earlier
// coordinator transaction left behind does not lock its source. Goal: a
// Prepared ledger of another transaction (never applied, it only pins) is
// replaced at every identity, the same one included, because a lost
// target-zero abort leaves it at the identity the next prepare captures; an
// Applied ledger is replaced when the incoming prepare names a later session
// or a later policy epoch, and still holds the source when the incoming
// identity is the same or older, because that prepare may itself be the stale
// one. Method: Reprepare over each stage and identity class.
// PREVENTS: a lost abort or finalize refusing every later transaction for the
// source until its session drops.
func TestLedgerReprepareReplacesAbandoned(t *testing.T) {
	const owner Owner = "test-owner"
	open := func(stage Stage) *Ledger {
		l := NewLedger(ledgerInput(rpc.SourcePolicyPrepare, "old", 0))
		l.Record(ledgerInput(rpc.SourcePolicyPrepare, "old", 0), StagePrepared, nil) //nolint:errcheck // nil by construction
		if stage == StageApplied {
			l.Record(ledgerInput(rpc.SourcePolicyApply, "old", 6), StageApplied, nil) //nolint:errcheck // nil by construction
		}
		return &l
	}
	identity := func(generation, epoch uint64) rpc.SourcePolicyInput {
		input := ledgerInput(rpc.SourcePolicyPrepare, "new", 0)
		input.Source.SessionGeneration, input.Source.PolicyEpoch = generation, epoch
		return input
	}
	cases := []struct {
		name    string
		input   rpc.SourcePolicyInput
		replace bool
	}{
		{"later-epoch", identity(2, 6), true},
		{"later-session", identity(3, 1), true},
		{"same-identity", identity(2, 5), false},
		{"older-epoch", identity(2, 4), false},
		{"older-session", identity(1, 9), false},
	}
	for _, stage := range []Stage{StagePrepared, StageApplied} {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				done, err := owner.Reprepare(open(stage), tc.input)
				if tc.replace || stage == StagePrepared {
					if done || err != nil {
						t.Fatalf("stage %d: an abandoned ledger must be replaced: %t %v", stage, done, err)
					}
					return
				}
				if !done || err == nil {
					t.Fatalf("stage %d: a ledger not older than the prepare must hold the source: %t %v", stage, done, err)
				}
			})
		}
	}
}

// TestLedgerAbortNeverPrepared proves a target-zero abort of a transaction the
// source never prepared is answered as done. Goal: the coordinator sends that
// abort to an owner whose prepare failed, since the prepare may still have
// pinned; when it did not, the abort has nothing to release and must not be
// reported as a lost owner. An abort carrying a target, and every other phase,
// keep their refusals. Method: Admit with no ledger, with another
// transaction's ledger, and with a target.
func TestLedgerAbortNeverPrepared(t *testing.T) {
	const owner Owner = "test-owner"
	if dup, err := owner.Admit(nil, ledgerInput(rpc.SourcePolicyAbort, "tx", 0)); !dup || err != nil {
		t.Fatalf("no ledger: %t %v", dup, err)
	}
	other := NewLedger(ledgerInput(rpc.SourcePolicyPrepare, "other", 0))
	other.Record(ledgerInput(rpc.SourcePolicyPrepare, "other", 0), StagePrepared, nil) //nolint:errcheck // nil by construction
	if dup, err := owner.Admit(&other, ledgerInput(rpc.SourcePolicyAbort, "tx", 0)); !dup || err != nil {
		t.Fatalf("another transaction's ledger: %t %v", dup, err)
	}
	if other.Stage() != StagePrepared {
		t.Fatal("the other transaction's ledger changed")
	}
	if _, err := owner.Admit(nil, ledgerInput(rpc.SourcePolicyAbort, "tx", 7)); err == nil {
		t.Fatal("an abort with a target and no ledger is refused")
	}
	if _, err := owner.Admit(nil, ledgerInput(rpc.SourcePolicyApply, "tx", 6)); err == nil {
		t.Fatal("an apply with no ledger is refused")
	}
}
