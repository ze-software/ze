package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// leafNodeAndBuildError loads one module, converts its leaf "a" through
// yangToNode, and returns the LeafNode it built with the schema build error
// the conversion recorded.
func leafNodeAndBuildError(t *testing.T, moduleText string) (*LeafNode, error) {
	t.Helper()
	loader := yang.NewLoader()
	require.NoError(t, loader.AddModuleFromText("m.yang", moduleText))
	schema, resolveErr := loader.Resolve()
	require.NoError(t, resolveErr)
	entry := schema.GetEntry("m")
	require.NotNil(t, entry)
	leaf := entry.Dir["a"]
	require.NotNil(t, leaf, "module must define leaf a")
	resetSchemaBuildErrors()
	node := yangToNode(leaf, "m/a")
	err := flushSchemaBuildErrors()
	leafNode, ok := node.(*LeafNode)
	require.True(t, ok, "leaf a must convert to a LeafNode, got %T", node)
	return leafNode, err
}

// rfc7950SmallTypedef is a typedef whose range is 1..10 and whose default is
// 5, the base type every case below derives from.
const rfc7950SmallTypedef = `module m { namespace "urn:m"; prefix m; typedef small { type uint8 { range "1..10"; } default 5; } `

// TestRFC7950DerivedTypeInheritsOrReplacesTheDefault converts leaves that
// derive from a typedef carrying default 5 and reads the default each leaf
// node ends up with, and the schema build error each records.
//
// RFC requirement: RFC7950-7.3.4-1 positive — a leaf of the typedef with no default of its own carries the typedef's default 5; a leaf that narrows the range to 6..10 and gives default 7 converts with no error and carries 7; a typedef that narrows the range and gives default 8 hands 8 to its leaf with no error.
func TestRFC7950DerivedTypeInheritsOrReplacesTheDefault(t *testing.T) {
	node, err := leafNodeAndBuildError(t, rfc7950SmallTypedef+`leaf a { type small; } }`)
	require.NoError(t, err)
	assert.Equal(t, "5", node.Default, "a derived type with no default of its own takes the base type's default")

	node, err = leafNodeAndBuildError(t, rfc7950SmallTypedef+`leaf a { type small { range "6..10"; } default 7; } }`)
	require.NoError(t, err, "a leaf that narrows the range and gives a new valid default is conforming")
	assert.Equal(t, "7", node.Default)

	node, err = leafNodeAndBuildError(t, rfc7950SmallTypedef+`typedef big { type small { range "6..10"; } default 8; } leaf a { type big; } }`)
	require.NoError(t, err, "a typedef that narrows the range and gives a new valid default is conforming")
	assert.Equal(t, "8", node.Default)
}

// TestRFC7950DerivedTypeMustReplaceAnInvalidatedDefault converts a leaf and
// a typedef that each narrow the base range to 6..10, which the inherited
// default 5 violates, and give no new default.
//
// RFC requirement: RFC7950-7.3.4-1 negative — a leaf, and a typedef, that narrow the range of a typedef defaulting to 5 to 6..10 and specify no new default each record a schema build error naming the default "5".
func TestRFC7950DerivedTypeMustReplaceAnInvalidatedDefault(t *testing.T) {
	_, err := leafNodeAndBuildError(t, rfc7950SmallTypedef+`leaf a { type small { range "6..10"; } } }`)
	require.Error(t, err, "a leaf whose restriction invalidates the inherited default must specify a new one")
	assert.Contains(t, err.Error(), `"5"`)

	_, err = leafNodeAndBuildError(t, rfc7950SmallTypedef+`typedef big { type small { range "6..10"; } } leaf a { type big; } }`)
	require.Error(t, err, "a typedef whose restriction invalidates the inherited default must specify a new one")
	assert.Contains(t, err.Error(), `"5"`)
}

// TestRFC7950DefaultNotAnIfFeatureEnum converts a leaf whose default names an
// enum carrying an if-feature statement, and its twin whose default names an
// enum with none.
//
// RFC requirement: RFC7950-7.6.4-2 negative — a leaf default naming enum x, which carries "if-feature f", records a schema build error naming the if-feature.
// RFC requirement: RFC7950-7.6.4-2 positive — the same leaf with its default naming enum y, which carries no if-feature, converts with no schema build error.
func TestRFC7950DefaultNotAnIfFeatureEnum(t *testing.T) {
	const head = `module m { namespace "urn:m"; prefix m; feature f; leaf a { type enumeration { enum x { if-feature f; } enum y; } `
	_, err := leafNodeAndBuildError(t, head+`default x; } }`)
	require.Error(t, err, "a default marked with an if-feature must be refused")
	assert.Contains(t, err.Error(), "if-feature")

	_, err = leafNodeAndBuildError(t, head+`default y; } }`)
	assert.NoError(t, err)
}

// TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef reaches the definition
// of a default through the three indirections a leaf type allows: a union
// member enumeration, a bits type, and a typedef the leaf derives from. Each
// case is converted once with a default naming a definition that carries an
// if-feature statement, and once naming one that does not.
//
// RFC requirement: RFC7950-7.6.4-2 negative — a union default "x", naming a member enumeration's enum x that carries "if-feature f", a bits default "b1 b2" whose bit b2 carries it, and a default "x" of a leaf whose typedef defines enum x with it, each record a schema build error naming the if-feature.
// RFC requirement: RFC7950-7.6.4-2 positive — the same three leaves with the default naming y, "b1", and y, whose definitions carry no if-feature, convert with no schema build error.
func TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef(t *testing.T) {
	const module = `module m { namespace "urn:m"; prefix m; feature f; `
	cases := []struct {
		name      string
		leaf      string
		violating string
		compliant string
	}{
		{"union member", `leaf a { type union { type int8; type enumeration { enum x { if-feature f; } enum y; } } `, "x", "y"},
		{"bits", `leaf a { type bits { bit b1; bit b2 { if-feature f; } } `, `"b1 b2"`, "b1"},
		{"typedef", `typedef e { type enumeration { enum x { if-feature f; } enum y; } } leaf a { type e; `, "x", "y"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := leafNodeAndBuildError(t, module+tc.leaf+`default `+tc.violating+`; } }`)
			require.Error(t, err, "a default whose definition carries an if-feature must be refused")
			assert.Contains(t, err.Error(), "if-feature")

			_, err = leafNodeAndBuildError(t, module+tc.leaf+`default `+tc.compliant+`; } }`)
			assert.NoError(t, err)
		})
	}
}
