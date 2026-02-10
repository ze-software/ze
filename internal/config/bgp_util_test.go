package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIPGlobMatch(t *testing.T) {
	tests := []struct {
		pattern string
		ip      string
		match   bool
	}{
		// Wildcard all
		{"*", "192.168.1.1", true},
		{"*", "10.0.0.1", true},
		{"*", "2001:db8::1", true},

		// Exact match
		{"192.168.1.1", "192.168.1.1", true},
		{"192.168.1.1", "192.168.1.2", false},

		// Single octet wildcard
		{"192.168.1.*", "192.168.1.1", true},
		{"192.168.1.*", "192.168.1.255", true},
		{"192.168.1.*", "192.168.2.1", false},

		// Middle octet wildcard
		{"192.168.*.1", "192.168.0.1", true},
		{"192.168.*.1", "192.168.255.1", true},
		{"192.168.*.1", "192.168.1.2", false},

		// Multiple wildcards
		{"192.*.*.*", "192.0.0.1", true},
		{"192.*.*.*", "192.255.255.255", true},
		{"192.*.*.*", "10.0.0.1", false},

		// First octet wildcard
		{"*.168.1.1", "192.168.1.1", true},
		{"*.168.1.1", "10.168.1.1", true},
		{"*.168.1.1", "192.169.1.1", false},

		// IPv6 - just exact and wildcard all
		{"2001:db8::1", "2001:db8::1", true},
		{"2001:db8::1", "2001:db8::2", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.ip, func(t *testing.T) {
			result := IPGlobMatch(tt.pattern, tt.ip)
			require.Equal(t, tt.match, result)
		})
	}
}

// =============================================================================
// V3 SYNTAX TESTS: template { group ... } and template { match ... }
// =============================================================================

// TestTemplateGroupBasic verifies template { group <name> { } } syntax.
//
// VALIDATES: Named templates can be defined using "group" instead of "neighbor".
//
// PREVENTS: Unable to use group syntax for named templates.
func TestIPv6GlobMatch(t *testing.T) {
	tests := []struct {
		pattern string
		ip      string
		match   bool
	}{
		// Wildcard all
		{"*", "2001:db8::1", true},

		// Exact match
		{"2001:db8::1", "2001:db8::1", true},
		{"2001:db8::1", "2001:db8::2", false},

		// Trailing wildcard
		{"2001:db8::*", "2001:db8::1", true},
		{"2001:db8::*", "2001:db8::ffff", true},
		{"2001:db8::*", "2001:db9::1", false},

		// Prefix wildcard
		{"2001:db8:abcd::*", "2001:db8:abcd::1", true},
		{"2001:db8:abcd::*", "2001:db8:abcd::ffff:1", true},
		{"2001:db8:abcd::*", "2001:db8:abce::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.ip, func(t *testing.T) {
			result := IPGlobMatch(tt.pattern, tt.ip)
			require.Equal(t, tt.match, result)
		})
	}
}

// TestCIDRPatternMatch verifies CIDR notation pattern matching.
//
// VALIDATES: CIDR patterns correctly match IP addresses.
//
// PREVENTS: Unable to use CIDR notation for peer matching.
func TestCIDRPatternMatch(t *testing.T) {
	tests := []struct {
		pattern string
		ip      string
		match   bool
	}{
		// IPv4 CIDR
		{"10.0.0.0/8", "10.0.0.1", true},
		{"10.0.0.0/8", "10.255.255.255", true},
		{"10.0.0.0/8", "11.0.0.1", false},
		{"192.168.1.0/24", "192.168.1.1", true},
		{"192.168.1.0/24", "192.168.2.1", false},

		// IPv6 CIDR
		{"2001:db8::/32", "2001:db8::1", true},
		{"2001:db8::/32", "2001:db8:ffff::1", true},
		{"2001:db8::/32", "2001:db9::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.ip, func(t *testing.T) {
			result := IPGlobMatch(tt.pattern, tt.ip)
			require.Equal(t, tt.match, result)
		})
	}
}

