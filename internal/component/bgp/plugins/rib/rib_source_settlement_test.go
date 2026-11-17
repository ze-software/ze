// Design: docs/architecture/plugin/rib-storage-design.md -- scoped completion.
// Related: source_policy.go -- a source phase settles its own released leases.
package rib

import (
	"context"
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourceSettlementFixture retains useful received routes and one unrelated
// source's failed release. Its transport rejects only that unrelated receipt.
func sourceSettlementFixture(t *testing.T) (*sourcePolicyFixture, *ribSourceLeases, *atomic.Uint32) {
	t.Helper()
	f := newSourcePolicyFixture(t)
	transport := newSourceFailureFixture(t)
	f.r.plugin = transport.r.plugin
	f.r.sourceLeases = newRIBSourceLeases(f.r)
	transport.r = f.r
	s := f.r.sourceLeases
	blocked := &atomic.Uint32{}
	transport.bridge.SetDispatchNative(func(_ context.Context, method string, value any) (any, bool, error) {
		if method != rpc.MethodSourceLease {
			return nil, false, errors.New("unexpected engine operation")
		}
		input, ok := value.(rpc.SourceLeaseInput)
		if !ok {
			return nil, true, errors.New("source lease input has the wrong type")
		}
		if input.Action == rpc.SourceLeaseRelease {
			if input.SourceReceipt == 900 {
				blocked.Add(1)
				return nil, true, errors.New("unrelated source release remains unsettled")
			}
		}
		return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, true, nil
	})
	// The source's policy requests already name owner1. The alternative and
	// unrelated source are distinct engine Peers, never aliases of that owner.
	for index, source := range []struct {
		peer  netip.Addr
		owner uint64
	}{{f.source, 1}, {f.alternative, 2}} {
		receipt := uint64(index) + 201
		ref, err := s.acquire(t.Context(), source.peer, source.owner, receipt)
		require.NoError(t, err)
		routes := f.r.bgpPeers[source.peer]
		for _, fam := range routes.Families() {
			routes.ModifyFamilyAll(fam, func(entry *storage.RouteEntry) {
				entry.SourceReceipt = receipt
				entry.SetSourceReference(ref)
			})
		}
		ref.Release()
	}
	unrelated := netip.MustParseAddr("192.0.2.200")
	ref, err := s.acquire(t.Context(), unrelated, 3, 900)
	require.NoError(t, err)
	ref.Release()
	require.ErrorContains(t, s.settle(t.Context()), "unrelated source release")
	t.Cleanup(f.r.shutdownReceivedSources)
	return f, s, blocked
}

// TestRIBSourcePolicySettlementIgnoresOtherSource drives every registered
// phase while another source's final release is refused. The alternative's
// useful route and the unrelated genuine release obligation both survive.
func TestRIBSourcePolicySettlementIgnoresOtherSource(t *testing.T) {
	for _, operation := range []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyAbort, rpc.SourcePolicyFinalize} {
		name, err := operation.MarshalText()
		require.NoError(t, err)
		t.Run(string(name), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			f, s, blocked := sourceSettlementFixture(t)
			prepare := sourcePolicyRequest(t, rpc.SourcePolicyPrepare, "source-scope", f.source, 5, 0)
			if operation != rpc.SourcePolicyPrepare {
				require.NoError(t, f.r.prepareSourcePolicy(f.source, prepare))
			}
			if operation == rpc.SourcePolicyAbort {
				require.NoError(t, f.r.applySourcePolicy(f.source, sourcePolicyRequest(t, rpc.SourcePolicyApply, "source-scope", f.source, 5, 6)))
			}
			if operation == rpc.SourcePolicyFinalize {
				require.NoError(t, f.r.applySourcePolicy(f.source, sourcePolicyRequest(t, rpc.SourcePolicyApply, "source-scope", f.source, 5, 6)))
			}
			next := uint64(6)
			if operation == rpc.SourcePolicyPrepare {
				next = 0
			}
			if operation == rpc.SourcePolicyFinalize {
				next = 0
			}
			if operation == rpc.SourcePolicyAbort {
				next = 7
			}
			before := blocked.Load()
			err := f.r.handleSourcePolicy(ctx, sourcePolicyRequest(t, operation, "source-scope", f.source, 5, next))
			require.NoError(t, err, "a healthy source's phase must not join another source's release")
			require.Equal(t, before, blocked.Load(), "source-scoped completion must not retry the unrelated release")
			require.Positive(t, f.r.bgpPeers[f.alternative].Len(), "the alternative's useful stored route survives")
			require.ErrorContains(t, s.failures()[netip.MustParseAddr("192.0.2.200")], "unrelated source release")
		})
	}
}

// TestRIBSourceDownSettlementIgnoresOtherSource covers the best-effort state
// return path while preserving its immediate final-release behavior.
func TestRIBSourceDownSettlementIgnoresOtherSource(t *testing.T) {
	f, s, blocked := sourceSettlementFixture(t)
	before := blocked.Load()
	f.r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: f.source.String(),
		State: rpc.SessionStateDown, Reason: rpc.ReasonPeerRemoved})
	require.Equal(t, before, blocked.Load(), "B's DOWN must not retry A's refused release")
	require.Nil(t, f.r.bgpPeers[f.source])
	require.Positive(t, f.r.bgpPeers[f.alternative].Len())
	s.mu.Lock()
	_, retained := s.leases[201]
	s.mu.Unlock()
	require.False(t, retained, "DOWN still completes its own final release synchronously")
	require.ErrorContains(t, s.failures()[netip.MustParseAddr("192.0.2.200")], "unrelated source release")
}

// TestRIBSourcePurgeSettlementIgnoresOtherSource verifies that completing B's
// stale purge releases B's last reference without waiting for unrelated A.
func TestRIBSourcePurgeSettlementIgnoresOtherSource(t *testing.T) {
	f, s, blocked := sourceSettlementFixture(t)
	_, _, err := f.r.markStaleCommand([]string{f.source.String(), "0"})
	require.NoError(t, err)
	before := blocked.Load()
	status, _, err := f.r.purgeStaleCommand([]string{f.source.String()})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Equal(t, before, blocked.Load())
	require.Zero(t, f.r.bgpPeers[f.source].Len())
	require.Positive(t, f.r.bgpPeers[f.alternative].Len(), "a useful unrelated candidate must stay stored")
	require.ErrorContains(t, s.failures()[netip.MustParseAddr("192.0.2.200")], "unrelated source release")
	s.mu.Lock()
	_, retained := s.leases[201]
	s.mu.Unlock()
	require.False(t, retained, "the purged source's actual final release settled")
}
