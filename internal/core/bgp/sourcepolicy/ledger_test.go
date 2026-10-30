// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owners

package sourcepolicy

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func ledgerInput(op rpc.SourcePolicyOperation, txn string, next uint64) rpc.SourcePolicyInput {
	return rpc.SourcePolicyInput{TransactionID: txn, Operation: op, NextPolicyEpoch: next,
		Source: rpc.SourcePolicyIdentity{Peer: "10.0.0.1", PeerOwner: 1, SessionGeneration: 2, PolicyEpoch: 5}}
}

// TestLedgerStateMachine drives the ledger every owner shares through its
// phases. Goal: a phase is admitted only in order, a duplicate returns the
// recorded outcome, and a foreign, stale or reordered phase is refused naming
// the owner. Method: each step asks Admit or Reprepare and checks the answer.
func TestLedgerStateMachine(t *testing.T) {
	const owner Owner = "test-owner"
	if _, err := owner.Admit(nil, ledgerInput(rpc.SourcePolicyApply, "tx", 6)); err == nil ||
		!strings.HasPrefix(err.Error(), "test-owner: ") {
		t.Fatalf("apply without a ledger: %v", err)
	}
	if done, err := owner.Reprepare(nil, ledgerInput(rpc.SourcePolicyPrepare, "tx", 0)); done || err != nil {
		t.Fatalf("no ledger must prepare: %t %v", done, err)
	}
	l := NewLedger(ledgerInput(rpc.SourcePolicyPrepare, "tx", 0))
	prepareErr := errors.New("recorded")
	if err := l.Record(ledgerInput(rpc.SourcePolicyPrepare, "tx", 0), StagePrepared, prepareErr); !errors.Is(err, prepareErr) {
		t.Fatal("record must return its error")
	}
	if done, err := owner.Reprepare(&l, ledgerInput(rpc.SourcePolicyPrepare, "tx", 0)); !done || !errors.Is(err, prepareErr) {
		t.Fatalf("a duplicate prepare returns the recorded outcome: %t %v", done, err)
	}
	// A Prepared ledger of another transaction was abandoned (Reprepare): the
	// coordinator claims a source for one transaction at a time.
	if done, err := owner.Reprepare(&l, ledgerInput(rpc.SourcePolicyPrepare, "other", 0)); done || err != nil {
		t.Fatalf("a prepared ledger of another transaction is replaced: %t %v", done, err)
	}
	if _, err := owner.Admit(&l, ledgerInput(rpc.SourcePolicyFinalize, "tx", 0)); err == nil {
		t.Fatal("finalize before apply is refused")
	}
	if _, err := owner.Admit(&l, ledgerInput(rpc.SourcePolicyApply, "other", 6)); err == nil {
		t.Fatal("a foreign token is refused")
	}
	foreign := ledgerInput(rpc.SourcePolicyApply, "tx", 6)
	foreign.Source.SessionGeneration = 3
	if _, err := owner.Admit(&l, foreign); err == nil {
		t.Fatal("another session of the source is refused")
	}
	if dup, err := owner.Admit(&l, ledgerInput(rpc.SourcePolicyApply, "tx", 6)); dup || err != nil {
		t.Fatalf("apply after prepare: %t %v", dup, err)
	}
	l.Record(ledgerInput(rpc.SourcePolicyApply, "tx", 6), StageApplied, nil) //nolint:errcheck // nil by construction
	if done, err := owner.Reprepare(&l, ledgerInput(rpc.SourcePolicyPrepare, "other", 0)); !done || err == nil {
		t.Fatal("a second transaction is refused while an applied one is open")
	}
	if dup, err := owner.Admit(&l, ledgerInput(rpc.SourcePolicyApply, "tx", 6)); !dup || err != nil {
		t.Fatalf("a duplicate apply returns its outcome: %t %v", dup, err)
	}
	if _, err := owner.Admit(&l, ledgerInput(rpc.SourcePolicyApply, "tx", 7)); err == nil {
		t.Fatal("a repeated phase with another target is refused")
	}
	if _, err := owner.Admit(&l, ledgerInput(rpc.SourcePolicyAbort, "tx", 6)); err == nil {
		t.Fatal("an abort target not newer than apply is refused")
	}
	if dup, err := owner.Admit(&l, ledgerInput(rpc.SourcePolicyAbort, "tx", 7)); dup || err != nil {
		t.Fatalf("abort after apply: %t %v", dup, err)
	}
	l.Record(ledgerInput(rpc.SourcePolicyAbort, "tx", 7), StageAborted, nil) //nolint:errcheck // nil by construction
	if _, err := owner.Admit(&l, ledgerInput(rpc.SourcePolicyFinalize, "tx", 0)); err == nil {
		t.Fatal("finalize after abort is refused")
	}
	if done, err := owner.Reprepare(&l, ledgerInput(rpc.SourcePolicyPrepare, "next", 0)); done || err != nil {
		t.Fatalf("a closed transaction lets the next prepare: %t %v", done, err)
	}

	f := NewLedger(ledgerInput(rpc.SourcePolicyPrepare, "f", 0))
	f.Record(ledgerInput(rpc.SourcePolicyPrepare, "f", 0), StagePrepared, nil)   //nolint:errcheck // nil by construction
	f.Record(ledgerInput(rpc.SourcePolicyApply, "f", 6), StageApplied, nil)      //nolint:errcheck // nil by construction
	f.Record(ledgerInput(rpc.SourcePolicyFinalize, "f", 0), StageFinalized, nil) //nolint:errcheck // nil by construction
	if _, err := owner.Admit(&f, ledgerInput(rpc.SourcePolicyAbort, "f", 8)); err == nil {
		t.Fatal("a stale abort cannot undo a finalized transaction")
	}
	if dup, err := owner.Admit(&f, ledgerInput(rpc.SourcePolicyPrepare, "f", 0)); !dup || err != nil {
		t.Fatalf("a repeated prepare is answered as a duplicate, never run again: %t %v", dup, err)
	}
	g := NewLedger(ledgerInput(rpc.SourcePolicyPrepare, "g", 0))
	g.Record(ledgerInput(rpc.SourcePolicyApply, "g", 6), StageApplied, nil) //nolint:errcheck // nil by construction
	if _, err := owner.Admit(&g, ledgerInput(rpc.SourcePolicyPrepare, "g", 0)); err == nil {
		t.Fatal("a first prepare is never admitted past prepare")
	}
}