// TestTemplateCIDRMatch verifies CIDR patterns work in template { match }.
//
// VALIDATES: CIDR patterns can be used in match blocks.
//
// PREVENTS: Unable to use CIDR notation in template matches.
func TestTemplateCIDRMatch(t *testing.T) {
	input := `
template {
    match 10.0.0.0/8 {
        hold-time 60;
    }
    match 192.168.0.0/16 {
        hold-time 90;
    }
}

bgp {
    peer 10.0.0.1 { local-as 65000; peer-as 65001; }
    peer 192.168.1.1 { local-as 65000; peer-as 65002; }
    peer 172.16.0.1 { local-as 65000; peer-as 65003; }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 3)
	m := peersByAddr(cfg.Peers)

	require.Equal(t, uint16(60), m["10.0.0.1"].HoldTime)
	require.Equal(t, uint16(90), m["192.168.1.1"].HoldTime)
	require.Equal(t, uint16(DefaultHoldTime), m["172.16.0.1"].HoldTime) // unset → default 90s
}

// =============================================================================
// TEMPLATE MATCH TESTS
// =============================================================================

// TestTemplateMatchConfig verifies template match patterns apply to peers.
//
// VALIDATES: template { match * { ... } } applies settings to all matching peers.
//
// PREVENTS: Unable to set defaults using template match patterns.
func TestPeerGlobConfig(t *testing.T) {
	t.Run("match * applies to all", func(t *testing.T) {
		input := `
template {
    match * {
        rib { out { group-updates false; auto-commit-delay 100ms; } }
    }
}
bgp {
    peer 192.0.2.1 { local-as 65000; peer-as 65001; }
    peer 192.0.2.2 { local-as 65000; peer-as 65002; }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 2)

		for _, n := range cfg.Peers {
			require.False(t, n.RIBOut.GroupUpdates, "match * should apply to %s", n.Address)
			require.Equal(t, 100*time.Millisecond, n.RIBOut.AutoCommitDelay)
		}
	})

	t.Run("match pattern applies to matching", func(t *testing.T) {
		input := `
template {
    match 192.0.2.* {
        rib { out { group-updates false; } }
    }
}
bgp {
    peer 192.0.2.1 { local-as 65000; peer-as 65001; }
    peer 192.0.2.2 { local-as 65000; peer-as 65002; }
    peer 10.0.0.1 { local-as 65000; peer-as 65003; }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 3)
		m := peersByAddr(cfg.Peers)

		// Matching peers get the match settings
		require.False(t, m["192.0.2.1"].RIBOut.GroupUpdates)
		require.False(t, m["192.0.2.2"].RIBOut.GroupUpdates)

		// Non-matching peer gets defaults
		require.True(t, m["10.0.0.1"].RIBOut.GroupUpdates)
	})

	t.Run("peer overrides match", func(t *testing.T) {
		input := `
template {
    match * {
        rib { out { group-updates false; auto-commit-delay 100ms; } }
    }
}
bgp {
    peer 192.0.2.1 { local-as 65000; peer-as 65001; }
    peer 192.0.2.2 {
        local-as 65000;
        peer-as 65002;
        rib { out { group-updates true; } }
    }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 2)
		m := peersByAddr(cfg.Peers)

		// First peer: match * settings
		require.False(t, m["192.0.2.1"].RIBOut.GroupUpdates)
		require.Equal(t, 100*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)

		// Second peer: explicit override for group-updates, inherits delay
		require.True(t, m["192.0.2.2"].RIBOut.GroupUpdates)
		require.Equal(t, 100*time.Millisecond, m["192.0.2.2"].RIBOut.AutoCommitDelay)
	})

	t.Run("later match overrides earlier (config order)", func(t *testing.T) {
		input := `
template {
    match * {
        rib { out { auto-commit-delay 100ms; } }
    }
    match 192.0.2.* {
        rib { out { auto-commit-delay 50ms; } }
    }
}
bgp {
    peer 192.0.2.1 { local-as 65000; peer-as 65001; }
    peer 10.0.0.1 { local-as 65000; peer-as 65002; }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 2)
		m := peersByAddr(cfg.Peers)

		// 192.0.2.1 matches both, later match wins (config order)
		require.Equal(t, 50*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)

		// 10.0.0.1 only matches *
		require.Equal(t, 100*time.Millisecond, m["10.0.0.1"].RIBOut.AutoCommitDelay)
	})
}

// =============================================================================
// API BINDING TESTS (Phase 1 - per-peer API configuration)
// =============================================================================
// Note: These tests use OLD syntax (api { processes [...] }) which works with Freeform().
// NEW syntax (api <name> { content {...} }) requires parser changes (Phase 2).

// TestPeerProcessBindingOldSyntax verifies API binding parsing with old syntax.
//
// VALIDATES: Config parsing extracts API bindings from processes array.
//
// PREVENTS: Silent failures when api block is malformed.
func TestPluginConfigTimeout(t *testing.T) {
	input := `
plugin {
    external myapp {
        run ./myapp;
        encoder json;
        timeout 10s;
    }
}

bgp { }
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Plugins, 1)
	require.Equal(t, "myapp", cfg.Plugins[0].Name)
	require.Equal(t, 10*time.Second, cfg.Plugins[0].StageTimeout)
}

