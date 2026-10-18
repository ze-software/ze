package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// integerDefaultLeaf loads one module, converts its leaf "a" through yangToNode,
// and returns the leaf's schema default and the schema build error recorded.
func integerDefaultLeaf(t *testing.T, leafText string) (string, error) {
	t.Helper()
	loader := yang.NewLoader()
	moduleText := `module m { namespace "urn:m"; prefix m; ` + leafText + ` }`
	require.NoError(t, loader.AddModuleFromText("m.yang", moduleText))
	schema, err := loader.Resolve()
	require.NoError(t, err)
	entry := schema.GetEntry("m")
	require.NotNil(t, entry)
	leafEntry := entry.Dir["a"]
	require.NotNil(t, leafEntry, "module must define leaf a")
	resetSchemaBuildErrors()
	node := yangToNode(leafEntry, "m/a")
	buildErr := flushSchemaBuildErrors()
	leaf, ok := node.(*LeafNode)
	require.True(t, ok, "leaf a converted to %T, want *LeafNode", node)
	return leaf.Default, buildErr
}

// TestRFC7950IntegerModuleDefaultHexAndOctal converts leaves whose default is
// written in the hexadecimal and octal notations a YANG module may use for an
// integer, and reads the schema default each one produced. The method is the
// schema build a module goes through on load, so a refused notation shows as a
// build error and an accepted one as the default the config tree receives.
//
// RFC requirement: RFC7950-9.1-1 positive -- a module default for uint8 written
// "0x10", "0x1f", "0x1F", "+0x10" or "020" converts with no schema build error and
// its schema default is the decimal value ("16", "31", "31", "16", "16"); "-0x10"
// and "-020" for int8 give "-16", and "-0x80" gives "-128", the int8 minimum; a
// leading zero is octal, so "010" gives "8"; "0xffffffffffffffff" for uint64
// gives "18446744073709551615" and "-0x8000000000000000" for int64 gives
// "-9223372036854775808"; the decimal forms "17", "+17", "0", "-0" and "00" give
// "17", "17", "0", "0" and "0"; and a union default is read as its first member
// type that accepts it, so "0x10" gives "16" for a union of int32 then string,
// and for a union of a uint8 limited to 0..10 then int32, and stays "0x10" for a
// union of string then int32.
func TestRFC7950IntegerModuleDefaultHexAndOctal(t *testing.T) {
	cases := []struct {
		leaf string
		want string
	}{
		{`leaf a { type uint8; default 0x10; }`, "16"},
		{`leaf a { type uint8; default 0x1f; }`, "31"},
		{`leaf a { type uint8; default 0x1F; }`, "31"},
		{`leaf a { type uint8; default "+0x10"; }`, "16"},
		{`leaf a { type uint8; default 020; }`, "16"},
		{`leaf a { type uint8; default 010; }`, "8"},
		{`leaf a { type int8; default "-0x10"; }`, "-16"},
		{`leaf a { type int8; default "-020"; }`, "-16"},
		{`leaf a { type uint8; default 17; }`, "17"},
		{`leaf a { type uint8; default "+17"; }`, "17"},
		{`leaf a { type uint8; default 0; }`, "0"},
		{`leaf a { type uint8; default "-0"; }`, "0"},
		{`leaf a { type uint8; default 00; }`, "0"},
		{`leaf a { type int8; default "-0x80"; }`, "-128"},
		{`leaf a { type uint64; default 0xffffffffffffffff; }`, "18446744073709551615"},
		{`leaf a { type int64; default "-0x8000000000000000"; }`, "-9223372036854775808"},
		{`leaf a { type union { type int32; type string; } default 0x10; }`, "16"},
		{`leaf a { type union { type uint8 { range "0..10"; } type int32; } default 0x10; }`, "16"},
		{`leaf a { type union { type string; type int32; } default 0x10; }`, "0x10"},
		{`typedef hex { type uint16; default 0xff; } leaf a { type hex; }`, "255"},
	}
	for _, tc := range cases {
		got, err := integerDefaultLeaf(t, tc.leaf)
		assert.NoError(t, err, "%s", tc.leaf)
		assert.Equal(t, tc.want, got, "%s", tc.leaf)
	}
}

// TestRFC7950IntegerModuleDefaultMalformedNotation converts leaves whose
// default only looks like one of the three integer notations, or names a value
// the type's range refuses once the notation is read, and expects a schema build
// error for each.
//
// RFC requirement: RFC7950-9.1-1 negative -- "0x" with no digit, "0x1g", "0X10"
// (the prefix is the characters "0x"), "09" (not an octal digit), "1_0", "0b1" and
// "0o17" each record a schema build error for uint8, and "0x100" and "0400" record
// one because 256 is outside uint8; "-0x81" records one for int8 because -129 is
// outside it, "-0x1" records one for uint8 because the type has no negative value,
// "0x10000000000000000" records one because no integer type holds it, and "09"
// records one for a union of uint8 then an enumeration, because no member reads it.
func TestRFC7950IntegerModuleDefaultMalformedNotation(t *testing.T) {
	for _, def := range []string{"0x", "0x1g", "0X10", "09", "1_0", "0b1", "0o17", "0x100", "0400", "-0x1"} {
		_, err := integerDefaultLeaf(t, `leaf a { type uint8; default "`+def+`"; }`)
		if assert.Error(t, err, "default %q must be refused for uint8", def) {
			assert.Contains(t, err.Error(), `default "`+def+`" is not valid for type uint8`)
		}
	}
	refused := []struct {
		leaf string
		want string
	}{
		{`leaf a { type int8; default "-0x81"; }`, `default "-0x81" is not valid for type int8`},
		{`leaf a { type uint64; default 0x10000000000000000; }`, "larger than any YANG integer type holds"},
		{`leaf a { type uint8; default 09; }`, "a leading 0 makes a YANG module default octal"},
		{`leaf a { type union { type uint8; type enumeration { enum unbounded; } } default 09; }`,
			"no member type of the union accepts it"},
	}
	for _, tc := range refused {
		_, err := integerDefaultLeaf(t, tc.leaf)
		if assert.Error(t, err, "%s must be refused", tc.leaf) {
			assert.Contains(t, err.Error(), tc.want, "%s", tc.leaf)
		}
	}
}
