// Design: docs/architecture/config/yang-config-design.md -- the model is the schema
//
// Related: peer_settings.go -- prefixReconnectNames, one of the tables held here
// Related: peer_forward_facts.go -- sendCommunitySuppression, another of them
// Related: config_capabilities.go -- parseAddPathDirection, another of them
//
// yang_enum_agreement_test.go holds this package's Go name tables to the YANG enumerations they
// spell.
//
// `./le enumeration report` names each table: the Go side and the model each
// declare the same set of words, and the gate cannot pick which one the other
// should derive from. It cannot be derived here, because each Go table carries
// what the model does not -- an RFC-assigned number, a typed value, a handler.
// So the model stays the schema, the Go table stays the mapping, and what the
// two owe each other is AGREEMENT. That is what these tests read, from the
// loaded model rather than from a copy of the values.

package reactor

import (
	"slices"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/require"

	bgpyang "github.com/ze-software/ze/internal/component/bgp/yang"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	hubyang "github.com/ze-software/ze/internal/component/hub/yang"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// testModules names the modules this file's model is built from: the closure of
// what the leaves under test need.
func testModules() map[string]string {
	return map[string]string{
		// ze-bgp-conf imports ze-hub-conf, so the closure carries both.
		"ze-hub-conf.yang": hubyang.ZeHubConfYANG,
		"ze-bgp-conf.yang": bgpyang.ZeBGPConfYANG,
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

// TestPrefixReconnectNamesMatchTheYANGModel proves prefixReconnectNames spells
// the `reconnect` enumeration and nothing else.
//
// PREVENTS: a value added to the model that parsePrefixReconnectMode refuses,
// which passes config validation and then fails the peer build; and a name
// changed on one side, which an operator writes in the config and never sees in
// the log line.
func TestPrefixReconnectNamesMatchTheYANGModel(t *testing.T) {
	declared := yangEnumValues(t, "ze-bgp-conf", "bgp", "group", "peer", "session", "family", "prefix", "reconnect")

	// Index 0 is PrefixReconnectUnset, whose name says the family stated no
	// value at all. The model has no value for that, so the comparison starts
	// at the first mode a config can state.
	var parsed []string
	for mode := PrefixReconnectNever; int(mode) < len(prefixReconnectNames); mode++ {
		parsed = append(parsed, prefixReconnectNames[mode])
	}
	slices.Sort(parsed)

	require.Equal(t, declared, parsed)

	// Every declared value parses back to the mode that names it, so the table
	// is read in both directions rather than merely compared as a set.
	for _, value := range declared {
		mode, ok := parsePrefixReconnectMode(value)
		require.True(t, ok, "the model declares %q and parsePrefixReconnectMode refuses it", value)
		require.Equal(t, value, mode.String())
	}

	// The word for "no value stated" is not one a config may state.
	_, ok := parsePrefixReconnectMode(prefixReconnectNames[PrefixReconnectUnset])
	require.False(t, ok, "parsePrefixReconnectMode accepts the unset word, which the model does not declare")
}

// TestSendCommunityKeywordsMatchTheYANGModel proves sendCommunitySuppression
// reads every value of the `community/send` enumeration.
//
// PREVENTS: a keyword the model accepts and the forward path ignores, which
// silently forwards a community type the operator asked to suppress.
func TestSendCommunityKeywordsMatchTheYANGModel(t *testing.T) {
	declared := yangEnumValues(t, "ze-bgp-conf", "bgp", "group", "peer", "session", "community", "send")

	// The mask a word the reader does not know leaves. Nothing was named, so
	// every type is suppressed: the reader fails closed rather than forwarding
	// a type on the strength of a config typo.
	closed := sendCommunitySuppression([]string{"sideways"})

	for _, value := range declared {
		// "none" asks for exactly what failing closed produces, so it is the
		// one declared value this probe cannot tell from an unknown word. It is
		// asserted the other way round instead.
		if value == "none" {
			require.Equal(t, closed, sendCommunitySuppression([]string{value}))
			continue
		}
		require.NotEqual(t, closed, sendCommunitySuppression([]string{value}),
			"the model declares %q and sendCommunitySuppression ignores it", value)
	}

	// This test catches a value ADDED to the model, RENAMED in it, or REMOVED
	// from the Go reader. It cannot catch a keyword the Go reader knows and the
	// model never declared, because a switch cannot be enumerated from outside
	// it; config validation refuses such a word before it reaches here.
}

// TestAddPathDirectionsMatchTheYANGModel proves parseAddPathDirection resolves
// every value of the `add-path/direction` enumeration to a real mode.
//
// PREVENTS: a direction the model accepts resolving to AddPathNone, which
// advertises no ADD-PATH for a family the operator enabled.
func TestAddPathDirectionsMatchTheYANGModel(t *testing.T) {
	declared := yangEnumValues(t, "ze-bgp-conf", "bgp", "group", "peer", "session", "capability", "add-path", "direction")

	for _, value := range declared {
		require.NotEqual(t, capability.AddPathNone, parseAddPathDirection(value),
			"the model declares direction %q and parseAddPathDirection resolves it to no mode", value)
	}

	// A word the model does not declare resolves to no mode, so the reader
	// fails closed rather than guessing a direction from a typo.
	require.Equal(t, capability.AddPathNone, parseAddPathDirection("sideways"))
}

// TestAddPathModeKeywordsMatchTheYANGModel proves the config token constants
// cover every value of the `add-path/family/mode` enumeration.
//
// PREVENTS: a mode the model accepts that parsePeerFromTree compares against no
// constant, so the family silently takes the default instead.
func TestAddPathModeKeywordsMatchTheYANGModel(t *testing.T) {
	declared := yangEnumValues(t, "ze-bgp-conf", "bgp", "group", "peer", "session", "capability", "add-path", "family", "mode")

	known := []string{valEnable, valDisable, valRequire, valRefuse}
	slices.Sort(known)
	require.Equal(t, declared, known)
}