// TestPluginConfigTimeoutDefault verifies default timeout when not specified.
//
// VALIDATES: Missing timeout → 0 (use default in server).
// PREVENTS: Non-zero default breaking existing configs.
func TestPluginConfigTimeoutDefault(t *testing.T) {
	input := `
plugin {
    external myapp {
        run ./myapp;
    }
}

bgp { }
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Plugins, 1)
	require.Equal(t, time.Duration(0), cfg.Plugins[0].StageTimeout)
}

// TestPluginConfigTimeoutInvalid verifies invalid timeout is rejected.
//
// VALIDATES: "timeout abc;" produces parse error.
// PREVENTS: Invalid durations being silently ignored.
func TestPluginConfigTimeoutInvalid(t *testing.T) {
	input := `
plugin {
    external myapp {
        run ./myapp;
        timeout abc;
    }
}

bgp { }
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err) // Parsing succeeds (schema accepts string)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid timeout")
}

// TestPluginConfigTimeoutNegative verifies negative timeout is rejected.
//
// VALIDATES: "timeout -5s;" produces error (not silently accepted).
// PREVENTS: Negative duration causing immediate context expiration.
func TestPluginConfigTimeoutNegative(t *testing.T) {
	input := `
plugin {
    external myapp {
        run ./myapp;
        timeout -5s;
    }
}

bgp { }
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err) // Parsing succeeds (schema accepts string)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be positive")
}

// TestPluginConfigTimeoutVariants verifies various duration formats.
//
// VALIDATES: Various Go duration formats parse correctly.
// PREVENTS: Only some formats working.
func TestPluginConfigTimeoutVariants(t *testing.T) {
	tests := []struct {
		name     string
		timeout  string
		expected time.Duration
	}{
		{"seconds", "5s", 5 * time.Second},
		{"milliseconds", "500ms", 500 * time.Millisecond},
		{"minutes", "2m", 2 * time.Minute},
		{"combined", "1m30s", 90 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `
plugin {
    external myapp {
        run ./myapp;
        timeout ` + tt.timeout + `;
    }
}

bgp { }
`
			cfg := parseConfig(t, input)
			require.Len(t, cfg.Plugins, 1)
			require.Equal(t, tt.expected, cfg.Plugins[0].StageTimeout)
		})
	}
}

// TestParseUpdateBlock_Basic verifies basic update block parsing.
//
// VALIDATES: Update block with attributes and NLRI parses correctly into StaticRouteConfig.
// PREVENTS: New native syntax being rejected or misinterpreted.
