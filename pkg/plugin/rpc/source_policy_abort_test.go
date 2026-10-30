// Design: docs/architecture/api/process-protocol.md -- source-policy lifecycle.

package rpc

import (
	"testing"
	"time"
)

// TestSourcePolicyAbortTargetValidation pins the abort target's wire contract.
// Goal: an abort carries either zero (the transaction was prepared and never
// applied, so the owner only releases its pins) or an epoch newer than the
// captured one (the abort follows apply and owners restamp to it); any other
// target, and a zero apply target, is refused at the transport. Method: one
// valid request per operation, each case changing only its target.
func TestSourcePolicyAbortTargetValidation(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	request := func(op SourcePolicyOperation, next uint64) SourcePolicyInput {
		return SourcePolicyInput{TransactionID: "tx", Operation: op, NextPolicyEpoch: next,
			DeadlineMS: now.Add(time.Minute).UnixMilli(),
			Source:     SourcePolicyIdentity{Peer: "192.0.2.1", PeerOwner: 1, SessionGeneration: 2, PolicyEpoch: 5}}
	}
	cases := []struct {
		name  string
		op    SourcePolicyOperation
		next  uint64
		valid bool
	}{
		{"abort-before-apply-zero", SourcePolicyAbort, 0, true},
		{"abort-after-apply-newer", SourcePolicyAbort, 6, true},
		{"abort-same-epoch", SourcePolicyAbort, 5, false},
		{"abort-older-epoch", SourcePolicyAbort, 4, false},
		{"apply-zero", SourcePolicyApply, 0, false},
		{"apply-same-epoch", SourcePolicyApply, 5, false},
		{"apply-newer", SourcePolicyApply, 6, true},
		{"prepare-target", SourcePolicyPrepare, 6, false},
		{"finalize-target", SourcePolicyFinalize, 6, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := request(tc.op, tc.next).Validate(now)
			if tc.valid && err != nil {
				t.Fatalf("refused a valid request: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("accepted an invalid target")
			}
		})
	}
}
