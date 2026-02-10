package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseNLRIEntriesValid(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		wantMode int // 0=all, 1=none, 2=selective
		families []string
	}{
		{"empty returns all", []string{}, 0, nil},
		{"all keyword", []string{"all"}, 0, nil},
		{"none keyword", []string{"none"}, 1, nil},
		{"ipv4/unicast", []string{"ipv4/unicast"}, 2, []string{"ipv4/unicast"}},
		{"ipv6/unicast", []string{"ipv6/unicast"}, 2, []string{"ipv6/unicast"}},
		{"multiple families", []string{"ipv4/unicast", "ipv6/unicast"}, 2, []string{"ipv4/unicast", "ipv6/unicast"}},
		{"case insensitive", []string{"IPv4/Unicast", "IPV6/UNICAST"}, 2, []string{"ipv4/unicast", "ipv6/unicast"}},
		{"l2vpn/evpn", []string{"l2vpn/evpn"}, 2, []string{"l2vpn/evpn"}},
		{"ipv4/flow", []string{"ipv4/flow"}, 2, []string{"ipv4/flow"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := parseNLRIEntries(tt.entries)
			require.NoError(t, err)

			switch tt.wantMode {
			case 0: // all
				require.True(t, filter.IncludesFamily("ipv4/unicast"), "all mode should include ipv4/unicast")
				require.True(t, filter.IncludesFamily("ipv6/unicast"), "all mode should include ipv6/unicast")
			case 1: // none
				require.True(t, filter.IsEmpty(), "none mode should be empty")
			case 2: // selective
				for _, f := range tt.families {
					require.True(t, filter.IncludesFamily(f), "should include %s", f)
				}
			}
		})
	}
}

// TestParseNLRIEntriesInvalid verifies invalid NLRI entries are rejected.
//
// VALIDATES: Invalid entries produce clear errors.
//
// PREVENTS: Silent failures on malformed NLRI config.
func TestParseNLRIEntriesInvalid(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		wantErr string
	}{
		{"missing safi", []string{"ipv4"}, "unknown family"},
		{"invalid format", []string{"ipv4/unicast extra"}, "unknown family"},
		{"unknown afi", []string{"bogus/unicast"}, "unknown family"},
		{"unknown safi", []string{"ipv4/bogus"}, "unknown family"},
		{"empty entry", []string{"ipv4/unicast", ""}, ""}, // empty entries skipped
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseNLRIEntries(tt.entries)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

// TestAPIConfigNLRIFilter verifies NLRI filter in API config.
//
// VALIDATES: Config with nlri entries parses correctly.
//
// PREVENTS: NLRI filter config not being applied.
func TestAPIConfigNLRIFilter(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder json; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            content {
                encoding json;
                nlri ipv4/unicast;
                nlri ipv6/unicast;
            }
            receive { update; }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].ProcessBindings, 1)

	binding := cfg.Peers[0].ProcessBindings[0]
	require.NotNil(t, binding.Content.NLRI, "NLRI filter should be set")
	require.True(t, binding.Content.NLRI.IncludesFamily("ipv4/unicast"))
	require.True(t, binding.Content.NLRI.IncludesFamily("ipv6/unicast"))
	require.False(t, binding.Content.NLRI.IncludesFamily("l2vpn/evpn"))
}

// TestAPIConfigAttributeFilterError verifies invalid attribute filter is rejected.
//
// VALIDATES: Invalid attribute config produces error.
//
// PREVENTS: Silent failures on malformed attribute config.
func TestAPIConfigAttributeFilterError(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder json; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            content {
                attribute bogus-attribute;
            }
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
	require.Contains(t, err.Error(), "invalid attribute filter")
}

