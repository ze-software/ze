// Design: docs/architecture/config/yang-config-design.md -- Value refusals.
// Related: yang_schema.go -- native type restrictions projected into schema nodes.

package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestYANGLeafListNativeRange loads ranged YANG types through the real schema
// converter and judges every leaf-list syntax at parser and editor boundaries.
func TestYANGLeafListNativeRange(t *testing.T) {
	checkYANGLeafListNativeRestriction(t, "range",
		[]string{"1", "3"}, []string{"0", "4"}, "Native numbers must be 1 through 3.")
}

// TestYANGLeafListNativeLength pairs both length boundaries with Unicode
// character controls, so a fix cannot replace the YANG length with byte count.
func TestYANGLeafListNativeLength(t *testing.T) {
	checkYANGLeafListNativeRestriction(t, "length",
		[]string{"ab", "abcd", "éé"}, []string{"a", "abcde"}, "Native names need 2 through 4 characters.")
}

// TestYANGLeafListNativeUnion retains restrictions on each member type. Neither
// an out-of-range number nor a string outside its own length/pattern can pass.
func TestYANGLeafListNativeUnion(t *testing.T) {
	checkYANGLeafListNativeRestriction(t, "union",
		[]string{"1", "3", "ab", "abcd"}, []string{"0", "4", "abcde", "99"}, "")
}

// TestYANGLeafListNativeDecimal retains fraction digits and compares decimal
// ranges as decimals. Integer-only range parsing cannot satisfy these controls.
func TestYANGLeafListNativeDecimal(t *testing.T) {
	checkYANGLeafListNativeRestriction(t, "decimal",
		[]string{"1.25", "2.50", "1.3"}, []string{"1.24", "2.51", "1.234"}, "")
}

// TestCoppProtectedPortNativeRange reads the owning shipped YANG file rather
// than restating its port restriction. A zero port must be refused before commit
// on the actual control-plane-protection bgp protected-port setting.
func TestCoppProtectedPortNativeRange(t *testing.T) {
	module, err := os.ReadFile("../../plugins/copp/yang/ze-copp-conf.yang")
	require.NoError(t, err)
	schema := nativeLeafListSchema(t, "ze-copp-conf", string(module), "control-plane-protection")
	path := []string{"control-plane-protection", "bgp", "protected-port"}
	for _, surface := range []string{"hierarchical", "set", "set-meta", "value guard"} {
		t.Run(surface, func(t *testing.T) {
			for _, value := range []string{"1", "179", "65535"} {
				require.NoError(t, nativeLeafListValue(t, schema, path, value, true, surface))
			}
			err := nativeLeafListValue(t, schema, path, "0", true, surface)
			require.Error(t, err, "a zero protected port must not be accepted")
			assert.Contains(t, err.Error(), "1..65535")
		})
	}
}

func checkYANGLeafListNativeRestriction(t *testing.T, restriction string, accepted, refused []string, message string) {
	t.Helper()
	schema := nativeLeafListSchema(t, "ze-native-leaf-list-test", nativeLeafListModule(), "top")
	for _, form := range []string{"plain", "explicit", "bracket", "multi"} {
		path := []string{"top", form + "-" + restriction}
		for _, surface := range []string{"hierarchical", "set", "set-meta", "value guard"} {
			t.Run(form+"/"+surface, func(t *testing.T) {
				for _, value := range accepted {
					require.NoError(t, nativeLeafListValue(t, schema, path, value, form != "multi", surface),
						"valid boundary %q must survive", value)
				}
				for _, value := range refused {
					err := nativeLeafListValue(t, schema, path, value, form != "multi", surface)
					if assert.Error(t, err, "restricted value %q was accepted", value) {
						if message != "" {
							assert.Contains(t, err.Error(), message)
						}
					}
				}
			})
		}
	}
}

// nativeLeafListValue checks one public value boundary. Successful parser reads
// must retain the supplied member, so dropping every value cannot pass controls.
func nativeLeafListValue(t *testing.T, schema *Schema, path []string, value string, bracket bool, surface string) error {
	t.Helper()
	leafName := path[len(path)-1]
	node := schema.Get(path[0])
	for _, name := range path[1:] {
		container, ok := node.(*ContainerNode)
		require.True(t, ok, "parent of %s must be a container", name)
		node = container.Get(name)
		require.NotNil(t, node, "schema must retain %s", name)
	}
	if surface == "value guard" {
		return ValidateNodeValue(node, path, value)
	}
	spelling := QuoteIfNeeded(value)
	if bracket {
		spelling = "[ " + spelling + " ]"
	}
	var tree *Tree
	var err error
	switch surface {
	case "hierarchical":
		input := leafName + " " + spelling + ";"
		for i := len(path) - 2; i >= 0; i-- {
			input = path[i] + " { " + input + " }"
		}
		tree, err = NewParser(schema).Parse(input)
	case "set":
		tree, err = NewSetParser(schema).Parse("set " + strings.Join(path, " ") + " " + spelling)
	case "set-meta":
		tree, _, err = NewSetParser(schema).ParseWithMeta("#reader @ssh %2026-11-17T18:00:00Z set " + strings.Join(path, " ") + " " + spelling)
	default:
		t.Fatalf("unknown test surface %q", surface)
	}
	if err != nil {
		return err
	}
	for _, name := range path[:len(path)-1] {
		tree = tree.GetContainer(name)
		require.NotNil(t, tree, "parsed input must retain %s", name)
	}
	assert.Equal(t, []string{value}, tree.GetSlice(leafName))
	return nil
}

func nativeLeafListSchema(t *testing.T, moduleName, content, root string) *Schema {
	t.Helper()
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText(moduleName+".yang", content))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	module := resolved.GetEntry(moduleName)
	require.NotNil(t, module)
	entry := module.Dir[root]
	require.NotNil(t, entry)
	resetSchemaBuildErrors()
	node := yangToNode(entry, root)
	require.NoError(t, flushSchemaBuildErrors())
	schema := NewSchema()
	schema.Define(root, node)
	return schema
}

// nativeLeafListModule declares the restrictions in YANG, then lets the normal
// converter project them. The four bounded forms include implicit plain syntax
// and the explicit ze:syntax form that builds the same ValueOrArrayNode type.
func nativeLeafListModule() string {
	var body textbuf.Buffer
	body.Str(`module ze-native-leaf-list-test {
		yang-version 1.1; namespace "urn:ze:native-leaf-list-test"; prefix test;
		import ze-extensions { prefix ze; }
		container top {`)
	for _, form := range []struct{ name, syntax string }{
		{"plain", ""}, {"explicit", "value-or-array"}, {"bracket", "bracket"}, {"multi", "multi-leaf"},
	} {
		for _, restriction := range []struct{ name, typ string }{
			{"range", `uint16 { range "1..3" { error-message "Native numbers must be 1 through 3."; } }`},
			{"length", `string { length "2..4" { error-message "Native names need 2 through 4 characters."; } }`},
			{"union", `union { type uint16 { range "1..3"; } type string { length "2..4"; pattern '[a-z]+'; } }`},
			{"decimal", `decimal64 { fraction-digits 2; range "1.25..2.50"; }`},
		} {
			body.Str("leaf-list ").Str(form.name).Byte('-').Str(restriction.name).
				Str(" { type ").Str(restriction.typ).Byte(' ')
			if form.syntax != "" {
				body.Str(`ze:syntax "`).Str(form.syntax).Str(`";`)
			}
			body.Str("}")
		}
	}
	return body.Str("}}").String()
}
