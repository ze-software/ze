// Design: docs/architecture/core-design.md — config tree parsing (PeersFromTree)
// Overview: config_prefix.go — the parser these tests drive
// Related: session_prefix_family_test.go — enforcement of the values parsed here

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// twoFamilyPrefixTree builds a peer config tree with two families, each with
// its own prefix block. Each block is a map of leaf name to value, matching the
// shape ApplyDefaults leaves behind: every leaf value is a string.
func twoFamilyPrefixTree(v4, v6 map[string]any) map[string]any {
	return map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "10.0.0.1"},
			"local":  map[string]any{"ip": "auto"},
		},
		"session": map[string]any{
			"asn": map[string]any{"remote": "65001"},
			"family": map[string]any{
				"ipv4/unicast": map[string]any{"mode": "enable", "prefix": v4},
				"ipv6/unicast": map[string]any{"mode": "enable", "prefix": v6},
			},
		},
	}
}

// TestPrefixTeardownPerFamilyDisagreement verifies two families keep opposite
// teardown settings through the parse.
//
// VALIDATES: AC-1, AC-2. The operator asked for teardown on ipv4/unicast and
// warn-only on ipv6/unicast, and gets exactly that.
// PREVENTS: The per-peer scalar this spec removed, where the last family in
// sorted key order set the value for every family. "ipv4/unicast" sorts before
// "ipv6/unicast" (byte '4' < byte '6'), so the IPv6 value used to win and both
// families became warn-only.
func TestPrefixTeardownPerFamilyDisagreement(t *testing.T) {
	tree := twoFamilyPrefixTree(
		map[string]any{"maximum": "100", "teardown": "true"},
		map[string]any{"maximum": "200", "teardown": "false"},
	)
	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	assert.True(t, ps.prefixTeardownFor("ipv4/unicast"),
		"ipv4/unicast asked for teardown and must keep it")
	assert.False(t, ps.prefixTeardownFor("ipv6/unicast"),
		"ipv6/unicast asked for warn-only and must keep it")
}

// TestPrefixTeardownOmittedFamilyDoesNotOverwrite verifies a family carrying
// only the materialized YANG default leaves an explicit sibling alone.
//
// VALIDATES: AC-3. The operator set `teardown false` on ipv4/unicast and said
// nothing about ipv6/unicast.
// PREVENTS: The worst case of the per-peer scalar. ApplyDefaults materializes
// `teardown true` into every family entry, so the silent family arrived with a
// value and overwrote the family that had expressed an opinion.
func TestPrefixTeardownOmittedFamilyDoesNotOverwrite(t *testing.T) {
	// ipv6/unicast carries the value ApplyDefaults materializes, not an
	// operator choice (internal/component/config/schema_defaults.go).
	tree := twoFamilyPrefixTree(
		map[string]any{"maximum": "100", "teardown": "false"},
		map[string]any{"maximum": "200", "teardown": "true"},
	)
	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	assert.False(t, ps.prefixTeardownFor("ipv4/unicast"),
		"the explicit warn-only choice survives a defaulted sibling")
	assert.True(t, ps.prefixTeardownFor("ipv6/unicast"))
}

// TestPrefixTeardownSortOrderIndependent verifies neither key-sort position
// decides the outcome.
//
// VALIDATES: AC-5. Both assignments of the same pair of values give each family
// its own value.
// PREVENTS: A fix that only moves which family wins. The parser sorts family
// keys to keep Multiprotocol capability order stable in the OPEN message
// (config.go, parseFamiliesFromTree), and that sort must decide nothing else.
func TestPrefixTeardownSortOrderIndependent(t *testing.T) {
	tests := []struct {
		name     string
		v4, v6   string
		wantIPv4 bool
		wantIPv6 bool
	}{
		{"teardown on the first key", "true", "false", true, false},
		{"teardown on the last key", "false", "true", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := twoFamilyPrefixTree(
				map[string]any{"maximum": "100", "teardown": tt.v4},
				map[string]any{"maximum": "200", "teardown": tt.v6},
			)
			ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
			require.NoError(t, err)
			assert.Equal(t, tt.wantIPv4, ps.prefixTeardownFor("ipv4/unicast"))
			assert.Equal(t, tt.wantIPv6, ps.prefixTeardownFor("ipv6/unicast"))
		})
	}
}

// TestPrefixIdleTimeoutPerFamilyParse verifies each family keeps its own
// idle-timeout through the parse.
//
// VALIDATES: AC-4, parse half. The enforcement half is
// TestPrefixIdleTimeoutPerFamily.
// PREVENTS: One family's reconnect delay being applied to another family.
func TestPrefixIdleTimeoutPerFamilyParse(t *testing.T) {
	tree := twoFamilyPrefixTree(
		map[string]any{"maximum": "100", "idle-timeout": "30"},
		map[string]any{"maximum": "200", "idle-timeout": "7"},
	)
	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	assert.Equal(t, uint16(30), ps.prefixIdleTimeoutFor("ipv4/unicast"))
	assert.Equal(t, uint16(7), ps.prefixIdleTimeoutFor("ipv6/unicast"))
}

// TestPrefixIdleTimeoutBoundaryPerFamily verifies the uint16 edges of
// idle-timeout survive per family.
//
// VALIDATES: idle-timeout 0 (the YANG default) and 65535 (the uint16 ceiling)
// are both stored against their own family.
// PREVENTS: A zero on one family reading as "unset" and picking up a sibling's
// value.
func TestPrefixIdleTimeoutBoundaryPerFamily(t *testing.T) {
	tree := twoFamilyPrefixTree(
		map[string]any{"maximum": "100", "idle-timeout": "0"},
		map[string]any{"maximum": "200", "idle-timeout": "65535"},
	)
	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	assert.Equal(t, uint16(0), ps.prefixIdleTimeoutFor("ipv4/unicast"))
	assert.Equal(t, uint16(65535), ps.prefixIdleTimeoutFor("ipv6/unicast"))
}

