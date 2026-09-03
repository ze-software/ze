// Design: docs/architecture/bgp/filter-path-asn.md -- the transit-leak filter obligation
package bgpconfig

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The tests below drive the obligation from the config text an operator types,
// through the peer pipeline, to the accept or the refusal. They name no filter
// type of their own where the rule reads one: the declaring type comes from the
// registry, exactly as it does at runtime.
//
// The reject-asn list, the prefix-list and the role container reach the schema
// through the plugin registrations loader_test.go links (plugin/all), which is
// how every schema-driven test in this package gets its YANG. The build tags
// select the plugins, so these tests run under `ze_core ze_bgp`.

// leakChains describes the two filter chains a test config gives its peer.
// An empty chain is written as no filter block at all, which is the shape of
// the config an operator forgets to add the filter to.
type leakChains struct {
	importChain string
	exportChain string
}

// leakPeerConfig builds one eBGP peer carrying the given role and chains, with
// both a declaring filter (the reject-asn list) and a non-declaring one (a
// prefix-list) defined under policy so a chain can name either.
//
// role is the RFC 9234 role, or empty for a peer that declares none.
func leakPeerConfig(role string, chains leakChains) string {
	var b strings.Builder
	b.WriteString(`
bgp {
    router-id 1.2.3.4;
    session {
        asn {
            local 65000
        }
    }
    policy {
        reject-asn NO-TRANSIT {
            indirect [ 3356 ]
        }
        prefix-list CUSTOMERS {
            entry 10.0.0.0/8 {
            }
        }
    }
    peer peer-a {
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
`)
	if role != "" {
		b.WriteString("        role {\n            import " + role + "\n        }\n")
	}
	if chains.importChain != "" || chains.exportChain != "" {
		b.WriteString("        filter {\n")
		writeChain(&b, "import", chains.importChain)
		writeChain(&b, "export", chains.exportChain)
		b.WriteString("        }\n")
	}
	b.WriteString("    }\n}\n")
	return b.String()
}

// writeChain writes one filter chain. A name prefixed with "inactive:" is
// written as the member plus the deactivation statement the serializer emits,
// which is the spelling an operator uses to record a deliberate opt-out.
func writeChain(b *strings.Builder, direction, name string) {
	if name == "" {
		return
	}
	member, deactivated := strings.CutPrefix(name, "inactive:")
	b.WriteString("            " + direction + " [ " + member + " ]\n")
	if deactivated {
		b.WriteString("            inactive: " + direction + " " + member + "\n")
	}
}

// loadLeakConfig parses a config and runs the whole peer pipeline over it,
// which is the path daemon startup, `ze config validate` and `ze doctor` all
// take.
func loadLeakConfig(t *testing.T, text string) error {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(text, nil)
	require.NoError(t, err, "the config must parse: this test is about the rule, not the schema")
	_, err = PeersFromConfigTree(tree)
	return err
}

// TestObligationMatrixEveryRoleAndDirection drives every cell of the obligation
// matrix: five RFC 9234 roles plus the absent role, each against the four
// possible chain populations.
//
// VALIDATES: AC-27 through AC-33 and AC-34's acceptance. RFC 9234 names the
// LOCAL speaker's position, so `customer` means the remote is our transit
// provider (export is required, import is not) and `provider` means the remote
// is our customer (import is required, export is not).
// PREVENTS: reading role/import as a filter direction, which inverts every cell
// and still passes a test that drives one role.
func TestObligationMatrixEveryRoleAndDirection(t *testing.T) {
	// requires says which chains each role obliges. It is the matrix under
	// test, written out rather than derived, so a changed expansion cannot
	// pass by agreeing with itself.
	requires := map[string]roleChainObligation{
		"peer":      {importChain: true, exportChain: true},
		"provider":  {importChain: true},
		"customer":  {exportChain: true},
		"rs":        {importChain: true, exportChain: true},
		"rs-client": {importChain: true, exportChain: true},
		"":          {},
	}
	populations := map[string]leakChains{
		"neither chain": {},
		"import only":   {importChain: "NO-TRANSIT"},
		"export only":   {exportChain: "NO-TRANSIT"},
		"both chains":   {importChain: "NO-TRANSIT", exportChain: "NO-TRANSIT"},
	}

	for role, obligation := range requires {
		for name, chains := range populations {
			roleName := role
			if roleName == "" {
				roleName = "no-role"
			}
			t.Run(roleName+"/"+name, func(t *testing.T) {
				satisfied := (!obligation.importChain || chains.importChain != "") &&
					(!obligation.exportChain || chains.exportChain != "")

				err := loadLeakConfig(t, leakPeerConfig(role, chains))
				if satisfied {
					assert.NoError(t, err, "role %q with %s must load", role, name)
					return
				}
				require.Error(t, err, "role %q with %s must be refused", role, name)
				assert.Contains(t, err.Error(), "peer-a")
			})
		}
	}
}

