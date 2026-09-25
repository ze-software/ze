// Design: docs/architecture/config/yang-config-design.md -- the model is the schema
//
// Related: blackhole.go -- the blackhole-propagation tokens held here
// Related: relation.go -- the RFC 9234 role tokens held here
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

package filter_community

import (
	"slices"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	fcyang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community/yang"
	roleyang "github.com/ze-software/ze/internal/component/bgp/plugins/role/yang"
	bgpyang "github.com/ze-software/ze/internal/component/bgp/yang"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	hubyang "github.com/ze-software/ze/internal/component/hub/yang"
)

// testModules names the modules this file's model is built from: the closure of
// what the leaves under test need.
func testModules() map[string]string {
	return map[string]string{
		// ze-bgp-conf imports ze-hub-conf, so the closure carries both.
		"ze-hub-conf.yang":         hubyang.ZeHubConfYANG,
		"ze-bgp-conf.yang":         bgpyang.ZeBGPConfYANG,
		"ze-filter-community.yang": fcyang.ZeFilterCommunityYANG,
		// The role module is named for its SCHEMA alone. relation.go spells the
		// role tokens itself rather than importing the role plugin, which is what
		// keeps one plugin from being a compile-time dependency of another
		// (ai/rules/plugins.md), and this file is where that spelling is proved.
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

// TestBlackholeGuardTokensMatchTheYANGModel proves the guard tokens spell the
// `blackhole-propagation` enumeration and nothing else.
//
// PREVENTS: a value added to the model that blackholeGuardCommunity does not
// recognize, which leaves the guard OFF for a peer the operator armed.
func TestBlackholeGuardTokensMatchTheYANGModel(t *testing.T) {
	declared := yangEnumValues(t, "ze-bgp-conf", "bgp", "filter", "ingress", "community", "blackhole-propagation")

	named := []string{blackholeGuardNone, blackholeGuardNoExport, blackholeGuardNoAdvertise}
	slices.Sort(named)
	require.Equal(t, declared, named)

	// The two live tokens each name a community, and the off token names none.
	for _, token := range []string{blackholeGuardNoExport, blackholeGuardNoAdvertise} {
		_, on := blackholeGuardCommunity(token)
		require.True(t, on, "the model declares guard %q and blackholeGuardCommunity ignores it", token)
	}
	_, on := blackholeGuardCommunity(blackholeGuardNone)
	require.False(t, on)
}

// TestRelationRoleTokensMatchTheYANGModel proves the role tokens this package
// spells are the values of the `role/import` enumeration.
//
// PREVENTS: the drift the agreed-spelling pattern risks. relation.go MUST NOT
// import the role plugin, so nothing but this test holds its five words to the
// model both packages read.
func TestRelationRoleTokensMatchTheYANGModel(t *testing.T) {
	declared := yangEnumValues(t, "ze-bgp-conf", "bgp", "group", "peer", "role", "import")

	named := []string{roleProvider, roleCustomer, rolePeer, roleRS, roleRSClient}
	slices.Sort(named)

	require.Equal(t, declared, named)
}
