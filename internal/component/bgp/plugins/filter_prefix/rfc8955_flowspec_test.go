package filter_prefix

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// RFC requirement: RFC8955-3-1 positive -- the configured unicast prefix/ge/le policy accepts FlowSpec destinations in the same permitted range (§3).
// RFC requirement: RFC8955-3-1 negative -- a denied, missing, or mixed permitted/denied FlowSpec destination cannot bypass prefix policy or become a unicast rewrite (§3).
func TestPrefixPolicyFiltersFlowSpecDestination(t *testing.T) {
	old := listsByName.Load()
	t.Cleanup(func() { listsByName.Store(old) })
	lists := map[string]*prefixList{"protected": {entries: []prefixEntry{{prefix: netip.MustParsePrefix("10.0.0.0/8"), ge: 16, le: 24, action: actionAccept}}}}
	listsByName.Store(&lists)
	for _, tc := range []struct {
		nlri   string
		action sdk.FilterAction
	}{
		{"ipv4/flow add 10.1.0.0/24", sdk.FilterAccept},
		{"ipv4/flow add 10.1.0.0/25", sdk.FilterReject},
		{"ipv4/flow add 192.0.2.0/24", sdk.FilterReject},
		{"ipv4/flow add invalid", sdk.FilterReject},
		{"ipv4/flow add 10.1.0.0/24 192.0.2.0/24", sdk.FilterReject},
		{"ipv4/flow-vpn add 10.1.0.0/24", sdk.FilterAccept},
	} {
		t.Run(tc.nlri, func(t *testing.T) {
			out := handleFilterUpdate(&sdk.FilterUpdateInput{Filter: "protected", Update: "nlri " + tc.nlri})
			require.Equal(t, tc.action, out.Action)
			require.Empty(t, out.Update, "FlowSpec components are never replaced by a destination projection")
		})
	}
}
