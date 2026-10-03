package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// leafDefaultBuildError loads one module, converts its leaf "a" through
// yangToNode and returns the schema build error that conversion recorded.
func leafDefaultBuildError(t *testing.T, moduleText string) error {
	t.Helper()
	loader := yang.NewLoader()
	require.NoError(t, loader.AddModuleFromText("m.yang", moduleText))
	require.NoError(t, loader.Resolve())
	entry := loader.GetEntry("m")
	require.NotNil(t, entry)
	leaf := entry.Dir["a"]
	require.NotNil(t, leaf, "module must define leaf a")
	resetSchemaBuildErrors()
	yangToNode(leaf, "m/a")
	return flushSchemaBuildErrors()
}

// TestRFC7950DefaultValidForType converts leaves whose default statement is
// valid for the leaf's type, the typedef's type, and the leaf-list's type, and
// expects no schema build error.
//
// RFC requirement: RFC7950-7.3.4-1 positive — a typedef whose default is inside its own range converts with no schema build error.
// RFC requirement: RFC7950-7.6.4-2 positive — a leaf whose default is inside its type's range, on a leaf that is not mandatory, converts with no schema build error.
// RFC requirement: RFC7950-7.7.4-1 positive — a leaf-list whose default is valid for its type and whose min-elements is 0 converts with no schema build error.
func TestRFC7950DefaultValidForType(t *testing.T) {
	assert.NoError(t, leafDefaultBuildError(t, `module m { namespace "urn:m"; prefix m; typedef small { type uint8 { range "1..10"; } default 5; } leaf a { type small; } }`))
	assert.NoError(t, leafDefaultBuildError(t, `module m { namespace "urn:m"; prefix m; leaf a { type uint8 { range "1..10"; } default 7; } }`))
	assert.NoError(t, leafDefaultBuildError(t, `module m { namespace "urn:m"; prefix m; leaf a { type enumeration { enum x; enum y; } default y; } }`))
	assert.NoError(t, leafDefaultBuildError(t, `module m { namespace "urn:m"; prefix m; leaf-list a { type uint8 { range "1..10"; } default 3; } }`))
}

// TestRFC7950DefaultInvalidForType converts leaves whose default statement
// its type refuses, a default beside mandatory true, and a leaf-list default
// beside min-elements 1, and expects each to record a schema build error
// naming the fault.
//
// RFC requirement: RFC7950-7.3.4-1 negative — a typedef whose default lies outside its own range records a schema build error naming the typedef default.
// RFC requirement: RFC7950-7.6.4-2 negative — a leaf default outside the type's range, a default that is not an enum name, and a default on a mandatory leaf each record a schema build error.
// RFC requirement: RFC7950-7.7.4-1 negative — a leaf-list default outside its type's range, and a leaf-list default beside min-elements 1, each record a schema build error.
func TestRFC7950DefaultInvalidForType(t *testing.T) {
	err := leafDefaultBuildError(t, `module m { namespace "urn:m"; prefix m; typedef small { type uint8 { range "1..10"; } default 50; } leaf a { type small; } }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `typedef default "50" is not valid`)

	err = leafDefaultBuildError(t, `module m { namespace "urn:m"; prefix m; leaf a { type uint8 { range "1..10"; } default 70; } }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `default "70" is not valid for type uint8`)

	err = leafDefaultBuildError(t, `module m { namespace "urn:m"; prefix m; leaf a { type enumeration { enum x; enum y; } default z; } }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `default "z" is not valid`)

	err = leafDefaultBuildError(t, `module m { namespace "urn:m"; prefix m; leaf a { type uint8; mandatory true; default 1; } }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "on a mandatory leaf is invalid")

	err = leafDefaultBuildError(t, `module m { namespace "urn:m"; prefix m; leaf-list a { type uint8 { range "1..10"; } default 30; } }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `default "30" is not valid for type uint8`)

	err = leafDefaultBuildError(t, `module m { namespace "urn:m"; prefix m; leaf-list a { type uint8; min-elements 1; default 1; } }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "min-elements 1 is invalid")
}
