// Design: docs/guide/config-editor.md -- refused keys create no entry.
// Related: editor_draft.go -- writeThroughCreate
// Related: model_commands_edit.go -- cmdInsert, runActivation

package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

const keyActivationProbe = `module ze-key-activation-probe-conf {
  yang-version 1.1;
  namespace "urn:ze:key-activation-probe";
  prefix probe;
  container key-probe {
    list lane {
      key "kind";
      leaf kind { type enumeration { enum blue; enum red; } }
      leaf-list member { type string; ordered-by user; }
      list child {
        key "name";
        leaf name { type string; }
      }
    }
  }
}`

// TestKeyCheckOnInsertNestedActivation proves every creating editor operation
// judges an ancestor's enum key before changing either file or session state.
// VALIDATES: AC-29..31 -- the complete command is the fix, nothing is stored,
// and the corresponding accepted key reaches the same operation successfully.
func TestKeyCheckOnInsertNestedActivation(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-key-activation-probe-conf.yang", keyActivationProbe))
	for _, user := range []string{"", "alice"} {
		mode := "file"
		if user != "" {
			mode = "session"
		}
		for _, command := range []string{
			"insert key-probe lane bogus member one last",
			"set key-probe lane bogus child name one",
			"deactivate key-probe lane bogus member one",
			"activate key-probe lane bogus member one",
		} {
			t.Run(mode+"/"+strings.Fields(command)[0], func(t *testing.T) {
				m, _, _ := newCommitGrammarModel(t, user)
				before := m.editor.WorkingContent()
				_, err := m.dispatchCommand(command)
				require.Error(t, err)
				var refusal *configyang.ValidationError
				require.True(t, errors.As(err, &refusal), "typed refusal: %v", err)
				assert.Contains(t, err.Error(), `key-probe lane bogus: "bogus" is not an accepted value`)
				assert.Equal(t, "fix: "+strings.Replace(command, "bogus", "<blue|red>", 1), lastLine(err.Error()))
				assert.Equal(t, before, m.editor.WorkingContent())
				assert.Empty(t, m.editor.tree.GetContainer("key-probe"), "no ancestor created")
				if user != "" {
					assert.Empty(t, m.editor.PendingChanges(m.editor.session.ID))
				}

				if !strings.HasPrefix(command, "set ") {
					member := "one"
					if strings.HasPrefix(command, "insert ") {
						member = "zero"
					}
					_, err = m.dispatchCommand("set key-probe lane blue member " + member)
					require.NoError(t, err)
					if strings.HasPrefix(command, "activate ") {
						_, err = m.dispatchCommand("deactivate key-probe lane blue member one")
						require.NoError(t, err)
					}
				}
				_, err = m.dispatchCommand(strings.Replace(command, "bogus", "blue", 1))
				require.NoError(t, err, "accepted key reaches the same operation")
				assert.NotNil(t, m.editor.tree.GetContainer("key-probe").GetList("lane")["blue"])
			})
		}
	}
}

// TestSessionCreateRefusalStartsWithTheSetting proves a session's direct entry
// creation preserves the shared refusal body and makes no change-file entry.
func TestSessionCreateRefusalStartsWithTheSetting(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-key-activation-probe-conf.yang", keyActivationProbe))
	ed, session, _ := newLoadSessionEditor(t, validBGPConfig)
	err := ed.CreateEntry([]string{"key-probe", "lane", "bogus"})
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), `key-probe lane bogus: "bogus" is not an accepted value`), "%v", err)
	assert.Empty(t, ed.PendingChanges(session.ID))
	assert.Nil(t, ed.tree.GetContainer("key-probe"))
	require.NoError(t, ed.CreateEntry([]string{"key-probe", "lane", "blue"}))
	assert.NotNil(t, ed.tree.GetContainer("key-probe").GetList("lane")["blue"])
}
