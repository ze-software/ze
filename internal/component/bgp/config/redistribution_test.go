// Design: (none -- redistribution filter config tests)
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestRedistributionConfigParse verifies parsing redistribution leaf-lists.
//
// VALIDATES: AC-2 -- Config with redistribution import/export validates successfully.
// PREVENTS: Redistribution config silently ignored.
func TestRedistributionConfigParse(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    session {
    	asn {
    		local 65000
    	}
    }
    redistribution {
        import [ rpki:validate ];
    }
    group customers {
        redistribution {
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
    redistribution {
        import [ a:x ];
    }
    group g1 {
        redistribution {
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
            redistribution {
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

// TestRedistributionConfigValidation verifies format validation.
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
        redistribution { import [ badformat ]; }
    }
}`,
			wantErr: "invalid filter reference",
		},
		{
			name: "empty plugin name",
			input: `
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
        redistribution { import [ :filter ]; }
    }
}`,
			wantErr: "empty plugin name",
		},
		{
			name: "empty filter name",
			input: `
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
        redistribution { export [ plugin: ]; }
    }
}`,
			wantErr: "empty filter name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := config.ParseTreeWithYANG(tt.input, nil)
			require.NoError(t, err)
			_, err = PeersFromConfigTree(tree)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestRedistributionStandalonePeer verifies standalone peers get bgp-level filters.
//
// VALIDATES: Standalone peers accumulate bgp-level filters.
// PREVENTS: Bgp-level filters lost for standalone peers.
func TestRedistributionStandalonePeer(t *testing.T) {
	input := `
bgp {
    router-id 1.2.3.4;
    session {
    	asn {
    		local 65000
    	}
    }
    redistribution {
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
        redistribution { export [ peer:export ]; }
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

// TestRedistributionEmpty verifies empty redistribution block is valid.
//
// VALIDATES: No filters = no crash.
// PREVENTS: Panic on missing redistribution block.
func TestRedistributionEmpty(t *testing.T) {
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

// TestDefaultFilterOverride verifies override removes default filters.
//
// VALIDATES: AC-19 -- Filter with overrides removes default filter.
// PREVENTS: Override declarations ignored.
func TestDefaultFilterOverride(t *testing.T) {
	defaults := []string{"rfc:no-self-as", "rfc:bogon-check"}
	userFilters := []string{"allow-own-as:relaxed", "rpki:validate"}
	overrideMap := map[string][]string{
		"allow-own-as:relaxed": {"rfc:no-self-as"},
	}
	result := applyOverrides(defaults, userFilters, overrideMap)
	assert.Equal(t, []string{"rfc:bogon-check"}, result)
}

// TestMandatoryFilterCannotBeOverridden verifies mandatory filters survive.
//
// VALIDATES: AC-21 -- Override targeting mandatory filter is ignored.
// PREVENTS: User removing RFC-mandated safety filters.
func TestMandatoryFilterCannotBeOverridden(t *testing.T) {
	// Mandatory filters are NOT in defaults -- they're always prepended
	// by the reactor. This test verifies overriding a non-default has no effect.
	defaults := []string{"rfc:no-self-as"}
	userFilters := []string{"evil:bypass"}
	overrideMap := map[string][]string{
		"evil:bypass": {"rfc:otc"}, // rfc:otc is mandatory, not in defaults
	}
	result := applyOverrides(defaults, userFilters, overrideMap)
	assert.Equal(t, []string{"rfc:no-self-as"}, result)
}

// TestApplyOverridesEmpty verifies no crash on empty inputs.
func TestApplyOverridesEmpty(t *testing.T) {
	assert.Nil(t, applyOverrides(nil, nil, nil))
	assert.Equal(t, []string{"a:b"}, applyOverrides([]string{"a:b"}, nil, nil))
	assert.Equal(t, []string{"a:b"}, applyOverrides([]string{"a:b"}, []string{"c:d"}, nil))
}
