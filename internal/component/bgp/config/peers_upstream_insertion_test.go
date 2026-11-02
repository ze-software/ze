// Design: docs/guide/bgp-role.md -- the upstream check a role inserts
package bgpconfig

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// upstreamCheckRef is the chain reference bgp-role inserts
// (role/upstream.go upstreamReference).
const upstreamCheckRef = "bgp-role:upstream-asn"

// TestUpstreamCheckInsertedPerRole loads one peer for each RFC 9234 local
// role through the real pipeline (the role plugin's registration, linked by
// plugin/all in loader_test.go) and reads which of the peer's lists carry the
// inserted upstream check.
//
// VALIDATES: spec-filter-wire-6-stage-filters D-16: upstreamImportRoles and
// upstreamExportRoles place bgp-role:upstream-asn on the side RFC 7454
// Section 9 and RFC 9234 Section 3.1 name. Import: provider (the session to a
// customer), peer and rs (a route a peer or an RS-client sends through Ze's
// own upstream was learned from neither). Export: peer and rs-client (the
// peering sessions, RFC 7454 Section 9 "SHOULD NOT advertise prefixes with
// upstream AS numbers in the AS path to their peering AS"). The local role
// customer (the session to a provider) carries neither.
// PREVENTS: the check landing on the wrong side, or a role's session missing
// it, with no other test going red.
func TestUpstreamCheckInsertedPerRole(t *testing.T) {
	cases := []struct {
		role     string
		onImport bool
		onExport bool
	}{
		{role: "provider", onImport: true, onExport: false},
		{role: "customer", onImport: false, onExport: false},
		{role: "peer", onImport: true, onExport: true},
		{role: "rs", onImport: true, onExport: false},
		{role: "rs-client", onImport: false, onExport: true},
	}

	var text strings.Builder
	text.WriteString("bgp {\n    router-id 1.2.3.4;\n    session { asn { local 65000 } }\n")
	for i, c := range cases {
		text.WriteString("    peer p-" + c.role + " {\n")
		text.WriteString("        connection { remote { ip 10.0.0." + string(rune('1'+i)) + " } local { ip auto } }\n")
		text.WriteString("        session { asn { remote 6500" + string(rune('1'+i)) + " } }\n")
		text.WriteString("        role { import " + c.role + " }\n")
		text.WriteString("    }\n")
	}
	text.WriteString("}\n")

	tree, err := config.ParseTreeWithYANG(text.String(), nil)
	require.NoError(t, err, "the config must parse: this test is about the insertion, not the schema")
	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)

	importsByPeer := make(map[string][]string, len(peers))
	exportsByPeer := make(map[string][]string, len(peers))
	for _, ps := range peers {
		importsByPeer[ps.Name] = refNames(ps.ImportPolicy.Filters())
		exportsByPeer[ps.Name] = refNames(ps.ExportPolicy.Filters())
	}
	for _, c := range cases {
		name := "p-" + c.role
		imports, found := importsByPeer[name]
		require.True(t, found, "peer %s must load", name)
		require.Equal(t, c.onImport, slices.Contains(imports, upstreamCheckRef),
			"role import %s: import list %v", c.role, imports)
		require.Equal(t, c.onExport, slices.Contains(exportsByPeer[name], upstreamCheckRef),
			"role import %s: export list %v", c.role, exportsByPeer[name])
	}
}