// TestAPIConfigNLRIFilterError verifies invalid NLRI filter is rejected.
//
// VALIDATES: Invalid NLRI config produces error.
//
// PREVENTS: Silent failures on malformed NLRI config.
func TestAPIConfigNLRIFilterError(t *testing.T) {
	input := `
plugin { external foo { run ./test; encoder json; } }
bgp {
    peer 10.0.0.1 {
        router-id 1.2.3.4;
        local-as 65001;
        peer-as 65002;
        process foo {
            content {
                nlri bogus family;
            }
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
	require.Contains(t, err.Error(), "invalid nlri filter")
}

// TestConfigValidationRouteRefreshRequiresProcess verifies route-refresh without process fails.
//
// VALIDATES: Config with route-refresh capability but no process binding is rejected.
// PREVENTS: Silent runtime failure when route-refresh request arrives with no plugin to handle it.
func TestParseUpdateBlock_Basic(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 1, "expected 1 static route from update block")

	route := cfg.Peers[0].StaticRoutes[0]
	require.Equal(t, "1.0.0.0/24", route.Prefix.String())
	require.Equal(t, "10.0.0.1", route.NextHop)
	require.Equal(t, "igp", route.Origin)
}

// TestParseUpdateBlock_Attributes verifies all attribute types.
//
// VALIDATES: All supported attributes parse correctly in update block.
// PREVENTS: Missing attribute support in native syntax.
func TestParseUpdateBlock_Attributes(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin egp;
                next-hop 10.0.0.1;
                med 100;
                local-preference 200;
                as-path [ 65001 65002 ];
                community [ 65000:1 65000:2 ];
                large-community [ 65000:0:1 ];
                extended-community [ target:65000:100 ];
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 1)

	route := cfg.Peers[0].StaticRoutes[0]
	require.Equal(t, "egp", route.Origin)
	require.Equal(t, "10.0.0.1", route.NextHop)
	require.Equal(t, uint32(100), route.MED)
	require.Equal(t, uint32(200), route.LocalPreference)
	// Value-or-array stores without brackets
	require.Equal(t, "65001 65002", route.ASPath)
	require.Equal(t, "65000:1 65000:2", route.Community)
	require.Equal(t, "65000:0:1", route.LargeCommunity)
	require.Equal(t, "target:65000:100", route.ExtendedCommunity)
}

// TestParseUpdateBlock_MultiplePrefixes verifies multiple prefixes in one family.
//
// VALIDATES: Multiple prefixes in a single nlri line are parsed correctly.
// PREVENTS: Only first prefix being processed.
func TestParseUpdateBlock_MultiplePrefixes(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24 2.0.0.0/24 3.0.0.0/24;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 3, "expected 3 routes")

	// All routes should have same attributes
	for _, route := range cfg.Peers[0].StaticRoutes {
		require.Equal(t, "igp", route.Origin)
		require.Equal(t, "10.0.0.1", route.NextHop)
	}
}

// TestParseUpdateBlock_NextHopSelf verifies next-hop self.
//
// VALIDATES: next-hop self is correctly parsed.
// PREVENTS: Literal "self" being stored instead of flag.
func TestParseUpdateBlock_NextHopSelf(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop self;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 1)

	route := cfg.Peers[0].StaticRoutes[0]
	require.True(t, route.NextHopSelf, "expected NextHopSelf to be true")
	require.Empty(t, route.NextHop, "expected NextHop to be empty when using self")
}

// TestParseUpdateBlock_MissingNLRI verifies error on missing nlri block.
//
// VALIDATES: Update block without nlri produces clear error.
// PREVENTS: Silent failures on malformed config.
func TestParseUpdateBlock_MissingNLRI(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nlri")
}

// TestParseUpdateBlock_InvalidFamily verifies error on invalid family.
//
// VALIDATES: Invalid family name produces clear error.
// PREVENTS: Silent failures or panics on invalid input.
func TestParseUpdateBlock_InvalidFamily(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                invalid/family 1.0.0.0/24;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	_, err = TreeToConfig(tree)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid family")
}

// TestParseUpdateBlock_Multiple verifies multiple update blocks per peer.
//
// VALIDATES: Multiple update blocks with different attributes are parsed correctly.
// PREVENTS: Only first update block being processed.
func TestParseUpdateBlock_Multiple(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
        update {
            attribute {
                origin egp;
                next-hop 10.0.0.2;
            }
            nlri {
                ipv4/unicast 2.0.0.0/24;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 2, "expected 2 routes from 2 update blocks")

	// Find routes by prefix and verify attributes
	var found1, found2 bool
	for _, r := range cfg.Peers[0].StaticRoutes {
		switch r.Prefix.String() {
		case "1.0.0.0/24":
			require.Equal(t, "igp", r.Origin)
			require.Equal(t, "10.0.0.1", r.NextHop)
			found1 = true
		case "2.0.0.0/24":
			require.Equal(t, "egp", r.Origin)
			require.Equal(t, "10.0.0.2", r.NextHop)
			found2 = true
		}
	}
	require.True(t, found1, "expected route 1.0.0.0/24")
	require.True(t, found2, "expected route 2.0.0.0/24")
}

// TestParseUpdateBlock_InvalidMED verifies error on invalid MED value.
//
// VALIDATES: Non-numeric MED produces clear error at parse time.
// PREVENTS: Silent failures with MED=0.
func TestParseUpdateBlock_InvalidMED(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
                med abc;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`
	p := NewParser(YANGSchema())
	_, err := p.Parse(input)
	// YANG schema validates uint32, so error happens at parse time
	require.Error(t, err)
	require.Contains(t, err.Error(), "med")
}