// TestRefusalMessageNamesPeerRoleAndChain checks the content of the refusal,
// not just that one happened.
//
// VALIDATES: AC-27. The operator is told which peer, which role bound it, which
// chain is short, and BOTH ways to satisfy the obligation.
// PREVENTS: a refusal that names only the peer, leaving the operator to guess
// what to write (R-6).
func TestRefusalMessageNamesPeerRoleAndChain(t *testing.T) {
	err := loadLeakConfig(t, leakPeerConfig("provider", leakChains{}))
	require.Error(t, err)

	msg := err.Error()
	assert.Contains(t, msg, "peer-a", "the message must name the peer")
	assert.Contains(t, msg, "provider", "the message must name the role that bound it")
	assert.Contains(t, msg, "import filter chain", "the message must name the chain that is short")
	assert.Contains(t, msg, "reject-asn", "the message must name a filter type that discharges the obligation")
	assert.Contains(t, msg, "inactive:", "the message must name the opt-out spelling")
	assert.NotContains(t, msg, "export", "a provider owes nothing on export, so the message must not mention it")
}

// TestInactiveRefSatisfiesObligation verifies the opt-out.
//
// VALIDATES: AC-30. A deactivated ref records that the operator considered the
// check and chose to run without it on this session, which is the decision the
// obligation asks for.
// PREVENTS: an operator having to choose between running a filter they do not
// want and being unable to load their config.
func TestInactiveRefSatisfiesObligation(t *testing.T) {
	err := loadLeakConfig(t, leakPeerConfig("peer", leakChains{
		importChain: "inactive:NO-TRANSIT",
		exportChain: "inactive:NO-TRANSIT",
	}))
	assert.NoError(t, err)
}

