// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- the neighbor set CoPP derives from the bgp section
// Overview: bgp_neighbor_sources.go -- BGPNeighborSources, the code under test

package config

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bgpSectionFromJSON decodes a bgp body in the shape the plugin SDK delivers,
// so the test reads what CoPP reads rather than a hand-built Go map.
func bgpSectionFromJSON(t *testing.T, body string) map[string]any {
	t.Helper()

	var bgp map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &bgp))
	return bgp
}

// TestNeighborSourcesFromBGPSection proves the one reader of
// `connection > remote` answers every source a configured neighbor may
// connect from: static IPv4, IPv6 and link-local peers as host prefixes,
// standalone and grouped, and a dynamic group's ranges as written.
//
// VALIDATES: BGPNeighborSources over a section holding each kind of source.
// PREVENTS: a CoPP neighbor set that drops a real peer's SYN because the set
// was built by a second reader that disagrees with the BGP parse.
func TestNeighborSourcesFromBGPSection(t *testing.T) {
	bgp := bgpSectionFromJSON(t, `{
		"peer": {
			"v4": {"connection": {"remote": {"ip": "192.0.2.1"}}},
			"v6": {"connection": {"remote": {"ip": "2001:db8::1"}}},
			"ll": {"connection": {"remote": {"ip": "fe80::1%eth0"}}}
		},
		"group": {
			"static": {"peer": {"member": {"connection": {"remote": {"ip": "198.51.100.7"}}}}},
			"inherit": {"connection": {"remote": {"ip": "203.0.113.9"}}, "peer": {"only": {}}},
			"dyn": {
				"connection": {"remote": {"ip": "dynamic", "range": ["10.10.0.0/16", "2001:db8:100::/48"], "connect": "false"}}
			},
			"dyn-one": {"connection": {"remote": {"ip": "dynamic", "range": "172.16.0.0/12", "connect": "false"}}}
		}
	}`)

	got, err := BGPNeighborSources(bgp)
	require.NoError(t, err)
	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("10.10.0.0/16"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.0.2.1/32"),
		netip.MustParsePrefix("198.51.100.7/32"),
		netip.MustParsePrefix("203.0.113.9/32"),
		netip.MustParsePrefix("2001:db8::1/128"),
		netip.MustParsePrefix("2001:db8:100::/48"),
		netip.MustParsePrefix("fe80::1/128"),
	}, got)
}

// TestNeighborSourcesEmptySection keeps an empty answer distinct from a
// failure: a bgp section with no peer and no group names no neighbor, and
// that is a valid set, not an error.
func TestNeighborSourcesEmptySection(t *testing.T) {
	got, err := BGPNeighborSources(bgpSectionFromJSON(t, `{"router-id": "192.0.2.254"}`))
	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestNeighborSourcesRefusesWhatTheParseRefuses proves the reader never
// answers a set for a section the BGP parse rejects, so it cannot omit a
// neighbor the parse would accept: each case names the source it refused.
func TestNeighborSourcesRefusesWhatTheParseRefuses(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"missing ip":      {`{"peer": {"p": {"connection": {"remote": {}}}}}`, "bgp/peer p: missing required connection > remote > ip"},
		"dynamic on peer": {`{"peer": {"p": {"connection": {"remote": {"ip": "dynamic"}}}}}`, "bgp/peer p: ip dynamic is only valid at group level"},
		"bad ip":          {`{"peer": {"p": {"connection": {"remote": {"ip": "not-an-ip"}}}}}`, `bgp/peer p: invalid remote ip "not-an-ip"`},
		"bad range":       {`{"group": {"g": {"connection": {"remote": {"ip": "dynamic", "range": ["10.0.0.0/33"]}}}}}`, `bgp/group g: invalid range "10.0.0.0/33"`},
		"no range":        {`{"group": {"g": {"connection": {"remote": {"ip": "dynamic"}}}}}`, "bgp/group g: ip dynamic requires at least one range"},
		"grouped missing": {`{"group": {"g": {"peer": {"m": {}}}}}`, "bgp/group g peer m: missing required connection > remote > ip"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := BGPNeighborSources(bgpSectionFromJSON(t, tc.body))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
