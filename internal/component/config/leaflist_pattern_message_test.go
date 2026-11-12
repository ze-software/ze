// Design: leaf_value.go -- validateLeafPatterns speaks a pattern's error-message
//
// VALIDATES: AC-12 of spec-operator-errors-name-the-fix for a leaf-list: a
// value refused by the pattern of a leaf-list's type is refused in the words
// the pattern's error-message gives, on the set parser and on the
// hierarchical parser, as a single leaf's refusal is.
// PREVENTS: a leaf-list, multi-leaf or bracket node showing the bare regular
// expression (`value "Core" does not match pattern "[a-z]+"`) while a leaf of
// the same type speaks its message.
package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// leafListPatternMessage is the error-message of the tags pattern in
// leafListPatternModule.
const leafListPatternMessage = "A tag is lower-case letters, such as core."

const leafListPatternModule = `module m { namespace "urn:m"; prefix m;
	container top {
		leaf-list tags {
			type string {
				pattern '[a-z]+' { error-message "` + leafListPatternMessage + `"; }
			}
		}
	}
}`

// leafListPatternSchema loads leafListPatternModule and defines its top
// container in a schema, converted by yangToNode as the schema build does.
func leafListPatternSchema(t *testing.T) *Schema {
	t.Helper()
	loader := yang.NewLoader()
	require.NoError(t, loader.AddModuleFromText("m.yang", leafListPatternModule))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	module := resolved.GetEntry("m")
	require.NotNil(t, module)
	entry := module.Dir["top"]
	require.NotNil(t, entry, "module must define container top")
	resetSchemaBuildErrors()
	node := yangToNode(entry, "top")
	require.NoError(t, flushSchemaBuildErrors())
	schema := NewSchema()
	schema.Define("top", node)
	return schema
}

// TestLeafListPatternRefusalSpeaksTheMessage parses a leaf-list member that
// breaks the pattern, through the set parser and the hierarchical parser,
// and a member that keeps it. Method: the refusal is compared with the
// message the pattern declares, spoken as the refusal's why after the problem
// that names the leaf-list's path and the member, and must not show the
// regular expression.
func TestLeafListPatternRefusalSpeaksTheMessage(t *testing.T) {
	schema := leafListPatternSchema(t)
	const want = `top tags: "Core" is not in the accepted format` + "\n" + leafListPatternMessage + "\nfix: "

	refusals := map[string]func() error{
		"set parser": func() error {
			_, err := NewSetParser(schema).Parse("set top tags Core\n")
			return err
		},
		"hierarchical parser": func() error {
			_, err := NewParser(schema).Parse("top {\n\ttags [ Core ];\n}\n")
			return err
		},
	}
	for name, parse := range refusals {
		t.Run(name, func(t *testing.T) {
			err := parse()
			if err == nil {
				t.Fatal("a tag that breaks the pattern parsed")
			}
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal does not speak the pattern's error-message:\n%s", err.Error())
			}
			if strings.Contains(err.Error(), "[a-z]+") {
				t.Errorf("the refusal still shows the regular expression:\n%s", err.Error())
			}
		})
	}

	if _, err := NewSetParser(schema).Parse("set top tags core\n"); err != nil {
		t.Errorf("a tag that keeps the pattern was refused: %v", err)
	}
}
