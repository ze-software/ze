// Design: docs/architecture/cli/error-surface.md -- accepted path words cannot add diagnostic lines.
// Related: key_refusal.go -- InEntryKeyLeafRefusal
// Related: validator.go -- validateWithYANG

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/configtext"
)

const problemPathEntryModule = `module ze-problem-path-entry-conf {
  yang-version 1.1;
  namespace "urn:ze:problem-path-entry";
  prefix probe;
  import ze-extensions { prefix ze; }
  container path-probe {
    list target {
      key "name";
      ze:required "address";
      leaf name { type string; }
      leaf mode { type enumeration { enum blue; enum red; } }
      leaf address { type string; }
    }
  }
}`

// TestModelKeyLeafRefusalQuotesAcceptedPathWords refuses setting an entry's
// key as a leaf, with its problem and printed rename describing the accepted
// identity. Replaying the printed rename changes that exact entry alone.
func TestModelKeyLeafRefusalQuotesAcceptedPathWords(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		for _, key := range []string{"line\nbreak", "tab\tkey", `quoted"key`} {
			t.Run(user+"/"+configtext.QuoteToken(key), func(t *testing.T) {
				m := problemPathEntryModel(t, user)
				address := "path-probe target " + configtext.QuoteCommandWord(key)
				_, err := m.dispatchCommand("set " + address + " mode red")
				require.NoError(t, err)
				before := m.editor.WorkingContent()
				_, err = m.dispatchCommand("set " + address + " name changed")
				require.Error(t, err)
				problem := address + " name: name is the key of list target and cannot be set inside an entry"
				assert.Equal(t, problem, strings.SplitN(err.Error(), "\n", 2)[0])
				fix := lastLine(err.Error())
				require.Equal(t, "fix: rename "+address+" to <new-key>", fix)
				assert.Equal(t, before, m.editor.WorkingContent())
				_, err = m.dispatchCommand(strings.Replace(strings.TrimPrefix(fix, "fix: "), "<new-key>", "renamed", 1))
				require.NoError(t, err, "the printed rename must address the accepted key")
				entries := m.editor.tree.GetContainer("path-probe").GetList("target")
				require.Len(t, entries, 1)
				assert.NotContains(t, entries, key)
				entry := entries["renamed"]
				require.NotNil(t, entry)
				mode, ok := entry.Get("mode")
				assert.True(t, ok)
				assert.Equal(t, "red", mode)
			})
		}
	}
}

// TestModelRequiredWarningQuotesAcceptedPathWords reads the public semantic
// validator's warning and replays only its value placeholder. A local module
// cannot pass commit verify's separately global plugin-content parser, so
// this invokes the same required validator that commit verification calls.
func TestModelRequiredWarningQuotesAcceptedPathWords(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		for _, key := range []string{"line\nbreak", "tab\tkey", `quoted"key`} {
			t.Run(user+"/"+configtext.QuoteToken(key), func(t *testing.T) {
				m := problemPathEntryModel(t, user)
				address := "path-probe target " + configtext.QuoteCommandWord(key)
				_, err := m.dispatchCommand("set " + address + " mode red")
				require.NoError(t, err)
				before := m.editor.WorkingContent()
				errs, warns := m.validator.ValidateWithYANG(m.editor.tree)
				require.Empty(t, errs)
				require.Len(t, warns, 1)
				printed := warns[0].Text(nil)
				problem := "path-probe/target " + configtext.QuoteToken(key) + `: missing required field "address" (`
				tail, found := strings.CutPrefix(printed, problem)
				require.True(t, found, "the actual warning must quote the key on one line: %s", printed)
				fix, _, found := strings.Cut(tail, ")")
				require.True(t, found, "the warning must carry a complete correction: %s", printed)
				require.Equal(t, "set "+address+" address <value>", fix)
				assert.Equal(t, before, m.editor.WorkingContent(), "verification applies nothing")
				_, err = m.dispatchCommand(strings.Replace(fix, "<value>", "ready", 1))
				require.NoError(t, err, "the warning's actual correction must address the original entry")
				entries := m.editor.tree.GetContainer("path-probe").GetList("target")
				require.Len(t, entries, 1)
				entry := entries[key]
				require.NotNil(t, entry, "the required fix changed the accepted key's bytes")
				value, ok := entry.Get("address")
				assert.True(t, ok)
				assert.Equal(t, "ready", value)
				errs, warns = m.validator.ValidateWithYANG(m.editor.tree)
				assert.Empty(t, errs)
				assert.Empty(t, warns, "the actual required warning must disappear after its printed fix")
			})
		}
	}
}

func problemPathEntryModel(t *testing.T, user string) *Model {
	t.Helper()
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.AddModuleFromText("ze-problem-path-entry-conf.yang", problemPathEntryModule))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	schema, err := config.SchemaOf(resolved)
	require.NoError(t, err)
	m, _, _ := newCommitGrammarModel(t, user)
	m.editor.schema = schema
	m.completer = &Completer{schema: resolved}
	m.validator = &ConfigValidator{schema: schema, yangValidator: configyang.NewValidator(resolved)}
	return m
}
