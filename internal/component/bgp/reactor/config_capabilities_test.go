// Design: docs/guide/add-path.md -- the add-path limit container and its receive leaf
// Related: config_capabilities.go -- parseAddPathFromTree, addPathReceiveLimit

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// addPathLimitPeerTree builds a peer tree negotiating ipv4/unicast and
// ipv6/unicast with the given add-path block.
func addPathLimitPeerTree(addPath map[string]any) map[string]any {
	return map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}, "local": map[string]any{"ip": "auto"}},
		"session": map[string]any{
			"asn": map[string]any{"remote": "65001"},
			"family": map[string]any{
				"ipv4/unicast": map[string]any{"prefix": map[string]any{"maximum": "100000"}},
				"ipv6/unicast": map[string]any{"prefix": map[string]any{"maximum": "100000"}},
			},
			"capability": map[string]any{"add-path": addPath},
		},
	}
}

// receiveLimit is the parsed shape of `limit { receive N; }`.
func receiveLimit(count string) map[string]any {
	return map[string]any{"receive": count}
}

// configuredPathsLimits parses the peer and returns the PATHS-LIMIT tuples it
// would advertise, keyed by family; nil when no PATHS-LIMIT capability is built.
func configuredPathsLimits(t *testing.T, addPath map[string]any) map[family.Family]uint16 {
	t.Helper()
	settings, err := parsePeerFromTree("peer1", addPathLimitPeerTree(addPath), 65000, 0)
	require.NoError(t, err)
	var limits map[family.Family]uint16
	for _, configured := range settings.Capabilities {
		pathsLimit, ok := configured.(*capability.PathsLimit)
		if !ok {
			continue
		}
		require.Nil(t, limits, "PATHS-LIMIT built more than once")
		limits = make(map[family.Family]uint16)
		for _, entry := range pathsLimit.Entries {
			limits[family.Family{AFI: entry.AFI, SAFI: entry.SAFI}] = entry.Limit
		}
	}
	return limits
}

// TestParseAddPathLimitContainerReceive proves AC-1: a container-level
// `limit { receive 10; }` with no family entries advertises 10 for every
// family Ze advertises in ADD-PATH. The method parses a two-family peer and
// reads back the tuples the OPEN would carry.
func TestParseAddPathLimitContainerReceive(t *testing.T) {
	got := configuredPathsLimits(t, map[string]any{
		"direction": "send/receive",
		"limit":     receiveLimit("10"),
	})
	assert.Equal(t, map[family.Family]uint16{family.IPv4Unicast: 10, family.IPv6Unicast: 10}, got)
}

// TestParseAddPathLimitContainerPerFamilyOverride proves the advertise half of
// AC-3-receive: a family `limit { receive 2; }` replaces the container value
// for that family alone, and every other family keeps the container's 10.
func TestParseAddPathLimitContainerPerFamilyOverride(t *testing.T) {
	got := configuredPathsLimits(t, map[string]any{
		"direction": "send/receive",
		"limit":     receiveLimit("10"),
		"family": map[string]any{
			"ipv4/unicast": map[string]any{"limit": receiveLimit("2")},
		},
	})
	assert.Equal(t, map[family.Family]uint16{family.IPv4Unicast: 2, family.IPv6Unicast: 10}, got)
}

// TestParseAddPathLimitReceiveWithoutReceiveDirection proves the advertise half
// of AC-11: a receive limit on a family whose ADD-PATH direction is send only
// puts no PATHS-LIMIT tuple on the wire, while a send/receive sibling keeps its
// tuple. With every family send-only, no PATHS-LIMIT capability is built.
func TestParseAddPathLimitReceiveWithoutReceiveDirection(t *testing.T) {
	got := configuredPathsLimits(t, map[string]any{
		"direction": "send/receive",
		"limit":     receiveLimit("10"),
		"family": map[string]any{
			"ipv4/unicast": map[string]any{"direction": "send", "limit": receiveLimit("2")},
		},
	})
	assert.Equal(t, map[family.Family]uint16{family.IPv6Unicast: 10}, got)

	got = configuredPathsLimits(t, map[string]any{
		"direction": "send",
		"limit":     receiveLimit("10"),
	})
	assert.Nil(t, got, "send-only ADD-PATH must advertise no PATHS-LIMIT")
}

