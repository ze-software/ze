// Design: docs/architecture/edge-cases/as4.md — where the reconciliation runs
// Related: session_read.go — processValidatedMessage treat-as-withdraw synthesis
// Related: forward_rs.go — reactorForwardRSSection runs the export chain

package reactor

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	as4WithdrawExport = "reactor-as4-withdraw-test export"
	as4WithdrawType   = "reactor-as4-withdraw-test-as-path"
)

func init() {
	reads := []routepolicy.Field{routepolicy.FieldASPath}
	routepolicy.MustRegisterAttachment("bgp", as4WithdrawExport, reads, nil)
	routepolicy.MustRegisterFilterType(as4WithdrawType, reads, nil)
	asPath, err := policyview.ReaderFor(routepolicy.FieldASPath)
	if err != nil {
		panic("BUG: reactor test: no as-path reader: " + err.Error())
	}
	policychain.MustRegisterEvaluator(as4WithdrawType, rpc.OnErrorReject, func(string, map[string]any, routepolicy.InstanceResolver) (policychain.Evaluator, error) {
		return func(view *policyview.View, _ *policychain.Sets) (policychain.Decision, error) {
			if _, err := view.Read(asPath); err != nil {
				return policychain.DecisionUnspecified, err
			}
			return policychain.DecisionPass, nil
		}, nil
	})
}

// TestRSFastPathTwoOctetWithdrawalReachesAnASPathFilter drives a treat-as-withdraw
// UPDATE from a two-octet source through ingest and the route-server fast path
// to a destination whose export chain reads as-path.
//
// Method: ORIGIN 7 makes the UPDATE treat-as-withdraw (RFC 7606 Section 7.1),
// so ingest synthesizes the withdrawal of its prefix. The export chain reads
// the four-octet form the ingest collapse promises every consumer. The
// withdrawal holds no AS number, so it is in that form; it MUST carry the
// four-octet context, or the member refuses it and the fast path hands the
// destination to the plugin rail.
func TestRSFastPathTwoOctetWithdrawalReachesAnASPathFilter(t *testing.T) {
	dests := []collapseDest{{name: "route-server client", addr: "10.9.0.4", rsClient: true}}
	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0x07})
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath),
		collapseASPathValue(2, collapseSourceAS))...)
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrNextHop), []byte{1, 1, 1, 1})...)
	env := collapseForwardIngest(t, attrs, dests)

	reg, err := routepolicy.NewRegistry(routepolicy.Instance{Name: "reads-as-path", Type: as4WithdrawType})
	require.NoError(t, err)
	chain, err := policychain.Compile(as4WithdrawExport, reg, []policychain.Member{{Name: "reads-as-path"}},
		routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	require.NoError(t, err)
	peer, found := env.reactor.lookupPeer(dests[0].addr)
	require.True(t, found)
	peer.mu.Lock()
	peer.settings.ExportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "reads-as-path"}}, routepolicy.FinalAccept).WithLibrary(chain)
	peer.mu.Unlock()
	peer.refreshForwardFacts()

	update, ok := env.reactor.recentUpdates.Get(env.updateID)
	require.True(t, ok, "the synthesized withdrawal must be in the forward cache")
	require.Equal(t, makeUpdateBody(fwdTestNLRI, nil, nil), update.WireUpdate.Payload(),
		"ingest must have synthesized the withdrawal")
	ctx := bgpctx.Registry.Get(update.WireUpdate.SourceCtxID())
	require.NotNil(t, ctx)
	assert.True(t, ctx.ASN4(), "every UPDATE leaving ingest is in the four-octet form")

	// The fixture's own pool reads announcements only, so this one takes the
	// withdrawal the client is sent.
	sent := make(chan [][]byte, 1)
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		var bodies [][]byte
		for i := range items {
			for _, body := range collapseDispatchedBodies(&items[i]) {
				bodies = append(bodies, bytes.Clone(body))
			}
			if items[i].done != nil {
				items[i].done()
			}
		}
		sent <- bodies
	}, fwdPoolConfig{chanSize: 4, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)
	env.reactor.fwdPool = pool

	skipped, dispatched := reactorForwardRS(env.reactor, update, env.updateID,
		netip.MustParseAddr(collapseSourceAddr), env.source)
	require.Empty(t, skipped, "the export chain must judge the withdrawal on the fast path")
	require.Equal(t, 1, dispatched, "the withdrawal must reach the client")
	select {
	case bodies := <-sent:
		assert.Equal(t, [][]byte{makeUpdateBody(fwdTestNLRI, nil, nil)}, bodies,
			"the client must be sent the withdrawal of the prefix")
	case <-time.After(5 * time.Second):
		t.Fatal("the client was sent nothing")
	}
}
