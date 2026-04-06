package bgpconfig

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
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

// =============================================================================
// PLUGIN TIMEOUT TESTS (use ExtractPluginsFromTree)
// =============================================================================

// TestPluginConfigTimeout verifies timeout parsing via ExtractPluginsFromTree.
//
// VALIDATES: Plugin timeout 10s parses correctly.
// PREVENTS: Invalid timeout being silently ignored.
func TestPluginConfigTimeout(t *testing.T) {
	input := `
plugin {
    external myapp {
        run ./myapp;
        encoder json;
        timeout 10s;
    }
}
`
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)
	plugins, err := config.ExtractPluginsFromTree(tree)
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	require.Equal(t, "myapp", plugins[0].Name)
	require.Equal(t, 10*time.Second, plugins[0].StageTimeout)
}

// TestPluginConfigTimeoutDefault verifies default timeout when not specified.
//
// VALIDATES: Missing timeout -> 0 (use default in server).
// PREVENTS: Non-zero default breaking existing configs.
func TestPluginConfigTimeoutDefault(t *testing.T) {
	input := `
plugin {
    external myapp {
        run ./myapp;
    }
}
`
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)
	plugins, err := config.ExtractPluginsFromTree(tree)
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	require.Equal(t, time.Duration(0), plugins[0].StageTimeout)
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
`
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err) // Parsing succeeds (schema accepts string)

	_, err = config.ExtractPluginsFromTree(tree)
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
`
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err) // Parsing succeeds (schema accepts string)

	_, err = config.ExtractPluginsFromTree(tree)
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
`
			schema, err := config.YANGSchema()
			if err != nil {
				t.Fatal(err)
			}
			p := config.NewParser(schema)
			tree, err := p.Parse(input)
			require.NoError(t, err)
			plugins, err := config.ExtractPluginsFromTree(tree)
			require.NoError(t, err)
			require.Len(t, plugins, 1)
			require.Equal(t, tt.expected, plugins[0].StageTimeout)
		})
	}
}
