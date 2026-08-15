package reactor

import (
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file pins ONE property on the two egress rails that used to discard the
// failure channel safeEgressFilter already produces: a recovered filter panic is
// not a policy decision, and each rail's caller must be able to tell the two
// apart. Every test here is a PAIR -- a panicking filter beside a filter that
// cleanly returns false -- because a test that only drives the panic passes
// against code that treats both the same way.
//
// The third rail, forwardUpdateCore (reactor_api_forward.go), already read both
// returns and is covered by reactor_api_relay_test.go.

// panicEgressFilter panics for one destination address and accepts every other.
func panicEgressFilter(victim netip.Addr) filterapi.EgressFilterFunc {
	return func(_, dest filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
		if dest.Address == victim {
			panic("egress filter under test")
		}
		return true
	}
}

// rejectEgressFilter cleanly rejects one destination address and accepts every
// other. This is the POLICY twin of the two failure filters: same withheld
// route, but a decision rather than a failure.
func rejectEgressFilter(victim netip.Addr) filterapi.EgressFilterFunc {
	return func(_, dest filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
		return dest.Address != victim
	}
}

// buildFailEgressFilter ACCEPTS one destination and asks for a modification that
// buildModifiedPayload must refuse: a withdrawn-routes rewrite past the 2-octet
// Withdrawn Routes Length field. The filter decided; realizing the decision is
// what fails. Same shape as the oversize filter in
// reactor_stale_readvertise_test.go.
func buildFailEgressFilter(victim netip.Addr) filterapi.EgressFilterFunc {
	return func(_, dest filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
		if dest.Address == victim {
			mods.SetWithdrawnRewrite(make([]byte, 65536))
		}
		return true
	}
}

// TestReactorForwardRSFilterPanicIsNotPolicy drives the RS fast path twice over
// one topology: once with an egress filter that PANICS for a destination, once
// with a filter that cleanly REJECTS the same destination.
//
// VALIDATES: AC-7 -- a recovered panic leaves the destination undecided, so the
// rail hands it to the plugin rail through FastPathSkipped, where
// forwardUpdateCore classifies the outcome. A clean reject is a decision and
// stays consumed here.
// PREVENTS: the discard this test was written for. While the rail bound
// `accept, _ :=`, a panicking filter and a rejecting filter produced the same
// `continue`. With any other destination dispatched, reactor_notify.go then set
// ReactorForwarded, bgp-rs took its `default: releaseCache` arm, and the
// destination whose filter CRASHED lost the route with no second rail and no
// accounting -- a failure reported as policy by construction.
func TestReactorForwardRSFilterPanicIsNotPolicy(t *testing.T) {
	victim := netip.MustParseAddr("10.0.0.3")

	for _, tc := range []struct {
		name        string
		filter      filterapi.EgressFilterFunc
		wantSkipped bool // the destination is handed to the plugin rail
	}{
		{
			name:        "panicking filter defers the destination to the plugin rail",
			filter:      panicEgressFilter(victim),
			wantSkipped: true,
		},
		{
			name:        "rejecting filter suppresses the destination here",
			filter:      rejectEgressFilter(victim),
			wantSkipped: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := bgpctx.EncodingContextForASN4(true)
			ctxID, err := bgpctx.Registry.Register(ctx)
			require.NoError(t, err)

			payload := []byte{0, 0, 0, 0}
			wu := wireu.NewWireUpdate(payload, ctxID)
			wu.SetMessageID(120)

			update := &ReceivedUpdate{
				WireUpdate:   wu,
				SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
				ReceivedAt:   time.Now(),
			}

			cache := newRecentUpdateCache(100)
			cache.Add(update)
			cache.Activate(120, 1)

			src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
			dst1 := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
			dst2 := makeRSPeer(t, victim.String(), 65003, ctx, ctxID)

			var dispatched []fwdItem
			var mu sync.Mutex
			done := make(chan struct{})
			testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
				mu.Lock()
				dispatched = append(dispatched, items...)
				mu.Unlock()
				close(done)
			}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
			defer testPool.Stop()

			r := &Reactor{
				attrModHandlers: attrModHandlersWithDefaults(),
				recentUpdates:   cache,
				egressFilters:   []filterapi.EgressFilterFunc{tc.filter},
				peers: map[netip.AddrPort]*Peer{
					src.Settings().PeerKey():  src,
					dst1.Settings().PeerKey(): dst1,
					dst2.Settings().PeerKey(): dst2,
				},
				fwdPool: testPool,
			}

			skipped, nDispatched := reactorForwardRS(r, update, 120, netip.MustParseAddr("10.0.0.1"), src)

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for dispatch")
			}

			assert.Equal(t, 1, nDispatched, "this rail dispatches only the destination no filter touched")
			if tc.wantSkipped {
				require.Len(t, skipped, 1, "a filter that could not decide must not consume the destination")
				assert.Equal(t, dst2.Settings().PeerKey(), skipped[0])
			} else {
				assert.Empty(t, skipped, "a policy reject is a decision: nothing is handed on")
			}

			mu.Lock()
			require.Len(t, dispatched, 1)
			assert.Equal(t, dst1, dispatched[0].peer)
			mu.Unlock()
		})
	}
}

