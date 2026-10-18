package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// VALIDATES: the default the analysis tree shows for a leaf is the value the
// config tree takes. The method loads one module per case and reads yangDefault
// over its leaf "a": an integer default a module wrote in hexadecimal or octal
// shows in decimal, a typedef's default shows when the leaf has none, a union
// default takes the form of the first member type that accepts it, and a
// notation the schema build refuses shows as written.
// PREVENTS: `ze` help and completion printing "0x10" for a leaf whose config
// default is 16.
func TestYangDefaultAgreesWithTheConfigTree(t *testing.T) {
	cases := []struct {
		leaf string
		want string
	}{
		{`leaf a { type uint8; default 0x10; }`, "16"},
		{`leaf a { type int8; default "-020"; }`, "-16"},
		{`leaf a { type string; default 0x10; }`, "0x10"},
		{`typedef hex { type uint16; default 0xff; } leaf a { type hex; }`, "255"},
		{`leaf a { type union { type int32; type string; } default 0x10; }`, "16"},
		{`leaf a { type union { type string; type int32; } default 0x10; }`, "0x10"},
		{`leaf a { type uint8; default 09; }`, "09"},
		{`leaf a { type uint8; }`, ""},
	}
	for _, tc := range cases {
		loader := yang.NewLoader()
		require.NoError(t, loader.AddModuleFromText("m.yang",
			`module m { namespace "urn:m"; prefix m; `+tc.leaf+` }`), "%s", tc.leaf)
		schema, err := loader.Resolve()
		require.NoError(t, err, "%s", tc.leaf)
		module := schema.GetEntry("m")
		require.NotNil(t, module, "%s", tc.leaf)
		leaf := module.Dir["a"]
		require.NotNil(t, leaf, "%s: module must define leaf a", tc.leaf)
		assert.Equal(t, tc.want, yangDefault(leaf), "%s", tc.leaf)
	}
}
