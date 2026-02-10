package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPerNeighborRIBOut(t *testing.T) {
	t.Run("explicit config", func(t *testing.T) {
		input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        rib { out { group-updates true; auto-commit-delay 100ms; max-batch-size 500; } }
    }
    peer 192.0.2.2 {
        local-as 65000;
        peer-as 65002;
        rib { out { group-updates false; auto-commit-delay 50ms; } }
    }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 2)
		m := peersByAddr(cfg.Peers)

		require.True(t, m["192.0.2.1"].RIBOut.GroupUpdates)
		require.Equal(t, 100*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)
		require.Equal(t, 500, m["192.0.2.1"].RIBOut.MaxBatchSize)

		require.False(t, m["192.0.2.2"].RIBOut.GroupUpdates)
		require.Equal(t, 50*time.Millisecond, m["192.0.2.2"].RIBOut.AutoCommitDelay)
		require.Equal(t, 0, m["192.0.2.2"].RIBOut.MaxBatchSize)
	})

	t.Run("defaults", func(t *testing.T) {
		input := `bgp { peer 192.0.2.1 { local-as 65000; peer-as 65001; } }`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 1)
		n := cfg.Peers[0]
		require.True(t, n.RIBOut.GroupUpdates)
		require.Equal(t, time.Duration(0), n.RIBOut.AutoCommitDelay)
		require.Equal(t, 0, n.RIBOut.MaxBatchSize)
	})

	t.Run("template inheritance", func(t *testing.T) {
		input := `
template { group rib-tmpl { rib { out { group-updates true; auto-commit-delay 200ms; max-batch-size 1000; } } } }
bgp {
    peer 192.0.2.1 { inherit rib-tmpl; local-as 65000; peer-as 65001; }
    peer 192.0.2.2 { inherit rib-tmpl; local-as 65000; peer-as 65002; rib { out { auto-commit-delay 50ms; } } }
}
`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 2)
		m := peersByAddr(cfg.Peers)

		// n1: full template inheritance
		require.True(t, m["192.0.2.1"].RIBOut.GroupUpdates)
		require.Equal(t, 200*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)
		require.Equal(t, 1000, m["192.0.2.1"].RIBOut.MaxBatchSize)

		// n2: template with override
		require.True(t, m["192.0.2.2"].RIBOut.GroupUpdates)
		require.Equal(t, 50*time.Millisecond, m["192.0.2.2"].RIBOut.AutoCommitDelay)
		require.Equal(t, 1000, m["192.0.2.2"].RIBOut.MaxBatchSize)
	})

	t.Run("legacy group-updates", func(t *testing.T) {
		input := `bgp { peer 192.0.2.1 { local-as 65000; peer-as 65001; group-updates false; } }`
		cfg := parseConfig(t, input)
		require.Len(t, cfg.Peers, 1)
		n := cfg.Peers[0]
		require.False(t, n.GroupUpdates)
		require.False(t, n.RIBOut.GroupUpdates)
	})
}

// parseConfig is a test helper that parses BGP config and returns the result.