// TestDecideStaleReadvertiseFailureIsNotPolicy pins the same discrimination on
// the RFC 9494 stale re-advertise rail's decision half, over BOTH of its failure
// modes and the policy control.
//
// VALIDATES: AC-7 -- a panicking filter yields staleFilterFailed, a filter whose
// modifications cannot be built yields staleBuildFailed, and only a filter that
// returns false yields staleSuppress. All three withhold the route: the
// destination's LLGR capability is what the readvertise decision turns on, so
// RFC 9494 Section 4.3 forbids sending the stale route on a guess. What differs
// is the cause the caller reports.
// PREVENTS: either failure branch returning staleSuppress, which reached the
// operator as a peer that declined the family.
func TestDecideStaleReadvertiseFailureIsNotPolicy(t *testing.T) {
	dest := filterapi.PeerFilterInfo{
		Address: mustParseAddr("10.0.0.2"),
		PeerAS:  65001,
		LocalAS: 65000,
	}

	for _, tc := range []struct {
		name   string
		filter filterapi.EgressFilterFunc
		want   staleOutcome
	}{
		{"panicking filter", panicEgressFilter(dest.Address), staleFilterFailed},
		{"unbuildable modification", buildFailEgressFilter(dest.Address), staleBuildFailed},
		{"rejecting filter", rejectEgressFilter(dest.Address), staleSuppress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &Reactor{
				readvertiseEgressFilters: []filterapi.EgressFilterFunc{tc.filter},
				attrModHandlers:          attrModHandlersWithDefaults(),
			}
			a := &reactorAPIAdapter{r: r}

			body := append([]byte(nil), minimalAnnounceBody...)
			outcome, modified := a.decideStaleReadvertise(dest, body, 1)

			assert.Equal(t, tc.want, outcome)
			assert.Nil(t, modified, "neither outcome carries a body")
		})
	}
}

// TestAnnounceNLRIBatchStaleFailureIsNotFamilyMismatch drives the same three
// cases from the entry point an LLGR re-advertise actually enters, so the
// distinction is proven where an operator reads it rather than only in the
// helper (ai/rules/evidence.md, test corollary).
//
// VALIDATES: AC-7 -- each failure surfaces its OWN cause, and only a policy
// suppression keeps ErrNoPeersAcceptedFamily.
// PREVENTS: reporting a defect in Ze as "no peers have family negotiated". That
// cause is untrue -- the family IS negotiated here -- and it is the cause
// DispatchNLRIGroups downgrades to a warning, so both failures reached the
// operator as a routine skip.
func TestAnnounceNLRIBatchStaleFailureIsNotFamilyMismatch(t *testing.T) {
	dest := netip.MustParseAddr("10.0.0.2")

	for _, tc := range []struct {
		name    string
		filter  filterapi.EgressFilterFunc
		wantErr error
	}{
		{"panicking filter", panicEgressFilter(dest), errStaleReadvertiseFilterPanic},
		{"unbuildable modification", buildFailEgressFilter(dest), errStaleReadvertiseBuildFailed},
		{"rejecting filter", rejectEgressFilter(dest), route.ErrNoPeersAcceptedFamily},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := &PeerSettings{
				Connection: ConnectionBoth,
				Address:    dest,
				LocalAS:    65000,
				PeerAS:     65001,
				RouterID:   0x01020301,
			}
			peer := NewPeer(settings)
			peer.state.Store(int32(PeerStateEstablished))
			peer.negotiated.Store(&NegotiatedCapabilities{
				families: map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
			})

			r := &Reactor{
				attrModHandlers:          attrModHandlersWithDefaults(),
				config:                   &Config{LocalAS: 65000},
				peers:                    map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
				readvertiseEgressFilters: []filterapi.EgressFilterFunc{tc.filter},
			}
			a := &reactorAPIAdapter{r: r}

			err := a.AnnounceNLRIBatch(selector.All(), staleReadvertiseBatch(t), plugin.OperatorSender())
			assert.ErrorIs(t, err, tc.wantErr)

			// The two failures must not ALSO read as a family mismatch, and
			// each must carry the shared wrapper so one errors.Is at the
			// caller catches both. Asserting only the specific error would
			// pass against a wrapper that had swallowed the distinction.
			if errors.Is(tc.wantErr, errStaleReadvertiseWithheld) {
				assert.NotErrorIs(t, err, route.ErrNoPeersAcceptedFamily,
					"a failure of this speaker must not be reported as a peer declining the family")
				assert.ErrorIs(t, err, errStaleReadvertiseWithheld)
			}
		})
	}
}
