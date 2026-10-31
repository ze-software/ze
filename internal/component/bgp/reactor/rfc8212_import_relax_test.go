// RFC naming: untagged -- tests the D-1 import relaxation RFC 8212 default reject relies on; rows RFC8212-3-1/3-2 are {gap}, which refuse a proof tag until the spec closes them (AC-11).
package reactor

import (
	"errors"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// relaxNegotiated is a published negotiation with or without Route Refresh,
// over IPv4 and IPv6 unicast.
func relaxNegotiated(routeRefresh bool) *NegotiatedCapabilities {
	return &NegotiatedCapabilities{RouteRefresh: routeRefresh,
		families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true}}
}

// newRelaxFixture is the transaction fixture (newRFC8212ImportFixture) with
// the peer's import policy absent, so its EBGP default rejects, and the
// session's negotiation published with or without Route Refresh.
func newRelaxFixture(t *testing.T, routeRefresh bool) *rfc8212ImportFixture {
	t.Helper()
	fx := newRFC8212ImportFixture(t, false)
	fx.peer.mu.Lock()
	fx.peer.settings.ImportPolicy = filterapi.Policy{}
	fx.peer.mu.Unlock()
	fx.peer.negotiated.Store(relaxNegotiated(routeRefresh))
	return fx
}

// relax lifts the peer's rejecting import decision through the journaled swap.
func relax(fx *rfc8212ImportFixture, j configJournal) (*PeerSettings, error) {
	next := fx.peer.settingsSnapshot()
	accept, err := filterapi.NewPolicy(nil, routepolicy.FinalAccept)
	if err != nil {
		return nil, err
	}
	next.ImportPolicy = accept
	copyImport := func(dst, src *PeerSettings) { dst.ImportPolicy = src.ImportPolicy }
	return next, (&reactorAPIAdapter{r: fx.r}).swapPeerSettingsJournaled([]peerSettingsSwap{{
		key: fx.peer.Settings().PeerKey(), next: next, apply: copyImport}}, j)
}

func relaxJournal(fx *rfc8212ImportFixture, txID string) *rfc8212TxJournal {
	return &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: txID, deadline: fx.f.clock.Now().Add(30 * time.Second)}
}

func importAccepts(p *Peer) bool {
	s := p.settingsSnapshot()
	return s.ImportPolicy.AcceptsAtEnd(s.IsEBGP())
}

// refreshesOnWire counts the ROUTE-REFRESH messages Ze wrote to the session
// until no message arrives for a short while.
func refreshesOnWire(t *testing.T, fx *rfc8212ImportFixture) int {
	t.Helper()
	count := 0
	for {
		select {
		case typ := <-fx.f.wire:
			if typ == msgtype.TypeROUTEREFRESH {
				count++
			}
		case <-time.After(300 * time.Millisecond):
			return count
		}
	}
}

// TestRFC8212RelaxWithoutRouteRefreshRefused proves D-1's refusal at both
// stages. Method: an Established EBGP peer whose import default rejects and
// whose session did not negotiate Route Refresh is offered an accepting
// import policy, first at verify (the operation's verify, and the section
// path's VerifyConfig over a reload function), then at apply.
// VALIDATES: every stage refuses naming the disable, commit, enable sequence;
// the running policy still rejects; no source-policy owner was called and
// the journal recorded nothing.
// PREVENTS: an accepting policy reported live while the routes the default
// discarded never return (RFC 8212 Section 3, D-1).
func TestRFC8212RelaxWithoutRouteRefreshRefused(t *testing.T) {
	fx := newRelaxFixture(t, false)
	next := fx.peer.settingsSnapshot()
	next.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	adapter := &reactorAPIAdapter{r: fx.r}
	const sequence = "`request peer 192.0.2.1 disable`, wait until the session is down, commit this change, then `request peer 192.0.2.1 enable`"

	require.ErrorContains(t, adapter.verifyImportRelaxation(next, true), sequence)
	fx.r.config.ConfigPath = "relax.conf"
	fx.r.SetReloadFunc(func(string) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error) {
		return []*PeerSettings{next}, Globals{}, nil, nil
	})
	require.ErrorContains(t, adapter.VerifyConfig(map[string]any{"bgp": map[string]any{}}), sequence)

	j := relaxJournal(fx, "tx-no-refresh")
	_, err := relax(fx, j)
	require.ErrorContains(t, err, sequence)
	require.False(t, importAccepts(fx.peer), "the refused change was published")
	require.Zero(t, j.Len(), "the refused change was journaled")
	ops, _, eligible, _ := fx.owner.seen()
	require.Empty(t, ops, "a source-policy owner was called for a refused change")
	require.True(t, eligible)
	require.Zero(t, refreshesOnWire(t, fx), "ROUTE-REFRESH sent to a peer that did not negotiate it")
}