// TestIPGlobMatch verifies IP glob pattern matching.
//
// VALIDATES: IP glob patterns correctly match IP addresses.
//
// PREVENTS: Incorrect peer glob matching behavior.
func TestTemplateGroupBasic(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
template {
    group ibgp-rr {
        peer-as 65000;
        hold-time 60;
        capability { route-refresh; }
        process rib { send { update; } }
    }
}

bgp {
    peer 192.0.2.1 {
        inherit ibgp-rr;
        local-as 65000;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]
	require.Equal(t, uint32(65000), n.PeerAS)
	require.Equal(t, uint16(60), n.HoldTime)
	require.True(t, n.Capabilities.RouteRefresh)
}

// TestTemplateMatchBasic verifies template { match <pattern> { } } syntax.
//
// VALIDATES: Glob patterns can be defined using "match" inside template block.
//
// PREVENTS: Unable to use match syntax for glob patterns.
func TestTemplateMatchBasic(t *testing.T) {
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
}

// TestTemplateMatchOrder verifies match blocks are applied in config order.
//
// VALIDATES: match blocks are applied in the order they appear in the config,
// allowing later matches to override earlier ones.
//
// PREVENTS: Unexpected behavior from specificity-based ordering.
func TestTemplateMatchOrder(t *testing.T) {
	input := `
template {
    match * {
        hold-time 90;
        rib { out { group-updates true; } }
    }
    match 192.0.2.* {
        hold-time 60;
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

	// 192.0.2.1: match * first, then match 192.0.2.* overrides hold-time
	require.Equal(t, uint16(60), m["192.0.2.1"].HoldTime)
	require.True(t, m["192.0.2.1"].RIBOut.GroupUpdates)
	require.Equal(t, 50*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)

	// 10.0.0.1: only match * applies
	require.Equal(t, uint16(90), m["10.0.0.1"].HoldTime)
	require.True(t, m["10.0.0.1"].RIBOut.GroupUpdates)
	require.Equal(t, time.Duration(0), m["10.0.0.1"].RIBOut.AutoCommitDelay)
}

// TestTemplateMatchConfigOrderNotSpecificity verifies config order, NOT specificity.
//
// VALIDATES: More specific match appearing BEFORE less specific is applied first,
// and less specific (appearing later) OVERRIDES it. Config order, not specificity!
//
// PREVENTS: Specificity-based ordering instead of config-file ordering.
func TestTemplateMatchConfigOrderNotSpecificity(t *testing.T) {
	// Critical: specific match BEFORE general match
	// Per plan: config order, so 10.0.0.0/8 applies first, then * overrides
	input := `
template {
    match 10.0.0.0/8 {
        hold-time 60;
    }
    match * {
        hold-time 90;
    }
}

bgp {
    peer 10.0.0.1 { local-as 65000; peer-as 65001; }
    peer 192.168.1.1 { local-as 65000; peer-as 65002; }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 2)
	m := peersByAddr(cfg.Peers)

	// 10.0.0.1: match 10.0.0.0/8 first (hold-time=60), then match * (hold-time=90 overrides)
	// Config order means later match wins, regardless of specificity!
	require.Equal(t, uint16(90), m["10.0.0.1"].HoldTime, "config order: * should override 10.0.0.0/8")

	// 192.168.1.1: only match * applies
	require.Equal(t, uint16(90), m["192.168.1.1"].HoldTime)
}

// TestTemplateGroupAndMatchCombined verifies combined group and match usage.
//
// VALIDATES: Both group and match can be used together with proper precedence.
// Order: match (config order) → group (inherit order) → peer
//
// PREVENTS: Incorrect precedence when mixing group and match.
func TestTemplateGroupAndMatchCombined(t *testing.T) {
	input := `
template {
    match * {
        hold-time 90;
    }
    match 192.0.2.* {
        hold-time 60;
    }
    group high-preference {
        rib { out { auto-commit-delay 100ms; } }
    }
}

bgp {
    peer 192.0.2.1 {
        inherit high-preference;
        local-as 65000;
        peer-as 65001;
    }
    peer 10.0.0.1 {
        local-as 65000;
        peer-as 65002;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 2)
	m := peersByAddr(cfg.Peers)

	// 192.0.2.1: match * (hold-time 90), match 192.0.2.* (hold-time 60), inherit high-preference
	require.Equal(t, uint16(60), m["192.0.2.1"].HoldTime)
	require.Equal(t, 100*time.Millisecond, m["192.0.2.1"].RIBOut.AutoCommitDelay)

	// 10.0.0.1: only match * applies, no inherit
	require.Equal(t, uint16(90), m["10.0.0.1"].HoldTime)
	require.Equal(t, time.Duration(0), m["10.0.0.1"].RIBOut.AutoCommitDelay)
}

// TestPeerKeywordForSessions verifies "peer" keyword works for BGP sessions.
//
// VALIDATES: "peer <IP> { }" syntax works as alias for "neighbor <IP> { }".
//
// PREVENTS: Unable to use peer syntax for BGP sessions.
func TestPeerKeywordForSessions(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        hold-time 90;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]
	require.Equal(t, "192.0.2.1", n.Address.String())
	require.Equal(t, uint32(65000), n.LocalAS)
	require.Equal(t, uint32(65001), n.PeerAS)
	require.Equal(t, uint16(90), n.HoldTime)
}

// TestSingleInheritance verifies single inherit statement works.
//
// VALIDATES: inherit statement applies template settings to peer.
//
// PREVENTS: Template inheritance not working.
func TestSingleInheritance(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
template {
    group ibgp-defaults {
        hold-time 60;
        peer-as 65000;
        capability { route-refresh; }
        process rib { send { update; } }
    }
}

bgp {
    peer 192.0.2.1 {
        inherit ibgp-defaults;
        local-as 65000;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	n := cfg.Peers[0]

	// Template settings applied
	require.Equal(t, uint16(60), n.HoldTime)
	require.Equal(t, uint32(65000), n.PeerAS)
	require.True(t, n.Capabilities.RouteRefresh)
	// Peer settings
	require.Equal(t, uint32(65000), n.LocalAS)
}

// =============================================================================
// V3 ERROR CASE TESTS
// =============================================================================

// TestInheritRejectedInTemplate verifies inherit is rejected inside template.
//
// VALIDATES: inherit statements inside template { group/match } are rejected.
//
// PREVENTS: Confusing nested inheritance in templates.
func TestInheritRejectedInTemplate(t *testing.T) {
	input := `
template {
    group base {
        hold-time 90;
    }
    group derived {
        inherit base;
        hold-time 60;
    }
}
bgp {
    peer 192.0.2.1 { inherit derived; local-as 65000; peer-as 65001; }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err) // Parse succeeds

	// Validation happens in TreeToConfig
	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "inherit")
}

// TestMatchRejectedAtRoot verifies match is rejected at root level.
//
// VALIDATES: match blocks are only valid inside template { }.
//
// PREVENTS: Confusion between root-level peer globs and template matches.
func TestMatchRejectedAtRoot(t *testing.T) {
	input := `
match * {
    hold-time 90;
}
`
	p := NewParser(YANGSchema())
	_, err := p.Parse(input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "match")
}

