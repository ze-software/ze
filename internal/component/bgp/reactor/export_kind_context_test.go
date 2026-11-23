// Design: docs/architecture/core-design.md -- per-route export disposition.
// Related: session_write.go -- final policy guard preserves withdrawals.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// explicitExportTestSession supplies the legacy fixtures' explicit output
// context without inventing peer/source facts their old bare Session lacked.
func explicitExportTestSession(external bool) *policyview.Session {
	session := policyview.ExportSession(filterapi.PeerFilterInfo{IsIBGP: !external}, filterapi.PeerFilterInfo{}, false, filterapi.ExportExplicit)
	session.PeerKnown = false
	return &session
}

// TestEgressPolicyFinalReadsRouteKind proves both shared egress entry points
// apply the absent final to the route's recorded authority, not source facts.
func TestEgressPolicyFinalReadsRouteKind(t *testing.T) {
	r := &Reactor{}
	for _, direct := range []bool{false, true} {
		for _, internal := range []bool{false, true} {
			for _, kind := range []filterapi.ExportKind{filterapi.ExportUnknown, filterapi.ExportOrdinaryLearned, filterapi.ExportExplicit, filterapi.ExportKind(255)} {
				session := policyview.ExportSession(filterapi.PeerFilterInfo{IsIBGP: internal}, filterapi.PeerFilterInfo{}, false, kind)
				wire := testWireUpdate([]byte{0, 0, 0, 0})
				var result egressStepResult
				if direct {
					result = r.runEgressPolicyChainASN4(filterapi.Policy{}, "192.0.2.231", 65000, 65000, internal, &session, wire, true)
				} else {
					result = r.runEgressPolicyChain(filterapi.Policy{}, "192.0.2.231", 65000, 65000, internal, &session, wire)
				}
				require.Equal(t, internal && kind == filterapi.ExportExplicit, result.accept, "direct=%v internal=%v kind=%d", direct, internal, kind)
				require.False(t, result.failed)
			}
		}
	}
}

// TestSelectedFinalPolicyRetiresDeniedLearnedOnly keeps a local announcement
// useful beside a learned winner. The final policy changes after egress
// materialization: its real writer must retire the old winner, emit no new
// learned announcement and still complete an explicit absence without a source.
func TestSelectedFinalPolicyRetiresDeniedLearnedOnly(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	f.peer.settings.PeerAS = f.peer.settings.LocalAS
	f.peer.refreshForwardFacts()
	first := f.route(t, 1, "192.0.2.79", 0, false)
	answer, err := f.send(t, t.Context(), first)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	local := []byte{24, 198, 18, 9}
	require.NoError(t, f.peer.session.SendUpdate(ownershipWriterUpdate(t, ownershipRailBody(family.IPv4Unicast, false, local, 0))))
	require.Len(t, f.peer.adjOut.routes, 2)
	// Stage facts still accept, as queued old work would. The final callback
	// reads the new absent internal export rule at the actual socket writer.
	f.peer.session.egressFinalAccepts = func(kind filterapi.ExportKind) bool {
		return (filterapi.Policy{}).AcceptsExportAtEnd(false, kind)
	}
	answer, err = f.send(t, t.Context(), f.route(t, 2, "192.0.2.78", 0, false))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedSuppressed, answer.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 3, "two useful initial announcements, then the exact learned withdrawal")
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, bodies[1], false))
	require.Empty(t, updatePrefixes(t, bodies[2], false))
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[2], true))
	require.Len(t, f.peer.adjOut.routes, 1, "the unrelated explicit local route remains owned")
	require.Empty(t, f.peer.adjOut.selectedPaths)
	absent := rpc.SelectedRoute{Family: first.Family, KeyNLRI: first.KeyNLRI, Revision: 3, Kind: rpc.SelectedRouteAbsent}
	answer, err = f.send(t, t.Context(), absent)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedAbsent, answer.Results[0].Outcome)
	require.Len(t, updateBodies(t, f.conn.written()), 3)
	// A later local announcement still passes the same final callback.
	require.NoError(t, f.peer.session.SendUpdate(ownershipWriterUpdate(t, ownershipRailBody(family.IPv4Unicast, false, []byte{24, 198, 18, 10}, 0))))
	require.Len(t, f.peer.adjOut.routes, 2)
	bodies = updateBodies(t, f.conn.written())
	require.Len(t, bodies, 4)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.10.0/24")}, updatePrefixes(t, bodies[3], false))
}
