// Design: docs/guide/config-editor.md -- refused composite keys keep their complete correction.
// Related: model_commands_edit.go -- cmdSet
// Related: config/list_key.go -- ParseListAddress, WalkListEntryPath

package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/configtext"
)

const modelMultikeyRefusalEntryModule = `module ze-model-multikey-refusal-entry-conf {
  yang-version 1.1;
  namespace "urn:ze:model-multikey-refusal-entry";
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
      leaf password { type string; ze:sensitive; }
    }
    list target {
      key "name";
      leaf name { type string; }
      leaf mode { type enumeration { enum blue; enum red; } }
    }
  }
}`

// TestModelNonFirstKeyRefusalReplaysPrintedFix enters the actual editor
// dispatch in both modes and positions. A refused non-first key must preserve
// the candidate and pending changes, then its printed correction must create
// exactly the composite identity with all later address and payload words.
func TestModelNonFirstKeyRefusalReplaysPrintedFix(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		mode := "file"
		if user != "" {
			mode = "session"
		}
		for _, contextual := range []bool{false, true} {
			position := "root"
			if contextual {
				position = "context"
			}
			t.Run(mode+"/"+position, func(t *testing.T) {
				m := multikeyRefusalEntryModel(t, user)
				_, err := m.dispatchCommand("set multikey-probe session 192.0.2.9 vrf red interface eth9 label untouched")
				require.NoError(t, err)
				prefix := "multikey-probe "
				if contextual {
					m.contextPath = []string{"multikey-probe"}
					prefix = ""
				}
				before := m.editor.WorkingContent()
				var pending []config.PendingChange
				if user != "" {
					pending = m.editor.PendingChanges(m.editor.session.ID)
				}
				_, err = m.dispatchCommand("set " + prefix + "session 192.0.2.1 vrf blu interface eth0 label tail")
				require.Error(t, err)
				var refusal *configyang.ValidationError
				require.True(t, errors.As(err, &refusal), "typed key refusal: %v", err)
				assert.Equal(t, []string{"multikey-probe", "session", "192.0.2.1", "vrf"}, refusal.Segments())
				assert.True(t, strings.HasPrefix(err.Error(), `multikey-probe session 192.0.2.1 vrf: "blu" is not an accepted value`), "%v", err)
				assert.Contains(t, err.Error(), "Blue routing table")
				assert.Contains(t, err.Error(), "Red routing table")
				fix := lastLine(err.Error())
				require.Equal(t, "fix: set "+prefix+"session 192.0.2.1 vrf blue interface eth0 label tail", fix)
				assert.Equal(t, before, m.editor.WorkingContent(), "refusal changed the candidate")
				if user != "" {
					assert.Equal(t, pending, m.editor.PendingChanges(m.editor.session.ID), "refusal changed pending operations")
				}
				_, err = m.dispatchCommand(strings.TrimPrefix(fix, "fix: "))
				require.NoError(t, err, "replay the actual printed correction")
				entries := m.editor.tree.GetContainer("multikey-probe").GetList("session")
				require.Len(t, entries, 2)
				entry := entries["192.0.2.1 vrf blue interface eth0"]
				require.NotNil(t, entry, "replay lost an address pair")
				value, ok := entry.Get("label")
				assert.True(t, ok)
				assert.Equal(t, "tail", value)
				control := entries["192.0.2.9 vrf red interface eth9"]
				require.NotNil(t, control)
				value, ok = control.Get("label")
				assert.True(t, ok)
				assert.Equal(t, "untouched", value)
				if contextual {
					assert.Equal(t, []string{"multikey-probe"}, m.contextPath)
				} else {
					assert.Empty(t, m.contextPath)
				}
			})
		}
	}
}

