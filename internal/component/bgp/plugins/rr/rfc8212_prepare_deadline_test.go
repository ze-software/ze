// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on; rows RFC8212-3-1/3-2 are {gap}, which refuse a proof tag until the spec closes them (AC-11).
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection

package rr

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// settledAtDeadline is a context whose deadline has passed but whose Done
// channel never closes, so a settle that selects on it finishes on its
// barrier: the shape of a settle that completed just as the coordinator's
// call timed out.
type settledAtDeadline struct{ context.Context }

func (settledAtDeadline) Err() error { return context.DeadlineExceeded }

// TestRFC8212PrepareTimeoutDoesNotLockSource proves a prepare whose call timed
// out leaves the source free for the next transaction, the bgp-rr counterpart
// of the bgp-rs test of the same name. Goal: once its settle returns past the
// deadline bgp-rr pins nothing, the coordinator's target-zero abort for that
// transaction is answered as done, and a second transaction for the same
// source and identity prepares. Method: the first prepare runs under a context
// that has ended while its Done channel stays open, so the settle completes on
// its barrier and only the re-check after it can refuse.
// PREVENTS: a late prepare recording StagePrepared that refuses every later
// transaction for the source until its session drops.
func TestRFC8212PrepareTimeoutDoesNotLockSource(t *testing.T) {
	rr, _, _ := newSourcePolicyReflector(t)
	input := func(tx string, op rpc.SourcePolicyOperation) rpc.SourcePolicyInput {
		in := rpc.SourcePolicyInput{TransactionID: tx, Operation: op,
			Source: rpc.SourcePolicyIdentity{Peer: spSource, PeerOwner: 1, SessionGeneration: 3,
				PolicyEpoch: 5, ReceiveFrontier: 9, TemplateGeneration: 1},
			DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
		require.NoError(t, in.Validate(time.Now()))
		return in
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	err := rr.handleSourcePolicy(settledAtDeadline{ctx}, input("tx-late", rpc.SourcePolicyPrepare))
	require.ErrorContains(t, err, "reached its deadline while settling")
	rr.withdrawalMu.Lock()
	held := rr.sourcePolicy[spSource]
	rr.withdrawalMu.Unlock()
	require.Nil(t, held, "a prepare past its deadline pinned the source")

	require.NoError(t, rr.handleSourcePolicy(ctx, input("tx-late", rpc.SourcePolicyAbort)),
		"the coordinator's target-zero abort of a transaction never prepared is refused")
	require.NoError(t, rr.handleSourcePolicy(ctx, input("tx-next", rpc.SourcePolicyPrepare)),
		"the next transaction for the source is refused")
}