// TestParseAddPathLimitRefusesInvalidReceive covers the boundary the YANG range
// 1..65535 guards at load, checked again where the reactor reads the tree: zero,
// 65536, a word, and the deleted scalar spelling are each refused with an error,
// never read as "no limit". 65535 is the last valid value.
func TestParseAddPathLimitRefusesInvalidReceive(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit any
	}{
		{"zero", receiveLimit("0")},
		{"above range", receiveLimit("65536")},
		{"not a number", receiveLimit("ten")},
		{"scalar spelling", "10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, addPath := range []map[string]any{
				{"direction": "send/receive", "limit": tc.limit},
				{"direction": "send/receive", "family": map[string]any{"ipv4/unicast": map[string]any{"limit": tc.limit}}},
			} {
				_, err := parsePeerFromTree("peer1", addPathLimitPeerTree(addPath), 65000, 0)
				assert.Error(t, err, "limit %v accepted", tc.limit)
			}
		})
	}

	got := configuredPathsLimits(t, map[string]any{"direction": "send/receive", "limit": receiveLimit("65535")})
	assert.Equal(t, map[family.Family]uint16{family.IPv4Unicast: 65535, family.IPv6Unicast: 65535}, got)
}

// TestParseAddPathLimitRefusesUnenforceableFamily proves that what Ze
// advertises and what it enforces cannot disagree: a receive limit that would
// reach a family Ze cannot count path by path (no NLRI splitter, the
// pathsLimitEnforceable predicate the admission also asks) is refused at config
// load with an error naming the family, whether the limit sits on the family or
// on the add-path container. A send-only entry for that family asks for no
// paths, so the container limit still loads and carries no tuple for it. The
// method registers a family with no splitter for the test's lifetime and parses
// a peer that negotiates it.
func TestParseAddPathLimitRefusesUnenforceableFamily(t *testing.T) {
	registration := []family.FamilyRegistration{{AFI: family.AFIIPv4, SAFI: 241, AFIName: "ipv4", SAFIName: "test-uncountable"}}
	_, err := family.RegisterFamilyBatch(registration)
	require.NoError(t, err)
	t.Cleanup(func() { family.UnregisterFamilyBatch(registration) })
	uncountable, ok := family.LookupFamily("ipv4/test-uncountable")
	require.True(t, ok)
	require.False(t, pathsLimitEnforceable(uncountable), "the test family gained a splitter; pick one with none")

	parse := func(addPath map[string]any) (*PeerSettings, error) {
		tree := addPathLimitPeerTree(addPath)
		session, _ := tree["session"].(map[string]any)
		families, _ := session["family"].(map[string]any)
		families["ipv4/test-uncountable"] = map[string]any{"prefix": map[string]any{"maximum": "100000"}}
		return parsePeerFromTree("peer1", tree, 65000, 0)
	}

	for name, addPath := range map[string]map[string]any{
		"family limit": {"direction": "send/receive", "family": map[string]any{
			"ipv4/test-uncountable": map[string]any{"limit": receiveLimit("2")}}},
		"container limit": {"direction": "send/receive", "limit": receiveLimit("10")},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parse(addPath)
			require.Error(t, err, "a receive limit Ze cannot enforce was accepted")
			assert.Contains(t, err.Error(), "ipv4/test-uncountable")
			assert.Contains(t, err.Error(), "does not enforce")
		})
	}

	settings, err := parse(map[string]any{"direction": "send/receive", "limit": receiveLimit("10"),
		"family": map[string]any{"ipv4/test-uncountable": map[string]any{"direction": "send"}}})
	require.NoError(t, err)
	for _, configured := range settings.Capabilities {
		pathsLimit, isPathsLimit := configured.(*capability.PathsLimit)
		if !isPathsLimit {
			continue
		}
		for _, entry := range pathsLimit.Entries {
			assert.NotEqual(t, uncountable, family.Family{AFI: entry.AFI, SAFI: entry.SAFI}, "a send-only family carries no tuple")
		}
	}
}