// TestRFC8212RelaxRequestsEveryFamily proves D-1's recovery. Method: the same
// peer with Route Refresh negotiated over IPv4 and IPv6 unicast takes the
// accepting policy through a transaction, which then commits.
// VALIDATES: the policy accepts, exactly one ROUTE-REFRESH per negotiated
// family reached the session's wire, the session and its generation are
// unchanged, and no owner was asked to invalidate anything.
// PREVENTS: an accepting policy whose denied routes are never asked for
// again (RFC 7313 Section 3.2, normal route refresh request).
func TestRFC8212RelaxRequestsEveryFamily(t *testing.T) {
	fx := newRelaxFixture(t, true)
	j := relaxJournal(fx, "tx-refresh")
	_, err := relax(fx, j)
	require.NoError(t, err)
	require.True(t, importAccepts(fx.peer))
	require.Equal(t, 2, refreshesOnWire(t, fx), "one ROUTE-REFRESH per negotiated family")
	require.Same(t, fx.f.s, fx.peer.currentSession())
	require.Equal(t, fx.generation, fx.peer.forwardGeneration.Load())
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, true))
	j.Discard()
	ops, _, _, _ := fx.owner.seen()
	require.Empty(t, ops, "an accepted relaxation invalidated the source")
}

// TestRFC8212RelaxReplacedSessionRefused proves the apply judges the session
// it holds, not the one verify judged. Method: verify passes on a session
// with Route Refresh; the session is then replaced by one that negotiated
// none, as a reconnect between verify and apply does.
// VALIDATES: the apply refuses, the policy still rejects, no ROUTE-REFRESH
// went out, and no owner was called.
// PREVENTS: a recovery request sent to, or skipped on, a session nobody judged.
func TestRFC8212RelaxReplacedSessionRefused(t *testing.T) {
	fx := newRelaxFixture(t, true)
	next := fx.peer.settingsSnapshot()
	next.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	require.NoError(t, (&reactorAPIAdapter{r: fx.r}).verifyImportRelaxation(next, true))

	successor := newAdmissionSession(t)
	fx.peer.mu.Lock()
	fx.peer.session = successor.s
	fx.peer.mu.Unlock()
	fx.peer.negotiated.Store(relaxNegotiated(false))

	_, err := relax(fx, relaxJournal(fx, "tx-replaced"))
	require.ErrorContains(t, err, "did not negotiate Route Refresh")
	require.False(t, importAccepts(fx.peer))
	require.Zero(t, refreshesOnWire(t, fx))
	ops, _, _, _ := fx.owner.seen()
	require.Empty(t, ops)
}

// relaxBudgetConn fails every write once its budget, when armed, is spent.
type relaxBudgetConn struct {
	net.Conn
	armed  atomic.Bool
	budget atomic.Int32
}

func (c *relaxBudgetConn) Write(b []byte) (int, error) {
	if c.armed.Load() && c.budget.Add(-1) < 0 {
		return 0, errors.New("injected write failure")
	}
	return c.Conn.Write(b)
}

// TestRFC8212RelaxPartialSendFails proves a partial recovery fails the apply
// and leaves nothing accepting. Method: the session's connection lets one
// more write through, then fails, so the first family's ROUTE-REFRESH is
// sent and the second is not.
// VALIDATES: the apply fails naming the family whose request was not sent;
// the policy rejects again and nothing was journaled. The failed write also
// ended the session's ability to pause its input, so the undo reports its
// compensation as incomplete, naming the hold-down sequence, rather than as a
// rollback, and no owner was asked to prepare.
// PREVENTS: a commit reported applied while half the denied routes can never
// return, a failed relaxation that leaves the accepting policy behind, or a
// compensation that failed reported as complete.
func TestRFC8212RelaxPartialSendFails(t *testing.T) {
	fx := newRelaxFixture(t, true)
	var conn *relaxBudgetConn
	fx.f = newAdmissionSessionWithConn(t, func(c net.Conn) net.Conn {
		conn = &relaxBudgetConn{Conn: c}
		return conn
	})
	fx.peer.mu.Lock()
	fx.peer.session = fx.f.s
	fx.peer.mu.Unlock()
	fx.owner.onPrepare, fx.owner.onApply = nil, nil
	conn.budget.Store(1)
	conn.armed.Store(true)

	j := relaxJournal(fx, "tx-partial")
	_, err := relax(fx, j)
	require.ErrorContains(t, err, "ROUTE-REFRESH for ipv6/unicast not sent")
	require.ErrorContains(t, err, "could not be withdrawn and stay eligible; run `request peer 192.0.2.1 disable`")
	require.False(t, importAccepts(fx.peer), "a failed relaxation left the accepting policy")
	require.Zero(t, j.Len(), "a failed apply was journaled")
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
	ops, _, _, _ := fx.owner.seen()
	require.Empty(t, ops, "an owner was prepared although the input could not be paused")
	require.False(t, fx.f.s.IsPaused(), "input left paused")
}

