// Design: (none -- filter chain config tests)
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_prefix"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestFilterConfigParse verifies parsing explicit filter policies.
//
// VALIDATES: AC-2 -- Config with filter import/export validates successfully.
// PREVENTS: Filter config silently ignored.
func TestFilterConfigParse(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    session {
    	asn {
    		local 65000
    	}
    }
    filter {
        import { filters [ rpki:validate ]; final accept; }
    }
    group customers {
        filter {
            import { filters [ community:scrub ]; final accept; }
            export { filters [ aspath:prepend ]; final accept; }
        }
        peer customer-a {
            connection {
                remote {
                    ip 10.0.0.1
                }
                local {
                    ip auto
                }
            }
            session {
                asn {
                    remote 65001
                }
            }
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)
	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	ps := peers[0]
	assert.Equal(t, refs("rpki:validate", "community:scrub"), ps.ImportPolicy.Filters())
	assert.Equal(t, refs("aspath:prepend"), ps.ExportPolicy.Filters())
}

// TestFilterChainResolution verifies bgp > group > peer cumulative merging.
//
// VALIDATES: AC-12 -- Config hierarchy produces correct chain order.
// PREVENTS: Filters from wrong level or wrong order.
func TestFilterChainResolution(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    session {
    	asn {
    		local 65000
    	}
    }
    filter {
        import { filters [ a:x ]; final accept; }
    }
    group g1 {
        filter {
            import { filters [ b:y ]; final accept; }
        }
        peer p1 {
            connection {
                remote {
                    ip 10.0.0.1
                }
                local {
                    ip auto
                }
            }
            session {
                asn {
                    remote 65001
                }
            }
            filter {
                import { filters [ c:z ]; final accept; }
            }
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)
	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	assert.Equal(t, refs("a:x", "b:y", "c:z"), peers[0].ImportPolicy.Filters())
}

// TestFilterStandalonePeer verifies standalone peers get bgp-level filters.
//
// VALIDATES: Standalone peers accumulate bgp-level filters.
// PREVENTS: Bgp-level filters lost for standalone peers.
func TestFilterStandalonePeer(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    session {
    	asn {
    		local 65000
    	}
    }
    filter {
        import { filters [ global:filter ]; final accept; }
    }
    peer standalone {
        connection {
            remote {
                ip 10.0.0.1
            }
            local {
                ip auto
            }
        }
        session {
            asn {
                remote 65001
            }
        }
        filter { export { filters [ peer:export ]; final accept; } }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)
	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	assert.Equal(t, refs("global:filter"), peers[0].ImportPolicy.Filters())
	assert.Equal(t, refs("peer:export"), peers[0].ExportPolicy.Filters())
}

// TestFilterEmpty verifies empty filter block is valid.
//
// VALIDATES: No filters = no crash.
// PREVENTS: Panic on missing filter block.
func TestFilterEmpty(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    session {
    	asn {
    		local 65000
    	}
    }
    peer p1 {
        connection {
            remote {
                ip 10.0.0.1
            }
            local {
                ip auto
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)
	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	assert.Empty(t, peers[0].ImportPolicy.Filters())
	assert.Empty(t, peers[0].ExportPolicy.Filters())
}

// TestFilterPlainNames verifies that plain names (without colons) are accepted
// when they exist in the policy section.
//
// VALIDATES: Filter names are plain names, validated against policy registry.
// PREVENTS: Regression to old colon-required validation.
func TestFilterPlainNames(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    session {
    	asn {
    		local 65000
    	}
    }
    policy {
        loop-detection my-filter {
        }
        loop-detection another-filter {
        }
    }
    filter {
        import { filters [ my-filter ]; final accept; }
        export { filters [ another-filter ]; final accept; }
    }
    peer p1 {
        connection {
            remote {
                ip 10.0.0.1
            }
            local {
                ip auto
            }
        }
        session {
            asn {
                remote 65001
            }
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)
	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	assert.True(t, filterChainContains(peers[0].ImportPolicy.Filters(), "my-filter"))
	assert.True(t, filterChainContains(peers[0].ExportPolicy.Filters(), "another-filter"))
}

// TestFilterUnknownNameRejectsAtParse verifies typos in filter names fail at parse time.
//
// VALIDATES: AC-1 -- plain filter name not in policy produces parse error.
// PREVENTS: Silent acceptance of misspelled filter names.
func TestFilterUnknownNameRejectsAtParse(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    session {
    	asn {
    		local 65000
    	}
    }
    peer p1 {
        connection {
            remote {
                ip 10.0.0.1
            }
            local {
                ip auto
            }
        }
        session {
            asn {
                remote 65001
            }
        }
        filter {
            import { filters [ nonexistent ]; final accept; }
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)
	_, err = PeersFromConfigTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown filter")
	assert.Contains(t, err.Error(), "nonexistent")
}

// TestFilterExternalPluginNamePassesParse verifies colon-containing names pass parse.
//
// VALIDATES: AC-3 -- external plugin filter names (with colon) skip parse-time validation.
// PREVENTS: External plugin filters blocked at config parse.
func TestFilterExternalPluginNamePassesParse(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    session {
    	asn {
    		local 65000
    	}
    }
    peer p1 {
        connection {
            remote {
                ip 10.0.0.1
            }
            local {
                ip auto
            }
        }
        session {
            asn {
                remote 65001
            }
        }
        filter {
            import { filters [ rpki:validate ]; final accept; }
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)
	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	assert.True(t, filterChainContains(peers[0].ImportPolicy.Filters(), "rpki:validate"))
}

// TestCanonicalizePlainPrefixRef verifies plain filter names resolve through
// the filter registry and FilterTypesMap for prefix-list filters.
//
// VALIDATES: AC-1, AC-2 -- plain unique name resolves to canonical plugin ref.
// PREVENTS: Plain name left unresolved, reaching runtime as bare string.
func TestCanonicalizePlainPrefixRef(t *testing.T) {
	reg := &FilterRegistry{entries: map[string]FilterEntry{
		"CUSTOMERS": {Name: "CUSTOMERS", Type: "prefix-list"},
	}}
	typesMap := registry.FilterTypesMap()

	got := canonicalizeOne("CUSTOMERS", reg, typesMap)
	assert.Equal(t, "bgp-filter-prefix:CUSTOMERS", got)

	got = canonicalizeOne("prefix-list:CUSTOMERS", reg, typesMap)
	assert.Equal(t, "bgp-filter-prefix:CUSTOMERS", got)
}

// TestCanonicalizeInactivePlainRef verifies the structural deactivation flag is
// preserved around plain name canonicalization while the name is rewritten.
//
// VALIDATES: inactive state survives plain-name resolution.
// PREVENTS: FilterRef.Inactive dropped or an inactive filter reactivated by
// canonicalization.
func TestCanonicalizeInactivePlainRef(t *testing.T) {
	reg := &FilterRegistry{entries: map[string]FilterEntry{
		"CUSTOMERS": {Name: "CUSTOMERS", Type: "prefix-list"},
	}}

	got := canonicalizeFilterRefs([]filterapi.FilterRef{{Name: "CUSTOMERS", Inactive: true}}, reg)
	assert.Equal(t, []filterapi.FilterRef{{Name: "bgp-filter-prefix:CUSTOMERS", Inactive: true}}, got)
}
