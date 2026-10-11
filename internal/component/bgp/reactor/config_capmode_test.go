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
// enable/disable/require/refuse is an error at every site that reads one,
// and the error names the word.
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

// TestCapabilityModeLeafRefusesOtherShapes proves route-refresh and
// extended-message are read as capability-mode leaves and nothing else. Until
// 2026-10-11 both were presence containers read through flexString: the
// editor's `route-refresh { }` reached the reactor as a map, flexString answered
// "" for it, and the peer advertised no capability and logged nothing. The words
// true and false belonged to that presence spelling and went with it.
//
// VALIDATES: a map value, an empty value and the words true/false are refused,
// naming the capability; each of the four modes is accepted; enable advertises
// and refuse does not.
// PREVENTS: a configured capability silently missing from the OPEN.
func TestCapabilityModeLeafRefusesOtherShapes(t *testing.T) {
	for _, name := range []string{"route-refresh", "extended-message"} {
		for _, value := range []any{map[string]any{}, "true", "false", ""} {
			_, err := parsePeerFromTree("peer1", capModeTree(map[string]any{name: value}), 65000, 0)
			require.Error(t, err, "%s %#v must be refused", name, value)
			require.Contains(t, err.Error(), name)
		}
		for _, mode := range []string{"enable", "disable", "require", "refuse"} {
			_, err := parsePeerFromTree("peer1", capModeTree(map[string]any{name: mode}), 65000, 0)
			require.NoError(t, err, "%s %s", name, mode)
		}
	}

	ps, err := parsePeerFromTree("peer1", capModeTree(map[string]any{"route-refresh": "enable"}), 65000, 0)
	require.NoError(t, err)
	require.True(t, hasRouteRefresh(ps), "route-refresh enable must advertise")

	ps, err = parsePeerFromTree("peer1", capModeTree(map[string]any{"route-refresh": "refuse"}), 65000, 0)
	require.NoError(t, err)
	require.False(t, hasRouteRefresh(ps), "route-refresh refuse must not advertise")
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
