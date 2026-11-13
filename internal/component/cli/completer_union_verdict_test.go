// Design: docs/architecture/config/yang-config-design.md -- parse-time validation.
// Related: completer.go -- ValidateValueAtPath
// Related: config/leaf_value.go -- validateUnionValue

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestUnionVerdictOnMergedDeclaration compares the editor, the parser and the
// resolved YANG entry's validator for a union leaf declared by two modules.
// The descriptions differ so getEntry builds a merged virtual entry; values
// exercise both union members and both sides of the numeric member's range.
func TestUnionVerdictOnMergedDeclaration(t *testing.T) {
	for _, name := range []string{"first", "second"} {
		module := "ze-union-" + name + "-conf"
		text := `module MODULE {
  yang-version 1.1;
  namespace "urn:MODULE";
  prefix probe;
  container union-probe {
    leaf port {
      description "DESCRIPTION declaration of the port.";
      type union {
        type enumeration { enum auto; }
        type uint16 { range "10..20"; }
      }
    }
  }
}`
		text = strings.ReplaceAll(text, "MODULE", module)
		text = strings.ReplaceAll(text, "DESCRIPTION", name)
		t.Cleanup(configyang.RegisterModuleForTest(module+".yang", text))
	}
	c := newTestCompleter(t)
	path := []string{"union-probe", "port"}
	entry := c.getEntry(path)
	require.NotNil(t, entry)
	assert.Contains(t, entry.Description, "first declaration")
	assert.Contains(t, entry.Description, "second declaration")
	schema, err := c.valueSchema()
	require.NoError(t, err)
	validator := configyang.NewValidator(c.schema)
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{value: "auto", valid: true},
		{value: "10", valid: true},
		{value: "20", valid: true},
		{value: "9"},
		{value: "21"},
		{value: "autoo"},
		{value: "bogus"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			editorErr := c.ValidateValueAtPath(path, tc.value)
			yangErr := validator.ValidateLeafType("union-probe/port", entry, entry.Type, tc.value)
			_, parserErr := config.NewParser(schema).Parse("union-probe { port " + tc.value + "; }")
			assert.Equal(t, tc.valid, editorErr == nil, "editor: %v", editorErr)
			assert.Equal(t, tc.valid, yangErr == nil, "YANG: %v", yangErr)
			assert.Equal(t, tc.valid, parserErr == nil, "parser: %v", parserErr)
			if !tc.valid {
				assert.Contains(t, editorErr.Error(), "auto")
				assert.Contains(t, editorErr.Error(), "uint16")
			}
		})
	}
}
