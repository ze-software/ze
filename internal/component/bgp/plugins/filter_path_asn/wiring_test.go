// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
package filter_path_asn

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/hub/yang"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// canonicalRef is the chain entry a bare NO-TRANSIT resolves to once the
// registration declares the reject-asn filter type. A chain that resolves to
// anything else never reaches this plugin.
const canonicalRef = "bgp-filter-path-asn:NO-TRANSIT"

// deliveredName is what the plugin sees in FilterUpdateInput.Filter.
// PolicyFilterChain (internal/component/bgp/reactor/filter_chain.go) cuts the
// canonical ref at the colon and passes the plugin name and the filter name as
// separate arguments, so the bare list name is what arrives over the RPC.
const deliveredName = "NO-TRANSIT"

// wiringConfig is one peer whose named chain carries the list, written the way
// an operator writes it: the bare list name, JunOS style.
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
    policy {
        reject-asn NO-TRANSIT {
            indirect [ 3356 ]
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
            ` + chain + ` [ NO-TRANSIT ]
        }
    }
}
`
}

// deliverConfig parses the config text, checks that the peer's chain resolves
// to this plugin, and hands the BGP subtree to the plugin the way the engine
// hands it over: the subtree config.ExtractConfigSubtree cuts, marshaled
// to JSON. It answers the bare list name, which is what the engine puts in
// FilterUpdateInput.Filter after PolicyFilterChain has cut the plugin prefix off
// the canonical ref.
func deliverConfig(t *testing.T, chain string) string {
	t.Helper()

	tree, err := config.ParseTreeWithYANG(wiringConfig(chain), nil)
	require.NoError(t, err, "the reject-asn list and its position leaf must parse against the registered YANG")

	peers, err := bgpconfig.PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 1)

	got := peers[0].ImportFilters
	if chain == "export" {
		got = peers[0].ExportFilters
	}
	require.Equal(t, []filterapi.FilterRef{{Name: canonicalRef}}, got,
		"the bare list name must canonicalize to this plugin: that resolution is the whole reachability chain")

	subtree := config.ExtractConfigSubtree(tree.ToMap(), "bgp")
	require.NotNil(t, subtree)
	data, err := json.Marshal(subtree)
	require.NoError(t, err)

	require.NoError(t, configure([]sdk.ConfigSection{{Root: "bgp", Data: string(data)}}))

	return deliveredName
}

// leakedUpdate renders the filter text for a route that reached peer AS65001
// through AS3356, using the producer of that format so the two cannot drift.
func leakedUpdate() string {
	path := &attribute.ASPath{Segments: []attribute.ASPathSegment{{
		Type: attribute.ASSequence,
		ASNs: []uint32{65001, 3356, 65002},
	}}}
	buf := attribute.Origin(0).AppendText(nil)
	buf = append(buf, ' ')
	buf = path.AppendText(buf)
	return string(buf)
}

// TestPathASNFilterReachedFromPeerImportChain drives the import half of the
// wiring: an operator's bare list name in a peer's import chain reaches this
// plugin's handler, and the handler rejects a path that runs through a listed
// ASN.
//
// VALIDATES: the Wiring Test row for the import chain.
// PREVENTS: a registered filter type that no peer chain can reach, which passes
// every unit test of the matcher and filters nothing.
func TestPathASNFilterReachedFromPeerImportChain(t *testing.T) {
	filter := deliverConfig(t, "import")

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter:    filter,
		Direction: "import",
		Peer:      "10.0.0.1",
		PeerAS:    65001,
		Update:    leakedUpdate(),
	})

	assert.Equal(t, sdk.FilterReject, out.Action,
		"AS3356 is transit in this path, so the route the peer leaked must be rejected")
}

// TestPathASNFilterReachedFromPeerExportChain drives the export half. The
// engine supplies the destination peer here, so no ASN is the neighbor and via
// covers the whole path.
//
// VALIDATES: the Wiring Test row for the export chain.
// PREVENTS: a list that attaches to import only, leaving RFC 7454 Section 9's
// export recommendation with no producer.
func TestPathASNFilterReachedFromPeerExportChain(t *testing.T) {
	filter := deliverConfig(t, "export")

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter:    filter,
		Direction: "export",
		Peer:      "10.0.0.1",
		PeerAS:    65001,
		Update:    leakedUpdate(),
	})

	assert.Equal(t, sdk.FilterReject, out.Action,
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

			container := tree.GetContainer("bgp")
			require.NotNil(t, container)

			var messages []string
			for _, ve := range validator.ValidateTreeAllModules("bgp", container.ToMap()) {
				messages = append(messages, ve.Path+": "+ve.Message)
			}
			require.NotEmpty(t, messages, "the bound must fire over the walked tree")
			assert.Contains(t, strings.Join(messages, "\n"), tc.want)
		})
	}
}

// policyOnly wraps one policy body in the smallest bgp block that parses.
func policyOnly(list string) string {
	return "bgp {\n    router-id 1.2.3.4;\n    policy {\n" + list + "\n    }\n}\n"
}
