// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owners

package sourcepolicy

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestLedgerAbortStage pins which abort target each stage admits. Goal: a zero
// target (abort before apply: release the pins, nothing else) is admitted only
// from Prepared; after apply only a target newer than the apply target is; a
// closed transaction admits no new abort; every refusal comes before the owner
// changes anything. A Prepared ledger also admits a newer target, because a
// coordinator whose apply fails part way has already advanced the source's
// epoch and every owner, applied or not, must restamp to it. Method: one ledger
// per stage, each asked Admit for every target class.
func TestLedgerAbortStage(t *testing.T) {
	const owner Owner = "test-owner"
	at := func(stages ...rpc.SourcePolicyOperation) *Ledger {
		l := NewLedger(ledgerInput(rpc.SourcePolicyPrepare, "tx", 0))
		l.Record(ledgerInput(rpc.SourcePolicyPrepare, "tx", 0), StagePrepared, nil) //nolint:errcheck // nil by construction
		for _, op := range stages {
			switch op {
			case rpc.SourcePolicyApply:
				l.Record(ledgerInput(op, "tx", 6), StageApplied, nil) //nolint:errcheck // nil by construction
			case rpc.SourcePolicyFinalize:
				l.Record(ledgerInput(op, "tx", 0), StageFinalized, nil) //nolint:errcheck // nil by construction
			case rpc.SourcePolicyAbort:
				l.Record(ledgerInput(op, "tx", 0), StageAborted, nil) //nolint:errcheck // nil by construction
			case rpc.SourcePolicyPrepare, rpc.SourcePolicyUnspecified:
				t.Fatalf("unsupported stage step %d", op)
			}
		}
		return &l
	}
	cases := []struct {
		name   string
		ledger *Ledger
		target uint64
		admit  bool
	}{
		{"prepared-zero", at(), 0, true},
		{"prepared-newer", at(), 7, true},
		{"applied-zero", at(rpc.SourcePolicyApply), 0, false},
		{"applied-same-as-apply", at(rpc.SourcePolicyApply), 6, false},
		{"applied-newer", at(rpc.SourcePolicyApply), 7, true},
		{"finalized-zero", at(rpc.SourcePolicyApply, rpc.SourcePolicyFinalize), 0, false},
		{"finalized-newer", at(rpc.SourcePolicyApply, rpc.SourcePolicyFinalize), 8, false},
		{"aborted-other-target", at(rpc.SourcePolicyAbort), 7, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dup, err := owner.Admit(tc.ledger, ledgerInput(rpc.SourcePolicyAbort, "tx", tc.target))
			if dup {
				t.Fatal("a first abort answered as a duplicate")
			}
			if tc.admit && err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !tc.admit && err == nil {
				t.Fatal("admitted")
			}
		})
	}
	// A zero-target abort delivered twice answers the recorded outcome.
	l := at(rpc.SourcePolicyAbort)
	if dup, err := owner.Admit(l, ledgerInput(rpc.SourcePolicyAbort, "tx", 0)); !dup || err != nil {
		t.Fatalf("duplicate zero abort: %t %v", dup, err)
	}
}