// TestMatchRejectedInPeer verifies match is rejected inside peer blocks.
//
// VALIDATES: match blocks cannot appear inside peer { }.
//
// PREVENTS: Invalid nested match syntax.
func TestMatchRejectedInPeer(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
        match * {
            hold-time 90;
        }
    }
}
`
	p := NewParser(YANGSchema())
	_, err := p.Parse(input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "match")
}

// TestGroupNameValidation verifies group name validation rules.
//
// VALIDATES: Group names must follow naming rules:
// - Start with letter
// - Contain letters, numbers, hyphens
// - Not end with hyphen
//
// PREVENTS: Invalid group names causing issues.
func TestGroupNameValidation(t *testing.T) {
	validNames := []string{"a", "ibgp", "ibgp-rr", "rr-client-v4", "Route-Reflector-1"}
	invalidNames := []string{"123", "-ibgp", "ibgp-", "ibgp_rr"}

	for _, name := range validNames {
		t.Run("valid:"+name, func(t *testing.T) {
			input := `template { group ` + name + ` { hold-time 90; } }
bgp { peer 192.0.2.1 { inherit ` + name + `; local-as 65000; peer-as 65001; } }`
			p := NewParser(YANGSchema())
			tree, err := p.Parse(input)
			require.NoError(t, err, "group name %q should parse", name)

			// Validation happens in TreeToConfig
			_, err = TreeToConfig(tree)
			require.NoError(t, err, "group name %q should be valid", name)
		})
	}

	for _, name := range invalidNames {
		t.Run("invalid:"+name, func(t *testing.T) {
			input := `template { group ` + name + ` { hold-time 90; } }
bgp { peer 192.0.2.1 { inherit ` + name + `; local-as 65000; peer-as 65001; } }`
			p := NewParser(YANGSchema())
			tree, err := p.Parse(input)
			require.NoError(t, err, "group name %q should parse", name)

			// Validation happens in TreeToConfig
			_, err = TreeToConfig(tree)
			require.Error(t, err, "group name %q should be invalid", name)
		})
	}
}

// =============================================================================
// IPV6 AND CIDR PATTERN MATCHING TESTS
// =============================================================================

// TestIPv6GlobMatch verifies IPv6 glob pattern matching.
//
// VALIDATES: IPv6 glob patterns correctly match IPv6 addresses.
//
// PREVENTS: Unable to use glob patterns with IPv6 peers.
func TestPeerProcessBindingOldSyntax(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder text; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process {
            processes [ foo ];
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "foo", binding.PluginName)
}

// TestAPIBindingMultipleProcesses verifies multiple processes in old syntax.
//
// VALIDATES: Multiple processes in array create multiple bindings.
//
// PREVENTS: Missing processes when multiple specified.
func TestAPIBindingMultipleProcesses(t *testing.T) {
	input := `
plugin {
    external collector { run ./collector; encoder json; }
    external logger { run ./logger; encoder text; }
}
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process {
            processes [ collector logger ];
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].ProcessBindings, 2)

	// Find bindings by name
	names := make([]string, 0, len(cfg.Peers[0].ProcessBindings))
	for _, b := range cfg.Peers[0].ProcessBindings {
		names = append(names, b.PluginName)
	}
	require.Contains(t, names, "collector")
	require.Contains(t, names, "logger")
}

// TestAPIBindingNeighborChanges verifies neighbor-changes flag sets receive.State.
//
// VALIDATES: neighbor-changes; in old syntax sets receive.State = true.
//
// PREVENTS: State change events being dropped for old-style configs.
func TestAPIBindingNeighborChanges(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder text; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process {
            processes [ foo ];
            neighbor-changes;
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "foo", binding.PluginName)
	require.True(t, binding.Receive.State, "neighbor-changes should set receive.State")
}

// TestAPIBindingReceiveNegotiated verifies negotiated flag in receive config.
//
// VALIDATES: receive { negotiated; } sets Receive.Negotiated = true.
//
// PREVENTS: Negotiated capabilities not being forwarded to plugins.
func TestAPIBindingReceiveNegotiated(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder json; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            receive { negotiated; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "foo", binding.PluginName)
	require.True(t, binding.Receive.Negotiated, "receive { negotiated; } should set Receive.Negotiated")
}

