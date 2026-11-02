// Design: (none -- filter chain config tests)
package bgpconfig

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_prefix"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
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
	assert.Equal(t, refs(blackholeAgreement, "aspath:prepend"), ps.ExportPolicy.Filters())
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
	assert.Equal(t, refs(blackholeAgreement, "peer:export"), peers[0].ExportPolicy.Filters())
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
	assert.Equal(t, refs(blackholeAgreement), peers[0].ExportPolicy.Filters())
	// The inserted member does not make the export chain operator-configured:
	// with no filter block, RFC 8212 still rejects at the end of the eBGP chain.
	assert.False(t, peers[0].ExportPolicy.Configured())
	assert.False(t, peers[0].ExportPolicy.AcceptsAtEnd(true))
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
policy {
    loop-detection my-filter {
    }
    prefix-list any-route {
        entry 0.0.0.0/0 {
            action accept;
        }
    }
    match another-filter {
        prefix-list any-route
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
	assert.Contains(t, err.Error(), "peer p1 import filters")
}

// TestFilterInactiveUnknownNameRejectsAtParse verifies an inactive chain entry
// is still checked against the policy namespace.
//
// VALIDATES: an inactive filter reference naming no policy entry is a parse error.
// PREVENTS: a typo hidden behind inactive: surviving until the filter is activated.
func TestFilterInactiveUnknownNameRejectsAtParse(t *testing.T) {
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
            import { inactive: filters undefined-name; final accept; }
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)
	_, err = PeersFromConfigTree(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown filter")
	assert.Contains(t, err.Error(), "undefined-name")
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

// TestCanonicalizeInactivePlainRef verifies the structural deactivation flag is
// preserved around plain name canonicalization while the name is rewritten.
//
// VALIDATES: inactive state survives plain-name resolution.
// PREVENTS: policy.Ref.Inactive dropped or an inactive filter reactivated by
// canonicalization.
func TestCanonicalizeInactivePlainRef(t *testing.T) {
	reg := mustPolicyRegistry(t, policy.Instance{Name: "CUSTOMERS", Type: "family-filter"})

	got, bound := canonicalizeFilterRefs([]policy.Ref{{Name: "CUSTOMERS", Inactive: true}}, reg)
	assert.Equal(t, []policy.Ref{{Name: "bgp-filter-family:CUSTOMERS", Inactive: true}}, got)
	assert.Nil(t, bound)
}

// mustPolicyRegistry builds a filter namespace holding exactly instances.
func mustPolicyRegistry(t *testing.T, instances ...policy.Instance) *policy.Registry {
	t.Helper()
	reg, err := policy.NewRegistry(instances...)
	if err != nil {
		t.Fatalf("policy registry: %v", err)
	}
	return reg
}

// blackholeAgreement is the member bgp-filter-community inserts, required, at
// the head of the export list of every eBGP session that is not a
// route-server client (RFC 7999 Section 3.1). This package's tests link
// plugin/all, so every eBGP export list they resolve carries it.
const blackholeAgreement = "bgp-filter-community:blackhole-agreement"

// insertedExport is the export list a peer resolves to before any operator
// member: the BLACKHOLE agreement check on an eBGP session, nothing on iBGP.
func insertedExport(ps *reactor.PeerSettings) []policy.Ref {
	if ps.IsEBGP() {
		return refs(blackholeAgreement)
	}
	return nil
}

// refs builds a policy.Ref chain from canonical strings, mapping an "inactive:"
// prefix to the structural deactivation flag. Shared by bgpconfig tests to keep
// filter chains expressed compactly against the []policy.Ref API.
func refs(names ...string) []policy.Ref {
	if len(names) == 0 {
		return nil
	}
	out := make([]policy.Ref, len(names))
	for i, n := range names {
		if rest, ok := strings.CutPrefix(n, "inactive:"); ok {
			out[i] = policy.Ref{Name: rest, Inactive: true}
		} else {
			out[i] = policy.Ref{Name: n}
		}
	}
	return out
}
