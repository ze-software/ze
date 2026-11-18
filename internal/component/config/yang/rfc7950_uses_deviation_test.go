// Design: docs/architecture/config/yang-config-design.md -- schema loading
// Related: loader.go -- Loader.Resolve is the production schema boundary
// Upstream: github.com/openconfig/goyang v1.6.4-0.20251113201521-a80f2799d7b9
// https://github.com/openconfig/goyang -- Uses and Entry.ApplyDeviate

package yang

import (
	"fmt"
	"math"
	"testing"

	goyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoaderUsesAppliesAnAugment proves that a uses-local augment reaches the
// resolved schema. Method: load one grouping use and inspect its added leaf.
// RFC 7950 Section 7.13: "The effect of a "uses" reference to a grouping is
// that the nodes defined by the grouping are copied into the current schema
// tree and are then updated according to the "refine" and "augment" statements."
func TestLoaderUsesAppliesAnAugment(t *testing.T) {
	resolved, err := resolveUsesDeviationTexts(t, `module uses-one {
    yang-version 1.1; namespace "urn:ze:test:uses-one"; prefix u;
    grouping g { container box { leaf original { type string; } } }
    container first { uses g { augment box { leaf added { type uint8; } } } }
}`)
	require.NoError(t, err)
	box := resolved.GetEntry("uses-one").Dir["first"].Dir["box"]
	require.NotNil(t, box)
	require.NotNil(t, box.Dir["original"])
	require.NotNil(t, box.Dir["added"], "uses-local augment was dropped")
	assert.Equal(t, goyang.Yuint8, box.Dir["added"].Type.Kind)
}

// TestLoaderUsesAppliesMultipleAugments proves that each augment on one uses
// contributes its leaf. Method: load two augments of the same grouping node.
// RFC 7950 Section 7.13.1 gives uses's augment substatement cardinality 0..n.
func TestLoaderUsesAppliesMultipleAugments(t *testing.T) {
	resolved, err := resolveUsesDeviationTexts(t, `module uses-many {
    yang-version 1.1; namespace "urn:ze:test:uses-many"; prefix u;
    grouping g { container box { leaf original { type string; } } }
    container first { uses g {
        augment box { leaf alpha { type uint8; } }
        augment box { leaf beta { type uint16; } }
    } }
}`)
	require.NoError(t, err, "uses permits multiple augment statements")
	box := resolved.GetEntry("uses-many").Dir["first"].Dir["box"]
	require.NotNil(t, box)
	assert.Contains(t, box.Dir, "original")
	assert.Contains(t, box.Dir, "alpha")
	assert.Contains(t, box.Dir, "beta")
}

// TestLoaderUsesAugmentCopiesAreIndependent proves that an augment affects
// its own use only. Method: compare two differently augmented copies with an
// unaugmented copy of the same grouping after the production loader resolves.
// RFC 7950 Sections 7.13 and 7.17 bind each augment to its target node.
func TestLoaderUsesAugmentCopiesAreIndependent(t *testing.T) {
	resolved, err := resolveUsesDeviationTexts(t, `module uses-copies {
    yang-version 1.1; namespace "urn:ze:test:uses-copies"; prefix u;
    grouping g { container box { leaf original { type string; } } }
    container first { uses g { augment box { leaf alpha { type string; } } } }
    container second { uses g { augment box { leaf beta { type string; } } } }
    container plain { uses g; }
}`)
	require.NoError(t, err)
	entry := resolved.GetEntry("uses-copies")
	for _, name := range []string{"first", "second", "plain"} {
		t.Run(name, func(t *testing.T) {
			box := entry.Dir[name].Dir["box"]
			require.NotNil(t, box)
			assert.Contains(t, box.Dir, "original")
			switch name {
			case "first":
				assert.Contains(t, box.Dir, "alpha")
				assert.NotContains(t, box.Dir, "beta")
			case "second":
				assert.Contains(t, box.Dir, "beta")
				assert.NotContains(t, box.Dir, "alpha")
			case "plain":
				assert.Len(t, box.Dir, 1, "an augment leaked into the plain grouping use")
			}
		})
	}
}

// TestLoaderUsesWithoutAugmentPreservesGrouping proves that a use with no
// augment remains valid. Method: inspect the copy's original leaf and size.
// RFC 7950 Section 7.13.1 permits zero augment statements under uses.
func TestLoaderUsesWithoutAugmentPreservesGrouping(t *testing.T) {
	resolved, err := resolveUsesDeviationTexts(t, `module uses-empty {
    yang-version 1.1; namespace "urn:ze:test:uses-empty"; prefix u;
    grouping g { container box { leaf original { type string; } } }
    container plain { uses g; }
}`)
	require.NoError(t, err)
	box := resolved.GetEntry("uses-empty").Dir["plain"].Dir["box"]
	require.NotNil(t, box)
	assert.Len(t, box.Dir, 1)
	assert.Contains(t, box.Dir, "original")
}

// TestLoaderUsesRefusesEmptyAugment proves that an augment must declare a
// child. Method: load an empty uses-local augment through the schema loader.
// RFC 7950 Section 14 requires at least one data definition, case, action or
// notification in uses-augment-stmt.
func TestLoaderUsesRefusesEmptyAugment(t *testing.T) {
	_, err := resolveUsesDeviationTexts(t, `module uses-empty-augment {
    yang-version 1.1; namespace "urn:ze:test:uses-empty-augment"; prefix u;
    grouping g { container box { leaf original { type string; } } }
    container copy { uses g { augment box {} } }
}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "augment")
}

// TestLoaderUsesAugmentBindsNamespaces proves that an imported grouping's
// copy and a uses-local addition take the using module's namespace. Method:
// inspect the entries the loader publishes, rather than the defining AST.
// RFC 7950 Sections 7.13 and 7.17.2 define these two namespace bindings.
func TestLoaderUsesAugmentBindsNamespaces(t *testing.T) {
	resolved, err := resolveUsesDeviationTexts(t,
		`module uses-library { yang-version 1.1; namespace "urn:ze:test:uses-library"; prefix l;
    grouping g { container box { leaf original { type string; } } }
}`,
		`module uses-import { yang-version 1.1; namespace "urn:ze:test:uses-import"; prefix u;
    import uses-library { prefix lib; }
    container copy { uses lib:g { augment box { leaf added { type string; } } } }
}`)
	require.NoError(t, err)
	box := resolved.GetEntry("uses-import").Dir["copy"].Dir["box"]
	require.NotNil(t, box)
	for _, name := range []string{"original", "added"} {
		leaf := box.Dir[name]
		require.NotNil(t, leaf, name)
		assert.Equal(t, "urn:ze:test:uses-import", leaf.Namespace().Name, name)
	}
}

// TestLoaderUsesAugmentRefusesMissingTarget proves an unresolvable local
// augment cannot disappear. Method: resolve a use whose augment names no node.
// RFC 7950 Section 7.17 requires an augment's target to be a schema node.
func TestLoaderUsesAugmentRefusesMissingTarget(t *testing.T) {
	resolved, err := resolveUsesDeviationTexts(t, `module uses-missing {
    yang-version 1.1; namespace "urn:ze:test:uses-missing"; prefix u;
    grouping g { container box { leaf original { type string; } } }
    container copy { uses g { augment absent { leaf added { type string; } } } }
}`)
	require.Error(t, err)
	assert.Nil(t, resolved)
	assert.Contains(t, err.Error(), "absent")
}

// deviationSingletonCase describes one explicitly written singleton property.
// Empty values and zero bounds are statements too, rather than absence guards.
type deviationSingletonCase struct {
	name     string
	node     string
	property string
	original string
	added    string
}

// deviationSingletonCases covers each singleton that deviate add permits,
// including explicit false, empty string, zero and unbounded statements.
func deviationSingletonCases() []deviationSingletonCase {
	return []deviationSingletonCase{
		{"config true", "leaf", "config", "true", "false"},
		{"config false", "leaf", "config", "false", "false"},
		{"mandatory true", "leaf", "mandatory", "true", "false"},
		{"mandatory false", "leaf", "mandatory", "false", "false"},
		{"units", "leaf", "units", `"packets"`, `"octets"`},
		{"units empty", "leaf", "units", `""`, `"octets"`},
		{"default", "leaf", "default", `"old"`, `"new"`},
		{"default empty", "leaf", "default", `""`, `"new"`},
		{"min-elements", "leaf-list", "min-elements", "2", "1"},
		{"min-elements zero", "leaf-list", "min-elements", "0", "1"},
		{"max-elements", "leaf-list", "max-elements", "8", "7"},
		{"max-elements unbounded", "leaf-list", "max-elements", "unbounded", "7"},
	}
}

// TestLoaderDeviateAddExistingSingletonProperty proves that add cannot
// overwrite a singleton already on its target. Method: load each explicit
// property and inspect the production loader's refusal.
// RFC requirement: RFC7950-7.20.3.2-1 negative -- Loader.Resolve refuses deviate add of an explicitly present config, mandatory, units, leaf default, min-elements or max-elements property, including false, empty, zero and unbounded values.
func TestLoaderDeviateAddExistingSingletonProperty(t *testing.T) {
	for _, c := range deviationSingletonCases() {
		t.Run(c.name, func(t *testing.T) {
			text := fmt.Sprintf(`module singleton-existing {
    yang-version 1.1; namespace "urn:ze:test:singleton-existing"; prefix d;
    %s target { type string; %s %s; }
    deviation "/d:target" { deviate add { %s %s; } }
}`, c.node, c.property, c.original, c.property, c.added)
			resolved, err := resolveUsesDeviationTexts(t, text)
			require.Error(t, err, "deviate add overwrote explicit %s %s", c.property, c.original)
			assert.Nil(t, resolved)
			assert.Contains(t, err.Error(), c.property)
		})
	}
}

// TestLoaderDeviateAddMissingSingletonProperty proves that add still adds
// each missing property. Method: load its opposite case and read the result.
// RFC requirement: RFC7950-7.20.3.2-1 positive -- Loader.Resolve accepts deviate add of a missing config, mandatory, units, leaf default, min-elements or max-elements property and publishes its specified value.
func TestLoaderDeviateAddMissingSingletonProperty(t *testing.T) {
	for _, c := range deviationSingletonCases() {
		t.Run(c.name, func(t *testing.T) {
			text := fmt.Sprintf(`module singleton-missing {
    yang-version 1.1; namespace "urn:ze:test:singleton-missing"; prefix d;
    %s target { type string; }
    deviation "/d:target" { deviate add { %s %s; } }
}`, c.node, c.property, c.added)
			resolved, err := resolveUsesDeviationTexts(t, text)
			require.NoError(t, err)
			assertDeviationSingletonValue(t, resolved.GetEntry("singleton-missing").Dir["target"], c.property)
		})
	}
}

// TestLoaderDeviateReplaceExistingSingletonProperty proves that replacing
// an existing singleton stays legal. Method: reuse the refused-add inputs
// with replace and inspect the effective value.
// RFC 7950 Section 7.20.3.2 permits replace when the property exists.
func TestLoaderDeviateReplaceExistingSingletonProperty(t *testing.T) {
	for _, c := range deviationSingletonCases() {
		t.Run(c.name, func(t *testing.T) {
			text := fmt.Sprintf(`module singleton-replace {
    yang-version 1.1; namespace "urn:ze:test:singleton-replace"; prefix d;
    %s target { type string; %s %s; }
    deviation "/d:target" { deviate replace { %s %s; } }
}`, c.node, c.property, c.original, c.property, c.added)
			resolved, err := resolveUsesDeviationTexts(t, text)
			require.NoError(t, err)
			assertDeviationSingletonValue(t, resolved.GetEntry("singleton-replace").Dir["target"], c.property)
		})
	}
}

// TestLoaderDeviateAddAllowsInheritedConfig proves that inherited config
// does not count as an explicit singleton on a child. Method: add config false
// to a child that inherits false from its container and read its own property.
// RFC 7950 Sections 7.20.3.2 and 7.21.1 distinguish these statements.
func TestLoaderDeviateAddAllowsInheritedConfig(t *testing.T) {
	resolved, err := resolveUsesDeviationTexts(t, `module singleton-inherited {
    yang-version 1.1; namespace "urn:ze:test:singleton-inherited"; prefix d;
    container parent { config false; leaf target { type string; } }
    deviation "/d:parent/d:target" { deviate add { config false; } }
}`)
	require.NoError(t, err)
	assert.Equal(t, goyang.TSFalse, resolved.GetEntry("singleton-inherited").Dir["parent"].Dir["target"].Config)
}

// TestLoaderDeviateAddZeroBounds proves that absent list bounds can be added
// at their default values. Method: add zero and unbounded to one leaf-list.
// RFC 7950 Section 7.20.3.2 tests presence, not whether a value is nonzero.
func TestLoaderDeviateAddZeroBounds(t *testing.T) {
	resolved, err := resolveUsesDeviationTexts(t, `module singleton-bounds {
    yang-version 1.1; namespace "urn:ze:test:singleton-bounds"; prefix d;
    leaf-list target { type string; }
    deviation "/d:target" { deviate add { min-elements 0; max-elements unbounded; } }
}`)
	require.NoError(t, err)
	entry := resolved.GetEntry("singleton-bounds").Dir["target"]
	require.NotNil(t, entry.ListAttr)
	assert.Zero(t, entry.ListAttr.MinElements)
	assert.Equal(t, uint64(math.MaxUint64), entry.ListAttr.MaxElements)
}

// assertDeviationSingletonValue reads the value shared by the add and replace
// controls. Each input specifies false, octets, new, one, or seven respectively.
func assertDeviationSingletonValue(t *testing.T, entry *goyang.Entry, property string) {
	t.Helper()
	require.NotNil(t, entry)
	switch property {
	case "config":
		assert.Equal(t, goyang.TSFalse, entry.Config)
	case "mandatory":
		assert.Equal(t, goyang.TSFalse, entry.Mandatory)
	case "units":
		assert.Equal(t, "octets", entry.Units)
	case "default":
		assert.Equal(t, []string{"new"}, entry.Default)
	case "min-elements":
		require.NotNil(t, entry.ListAttr)
		assert.Equal(t, uint64(1), entry.ListAttr.MinElements)
	case "max-elements":
		require.NotNil(t, entry.ListAttr)
		assert.Equal(t, uint64(7), entry.ListAttr.MaxElements)
	default:
		t.Fatalf("no singleton assertion for %q", property)
	}
}

// resolveUsesDeviationTexts returns the checked module set, or its production
// loader error. Callers inspect entries only after a successful Resolve.
func resolveUsesDeviationTexts(t *testing.T, texts ...string) (*Resolved, error) {
	t.Helper()
	loader := NewLoader()
	for i, text := range texts {
		if err := loader.AddModuleFromText(moduleFileName(text, i), text); err != nil {
			return nil, err
		}
	}
	// RFC 7950 Sections 7.13, 7.17 and 7.20.3.2.
	return loader.Resolve()
}