// TestAPIBindingReceiveAll verifies all flag sets all receive options.
//
// VALIDATES: receive { all; } sets all Receive flags including Negotiated.
//
// PREVENTS: Missing negotiated in all shorthand.
func TestAPIBindingReceiveAll(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder json; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            receive { all; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.True(t, binding.Receive.Update, "all should set Update")
	require.True(t, binding.Receive.Open, "all should set Open")
	require.True(t, binding.Receive.Notification, "all should set Notification")
	require.True(t, binding.Receive.Keepalive, "all should set Keepalive")
	require.True(t, binding.Receive.Refresh, "all should set Refresh")
	require.True(t, binding.Receive.State, "all should set State")
	require.True(t, binding.Receive.Sent, "all should set Sent")
	require.True(t, binding.Receive.Negotiated, "all should set Negotiated")
}

// TestAPIBindingUndefinedProcess verifies error on undefined plugin reference.
//
// VALIDATES: Error when api references non-existent process.
//
// PREVENTS: Runtime crashes from nil process lookup.
func TestAPIBindingUndefinedProcess(t *testing.T) {
	input := `
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process {
            processes [ nonexistent ];
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "undefined plugin")
}

// TestEmptyAPIBlock verifies empty api block creates no bindings.
//
// VALIDATES: Empty api block (no processes) creates no bindings.
//
// PREVENTS: Crash on empty api block.
func TestEmptyAPIBlock(t *testing.T) {
	input := `
plugin { external foo { run ./test; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process { }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].ProcessBindings, 0, "Empty api block should have no bindings")
}

// TestAPIBindingConfigStructs verifies the config struct fields exist.
//
// VALIDATES: PeerProcessBinding struct has all required fields.
//
// PREVENTS: Missing fields for Phase 2 new syntax support.
func TestAPIBindingConfigStructs(t *testing.T) {
	// Verify struct fields exist (compile-time check)
	binding := PeerProcessBinding{
		PluginName: "test",
		Content: PeerContentConfig{
			Encoding: "json",
			Format:   "full",
		},
		Receive: PeerReceiveConfig{
			Update:       true,
			Open:         true,
			Notification: true,
			Keepalive:    true,
			Refresh:      true,
			State:        true,
		},
		Send: PeerSendConfig{
			Update:  true,
			Refresh: true,
		},
	}
	require.Equal(t, "test", binding.PluginName)
	require.Equal(t, "json", binding.Content.Encoding)
	require.True(t, binding.Receive.Update)
	require.True(t, binding.Send.Update)
}

// =============================================================================
// NEW API SYNTAX TESTS (api <process-name> { content {...} receive {...} })
// =============================================================================

// TestPeerProcessBindingNewSyntax verifies API binding parsing with new syntax.
//
// VALIDATES: New syntax (api <name> { content {...} }) parses correctly.
//
// PREVENTS: Silent failures when using new api syntax.
func TestPeerProcessBindingNewSyntax(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder text; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            content { encoding json; format full; }
            receive { update; notification; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "foo", binding.PluginName)
	require.Equal(t, "json", binding.Content.Encoding)
	require.Equal(t, "full", binding.Content.Format)
	require.True(t, binding.Receive.Update)
	require.True(t, binding.Receive.Notification)
	require.False(t, binding.Receive.Open) // Not specified
}

// TestReceiveAllExpansion verifies "all" keyword expands to all message types.
//
// VALIDATES: "all" keyword sets all receive flags true.
//
// PREVENTS: Missing messages when user specifies "all".
func TestReceiveAllExpansion(t *testing.T) {
	input := `
plugin { external foo { run ./test; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            receive { all; }
        }
    }
}
`
	cfg := parseConfig(t, input)

	recv := cfg.Peers[0].ProcessBindings[0].Receive
	require.True(t, recv.Update, "all should set Update")
	require.True(t, recv.Open, "all should set Open")
	require.True(t, recv.Notification, "all should set Notification")
	require.True(t, recv.Keepalive, "all should set Keepalive")
	require.True(t, recv.Refresh, "all should set Refresh")
	require.True(t, recv.State, "all should set State")
}

// TestSendAllExpansion verifies "all" keyword in send block.
//
// VALIDATES: "all" keyword sets all send flags true.
//
// PREVENTS: Missing send capabilities when user specifies "all".
func TestSendAllExpansion(t *testing.T) {
	input := `
plugin { external foo { run ./test; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            send { all; }
        }
    }
}
`
	cfg := parseConfig(t, input)

	send := cfg.Peers[0].ProcessBindings[0].Send
	require.True(t, send.Update, "all should set Update")
	require.True(t, send.Refresh, "all should set Refresh")
}

// TestEmptyAPIBindingNewSyntax verifies empty new-syntax api block creates binding with defaults.
//
// VALIDATES: Empty api block creates binding with process name but empty config.
//
// PREVENTS: Crash on minimal api binding.
func TestEmptyAPIBindingNewSyntax(t *testing.T) {
	input := `
plugin { external foo { run ./test; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo { }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "foo", binding.PluginName)
	require.Empty(t, binding.Content.Encoding) // Inherit from process at runtime
	require.Empty(t, binding.Content.Format)   // Default to "parsed" at runtime
	require.False(t, binding.Receive.Update)   // No messages subscribed
}

// TestAPIBindingUndefinedProcessNewSyntax verifies error on undefined plugin in new syntax.
//
// VALIDATES: Error when api references non-existent process.
//
// PREVENTS: Runtime crashes from nil process lookup.
func TestAPIBindingUndefinedProcessNewSyntax(t *testing.T) {
	input := `
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process nonexistent {
            receive { update; }
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "undefined plugin")
}

// TestMultipleProcessBindingsNewSyntax verifies multiple api blocks for different processes.
//
// VALIDATES: Multiple api <name> blocks create separate bindings.
//
// PREVENTS: Only first api block being parsed.
func TestMultipleProcessBindingsNewSyntax(t *testing.T) {
	input := `
plugin {
    external collector { run ./collector; encoder json; }
    external logger { run ./logger; encoder text; }
}
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process collector {
            content { encoding json; }
            receive { update; }
        }
        process logger {
            content { encoding text; format full; }
            receive { all; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers[0].ProcessBindings, 2)

	// Find bindings by name
	var collectorBinding, loggerBinding *PeerProcessBinding
	for i := range cfg.Peers[0].ProcessBindings {
		b := &cfg.Peers[0].ProcessBindings[i]
		switch b.PluginName {
		case "collector":
			collectorBinding = b
		case "logger":
			loggerBinding = b
		}
	}

	require.NotNil(t, collectorBinding, "collector binding not found")
	require.NotNil(t, loggerBinding, "logger binding not found")

	require.Equal(t, "json", collectorBinding.Content.Encoding)
	require.True(t, collectorBinding.Receive.Update)
	require.False(t, collectorBinding.Receive.Open)

	require.Equal(t, "text", loggerBinding.Content.Encoding)
	require.Equal(t, "full", loggerBinding.Content.Format)
	require.True(t, loggerBinding.Receive.Update)
	require.True(t, loggerBinding.Receive.Open) // all expands to true
}

// =============================================================================
// TEMPLATE API BINDING INHERITANCE TESTS
// =============================================================================

// TestTemplateAPIBindingInheritance verifies API bindings are inherited from templates.
//
// VALIDATES: Peer inheriting a template gets the template's API bindings.
//
// PREVENTS: Lost API bindings when using template inheritance.
func TestTemplateAPIBindingInheritance(t *testing.T) {
	input := `
plugin { external collector { run ./collector; encoder json; } }
template {
    group api-template {
        process collector {
            content { encoding json; format full; }
            receive { update; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        inherit api-template;
        local-as 65000;
        peer-as 65001;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "collector", binding.PluginName)
	require.Equal(t, "json", binding.Content.Encoding)
	require.Equal(t, "full", binding.Content.Format)
	require.True(t, binding.Receive.Update)
}

// TestTemplateAPIBindingPeerOverride verifies peer API bindings override template bindings.
//
// VALIDATES: Peer API binding with same process name replaces template binding.
//
// PREVENTS: Template bindings not being overridden by peer-specific config.
func TestTemplateAPIBindingPeerOverride(t *testing.T) {
	input := `
plugin { external collector { run ./collector; encoder json; } }
template {
    group api-template {
        process collector {
            content { encoding json; format parsed; }
            receive { update; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        inherit api-template;
        local-as 65000;
        peer-as 65001;
        process collector {
            content { encoding text; format full; }
            receive { all; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1, "should have 1 binding (peer overrides template)")

	binding := cfg.Peers[0].ProcessBindings[0]
	require.Equal(t, "collector", binding.PluginName)
	require.Equal(t, "text", binding.Content.Encoding, "peer should override template encoding")
	require.Equal(t, "full", binding.Content.Format, "peer should override template format")
	require.True(t, binding.Receive.Update, "peer 'all' should set Update")
	require.True(t, binding.Receive.Open, "peer 'all' should set Open (template didn't have it)")
}

// TestTemplateAPIBindingMergeMultipleProcesses verifies merging different process bindings.
//
// VALIDATES: Template and peer with different process names both appear in result.
//
// PREVENTS: Missing bindings when template and peer bind different processes.
func TestTemplateAPIBindingMergeMultipleProcesses(t *testing.T) {
	input := `
plugin {
    external collector { run ./collector; encoder json; }
    external logger { run ./logger; encoder text; }
}
template {
    group api-template {
        process collector {
            receive { update; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        inherit api-template;
        local-as 65000;
        peer-as 65001;
        process logger {
            receive { notification; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 2, "should have 2 bindings (template + peer)")

	// Find bindings by name
	m := make(map[string]PeerProcessBinding)
	for _, b := range cfg.Peers[0].ProcessBindings {
		m[b.PluginName] = b
	}

	require.Contains(t, m, "collector", "should have collector from template")
	require.Contains(t, m, "logger", "should have logger from peer")
	require.True(t, m["collector"].Receive.Update)
	require.True(t, m["logger"].Receive.Notification)
}

// TestTemplateWithMultipleProcessBindings verifies single template with multiple API bindings.
//
// VALIDATES: A template can have multiple API bindings for different processes.
//
// PREVENTS: Lost bindings when template has multiple process bindings.
//
// Note: Multiple inherit statements are NOT supported (inherit is a Leaf type,
// second inherit overwrites first). This test uses a single template with
// multiple api blocks instead.
func TestTemplateWithMultipleProcessBindings(t *testing.T) {
	input := `
plugin {
    external collector { run ./collector; encoder json; }
    external logger { run ./logger; encoder text; }
}
template {
    group multi-api {
        process collector {
            content { encoding json; }
            receive { update; }
        }
        process logger {
            content { encoding text; }
            receive { notification; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        inherit multi-api;
        local-as 65000;
        peer-as 65001;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 2, "should have 2 bindings from template")

	m := make(map[string]PeerProcessBinding)
	for _, b := range cfg.Peers[0].ProcessBindings {
		m[b.PluginName] = b
	}

	require.Equal(t, "json", m["collector"].Content.Encoding)
	require.Equal(t, "text", m["logger"].Content.Encoding)
}

// TestMatchTemplateAPIBinding verifies API bindings from match templates.
//
// VALIDATES: Match patterns can specify API bindings that apply to matching peers.
//
// PREVENTS: Unable to set default API bindings via match patterns.
func TestMatchTemplateAPIBinding(t *testing.T) {
	input := `
plugin { external collector { run ./collector; encoder json; } }
template {
    match * {
        process collector {
            receive { update; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
    }
    peer 10.0.0.1 {
        local-as 65000;
        peer-as 65002;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 2)

	for _, peer := range cfg.Peers {
		require.Len(t, peer.ProcessBindings, 1, "match * should apply to %s", peer.Address)
		require.Equal(t, "collector", peer.ProcessBindings[0].PluginName)
		require.True(t, peer.ProcessBindings[0].Receive.Update)
	}
}

// TestMatchAndInheritAPIBindingPrecedence verifies correct precedence: match → inherit → peer.
//
// VALIDATES: Inherit overrides match, peer overrides both.
//
// PREVENTS: Wrong API binding precedence causing unexpected behavior.
func TestMatchAndInheritAPIBindingPrecedence(t *testing.T) {
	input := `
plugin { external collector { run ./collector; encoder json; } }
template {
    match * {
        process collector {
            content { encoding json; }
            receive { update; }
        }
    }
    group override-template {
        process collector {
            content { encoding text; }
            receive { notification; }
        }
    }
}
bgp {
    peer 192.0.2.1 {
        inherit override-template;
        local-as 65000;
        peer-as 65001;
    }
    peer 10.0.0.1 {
        local-as 65000;
        peer-as 65002;
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 2)
	m := peersByAddr(cfg.Peers)

	// 192.0.2.1: match * first, then inherit override-template (inherit wins)
	require.Len(t, m["192.0.2.1"].ProcessBindings, 1)
	require.Equal(t, "text", m["192.0.2.1"].ProcessBindings[0].Content.Encoding, "inherit should override match")
	require.False(t, m["192.0.2.1"].ProcessBindings[0].Receive.Update, "inherit replaces entire binding")
	require.True(t, m["192.0.2.1"].ProcessBindings[0].Receive.Notification)

	// 10.0.0.1: only match * applies
	require.Len(t, m["10.0.0.1"].ProcessBindings, 1)
	require.Equal(t, "json", m["10.0.0.1"].ProcessBindings[0].Content.Encoding)
	require.True(t, m["10.0.0.1"].ProcessBindings[0].Receive.Update)
}

// =============================================================================
// V2 SYNTAX REJECTION TESTS
// =============================================================================

// =============================================================================
// MERGE API BINDINGS TESTS
// =============================================================================

// TestMergeProcessBindingsEmptyNew verifies that empty new returns existing unchanged.
//
// VALIDATES: When new is empty, existing bindings are returned as-is.
//
// PREVENTS: Lost bindings when merging with empty list.
func TestMergeProcessBindingsEmptyNew(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "json"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "text"}},
	}
	result := mergeProcessBindings(existing, nil)
	require.Equal(t, existing, result)

	result = mergeProcessBindings(existing, []PeerProcessBinding{})
	require.Equal(t, existing, result)
}

// TestMergeProcessBindingsEmptyExisting verifies that empty existing returns new unchanged.
//
// VALIDATES: When existing is empty, new bindings are returned as-is.
//
// PREVENTS: Lost bindings when starting from empty.
func TestMergeProcessBindingsEmptyExisting(t *testing.T) {
	newBindings := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "json"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "text"}},
	}
	result := mergeProcessBindings(nil, newBindings)
	require.Equal(t, newBindings, result)

	result = mergeProcessBindings([]PeerProcessBinding{}, newBindings)
	require.Equal(t, newBindings, result)
}

// TestMergeProcessBindingsAppend verifies that new bindings with different names are appended.
//
// VALIDATES: Bindings with unique names are appended to result.
//
// PREVENTS: Missing bindings when names don't overlap.
func TestMergeProcessBindingsAppend(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "json"}},
	}
	newBindings := []PeerProcessBinding{
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "text"}},
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 2)
	require.Equal(t, "foo", result[0].PluginName)
	require.Equal(t, "bar", result[1].PluginName)
}

// TestMergeProcessBindingsReplace verifies that new bindings replace existing with same name.
//
// VALIDATES: Bindings with same ProcessName are replaced (new overrides existing).
//
// PREVENTS: Duplicate bindings for same process, wrong override semantics.
func TestMergeProcessBindingsReplace(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "json", Format: "parsed"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "text"}},
	}
	newBindings := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "text", Format: "full"}},
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 2)

	// foo should be replaced with new binding
	require.Equal(t, "foo", result[0].PluginName)
	require.Equal(t, "text", result[0].Content.Encoding)
	require.Equal(t, "full", result[0].Content.Format)

	// bar should remain unchanged
	require.Equal(t, "bar", result[1].PluginName)
	require.Equal(t, "text", result[1].Content.Encoding)
}

// TestMergeProcessBindingsMixed verifies mixed append and replace operations.
//
// VALIDATES: Some bindings replaced, others appended in single merge.
//
// PREVENTS: Incorrect behavior when mix of new and existing names.
func TestMergeProcessBindingsMixed(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "json"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "json"}},
	}
	newBindings := []PeerProcessBinding{
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "text"}}, // replace
		{PluginName: "baz", Content: PeerContentConfig{Encoding: "json"}}, // append
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 3)

	// Build map for easier checking
	m := make(map[string]PeerProcessBinding)
	for _, b := range result {
		m[b.PluginName] = b
	}

	require.Equal(t, "json", m["foo"].Content.Encoding) // unchanged
	require.Equal(t, "text", m["bar"].Content.Encoding) // replaced
	require.Equal(t, "json", m["baz"].Content.Encoding) // appended
}

// TestMergeProcessBindingsPreservesOrder verifies that result order is: existing (replaced in place), new appends.
//
// VALIDATES: Order is deterministic: existing items in original positions, new items appended.
//
// PREVENTS: Non-deterministic ordering causing config instability.
func TestMergeProcessBindingsPreservesOrder(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "a"},
		{PluginName: "b"},
		{PluginName: "c"},
	}
	newBindings := []PeerProcessBinding{
		{PluginName: "b", Content: PeerContentConfig{Encoding: "replaced"}}, // replace in position 1
		{PluginName: "d"}, // append
		{PluginName: "e"}, // append
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 5)
	require.Equal(t, "a", result[0].PluginName)
	require.Equal(t, "b", result[1].PluginName)
	require.Equal(t, "replaced", result[1].Content.Encoding)
	require.Equal(t, "c", result[2].PluginName)
	require.Equal(t, "d", result[3].PluginName)
	require.Equal(t, "e", result[4].PluginName)
}

// TestMergeProcessBindingsFullReplace verifies complete replacement of all existing bindings.
//
// VALIDATES: When all existing names are in new, all are replaced.
//
// PREVENTS: Stale existing bindings remaining after full override.
func TestMergeProcessBindingsFullReplace(t *testing.T) {
	existing := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "old"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "old"}},
	}
	newBindings := []PeerProcessBinding{
		{PluginName: "foo", Content: PeerContentConfig{Encoding: "new"}},
		{PluginName: "bar", Content: PeerContentConfig{Encoding: "new"}},
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 2)
	for _, b := range result {
		require.Equal(t, "new", b.Content.Encoding, "binding %s should be replaced", b.PluginName)
	}
}

// TestMergeProcessBindingsReceiveConfig verifies Receive config is properly merged.
//
// VALIDATES: Receive flags are replaced along with the binding.
//
// PREVENTS: Receive config not being properly copied during replace.
func TestMergeProcessBindingsReceiveConfig(t *testing.T) {
	existing := []PeerProcessBinding{
		{
			PluginName: "foo",
			Receive:    PeerReceiveConfig{Update: true, Open: false},
		},
	}
	newBindings := []PeerProcessBinding{
		{
			PluginName: "foo",
			Receive:    PeerReceiveConfig{Update: false, Open: true, Notification: true},
		},
	}

	result := mergeProcessBindings(existing, newBindings)

	require.Len(t, result, 1)
	require.False(t, result[0].Receive.Update, "Update should be false after replace")
	require.True(t, result[0].Receive.Open, "Open should be true after replace")
	require.True(t, result[0].Receive.Notification, "Notification should be true after replace")
}

// =============================================================================
// OLD SYNTAX REJECTION TESTS
// =============================================================================

// TestOldSyntaxRejected verifies that old syntax is rejected by BGPSchema.
//
// VALIDATES: YANGSchema() only accepts current syntax.
//
// PREVENTS: Accidentally accepting deprecated configs.
func TestOldSyntaxRejected(t *testing.T) {
	t.Run("neighbor at root rejected", func(t *testing.T) {
		input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
		p := NewParser(YANGSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown top-level keyword: neighbor")
	})

	t.Run("peer at root rejected (requires bgp block)", func(t *testing.T) {
		input := `
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
		p := NewParser(YANGSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown top-level keyword: peer")
	})

	t.Run("peer glob in bgp block rejected", func(t *testing.T) {
		input := `
bgp {
    peer * {
        local-as 65000;
    }
}
`
		p := NewParser(YANGSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid")
	})

	t.Run("peer CIDR pattern in bgp block rejected", func(t *testing.T) {
		input := `
bgp {
    peer 192.0.2.0/24 {
        local-as 65000;
    }
}
`
		p := NewParser(YANGSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid")
	})

	t.Run("template.neighbor rejected", func(t *testing.T) {
		input := `
template {
    neighbor mytemplate {
        local-as 65000;
    }
}
`
		p := NewParser(YANGSchema())
		_, err := p.Parse(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown field in template: neighbor")
	})

	t.Run("current syntax accepted", func(t *testing.T) {
		input := `
bgp {
    peer 192.0.2.1 {
        local-as 65000;
        peer-as 65001;
    }
}
template {
    group mytemplate {
        local-as 65000;
    }
    match * {
        hold-time 90;
    }
}
`
		p := NewParser(YANGSchema())
		tree, err := p.Parse(input)
		require.NoError(t, err)
		require.NotNil(t, tree)
	})
}

// =============================================================================
// NLRI FILTER PARSING TESTS
// =============================================================================

// TestParseNLRIEntriesValid verifies valid NLRI family entries are parsed.
//
// VALIDATES: Space-separated AFI SAFI entries are correctly parsed.
//
// PREVENTS: Valid NLRI filter configs being rejected.
func TestConfigValidationRouteRefreshRequiresProcess(t *testing.T) {
	input := `
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { route-refresh; }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "route-refresh requires process with send { update; }")
	require.Contains(t, err.Error(), "no process bindings configured")
}

// TestConfigValidationGracefulRestartRequiresProcess verifies graceful-restart without process fails.
//
// VALIDATES: Config with graceful-restart capability but no process binding is rejected.
// PREVENTS: Silent runtime failure when peer reconnects and expects routes to be replayed.
func TestConfigValidationGracefulRestartRequiresProcess(t *testing.T) {
	input := `
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { graceful-restart { restart-time 120; } }
    }
}
`
	p := NewParser(schemaWithGR()) // Use schema with GR plugin YANG
	tree, err := p.Parse(input)
	require.NoError(t, err)
	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "graceful-restart requires process with send { update; }")
	require.Contains(t, err.Error(), "no process bindings configured")
}

// TestConfigValidationRouteRefreshWithProcess verifies route-refresh with proper process succeeds.
//
// VALIDATES: Config with route-refresh and process with send { update; } is accepted.
// PREVENTS: False positives rejecting valid configurations.
func TestConfigValidationRouteRefreshWithProcess(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { route-refresh; }
        process rib { send { update; } }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.True(t, cfg.Peers[0].Capabilities.RouteRefresh)
}

// TestConfigValidationGracefulRestartWithProcess verifies graceful-restart with proper process succeeds.
//
// VALIDATES: Config with graceful-restart and process with send { update; } is accepted.
// PREVENTS: False positives rejecting valid configurations.
func TestConfigValidationGracefulRestartWithProcess(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { graceful-restart { restart-time 120; } }
        process rib { send { update; } }
    }
}
`
	cfg := parseConfigWithGR(t, input) // Use schema with GR plugin YANG
	require.Len(t, cfg.Peers, 1)
	require.True(t, cfg.Peers[0].Capabilities.GracefulRestart)
}

// TestConfigValidationRouteRefreshProcessNoSendUpdate verifies route-refresh with process lacking send { update; } fails.
//
// VALIDATES: Config with route-refresh and process without send { update; } is rejected.
// PREVENTS: Misconfiguration where process cannot respond to route-refresh.
func TestConfigValidationRouteRefreshProcessNoSendUpdate(t *testing.T) {
	input := `
plugin { external logger { run ./logger; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { route-refresh; }
        process logger { receive { update; } }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "route-refresh requires process with send { update; }")
	require.Contains(t, err.Error(), "configured: process logger")
	require.Contains(t, err.Error(), "none have send { update; }")
}

// TestConfigValidationBothCapabilitiesWithProcess verifies both capabilities with proper process succeeds.
//
// VALIDATES: Config with both route-refresh and graceful-restart with proper process is accepted.
// PREVENTS: False positives when multiple capabilities are configured.
func TestConfigValidationBothCapabilitiesWithProcess(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability {
            route-refresh;
            graceful-restart { restart-time 120; }
        }
        process rib { send { update; } }
    }
}
`
	cfg := parseConfigWithGR(t, input) // Use schema with GR plugin YANG
	require.Len(t, cfg.Peers, 1)
	require.True(t, cfg.Peers[0].Capabilities.RouteRefresh)
	require.True(t, cfg.Peers[0].Capabilities.GracefulRestart)
}

// TestConfigValidationRouteRefreshFromTemplate verifies template with route-refresh, peer without process fails.
//
// VALIDATES: Config where peer inherits route-refresh from template but has no process is rejected.
// PREVENTS: Misconfiguration through template inheritance.
func TestConfigValidationRouteRefreshFromTemplate(t *testing.T) {
	input := `
template {
    group rr {
        capability { route-refresh; }
    }
}
bgp {
    peer 10.0.0.1 {
        inherit rr;
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)
	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "route-refresh requires process with send { update; }")
}

// TestConfigValidationSendAllSatisfiesRequirement verifies send { all; } satisfies the requirement.
//
// VALIDATES: Config with route-refresh and process with send { all; } is accepted.
// PREVENTS: False rejection when using "all" keyword.
func TestConfigValidationSendAllSatisfiesRequirement(t *testing.T) {
	input := `
plugin { external rib { run ./rib; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        capability { route-refresh; }
        process rib { send { all; } }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.True(t, cfg.Peers[0].Capabilities.RouteRefresh)
}

// TestPluginConfigTimeout verifies plugin timeout configuration parsing.
//
// VALIDATES: "timeout 10s;" in plugin block parses to 10 seconds.
// PREVENTS: Plugin timeout config being ignored.