// TestModelRefusalQuotesAcceptedPathWords checks the displayed problem and
// printed correction without normalizing the identity an accepted key holds.
func TestModelRefusalQuotesAcceptedPathWords(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		for _, key := range []string{"line\nbreak", "tab\tkey", `quoted"key`} {
			t.Run(user+"/"+configtext.QuoteToken(key), func(t *testing.T) {
				m := multikeyRefusalEntryModel(t, user)
				setting := "multikey-probe target " + configtext.QuoteCommandWord(key) + " mode "
				_, err := m.dispatchCommand("set " + setting + "red")
				require.NoError(t, err, "the path key is accepted before its mode is refused")
				before := m.editor.WorkingContent()
				_, err = m.dispatchCommand("set " + setting + "blu")
				require.Error(t, err)
				var refusal *configyang.ValidationError
				require.True(t, errors.As(err, &refusal), "typed value refusal: %v", err)
				assert.Equal(t, []string{"multikey-probe", "target", key, "mode"}, refusal.Segments())
				problem := "multikey-probe target " + configtext.QuoteToken(key) + ` mode: "blu" is not an accepted value (did you mean blue?)`
				assert.Equal(t, problem, strings.SplitN(err.Error(), "\n", 2)[0])
				fix := lastLine(err.Error())
				require.Equal(t, "fix: set "+setting+"blue", fix)
				assert.Equal(t, before, m.editor.WorkingContent())
				_, err = m.dispatchCommand(strings.TrimPrefix(fix, "fix: "))
				require.NoError(t, err, "the actual printed correction must preserve the accepted key")
				entries := m.editor.tree.GetContainer("multikey-probe").GetList("target")
				require.Len(t, entries, 1)
				entry := entries[key]
				require.NotNil(t, entry, "replay changed the accepted key's bytes")
				value, ok := entry.Get("mode")
				assert.True(t, ok)
				assert.Equal(t, "blue", value)
			})
		}
	}
}

// TestModelThirdKeyRefusalReplaysPrintedFix distinguishes the refused third
// key's value from the accepted pair before it. The complete printed fix must
// preserve that earlier pair and the later payload, in either editor mode.
func TestModelThirdKeyRefusalReplaysPrintedFix(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		for _, contextual := range []bool{false, true} {
			position := "root"
			if contextual {
				position = "context"
			}
			t.Run(user+"/"+position, func(t *testing.T) {
				m := multikeyRefusalEntryModel(t, user)
				prefix := "multikey-probe "
				if contextual {
					m.contextPath = []string{"multikey-probe"}
					prefix = ""
				}
				before := m.editor.WorkingContent()
				_, err := m.dispatchCommand("set " + prefix + "session 192.0.2.1 vrf blue interface eth0x label tail")
				require.Error(t, err)
				var refusal *configyang.ValidationError
				require.True(t, errors.As(err, &refusal), "typed third-key refusal: %v", err)
				assert.Equal(t, []string{"multikey-probe", "session", "192.0.2.1", "vrf", "blue", "interface"}, refusal.Segments())
				assert.True(t, strings.HasPrefix(err.Error(), `multikey-probe session 192.0.2.1 vrf blue interface: "eth0x" is not an accepted value`), "%v", err)
				fix := lastLine(err.Error())
				require.Equal(t, "fix: set "+prefix+"session 192.0.2.1 vrf blue interface eth0 label tail", fix)
				assert.Equal(t, before, m.editor.WorkingContent())
				_, err = m.dispatchCommand(strings.TrimPrefix(fix, "fix: "))
				require.NoError(t, err, "the printed correction must replace only the third key's value")
				entries := m.editor.tree.GetContainer("multikey-probe").GetList("session")
				require.Len(t, entries, 1)
				entry := entries["192.0.2.1 vrf blue interface eth0"]
				require.NotNil(t, entry)
				label, ok := entry.Get("label")
				assert.True(t, ok)
				assert.Equal(t, "tail", label)
			})
		}
	}
}

func multikeyRefusalEntryModel(t *testing.T, user string) *Model {
	t.Helper()
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.AddModuleFromText("ze-model-multikey-refusal-entry-conf.yang", modelMultikeyRefusalEntryModule))
	resolved, err := loader.Resolve()
	require.NoError(t, err)
	schema, err := config.SchemaOf(resolved)
	require.NoError(t, err)
	m, _, _ := newCommitGrammarModel(t, user)
	m.editor.schema = schema
	m.completer = &Completer{schema: resolved}
	return m
}