// TestParseUpdateBlock_IPv6 verifies IPv6 prefix parsing.
//
// VALIDATES: IPv6 prefixes parse correctly in update block.
// PREVENTS: IPv4-only assumption in NLRI parsing.
func TestParseUpdateBlock_IPv6(t *testing.T) {
	input := `
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 2001:db8::1;
            }
            nlri {
                ipv6/unicast 2001:db8::/32 2001:db8:1::/48;
            }
        }
    }
}
`
	cfg := parseConfig(t, input)
	require.Len(t, cfg.Peers, 1)
	require.Len(t, cfg.Peers[0].StaticRoutes, 2, "expected 2 IPv6 routes")

	// Verify both prefixes parsed correctly
	prefixes := make(map[string]bool)
	for _, route := range cfg.Peers[0].StaticRoutes {
		prefixes[route.Prefix.String()] = true
		require.Equal(t, "igp", route.Origin)
		require.Equal(t, "2001:db8::1", route.NextHop)
	}
	require.True(t, prefixes["2001:db8::/32"], "expected 2001:db8::/32")
	require.True(t, prefixes["2001:db8:1::/48"], "expected 2001:db8:1::/48")
}

// TestParseUpdateBlock_MEDBoundary verifies MED boundary values.
//
// VALIDATES: MED accepts 0 and max uint32 (4294967295).
// PREVENTS: Off-by-one errors in MED validation.
// BOUNDARY: 0 (valid min), 4294967295 (valid max).
func TestParseUpdateBlock_MEDBoundary(t *testing.T) {
	tests := []struct {
		name    string
		med     string
		want    uint32
		wantErr bool
	}{
		{"min_valid_0", "0", 0, false},
		{"max_valid_4294967295", "4294967295", 4294967295, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(`
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
                med %s;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`, tt.med)
			cfg := parseConfig(t, input)
			require.Len(t, cfg.Peers, 1)
			require.Len(t, cfg.Peers[0].StaticRoutes, 1)
			require.Equal(t, tt.want, cfg.Peers[0].StaticRoutes[0].MED)
		})
	}
}

// TestParseUpdateBlock_LocalPrefBoundary verifies local-preference boundary values.
//
// VALIDATES: local-preference accepts 0 and max uint32 (4294967295).
// PREVENTS: Off-by-one errors in local-preference validation.
// BOUNDARY: 0 (valid min), 4294967295 (valid max).
func TestParseUpdateBlock_LocalPrefBoundary(t *testing.T) {
	tests := []struct {
		name    string
		locpref string
		want    uint32
	}{
		{"min_valid_0", "0", 0},
		{"max_valid_4294967295", "4294967295", 4294967295},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(`
bgp {
    peer 192.0.2.1 {
        peer-as 65001;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.1;
                local-preference %s;
            }
            nlri {
                ipv4/unicast 1.0.0.0/24;
            }
        }
    }
}
`, tt.locpref)
			cfg := parseConfig(t, input)
			require.Len(t, cfg.Peers, 1)
			require.Len(t, cfg.Peers[0].StaticRoutes, 1)
			require.Equal(t, tt.want, cfg.Peers[0].StaticRoutes[0].LocalPreference)
		})
	}
}
