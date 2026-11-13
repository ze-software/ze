// Design: docs/architecture/config/syntax.md -- a non-first list key keeps its typed refusal.
// Related: parser_list.go -- parseListFieldBlock
// Related: setparser.go -- Parse

package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/configtext"
)

const multikeyRefusalEntryModule = `module ze-multikey-refusal-entry-conf {
  yang-version 1.1;
  namespace "urn:ze:multikey-refusal-entry";
  prefix probe;
  import ze-extensions { prefix ze; }
  container multikey-probe {
    list session {
      key "peer vrf interface";
      leaf peer { type string; }
      leaf vrf {
        type enumeration {
          enum blue { ze:help "Blue routing table"; }
          enum red { ze:help "Red routing table"; }
        }
      }
      leaf interface { type enumeration { enum eth0; enum eth9; } }
      leaf label { type string; }
    }
    list target {
      key "name";
      leaf name { type string; }
      leaf mode { type enumeration { enum blue; enum red; } }
    }
  }
}`

// TestParserNonFirstKeyRefusalAtEntryPoint loads a real YANG declaration into
// a local loader, then drives both hierarchical parser entry points and the
// set parser. Every refused input has an accepted correction whose stored
// identity still includes both later key pairs and whose payload survives.
func TestParserNonFirstKeyRefusalAtEntryPoint(t *testing.T) {
	schema := multikeyRefusalEntrySchema(t)

	const body = "session 192.0.2.1 {\n  vrf blu;\n  interface eth0;\n  label tail;\n}\n"
	for _, tc := range []struct {
		name  string
		input string
		line  int
		fix   string
		parse func(string) (*Tree, error)
		entry func(*Tree) *Tree
	}{
		{
			name:  "hierarchical file",
			input: "multikey-probe {\n" + body + "}\n",
			line:  3,
			fix:   "vrf blue;",
			parse: NewParser(schema).Parse,
			entry: func(tree *Tree) *Tree { return tree.GetContainer("multikey-probe") },
		},
		{
			name:  "hierarchical context",
			input: body,
			line:  2,
			fix:   "vrf blue;",
			parse: func(input string) (*Tree, error) { return NewParser(schema).ParseAt(input, []string{"multikey-probe"}) },
			entry: func(tree *Tree) *Tree { return tree },
		},
		{
			name:  "set file",
			input: "set multikey-probe session 192.0.2.1 vrf blu interface eth0 label tail\n",
			line:  1,
			fix:   "set multikey-probe session 192.0.2.1 vrf blue interface eth0 label tail",
			parse: NewSetParser(schema).Parse,
			entry: func(tree *Tree) *Tree { return tree.GetContainer("multikey-probe") },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := tc.parse(tc.input)
			require.Error(t, err)
			assert.Nil(t, tree, "a refused file must not return usable configuration")
			var line *LineError
			require.True(t, errors.As(err, &line), "line identity: %v", err)
			assert.Equal(t, tc.line, line.Line())
			var refusal *configyang.ValidationError
			require.True(t, errors.As(err, &refusal), "typed key refusal: %v", err)
			assert.Equal(t, []string{"multikey-probe", "session", "192.0.2.1", "vrf"}, refusal.Segments())
			assert.Equal(t, "vrf", refusal.Leaf())
			assert.Contains(t, err.Error(), `multikey-probe session 192.0.2.1 vrf: "blu" is not an accepted value`)
			assert.Contains(t, err.Error(), "Blue routing table")
			assert.Contains(t, err.Error(), "Red routing table")
			assert.Equal(t, "fix: "+tc.fix, err.Error()[strings.LastIndex(err.Error(), "\n")+1:])
			assert.NotContains(t, err.Error(), "invalid vrf for session")
			corrected := strings.Replace(tc.input, "vrf blu", "vrf blue", 1)
			if tc.name == "set file" {
				corrected = tc.fix + "\n"
			} else {
				corrected = strings.Replace(tc.input, "vrf blu;", tc.fix, 1)
			}
			tree, err = tc.parse(corrected)
			require.NoError(t, err, "the actual printed correction must parse")
			parent := tc.entry(tree)
			require.NotNil(t, parent)
			entries := parent.GetList("session")
			require.Len(t, entries, 1)
			entry := entries["192.0.2.1 vrf blue interface eth0"]
			require.NotNil(t, entry, "both non-first keys remain in the stored identity")
			label, ok := entry.Get("label")
			assert.True(t, ok)
			assert.Equal(t, "tail", label)
		})
	}
}

// TestParserRefusalQuotesAcceptedPathWords keeps display escaping separate
// from identity: a permitted key may contain control characters or a quote,
// but none may add a line to the refusal's problem or change its typed path.
func TestParserRefusalQuotesAcceptedPathWords(t *testing.T) {
	schema := multikeyRefusalEntrySchema(t)
	for _, key := range []string{"line\nbreak", "tab\tkey", `quoted"key`, "return\rkey"} {
		t.Run(configtext.QuoteToken(key), func(t *testing.T) {
			input := "multikey-probe { target " + configtext.QuoteToken(key) + " { mode blu; } }\n"
			_, err := NewParser(schema).Parse(input)
			require.Error(t, err)
			var refusal *configyang.ValidationError
			require.True(t, errors.As(err, &refusal), "typed value refusal: %v", err)
			assert.Equal(t, []string{"multikey-probe", "target", key, "mode"}, refusal.Segments())
			problem := `line 1: multikey-probe target ` + configtext.QuoteToken(key) + ` mode: "blu" is not an accepted value (did you mean blue?)`
			problemLine := strings.SplitN(err.Error(), "\n", 2)[0]
			assert.Equal(t, problem, problemLine)
			assert.NotContains(t, problemLine, "\r", "a path value must not overwrite its displayed problem line")
			fix := err.Error()[strings.LastIndex(err.Error(), "\n")+1:]
			require.Equal(t, "fix: mode blue;", fix)
			corrected := strings.Replace(input, "mode blu;", strings.TrimPrefix(fix, "fix: "), 1)
			tree, err := NewParser(schema).Parse(corrected)
			require.NoError(t, err, "the printed file correction must preserve the accepted key")
			entries := tree.GetContainer("multikey-probe").GetList("target")
			require.Len(t, entries, 1)
			entry := entries[key]
			require.NotNil(t, entry)
			value, ok := entry.Get("mode")
			assert.True(t, ok)
			assert.Equal(t, "blue", value)
		})
	}
}

func multikeyRefusalEntrySchema(t *testing.T) *Schema {
	t.Helper()
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("ze-multikey-refusal-entry-conf.yang", multikeyRefusalEntryModule))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	schema, err := SchemaOf(resolved)
	require.NoError(t, err)
	return schema
}