// TestRFC8212RelaxUndoFencesAdmittedPaths proves the "Failed import
// relaxation" boundary. Method: the relaxation applies with Route Refresh
// (the owner's path stands for a route admitted under it); the transaction
// then rolls back. While the undo holds input paused, the peer sends one more
// UPDATE, a late answer to the ROUTE-REFRESH; the rejection receipt follows.
// VALIDATES: the owner made the admitted path ineligible (prepare, apply,
// finalize) under a fresh epoch; the late UPDATE was admitted once, under the
// restored rejecting policy and that epoch; the session was kept.
// PREVENTS: routes admitted under a rolled-back accepting policy staying
// eligible, or a late refresh answer repopulating them (RFC 8212 Section 3).
func TestRFC8212RelaxUndoFencesAdmittedPaths(t *testing.T) {
	fx := newRelaxFixture(t, true)
	j := relaxJournal(fx, "tx-undo")
	_, err := relax(fx, j)
	require.NoError(t, err)
	require.True(t, importAccepts(fx.peer))
	require.Equal(t, 2, refreshesOnWire(t, fx))

	require.Empty(t, j.Rollback())
	require.True(t, fx.f.s.IsPaused(), "input resumed before the transaction's receipt")
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
	ops, _, eligible, epoch := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyFinalize}, ops)
	require.False(t, eligible, "a path admitted under the undone relaxation stays eligible")
	require.Equal(t, fx.original, epoch, "the owner restamped a path it should have excluded")
	require.Greater(t, fx.peer.sourcePolicyEpoch.Load(), fx.original, "the undo did not fence with a fresh epoch")
	fx.settled(t, false)
}

// TestRFC8212DynamicTemplateRelaxation proves a dynamic group's template
// takes the same import transitions as a configured peer. Method: one group
// template edit at a time, on an Established dynamic EBGP peer (no Route
// Refresh, no config transaction) and on a dynamic peer that is not yet
// Established, through the verify check and the reload's dynamic step.
// VALIDATES: on the Established peer, lifting the rejection is refused at
// verify and at apply, and removing the accepting policy is refused at apply,
// each leaving the running policy and the group set; on the peer that is not
// Established both edits publish, since it admits nothing yet.
// PREVENTS: a dynamic template edit bypassing D-1 and D-2 that a configured
// peer's identical edit takes.
func TestRFC8212DynamicTemplateRelaxation(t *testing.T) {
	accept := mustPolicy(t, nil, routepolicy.FinalAccept)
	for _, tc := range []struct {
		name        string
		from, to    filterapi.Policy
		established bool
		refused     string
	}{
		{name: "established rise", to: accept, established: true, refused: "not negotiated capabilities"},
		{name: "established fall", from: accept, established: true, refused: "needs a config transaction operation"},
		{name: "idle rise", to: accept},
		{name: "idle fall", from: accept},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := dynamicReloadGroup(t)
			group.Settings.ImportPolicy = tc.from
			r, peer := newDynamicReloadReactor(t, group, netip.MustParseAddr("185.1.69.42"), nil)
			if tc.established {
				establishDynamicPeer(t, r, peer)
				peer.state.Store(int32(PeerStateEstablished))
			} else {
				peer.state.Store(int32(PeerStateActive))
			}
			next := dynamicReloadGroup(t)
			next.Settings.ImportPolicy = tc.to
			adapter := &reactorAPIAdapter{r: r}
			verify := adapter.verifyDynamicRelaxations(dynamicGroupsReadAs([]*DynamicGroupConfig{next}))
			err := adapter.reconcileDynamicGroupsJournaled([]*DynamicGroupConfig{next}, "test", &internalJournal{})
			template := peer.readDynamicTemplate()
			if tc.refused == "" {
				require.NoError(t, verify)
				require.NoError(t, err)
				require.Equal(t, tc.to, template.importTemplate)
				return
			}
			if tc.to.AcceptsAtEnd(true) {
				require.ErrorContains(t, verify, tc.refused)
			}
			require.ErrorContains(t, err, tc.refused)
			require.Equal(t, tc.from, template.importTemplate, "the refused template was published")
			r.mu.RLock()
			groups := r.dynamicGroups
			r.mu.RUnlock()
			require.Equal(t, []*DynamicGroupConfig{group}, groups, "the refused step left the new group set")
			require.Same(t, peer, r.Peers()[0])
		})
	}
}
