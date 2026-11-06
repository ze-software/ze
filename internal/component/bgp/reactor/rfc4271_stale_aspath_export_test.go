// Design: docs/architecture/wire/attributes.md -- AS_PATH toward an internal peer
// Related: stale_readvertise_chain.go -- runStaleChain, the guard these cases reach

package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staleOptedTransport is aspathEditTransport with its filter opted into the
// stale readvertise rail, as llgr-egress is, so the rail runs the filter with no
// export policy change behind it.
type staleOptedTransport struct{ *answerTransport }

// FilterReadvertise opts every answerPlugin filter in.
func (s *staleOptedTransport) FilterReadvertise(plugin, _ string) (readvertise, ok bool) {
	return plugin == answerPlugin, plugin == answerPlugin
}

// staleEditToward runs decideStaleReadvertise toward a peer built from settings,
// whose export chain is the one aspath-edit filter answering delta, with the
// destination facts sendStaleReadvertiseUnit reads from that peer and local AS
// 65000. judgeAll picks the rail's whole-chain run; otherwise the filter opts in.
// It answers the outcome, the body the destination is sent under that outcome,
// and how many times the filter was asked.
func staleEditToward(t *testing.T, settings *PeerSettings, internal, judgeAll bool, delta string, body []byte) (staleOutcome, []byte, int) {
	t.Helper()
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)

	peer := NewPeer(settings)
	require.Equal(t, internal, peer.IsIBGP(), "the session's own internal verdict")
	dest := filterapi.PeerFilterInfo{Address: settings.Address, PeerAS: peer.PeerAS(),
		LocalAS: settings.LocalAS, IsIBGP: peer.IsIBGP()}
	chain := staleChain{policy: answerPolicyAt(t, libraryIntentExport, routepolicy.FinalAccept, "aspath-edit"),
		ctxID: ctxID, asn4: true, judgeAll: judgeAll}

	transport := aspathEditTransport(t, delta)
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
	if !judgeAll {
		r.filterTransportSeam = &staleOptedTransport{answerTransport: transport}
	}

	outcome, modified := (&reactorAPIAdapter{r: r}).decideStaleReadvertise(dest, chain, body, 1)
	if modified != nil {
		return outcome, modified, len(transport.sent)
	}
	return outcome, body, len(transport.sent)
}

// TestRFC4271StaleReadvertiseASPathEditsSkipInternalPeers drives the operator
// export edits of the AS_PATH, "as-path-prepend 2" and "remove-private strip",
// through decideStaleReadvertise on both of the stale rail's runs: a filter opted
// into readvertise, and the whole chain a policy change owes. RFC 4271 Section
// 5.1.2: "When a given BGP speaker advertises the route to an internal peer, the
// advertising speaker SHALL NOT modify the AS_PATH attribute associated with the
// route."
//
// VALIDATES: the filter is asked in every case; an internal destination and one
// internal by RFC 7705 Section 4.2 get staleKeep and the body byte for byte; the
// same peer AS with no migration AS gets staleModify and a body whose AS_PATH is
// edited, [65000 65000 64496] and [64496].
// PREVENTS: the stale rail handing the chain an AS_PATH it may edit toward an
// internal peer, which every other egress rail refuses.
// MUTATION: in runStaleChain, "asPathEditable: true}" for
// "asPathEditable: !dest.IsIBGP}" reddens every internal and migration-internal
// subtest with staleModify and a prepended or stripped AS_PATH.
//
// RFC requirement: RFC4271-5.1.2-2 negative -- on the stale readvertise rail, an
// export as-path-prepend or remove-private toward an internal destination, or
// toward one internal by RFC 7705 Section 4.2, leaves the body byte for byte
// (decideStaleReadvertise answers staleKeep), while the same peer AS with no
// migration AS answers staleModify with the AS_PATH prepended and stripped.
func TestRFC4271StaleReadvertiseASPathEditsSkipInternalPeers(t *testing.T) {
	edits := []struct {
		delta    string
		body     func() []byte
		external []uint32
	}{
		{"as-path-prepend 2", func() []byte { return asPathBodyAtWidth(true) }, []uint32{65000, 65000, 64496}},
		{"remove-private strip", privateASPathBody, []uint32{64496}},
	}
	for _, rail := range []struct {
		name     string
		judgeAll bool
	}{
		{"opted-filter", false},
		{"whole-chain", true},
	} {
		for _, edit := range edits {
			t.Run(rail.name+"/"+edit.delta, func(t *testing.T) {
				body := edit.body()

				ordinary := NewPeerSettings(mustParseAddr("192.0.2.61"), 65000, 65000, 0x01010101)
				outcome, wire, asked := staleEditToward(t, ordinary, true, rail.judgeAll, edit.delta, body)
				assert.Positive(t, asked, "internal destination: the filter is asked")
				assert.Equal(t, staleKeep, outcome, "internal destination: nothing is rewritten")
				assert.Equal(t, body, wire, "internal destination: the AS_PATH SHALL NOT be modified")

				outcome, wire, asked = staleEditToward(t, migrationSettings("192.0.2.62", true), true, rail.judgeAll, edit.delta, body)
				assert.Positive(t, asked, "migration-internal destination: the filter is asked")
				assert.Equal(t, staleKeep, outcome, "migration-internal destination: nothing is rewritten")
				assert.Equal(t, body, wire, "migration-internal destination: the AS_PATH SHALL NOT be modified")

				outcome, wire, asked = staleEditToward(t, migrationSettings("192.0.2.63", false), false, rail.judgeAll, edit.delta, body)
				assert.Positive(t, asked, "external destination: the filter is asked")
				assert.Equal(t, staleModify, outcome, "external destination: the edit rewrites the body")
				value, found := attrValueFromPayload(t, wire, attribute.AttrASPath)
				require.True(t, found, "external destination: the route leaves with an AS_PATH")
				path, err := attribute.ParseASPath(value, true)
				require.NoError(t, err)
				assert.Equal(t, edit.external, flatASNs(path), "external destination: the edit applies")
			})
		}
	}
}
