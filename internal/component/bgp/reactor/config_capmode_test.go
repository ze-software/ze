// Design: docs/architecture/config/syntax.md -- capability modes
// Related: config_capabilities.go -- parseCapMode and every caller that reads a mode word

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// capModeTree builds a peer tree whose capability block is capConfig.
func capModeTree(capConfig map[string]any) map[string]any {
	return map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}, "local": map[string]any{"ip": "auto"}},
		"session":    map[string]any{"asn": map[string]any{"remote": "65001"}, "capability": capConfig},
	}
}

// TestCapabilityModeRefusesUnknownWord proves a mode word outside
// enable/disable/require/refuse (plus true/false for a presence capability)
// is an error at every site that reads one, and the error names the word.
// Until 2026-09-15 parseCapMode answered enable for a word it did not know,
// so a typo advertised the capability with no line in the log.
//
// VALIDATES: parsePeerFromTree returns an error naming the word for asn4,
// extended-message, route-refresh, graceful-restart mode, an inline nexthop
// mode, a nexthop list-entry mode, and a per-family add-path mode.
// PREVENTS: a silent default at a guard (ai/rules/principles.md).
func TestCapabilityModeRefusesUnknownWord(t *testing.T) {
	sites := []struct {
		name      string
		capConfig map[string]any
	}{
		{"asn4", map[string]any{"asn4": "bogus"}},
		{"extended-message", map[string]any{"extended-message": "bogus"}},
		{"route-refresh", map[string]any{"route-refresh": "bogus"}},
		{"graceful-restart mode", map[string]any{"graceful-restart": map[string]any{"mode": "bogus"}}},
		{"nexthop inline mode", map[string]any{"nexthop": map[string]any{"ipv4/unicast": "ipv6 bogus"}}},
		{"nexthop entry mode", map[string]any{"nexthop": map[string]any{"ipv4/unicast": map[string]any{"nhafi": "ipv6", "mode": "bogus"}}}},
		{"add-path family mode", map[string]any{"add-path": map[string]any{
			"direction": "send/receive",
			"family":    map[string]any{"ipv4/unicast": map[string]any{"mode": "bogus"}},
		}}},
	}
	for _, site := range sites {
		t.Run(site.name, func(t *testing.T) {
			_, err := parsePeerFromTree("peer1", capModeTree(site.capConfig), 65000, 0)
			require.Error(t, err, "an unknown mode word must be refused")
			require.Contains(t, err.Error(), `"bogus"`)
			require.Contains(t, err.Error(), "enable, disable, require, refuse")
		})
	}
}

// TestCapabilityModeKeepsPresenceSpelling proves the true/false arm a bare
// presence capability relies on survives the refusal above: the parser stores
// `route-refresh;` as "true", and that word must still mean enable.
//
// VALIDATES: "true" advertises route-refresh and "false" does not, with no error.
// PREVENTS: the unknown-word refusal swallowing the presence spelling.
func TestCapabilityModeKeepsPresenceSpelling(t *testing.T) {
	ps, err := parsePeerFromTree("peer1", capModeTree(map[string]any{"route-refresh": "true"}), 65000, 0)
	require.NoError(t, err)
	require.True(t, hasRouteRefresh(ps), "route-refresh true must advertise")

	ps, err = parsePeerFromTree("peer1", capModeTree(map[string]any{"route-refresh": "false"}), 65000, 0)
	require.NoError(t, err)
	require.False(t, hasRouteRefresh(ps), "route-refresh false must not advertise")
}

// hasRouteRefresh reports whether ps advertises the RFC 2918 route-refresh capability.
func hasRouteRefresh(ps *PeerSettings) bool {
	for _, c := range ps.Capabilities {
		if _, ok := c.(*capability.RouteRefresh); ok {
			return true
		}
	}
	return false
}
