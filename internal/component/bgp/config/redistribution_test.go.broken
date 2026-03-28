// Design: (none -- new redistribution filter config parsing)
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedistributionConfigParse verifies parsing redistribution leaf-lists from config tree.
//
// VALIDATES: AC-2 -- Config with redistribution import/export validates successfully.
// PREVENTS: Redistribution config silently ignored.
func TestRedistributionConfigParse(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    local { as 65000; }
    redistribution {
        import [ rpki:validate ];
    }
    group customers {
        redistribution {
            import [ community:scrub ];
            export [ aspath:prepend ];
        }
        peer customer-a {
            remote { ip 10.0.0.1; as 65001; }
            local { ip auto; }
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	// Cumulative: bgp import + group import
	assert.Equal(t, []string{"rpki:validate", "community:scrub"}, ps.ImportFilters)
	// Only group export
	assert.Equal(t, []string{"aspath:prepend"}, ps.ExportFilters)
}

// TestFilterChainResolution verifies bgp > group > peer cumulative merging.
//
// VALIDATES: AC-12 -- Config hierarchy bgp > group > peer produces correct chain.
// PREVENTS: Filters from wrong level or wrong order.
func TestFilterChainResolution(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    local { as 65000; }
    redistribution {
        import [ a:x ];
    }
    group g1 {
        redistribution {
            import [ b:y ];
        }
        peer p1 {
            remote { ip 10.0.0.1; as 65001; }
            local { ip auto; }
            redistribution {
                import [ c:z ];
            }
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	assert.Equal(t, []string{"a:x", "b:y", "c:z"}, ps.ImportFilters)
}

// TestRedistributionConfigValidation verifies format validation of filter references.
//
// VALIDATES: AC-3c -- Config error on invalid filter reference format.
// PREVENTS: Malformed filter references reaching the reactor.
func TestRedistributionConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "missing colon",
			input: `
bgp {
    router-id 1.2.3.4;
    local { as 65000; }
    peer p1 {
        remote { ip 10.0.0.1; as 65001; }
            local { ip auto; }
        redistribution {
            import [ badformat ];
        }
    }
}`,
			wantErr: "invalid filter reference",
		},
		{
			name: "empty plugin name",
			input: `
bgp {
    router-id 1.2.3.4;
    local { as 65000; }
    peer p1 {
        remote { ip 10.0.0.1; as 65001; }
            local { ip auto; }
        redistribution {
            import [ :filter ];
        }
    }
}`,
			wantErr: "empty plugin name",
		},
		{
			name: "empty filter name",
			input: `
bgp {
    router-id 1.2.3.4;
    local { as 65000; }
    peer p1 {
        remote { ip 10.0.0.1; as 65001; }
            local { ip auto; }
        redistribution {
            export [ plugin: ];
        }
    }
}`,
			wantErr: "empty filter name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := parseTreeWithYANG(tt.input, nil)
			require.NoError(t, err)

			_, err = PeersFromConfigTree(tree)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestRedistributionStandalonePeer verifies standalone peers get bgp-level filters.
//
// VALIDATES: Standalone peers (no group) still accumulate bgp-level filters.
// PREVENTS: Bgp-level filters lost for standalone peers.
func TestRedistributionStandalonePeer(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    local { as 65000; }
    redistribution {
        import [ global:filter ];
    }
    peer standalone {
        remote { ip 10.0.0.1; as 65001; }
            local { ip auto; }
        redistribution {
            export [ peer:export ];
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	assert.Equal(t, []string{"global:filter"}, ps.ImportFilters)
	assert.Equal(t, []string{"peer:export"}, ps.ExportFilters)
}

// TestRedistributionEmpty verifies empty redistribution block is valid.
//
// VALIDATES: No filters configured = no filters in chain (no crash).
// PREVENTS: Panic or error on missing redistribution block.
func TestRedistributionEmpty(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    local { as 65000; }
    peer p1 {
        remote { ip 10.0.0.1; as 65001; }
            local { ip auto; }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	assert.Empty(t, ps.ImportFilters)
	assert.Empty(t, ps.ExportFilters)
}
