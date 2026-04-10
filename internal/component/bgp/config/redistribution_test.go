// Design: (none -- filter chain config tests)
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestFilterConfigParse verifies parsing filter leaf-lists.
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
        import [ rpki:validate ];
    }
    group customers {
        filter {
            import [ community:scrub ];
            export [ aspath:prepend ];
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
	assert.Equal(t, []string{"rpki:validate", "community:scrub"}, ps.ImportFilters)
	assert.Equal(t, []string{"aspath:prepend"}, ps.ExportFilters)
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
        import [ a:x ];
    }
    group g1 {
        filter {
            import [ b:y ];
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
                import [ c:z ];
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
	assert.Equal(t, []string{"a:x", "b:y", "c:z"}, peers[0].ImportFilters)
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
        import [ global:filter ];
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
        filter { export [ peer:export ]; }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)
	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	assert.Equal(t, []string{"global:filter"}, peers[0].ImportFilters)
	assert.Equal(t, []string{"peer:export"}, peers[0].ExportFilters)
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
	assert.Empty(t, peers[0].ImportFilters)
	assert.Empty(t, peers[0].ExportFilters)
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
        import [ my-filter ];
        export [ another-filter ];
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
	assert.Contains(t, peers[0].ImportFilters, "my-filter")
	assert.Contains(t, peers[0].ExportFilters, "another-filter")
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
            import [ nonexistent ];
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
            import [ rpki:validate ];
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)
	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	assert.Contains(t, peers[0].ImportFilters, "rpki:validate")
}
