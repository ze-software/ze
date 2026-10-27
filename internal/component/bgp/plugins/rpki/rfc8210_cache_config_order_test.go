package rpki

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// The schema the configuration under test is parsed against: ze-bgp-conf and
	// the ze-hub-conf it imports. This plugin's own ze-rpki is registered by the
	// package under test (register.go).
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	_ "github.com/ze-software/ze/internal/component/hub/yang"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// RFC naming: untagged -- RFC 8210 Section 10 permits a non-unique preference and names no order among equal values, so the tie order proven here is Ze's configuration contract, not a requirement row.

// VALIDATES: caches that share a preference are tried in the order the operator
// wrote them, through the whole production path from config text to the order
// startSessions opens the RTR sessions in.
// PREVENTS: parseRPKIConfig ranging over the delivered cache-server object, a Go
// map, so equal-preference caches came out in a different order on each load and
// the stable sort in startSessions had no operator order left to keep.
//
// RFC 8210 Section 10: "In addition, in a router, this list has a non-unique
// preference value for each server." Section 10 then says "The client router
// attempts to establish a session with each potential serving cache in preference
// order", so a tie leaves the configured order as the only order there is.
//
// Method: the configuration is loaded by the real loader and lowered by
// ToPluginMap, the map a plugin is delivered, then marshaled the way the config
// RPC sends it. That one payload is parsed many times, because a map range picks
// a fresh random start on every iteration and a single parse can match the
// expected order by chance. Six entries leave a chance ordering out of reach.
func TestRFC8210EqualPreferenceCachesKeepConfigOrder(t *testing.T) {
	// Written order, with two preference classes interleaved. Neither class is in
	// address order, so a sort by address cannot pass either.
	written := []struct {
		address    string
		preference string
	}{
		{"192.0.2.40", "100"},
		{"192.0.2.3", "50"},
		{"192.0.2.200", "100"},
		{"192.0.2.1", "100"},
		{"192.0.2.77", "50"},
		{"192.0.2.9", "100"},
	}
	want := []string{
		"192.0.2.3", "192.0.2.77", // preference 50, in written order
		"192.0.2.40", "192.0.2.200", "192.0.2.1", "192.0.2.9", // preference 100, in written order
	}

	var text strings.Builder
	text.WriteString("bgp {\n\trouter-id 192.0.2.254\n\tsession {\n\t\tasn {\n\t\t\tlocal 65000\n\t\t}\n\t}\n\trpki {\n")
	for _, cache := range written {
		text.WriteString("\t\tcache-server " + cache.address + " {\n")
		text.WriteString("\t\t\ttrusted-network true\n")
		text.WriteString("\t\t\tpreference " + cache.preference + "\n")
		text.WriteString("\t\t}\n")
	}
	text.WriteString("\t}\n}\n")

	result, err := zeconfig.LoadConfig(text.String(), "test.conf", nil)
	require.NoError(t, err, "the configuration under test must parse")
	subtree := zeconfig.ExtractConfigSubtree(result.Tree.ToPluginMap(), configRootBGP)
	require.NotNil(t, subtree, "ExtractConfigSubtree returned nil, so the plugin would be handed {}")
	payload, err := json.Marshal(subtree)
	require.NoError(t, err, "marshal the delivered subtree")

	const parses = 200
	for run := range parses {
		cfg, err := parseRPKIConfig(string(payload))
		require.NoError(t, err, "parse %d", run)
		sortCachesByPreference(cfg.CacheServers)

		got := make([]string, 0, len(cfg.CacheServers))
		for _, cache := range cfg.CacheServers {
			got = append(got, cache.Address)
		}
		require.Equal(t, want, got, "parse %d: session order must be preference, then config order", run)
	}
}

// VALIDATES: a cache-server list of two or more entries that arrives with no
// recorded order is refused, naming the list.
// PREVENTS: the silent path coming back. Reading such a list in any order is a
// guess at what the operator wrote, and two equal-preference caches would be
// tried in that guessed order with nothing in the log.
func TestParseRPKIConfigRefusesCachesWithoutOrder(t *testing.T) {
	jsonStr := `{
		"rpki": {
			"cache-server": {
				"192.0.2.1": {"trusted-network": "true"},
				"192.0.2.2": {"trusted-network": "true"}
			}
		}
	}`

	_, err := parseRPKIConfig(jsonStr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cache-server")
}
