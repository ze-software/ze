// Design: docs/architecture/api/json-format.md -- State Events, forwarder session-scope flags.
// Related: text_human.go -- appendStateChangeText.
// Related: text_json.go -- appendStateChangeJSON.

package format

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestStateCarriesSessionScopeFlags verifies forwarder session-scope flag rendering.
// VALIDATES: AppendStateChange emits rs-client, is-ibgp, and rr-client in both
// text and JSON state events when true, and omits them when false.
// PREVENTS: Forwarders (bgp-rs, bgp-rr) failing to distinguish client, internal,
// and external sessions over text or JSON transports.
func TestStateCarriesSessionScopeFlags(t *testing.T) {
	cases := []struct {
		name     string
		rsClient bool
		isIBGP   bool
		rrClient bool
	}{
		{name: "external-plain"},
		{name: "rs-client", rsClient: true},
		{name: "ibgp-non-client", isIBGP: true},
		{name: "ibgp-rr-client", isIBGP: true, rrClient: true},
		{name: "all-flags", rsClient: true, isIBGP: true, rrClient: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer := plugin.PeerInfo{
				Address:              netip.MustParseAddr("192.0.2.50"),
				PeerAS:               rpc.RemoteASOf(65001),
				RSClient:             tc.rsClient,
				IsIBGP:               tc.isIBGP,
				RouteReflectorClient: tc.rrClient,
			}

			text := string(AppendStateChange(nil, &peer, rpc.SessionStateUp, "", nil, plugin.EncodingText))
			require.Equal(t, tc.rsClient, strings.Contains(text, " rs-client true"))
			require.Equal(t, tc.isIBGP, strings.Contains(text, " is-ibgp true"))
			require.Equal(t, tc.rrClient, strings.Contains(text, " rr-client true"))
			if !tc.rsClient {
				require.NotContains(t, text, "rs-client")
			}
			if !tc.isIBGP {
				require.NotContains(t, text, "is-ibgp")
			}
			if !tc.rrClient {
				require.NotContains(t, text, "rr-client")
			}

			rawJSON := AppendStateChange(nil, &peer, rpc.SessionStateUp, "", nil, plugin.EncodingJSON)
			jsonStr := string(rawJSON)
			require.Equal(t, tc.rsClient, strings.Contains(jsonStr, `"rs-client":true`))
			require.Equal(t, tc.isIBGP, strings.Contains(jsonStr, `"is-ibgp":true`))
			require.Equal(t, tc.rrClient, strings.Contains(jsonStr, `"rr-client":true`))

			var envelope struct {
				BGP struct {
					RSClient bool `json:"rs-client"`
					IsIBGP   bool `json:"is-ibgp"`
					RRClient bool `json:"rr-client"`
				} `json:"bgp"`
			}
			require.NoError(t, json.Unmarshal(rawJSON, &envelope))
			require.Equal(t, tc.rsClient, envelope.BGP.RSClient)
			require.Equal(t, tc.isIBGP, envelope.BGP.IsIBGP)
			require.Equal(t, tc.rrClient, envelope.BGP.RRClient)
		})
	}
}
