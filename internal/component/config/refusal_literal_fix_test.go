// Design: docs/architecture/config/yang-config-design.md -- corrections preserve accepted values.

package config

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/suggest"
)

func literalRefusalSchema(t *testing.T, accepted string) *Schema {
	t.Helper()
	module := fmt.Sprintf(`module ze-literal-refusal-conf {
  yang-version 1.1;
  namespace "urn:ze:literal-refusal";
  prefix probe;
  container fix-literal {
    leaf mode { type enumeration { enum %s; } }
  }
}`, strconv.Quote(accepted))
	t.Cleanup(configyang.RegisterModuleForTest("ze-literal-refusal-conf.yang", module))
	schema, err := YANGSchema()
	require.NoError(t, err)
	return schema
}

func literalRefusalFix(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err)
	lines := strings.Split(err.Error(), "\n")
	last := lines[len(lines)-1]
	require.True(t, strings.HasPrefix(last, "fix: "), "%v", err)
	return strings.TrimPrefix(last, "fix: ")
}

// TestHierarchicalRefusalReplaysLiteralEnumCorrection reads the exact printed
// correction as a file statement under the same schema that refused the value.
func TestHierarchicalRefusalReplaysLiteralEnumCorrection(t *testing.T) {
	for _, accepted := range []string{"in progress", `say "hello"`, "[one two]", "<one two>"} {
		t.Run(accepted, func(t *testing.T) {
			schema := literalRefusalSchema(t, accepted)
			_, err := NewParser(schema).Parse("fix-literal { mode bogus; }")
			fix := literalRefusalFix(t, err)
			tree, err := NewParser(schema).Parse("fix-literal { " + fix + " }")
			require.NoError(t, err, "printed file correction %q", fix)
			value, ok := tree.GetContainer("fix-literal").Get("mode")
			assert.True(t, ok)
			assert.Equal(t, accepted, value)
		})
	}
}

// TestSetRefusalReplaysLiteralEnumCorrection proves a set-file correction
// preserves an actual enum name that needs quoting rather than a placeholder.
func TestSetRefusalReplaysLiteralEnumCorrection(t *testing.T) {
	for _, accepted := range []string{"in progress", `say "hello"`, "[one two]", "<one two>"} {
		t.Run(accepted, func(t *testing.T) {
			schema := literalRefusalSchema(t, accepted)
			_, err := NewSetParser(schema).Parse("set fix-literal mode bogus\n")
			fix := literalRefusalFix(t, err)
			tree, err := NewSetParser(schema).Parse(fix + "\n")
			require.NoError(t, err, "printed set correction %q", fix)
			value, ok := tree.GetContainer("fix-literal").Get("mode")
			assert.True(t, ok)
			assert.Equal(t, accepted, value)
		})
	}
}

// TestDefaultRefusalReplaysLiteralEnumCorrection covers the shared YANG
// producer's root set form even when no surface replaces its default fix.
func TestDefaultRefusalReplaysLiteralEnumCorrection(t *testing.T) {
	for _, accepted := range []string{"in progress", `say "hello"`, "[one two]", "<one two>"} {
		t.Run(accepted, func(t *testing.T) {
			schema := literalRefusalSchema(t, accepted)
			err := configyang.EnumRefusal(configyang.RefusedValue{
				Path: []string{"fix-literal", "mode"}, Leaf: "mode", Value: "bogus",
			}, []suggest.Value{{Name: accepted}})
			fix := literalRefusalFix(t, err)
			tree, parseErr := NewSetParser(schema).Parse(fix + "\n")
			require.NoError(t, parseErr, "shared default correction %q", fix)
			value, ok := tree.GetContainer("fix-literal").Get("mode")
			assert.True(t, ok)
			assert.Equal(t, accepted, value)
		})
	}
}
