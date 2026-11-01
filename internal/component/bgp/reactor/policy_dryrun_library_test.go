package reactor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
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

// twoMemberLibraryPolicy is an import policy whose library chain runs "first"
// (filterType first) then "second" (filterType second), final accept.
func twoMemberLibraryPolicy(t *testing.T, first, second string) filterapi.Policy {
	t.Helper()
	reg, err := routepolicy.NewRegistry(routepolicy.Instance{Name: "first", Type: first}, routepolicy.Instance{Name: "second", Type: second})
	require.NoError(t, err)
	chain, err := policychain.Compile(libraryTestImport, reg, []policychain.Member{{Name: "first"}, {Name: "second"}},
		routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	require.NoError(t, err)
	refs := []routepolicy.Ref{{Name: "first"}, {Name: "second"}}
	return mustPolicy(t, refs, routepolicy.FinalAccept).WithLibrary(chain)
}

// TestPolicyDryRunLibraryFilterOverride proves show policy test's single
// filter override on a library chain runs that one member through the chain,
// display only, rather than a filter-update to a plugin named after it.
//
// Method: the chain's first member rejects and its second sets
// local-preference 200. The override names the second, so the run must
// modify with only the second's entry: running the whole chain rejects, and
// the text path answers a failed RPC. An unknown name is refused.
func TestPolicyDryRunLibraryFilterOverride(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	settings.ImportPolicy = twoMemberLibraryPolicy(t, libraryTypeReject, libraryTypeLP200)
	r.peers[settings.PeerKey()] = NewPeer(settings)
	adapter := &reactorAPIAdapter{r: r}

	result, err := adapter.PolicyDryRun("192.0.2.1", directionImport, "second", libraryTestBody(100), true)
	require.NoError(t, err)
	require.Equal(t, dryRunActionModify, result.Action)
	require.Len(t, result.Trace, 1)
	entry := result.Trace[0]
	require.Equal(t, "second", entry.Canonical)
	require.Equal(t, dryRunActionModify, entry.Action)
	require.Equal(t, "local-preference 200", entry.Delta)
	require.Contains(t, result.TextAfter, "local-preference 200")
	require.Equal(t, []string{"first", "second"}, result.Policy.Filters,
		"the report names the policy the live session runs")

	_, err = adapter.PolicyDryRun("192.0.2.1", directionImport, "absent", libraryTestBody(100), true)
	require.ErrorIs(t, err, errFilterNotFound)

	chain, ok := settings.ImportPolicy.Library().(*policychain.Chain)
	require.True(t, ok)
	require.Zero(t, chain.Refused(), "a dry-run refuses no route")
}
