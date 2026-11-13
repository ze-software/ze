// Design: docs/architecture/api/architecture.md -- session key refusals retain the complete set command.

package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestEditorSetValueFramesAncestorKeyRefusal exercises the direct editor call
// that REST, gRPC and gNMI share without a model to frame its error afterward.
// VALIDATES: AC-43 -- the enum key refusal retains the rest of the set command,
// both in file and session modes, and leaves the candidate unchanged.
func TestEditorSetValueFramesAncestorKeyRefusal(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		t.Run("user="+user, func(t *testing.T) {
			m, _, _ := newCommitGrammarModel(t, user)
			before := m.editor.WorkingContent()
			err := m.editor.SetValue([]string{"ospf", "redistribute", "bogus"}, "nssa-propagate", "true")
			require.Error(t, err)
			var refusal *configyang.ValidationError
			require.True(t, errors.As(err, &refusal), "%v", err)
			assert.True(t, strings.HasPrefix(err.Error(), `ospf redistribute bogus: "bogus" is not an accepted value`), "%v", err)
			assert.Equal(t, "fix: set ospf redistribute <connected|static|kernel|bgp|isis> nssa-propagate true", lastLine(err.Error()))
			assert.Equal(t, before, m.editor.WorkingContent())
			assert.Nil(t, m.editor.tree.GetContainer("ospf"))
			if user != "" {
				assert.Empty(t, m.editor.PendingChanges(m.editor.session.ID))
			}

			require.NoError(t, m.editor.SetValue([]string{"ospf", "redistribute", "connected"}, "nssa-propagate", "true"))
			entry := m.editor.tree.GetContainer("ospf").GetList("redistribute")["connected"]
			require.NotNil(t, entry)
			value, ok := entry.Get("nssa-propagate")
			assert.True(t, ok)
			assert.Equal(t, "true", value)
		})
	}
}

// TestEditorKeyRefusalMasksTheFollowingSecret proves that retaining the rest
// of a refused set command cannot reveal a sensitive leaf's supplied value.
func TestEditorKeyRefusalMasksTheFollowingSecret(t *testing.T) {
	const module = `module ze-key-refusal-secret-conf {
  yang-version 1.1;
  namespace "urn:ze:key-refusal-secret";
  prefix probe;
  import ze-extensions { prefix ze; }
  container key-secret {
    list lane {
      key "kind";
      leaf kind { type enumeration { enum blue; enum red; } }
      leaf secret { type string; ze:sensitive; }
    }
  }
}`
	t.Cleanup(configyang.RegisterModuleForTest("ze-key-refusal-secret-conf.yang", module))
	m, _, _ := newCommitGrammarModel(t, "")
	err := m.editor.SetValue([]string{"key-secret", "lane", "bogus"}, "secret", "private-value")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "private-value")
	assert.Equal(t, "fix: set key-secret lane <blue|red> secret /* SECRET-DATA */", lastLine(err.Error()))
	assert.Nil(t, m.editor.tree.GetContainer("key-secret"))
}
