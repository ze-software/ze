// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
package filter_path_asn

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/hub/yang"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// wiringMatch is the match instance the peer's chain names: its reject-asn
// condition names the NO-TRANSIT list.
const wiringMatch = "no-transit"

// wiringConfig is one peer whose named chain carries a match instance naming
// the list, written the way an operator writes it.
//
// chain is "import" or "export", so both directions come off one config.
func wiringConfig(chain string) string {
	return `
bgp {
    router-id 1.2.3.4;
    session {
        asn {
            local 65000
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
        filter {
            ` + chain + ` { filters [ ` + wiringMatch + ` ]; final accept; }
        }
    }
}
policy {
    reject-asn NO-TRANSIT {
        indirect [ 3356 ]
    }
    match ` + wiringMatch + ` {
        reject-asn NO-TRANSIT;
    }
}
`
}

// peerChain parses the config text and answers the library chain the peer's
// chain compiled to at peer load: the chain names the match instance, and the
// match's reject-asn condition is what judges the route.
func peerChain(t *testing.T, chain string) *policychain.Chain {
	t.Helper()

	tree, err := config.ParseTreeWithYANG(wiringConfig(chain), nil)
	require.NoError(t, err, "the reject-asn list, its position leaf and the match must parse against the registered YANG")

	peers, err := bgpconfig.PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	p := peers[0].ImportPolicy
	if chain == "export" {
		p = peers[0].ExportPolicy
	}
	require.Equal(t, []policy.Ref{{Name: wiringMatch}}, p.Filters(),
		"the chain must name the match instance")
	compiled, ok := p.Library().(*policychain.Chain)
	require.True(t, ok, "a chain naming a match instance must compile to the library chain")
	return compiled
}

// judgeLeak runs chain over a route that reached AS65001 through AS3356.
// On import the session names the sender; on export it names none.
func judgeLeak(t *testing.T, chain *policychain.Chain, session policyview.Session) policychain.Outcome {
	t.Helper()
	var view policyview.View
	view.Reset(pathUpdate(t, 65001, 3356, 65002), session)
	defer view.Clear()
	outcome := chain.Run(&view, new(policychain.RunSets))
	require.False(t, outcome.Failed, "the chain failed the route: %+v", outcome)
	return outcome
}

// TestPathASNFilterReachedFromPeerImportChain drives the import half of the
// wiring: a match instance naming the list in a peer's import chain compiles
// to the peer's library chain, whose reject-asn condition rejects a path that
// runs through a listed ASN.
//
// VALIDATES: the Wiring Test row for the import chain.
// PREVENTS: a registered condition that no peer chain can reach, which passes
// every unit test of the matcher and filters nothing.
func TestPathASNFilterReachedFromPeerImportChain(t *testing.T) {
	chain := peerChain(t, "import")

	outcome := judgeLeak(t, chain, policyview.Session{EBGP: true, Sender: 65001, SenderKnown: true})

	assert.Equal(t, policychain.OutcomeReject, outcome.Action,
		"AS3356 is transit in this path, so the route the peer leaked must be rejected")
}

// TestPathASNFilterReachedFromPeerExportChain drives the export half. An
// export session names no sender, so no ASN is the neighbor and indirect
// covers the whole path.
//
// VALIDATES: the Wiring Test row for the export chain.
// PREVENTS: a list that attaches to import only, leaving RFC 7454 Section 9's
// export recommendation with no producer.
func TestPathASNFilterReachedFromPeerExportChain(t *testing.T) {
	chain := peerChain(t, "export")

	outcome := judgeLeak(t, chain, policyview.Session{EBGP: true})

	assert.Equal(t, policychain.OutcomeReject, outcome.Action,
		"a path through AS3356 must not be advertised to a peer whose export chain names the list")
}

// TestSchemaRefusesAnUnknownPositionKey pins the refusal the config parse makes
// on its own: the six positions are the reject-asn list's own leaves, so a word
// outside the vocabulary is an unknown field and never reaches Go.
//
// VALIDATES: AC-18.
// PREVENTS: a mistyped position silently creating a leaf that matches nothing,
// which reads in the config file exactly like a rule that works.
func TestSchemaRefusesAnUnknownPositionKey(t *testing.T) {
	_, err := config.ParseTreeWithYANG(policyOnly(`        reject-asn NO-TRANSIT {
            upstream [ 3356 ]
        }`), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field in reject-asn: upstream",
		"the refusal must name the leaf the operator wrote")
}

// TestPositionBoundsFireWhenTheTreeIsWalked pins the YANG bound the schema
// carries beyond the leaf types: the 512-character cap on a regex pattern
// (AC-47).
//
// It is reported by the module walk, and NOT by the parse. The bgp section is
// deliberately outside the validatedSections list that ValidateCustomSections
// walks (internal/component/config/validate_sections.go), so no daemon path runs
// this walk over bgp today. The bound is a declaration this test proves live, and
// the refusal an operator meets is owed by the plugin's own config parse.
//
// VALIDATES: the regex leaf-list carries a 512-character bound, and it fires.
// PREVENTS: a bound written into the schema, believed to be enforced, and
// enforced by nothing.
func TestPositionBoundsFireWhenTheTreeIsWalked(t *testing.T) {
	cases := []struct {
		name string
		list string
		want string
	}{{
		name: "pattern_over_512_characters",
		list: `        reject-asn NO-TRANSIT {
            regex [ "` + strings.Repeat("3", 513) + `" ]
        }`,
		want: "length",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := config.ParseTreeWithYANG(policyOnly(tc.list), nil)
			require.NoError(t, err, "the parse does not carry these bounds; the walk does")

			validator, err := config.YANGValidatorWithPlugins(nil)
			require.NoError(t, err)

			container := tree.GetContainer(policy.RootName)
			require.NotNil(t, container)

			var messages []string
			for _, ve := range validator.ValidateTreeAllModules(policy.RootName, container.ToMap()) {
				messages = append(messages, ve.Path+": "+ve.Message)
			}
			require.NotEmpty(t, messages, "the bound must fire over the walked tree")
			assert.Contains(t, strings.Join(messages, "\n"), tc.want)
		})
	}
}

// policyOnly wraps one policy body in a policy root, beside the smallest bgp
// block that parses.
func policyOnly(list string) string {
	return "bgp {\n    router-id 1.2.3.4;\n    session { asn { local 65000; } }\n}\npolicy {\n" + list + "\n}\n"
}

// configure runs one Stage 2 delivery through a fresh config transaction, as
// the plugin's Register binds it.
func configure(sections []sdk.ConfigSection) error {
	return newRejectASNConfigTx().Configure(sections)
}
