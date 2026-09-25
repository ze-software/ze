// Design: docs/architecture/config/yang-config-design.md -- the model is the schema
//
// Related: peers.go -- leakFilterByRole, the table held here
// Related: internal/core/bgp/attribute/origin.go -- originTextNames, held here
//	because internal/core MUST NOT import a component package
// Related: internal/core/bgp/asn/asn.go -- the notation tokens, held here for
//	the same reason
//
// yang_enum_agreement_test.go holds this package's Go name tables to the YANG enumerations they
// spell.
//
// `./le arch enumeration report` names each table: the Go side and the model each
// declare the same set of words, and the gate cannot pick which one the other
// should derive from. It cannot be derived here, because each Go table carries
// what the model does not -- an RFC-assigned number, a typed value, a handler.
// So the model stays the schema, the Go table stays the mapping, and what the
// two owe each other is AGREEMENT. That is what these tests read, from the
// loaded model rather than from a copy of the values.

package bgpconfig

import (
	"slices"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	roleyang "github.com/ze-software/ze/internal/component/bgp/plugins/role/yang"
	bgpyang "github.com/ze-software/ze/internal/component/bgp/yang"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	hubyang "github.com/ze-software/ze/internal/component/hub/yang"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// testModules names the modules this file's model is built from: the closure of
// what the leaves under test need.
func testModules() map[string]string {
	return map[string]string{
		// ze-bgp-conf imports ze-hub-conf, so the closure carries both.
		"ze-hub-conf.yang": hubyang.ZeHubConfYANG,
		"ze-bgp-conf.yang": bgpyang.ZeBGPConfYANG,
		// The role plugin augments the `role` container in.
		"ze-role.yang": roleyang.ZeRoleYANG,
	}
}

// yangModel builds a model from the embedded library modules plus the modules
// testModules names, and REFUSES one that does not resolve.
//
// The modules are NAMED rather than read from the registry yang.DefaultLoader
// reads. A test binary registers whichever modules its own imports happen to
// pull in, goyang refuses the WHOLE set when one of them imports a module
// nobody loaded, and DefaultLoader discards that error by design. The augment
// under test then never lands, and a test reading the model would compare a Go
// table against an absent leaf and pass. Naming the closure makes the refusal
// exact.
func yangModel(t *testing.T) *configyang.Loader {
	t.Helper()

	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	for name, content := range testModules() {
		require.NoError(t, loader.AddModuleFromText(name, content))
	}
	require.NoError(t, loader.Resolve())
	return loader
}

// yangEnumValues answers the values of the enumeration the leaf at path
// declares, sorted. module is the module name without its `.yang` suffix.
//
// It FAILS rather than answering an empty set for a leaf it cannot reach: a
// test comparing a Go table against nothing passes over every drift there is.
func yangEnumValues(t *testing.T, module string, path ...string) []string {
	t.Helper()

	entry := yangModel(t).GetEntry(module)
	require.NotNil(t, entry, "YANG module %s is not loaded", module)
	for _, step := range path {
		entry = entry.Dir[step]
		require.NotNil(t, entry, "YANG module %s declares no %s under %v", module, step, path)
	}
	require.NotNil(t, entry.Type, "YANG leaf %s/%v has no type", module, path)
	require.Equal(t, gyang.Yenum, entry.Type.Kind, "YANG leaf %s/%v is not an enumeration", module, path)
	require.NotNil(t, entry.Type.Enum, "YANG leaf %s/%v declares no enumeration values", module, path)

	values := slices.Clone(entry.Type.Enum.Names())
	slices.Sort(values)
	return values
}

// TestLeakFilterRolesMatchTheYANGModel proves leakFilterByRole carries a row for
// every role the model declares.
//
// PREVENTS: a role added to the model that obliges no leak filter, so a peer
// configured with it silently gets no import or export chain at all.
func TestLeakFilterRolesMatchTheYANGModel(t *testing.T) {
	declared := yangEnumValues(t, "ze-bgp-conf", "bgp", "group", "peer", "role", "import")

	keyed := make([]string, 0, len(leakFilterByRole))
	for role := range leakFilterByRole {
		keyed = append(keyed, role)
	}
	slices.Sort(keyed)

	require.Equal(t, declared, keyed)
}

// TestOriginTextNamesMatchTheYANGModel proves the attribute package's origin
// names are the values of the `origin` enumeration.
//
// The test lives here rather than beside the table: internal/core MUST NOT
// import a component package, and the YANG loader is one
// (`./le arch tier check` reads a test file too).
//
// PREVENTS: an origin value added to the model that OriginFromText refuses, so
// a config the schema accepts fails at the point the UPDATE is built.
func TestOriginTextNamesMatchTheYANGModel(t *testing.T) {
	declared := yangEnumValues(t, "ze-bgp-conf", "bgp", "group", "peer", "update", "attribute", "origin")

	named := attribute.OriginTextNames()
	slices.Sort(named)
	require.Equal(t, declared, named)

	// Every declared value parses back to the origin that names it, so the
	// table is read in both directions rather than compared as a set.
	for _, value := range declared {
		origin, ok := attribute.OriginFromText(value)
		require.True(t, ok, "the model declares origin %q and OriginFromText refuses it", value)
		require.Equal(t, value, origin.LowerString())
	}
}

// TestASNotationTokensMatchTheYANGModel proves the asn package's notation
// tokens are the values of the `as-notation` enumeration.
//
// The test lives here for the reason the origin test gives.
//
// PREVENTS: a notation added to the model that Notation.String never answers,
// so `show bgp` prints AS numbers in a scheme the operator did not ask for.
func TestASNotationTokensMatchTheYANGModel(t *testing.T) {
	declared := yangEnumValues(t, "ze-bgp-conf", "bgp", "as-notation")

	named := []string{
		asn.NotationPlain.String(),
		asn.NotationDot.String(),
		asn.NotationDotPlus.String(),
	}
	slices.Sort(named)

	require.Equal(t, declared, named)
}
