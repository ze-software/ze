// Design: docs/guide/bgp-policy.md -- read-only route-kind prediction.
package reactor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// TestPolicyDryRunExportSimulation applies each simulated class through empty
// and compiled automatic chains, with an external session as the refusal control.
func TestPolicyDryRunExportSimulation(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		for _, remoteAS := range []uint32{65000, 65001} {
			for _, kind := range []string{"", "learned", "local"} {
				r := New(&Config{})
				settings := NewPeerSettings(mustParseAddr("192.0.2.233"), 65000, remoteAS, 1)
				if compiled {
					reg, err := routepolicy.NewRegistry()
					require.NoError(t, err)
					chain, err := policychain.CompileUnconfigured(answerAttachment, reg, nil, policychain.Bases{})
					require.NoError(t, err)
					settings.ExportPolicy = filterapi.InternalPolicy(nil).WithLibrary(chain)
				}
				peer := NewPeer(settings)
				r.peers[settings.PeerKey()] = peer
				result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(peer.sourceOwner, directionExport, "", dryRunUpdateBody(t), true, kind)
				require.NoError(t, err)
				wantKind := kind
				if wantKind == "" {
					wantKind = "learned"
				}
				require.Equal(t, wantKind, result.SimulatedRouteKind)
				want := dryRunActionReject
				if remoteAS == 65000 && kind == "local" {
					want = dryRunActionAccept
				}
				require.Equal(t, want, result.Action, "compiled=%v remote-as=%d kind=%q", compiled, remoteAS, kind)
				if remoteAS == 65000 && kind != "local" {
					require.NotNil(t, result.Policy)
					require.Equal(t, plugin.PolicyFinalSourceLearnedExport, result.Policy.FinalSource)
				}
				require.False(t, settings.ExportPolicy.Configured())
				require.Empty(t, peer.adjOut.routes)
			}
		}
	}
}

// TestPolicyDryRunSimulationRefusals exercises the public adapter rather than
// its token parser, so direct callers cannot bypass import or unknown checks.
func TestPolicyDryRunSimulationRefusals(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.234"), 65000, 65000, 1)
	settings.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	settings.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	peer := NewPeer(settings)
	r.peers[settings.PeerKey()] = peer
	api := &reactorAPIAdapter{r: r}
	for _, test := range []struct{ direction, kind string }{
		{directionImport, "learned"}, {directionImport, "local"},
		{directionExport, "role"}, {directionExport, "unknown"},
	} {
		result, err := api.PolicyDryRun(peer.sourceOwner, test.direction, "", dryRunUpdateBody(t), true, test.kind)
		require.Error(t, err, "%+v", test)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "route-kind")
		require.Contains(t, err.Error(), "fix: ze help show policy test peer")
	}
	result, err := api.PolicyDryRun(peer.sourceOwner, directionImport, "", dryRunUpdateBody(t), true, "")
	require.NoError(t, err)
	require.Equal(t, dryRunActionAccept, result.Action)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "simulated-route-kind")
	require.Empty(t, peer.adjOut.routes)
}

// TestPolicyDryRunLearnedDefaultKeepsWithdrawals proves a default rejection
// removes announcements only, on both empty and compiled automatic chains.
func TestPolicyDryRunLearnedDefaultKeepsWithdrawals(t *testing.T) {
	announcement := dryRunUpdateBody(t)
	mixed := append([]byte{0, 4, 24, 192, 0, 2}, announcement[2:]...)
	for _, compiled := range []bool{false, true} {
		for _, body := range []struct {
			update []byte
			action string
		}{
			{[]byte{0, 4, 24, 192, 0, 2, 0, 0}, dryRunActionAccept},
			{mixed, dryRunActionModify},
		} {
			r := New(&Config{})
			settings := NewPeerSettings(mustParseAddr("192.0.2.235"), 65000, 65000, 1)
			if compiled {
				reg, err := routepolicy.NewRegistry()
				require.NoError(t, err)
				chain, err := policychain.CompileUnconfigured(answerAttachment, reg, nil, policychain.Bases{})
				require.NoError(t, err)
				settings.ExportPolicy = filterapi.InternalPolicy(nil).WithLibrary(chain)
			}
			peer := NewPeer(settings)
			r.peers[settings.PeerKey()] = peer
			result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(peer.sourceOwner, directionExport, "", body.update, true, "")
			require.NoError(t, err)
			require.Equal(t, body.action, result.Action, "compiled=%v", compiled)
			require.Equal(t, "nlri del 192.0.2.0/24", result.TextAfter)
			require.NotContains(t, result.TextAfter, "nlri add")
			require.Empty(t, peer.adjOut.routes)
		}
	}
}