// TestUnrelatedFilterDoesNotSatisfyObligation names a filter of a type that
// declares no obligation.
//
// VALIDATES: AC-37. A prefix-list in the chain does not discharge a transit-leak
// obligation.
// PREVENTS: the rule counting any filter at all, which would make it a check
// that the chain is non-empty rather than a check that the leak is covered.
func TestUnrelatedFilterDoesNotSatisfyObligation(t *testing.T) {
	err := loadLeakConfig(t, leakPeerConfig("peer", leakChains{
		importChain: "CUSTOMERS",
		exportChain: "CUSTOMERS",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer-a")
}

// TestInheritedGroupRoleBindsThePeer sets the role on the group.
//
// VALIDATES: AC-36. ResolveBGPTree deep-merges the group's fields into each
// peer, so the rule reads an inherited role exactly as a directly set one.
// PREVENTS: group-configured deployments -- the common shape at an IXP --
// staying unguarded while single-peer tests pass.
func TestInheritedGroupRoleBindsThePeer(t *testing.T) {
	text := `
bgp {
    router-id 1.2.3.4;
    session {
        asn {
            local 65000
        }
    }
    policy {
        reject-asn NO-TRANSIT {
            indirect [ 3356 ]
        }
    }
    group upstreams {
        role {
            import customer
        }
        peer peer-a {
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
	err := loadLeakConfig(t, text)
	require.Error(t, err, "the group's customer role must bind its peer")
	assert.Contains(t, err.Error(), "export filter chain",
		"a customer session owes the export chain: the remote is our transit provider")
}

// TestPeerWithNoRoleIsAcceptedAndWarnedOnce loads three peers that declare no
// role and captures what the config logger wrote.
//
// VALIDATES: AC-34. The config loads, and ONE line reports the whole set with
// the exact count.
// PREVENTS: one warning per peer, which buries the fact it exists to report on
// a router carrying a thousand sessions.
func TestPeerWithNoRoleIsAcceptedAndWarnedOnce(t *testing.T) {
	var logged bytes.Buffer
	previous := configLogger
	configLogger = func() *slog.Logger {
		return slogutil.LoggerWithOutput("bgp.config", "warn", &logged)
	}
	t.Cleanup(func() { configLogger = previous })

	text := `
bgp {
    router-id 1.2.3.4;
    session {
        asn {
            local 65000
        }
    }
    peer peer-a {
        connection { remote { ip 10.0.0.1 } local { ip auto } }
        session { asn { remote 65001 } }
    }
    peer peer-b {
        connection { remote { ip 10.0.0.2 } local { ip auto } }
        session { asn { remote 65002 } }
    }
    peer ibgp-a {
        connection { remote { ip 10.0.0.3 } local { ip auto } }
        session { asn { remote 65000 } }
    }
}
`
	require.NoError(t, loadLeakConfig(t, text), "a peer with no role is accepted")

	lines := strings.Count(strings.TrimSpace(logged.String()), "\n") + 1
	assert.Equal(t, 1, lines, "one aggregated line, never one per peer:\n%s", logged.String())
	assert.Contains(t, logged.String(), "peers=2",
		"the count covers the two eBGP peers and leaves the iBGP session out")
	assert.Contains(t, logged.String(), "peer-a")
	assert.Contains(t, logged.String(), "peer-b")
	assert.NotContains(t, logged.String(), "ibgp-a",
		"an RFC 9234 role describes an eBGP relationship, so an iBGP session is not unbound")
}

// TestNoDeclaringFilterTypeMeansNoEnforcement drives the rule with an empty set
// of declaring filter types, which is what a build with the implementing
// plugin's feature tag off hands it.
//
// VALIDATES: AC-40. This is a GUARD: an obligation nothing in the binary can
// discharge is not enforced.
// PREVENTS: a minimal build refusing every config that declares a role (R-8).
func TestNoDeclaringFilterTypeMeansNoEnforcement(t *testing.T) {
	tree, err := config.ParseTreeWithYANG(leakPeerConfig("peer", leakChains{}), nil)
	require.NoError(t, err)
	_, err = PeersFromConfigTree(tree)
	require.Error(t, err, "with the plugin registered, the same config is refused")

	bgpTree, err := ResolveBGPTree(tree)
	require.NoError(t, err)
	peers, err := reactor.PeersFromTree(bgpTree)
	require.NoError(t, err)

	assert.NoError(t, validateLeakFilterObligations(peerRoles(bgpTree, peers, nil), nil, nil),
		"no declaring filter type means no enforcement")
}

// TestRuleReadsTheObligationFromTheRegistry proves the rule and the plugin meet
// only through the registry declaration.
//
// VALIDATES: the coupling the design forbids. The rule asks which filter types
// discharge the obligation; the answer comes from the plugin's registration.
// PREVENTS: internal/component/bgp/config naming a plugin's filter type, the
// coupling behind the loop-detection defect in plan/journal/unwired-feature.md.
func TestRuleReadsTheObligationFromTheRegistry(t *testing.T) {
	assert.Equal(t, []string{"reject-asn"},
		registry.FilterTypesDischarging(filterapi.ObligationTransitLeak),
		"the reject-asn plugin is the filter type that discharges the transit-leak obligation")
}

// TestFilterInstanceNameStripsEveryPrefixForm checks the four chain-ref forms
// resolve to one instance name.
//
// VALIDATES: the rule finds the filter whichever form the operator wrote and
// whichever form canonicalization left behind.
// PREVENTS: an obligation satisfied in the plain form and refused in the
// canonical one, or the reverse.
func TestFilterInstanceNameStripsEveryPrefixForm(t *testing.T) {
	for _, ref := range []string{
		"NO-TRANSIT",
		"reject-asn:NO-TRANSIT",
		"bgp-filter-path-asn:NO-TRANSIT",
	} {
		assert.Equal(t, "NO-TRANSIT", filterInstanceName(ref))
	}
}
