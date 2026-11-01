package reactor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// TestPolicyDryRunRunsLibraryChain proves the in-process half of AC-16
// (D-11): show policy test on a library chain runs the chain the live import
// step runs, display only, instead of sending a filter-update for a member no
// plugin serves. The decision and the resulting text are the live path's, the
// trace names each member with the sets it asked, and the chain counts no
// failure.
//
// Method: a member that sets local-preference 200 must report modify with
// that set and the rewritten text; a member that rejects must report reject.
// The text path answered both as a failed RPC to a plugin named "member".
func TestPolicyDryRunRunsLibraryChain(t *testing.T) {
	tests := []struct {
		name, filterType, action string
		entry                    plugin.PolicyTraceEntry
		textAfter                string
	}{
		{name: "sets", filterType: libraryTypeLP200, action: dryRunActionModify,
			entry:     plugin.PolicyTraceEntry{Filter: "member", Canonical: "member", Action: dryRunActionModify, Delta: "local-preference 200"},
			textAfter: "local-preference 200"},
		{name: "rejects", filterType: libraryTypeReject, action: dryRunActionReject,
			entry: plugin.PolicyTraceEntry{Filter: "member", Canonical: "member", Action: dryRunActionReject}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := New(&Config{})
			settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
			settings.ImportPolicy = libraryPolicy(t, libraryTestImport, tc.filterType, routepolicy.FinalAccept)
			r.peers[settings.PeerKey()] = NewPeer(settings)
			result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
				"192.0.2.1", directionImport, "", libraryTestBody(100), true)
			require.NoError(t, err)
			require.Equal(t, tc.action, result.Action)
			require.Equal(t, []plugin.PolicyTraceEntry{tc.entry}, result.Trace)
			if tc.textAfter != "" {
				require.True(t, strings.Contains(result.TextAfter, tc.textAfter), "text-after %q lacks %q", result.TextAfter, tc.textAfter)
			}
			chain, ok := settings.ImportPolicy.Library().(*policychain.Chain)
			require.True(t, ok)
			require.Zero(t, chain.Refused(), "a dry-run refuses no route")
		})
	}
}
