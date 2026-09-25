// Design: docs/architecture/config/yang-config-design.md -- the model is the schema
//
// Related: raw.go -- parseMessageType and msgTypeName, both derived from msgtype
// Related: internal/core/bgp/msgtype/msgtype.go -- typeNames, the table held
//	here because internal/core MUST NOT import a component package
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

package raw

import (
	"slices"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	rawyang "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/raw/yang"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// testModules names the modules this file's model is built from: the closure of
// what the leaves under test need.
func testModules() map[string]string {
	return map[string]string{
		"ze-raw-cmd.yang": rawyang.ZeRawCmdYANG,
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

// TestMessageTypeNamesMatchTheYANGModel proves the msgtype name table spells the
// `raw/type` enumeration and nothing else.
//
// The test lives here rather than beside the table: internal/core MUST NOT
// import a component package, and the YANG loader is one.
//
// PREVENTS: a message type added to the model that parseMessageType refuses, so
// the CLI grammar offers a word `send bgp raw` then rejects.
func TestMessageTypeNamesMatchTheYANGModel(t *testing.T) {
	declared := yangEnumValues(t, "ze-raw-cmd", "send", "bgp", "raw", "type")

	named := msgtype.TextNames()
	slices.Sort(named)
	require.Equal(t, declared, named)

	// Every declared value parses back to the type that names it, through the
	// two functions this package reaches the table with.
	for _, value := range declared {
		code, ok := parseMessageType(value)
		require.True(t, ok, "the model declares type %q and parseMessageType refuses it", value)
		require.Equal(t, value, msgTypeName(code))
	}
}