// TestPrefixReconnectPerFamilyParse verifies each family keeps its own
// reconnect mode through the parse, and that an absent leaf derives its mode
// from that family's own idle-timeout.
//
// VALIDATES: the `reconnect` leaf added on 2026-08-03, when Thomas ruled that
// `idle-timeout 0` keeps a peer DOWN. `backoff` is the explicit way to ask for
// the reconnect the old code did by accident.
// PREVENTS: one family's reconnect answer governing the peer, which is the
// defect this spec removed for the three sibling leaves.
func TestPrefixReconnectPerFamilyParse(t *testing.T) {
	tree := twoFamilyPrefixTree(
		map[string]any{"maximum": "100", "reconnect": "backoff"},
		map[string]any{"maximum": "200", "idle-timeout": "30", "reconnect": "timer"},
	)
	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	assert.Equal(t, PrefixReconnectBackoff, ps.PrefixReconnectFor("ipv4/unicast"))
	assert.Equal(t, PrefixReconnectTimer, ps.PrefixReconnectFor("ipv6/unicast"))
}

// TestPrefixReconnectDefaults verifies what a config that never mentions
// `reconnect` means, which is every config written before the leaf existed.
//
// VALIDATES: an absent leaf with no idle-timeout reads as never, and an absent
// leaf with a timer keeps the timer. The second half is the migration promise:
// a peer configured with `idle-timeout 30` behaves exactly as it did.
// PREVENTS: the new default silently disabling a configured reconnect timer.
func TestPrefixReconnectDefaults(t *testing.T) {
	tree := twoFamilyPrefixTree(
		map[string]any{"maximum": "100", "idle-timeout": "0"},
		map[string]any{"maximum": "200", "idle-timeout": "30"},
	)
	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	assert.Equal(t, PrefixReconnectNever, ps.PrefixReconnectFor("ipv4/unicast"))
	assert.Equal(t, PrefixReconnectTimer, ps.PrefixReconnectFor("ipv6/unicast"))
	assert.Equal(t, PrefixReconnectNever, ps.PrefixReconnectFor("ipv6/multicast"),
		"a family the peer never configured must not read as reconnecting")
}

// TestPrefixReconnectRejectsContradiction verifies a config that says two
// things at once is refused rather than approximated.
//
// VALIDATES: ai/rules/exact-or-reject.md at the config boundary. A timer of
// zero seconds is not a timer, and a peer told to stay down has no use for a
// wait.
// PREVENTS: an operator believing a wait applies while the peer stays down, or
// the reverse.
func TestPrefixReconnectRejectsContradiction(t *testing.T) {
	tests := []struct {
		name  string
		v4    map[string]any
		wants string
	}{
		{
			"timer with no wait",
			map[string]any{"maximum": "100", "idle-timeout": "0", "reconnect": "timer"},
			"needs idle-timeout above 0",
		},
		{
			"never with a wait",
			map[string]any{"maximum": "100", "idle-timeout": "30", "reconnect": "never"},
			"conflicts with idle-timeout 30",
		},
		{
			"backoff with a wait",
			map[string]any{"maximum": "100", "idle-timeout": "30", "reconnect": "backoff"},
			"conflicts with idle-timeout 30",
		},
		{
			"a value the enum does not carry",
			map[string]any{"maximum": "100", "reconnect": "sometimes"},
			"not one of never, backoff, timer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := twoFamilyPrefixTree(tt.v4, map[string]any{"maximum": "200"})
			_, err := parsePeerFromTree("peer1", tree, 65000, 0)
			require.Error(t, err, "the config must be refused, not approximated")
			assert.Contains(t, err.Error(), tt.wants)
		})
	}
}

// TestPrefixUpdatedPerFamily verifies both families keep their own updated date.
//
// VALIDATES: AC-7, storage half. The aggregation half is
// TestPrefixUpdatedAggregatesOldest.
// PREVENTS: One family's PeeringDB refresh date standing in for another's.
func TestPrefixUpdatedPerFamily(t *testing.T) {
	tree := twoFamilyPrefixTree(
		map[string]any{"maximum": "100", "updated": "2020-01-01"},
		map[string]any{"maximum": "200", "updated": "2026-07-30"},
	)
	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	assert.Equal(t, "2020-01-01", ps.PrefixUpdated["ipv4/unicast"])
	assert.Equal(t, "2026-07-30", ps.PrefixUpdated["ipv6/unicast"])
	assert.Equal(t, "2020-01-01", ps.OldestPrefixUpdated(),
		"the peer-level surfaces report the oldest date")
}

// TestPrefixMaximumStillPerFamily verifies the two leaves that were already
// per-family did not regress.
//
// VALIDATES: The sibling maps keep their behavior, including the warning
// default of 90 percent of maximum.
// PREVENTS: A regression in the two fields this spec did not set out to change.
func TestPrefixMaximumStillPerFamily(t *testing.T) {
	tree := twoFamilyPrefixTree(
		map[string]any{"maximum": "100"},
		map[string]any{"maximum": "200", "warning": "150"},
	)
	ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
	require.NoError(t, err)

	assert.Equal(t, uint32(100), ps.PrefixMaximum["ipv4/unicast"])
	assert.Equal(t, uint32(200), ps.PrefixMaximum["ipv6/unicast"])
	assert.Equal(t, uint32(90), ps.PrefixWarning["ipv4/unicast"], "90 percent of maximum")
	assert.Equal(t, uint32(150), ps.PrefixWarning["ipv6/unicast"])
}
