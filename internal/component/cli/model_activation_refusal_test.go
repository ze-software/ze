// Design: docs/guide/config-editor.md -- refusals and their fix lines
// Related: model_commands_edit.go -- runActivation, activationRefusal
// Related: key_refusal.go -- FramedKeyRefusal

package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"

	_ "github.com/ze-software/ze/internal/plugins/ospf/yang"
)

// TestActivationFramesAKeyRefusal proves activate and deactivate answer a key
// the list refuses as the shared key refusal, with the line typed as the fix
// and the key corrected, at the root and inside a context, and answer any
// other error as it is.
// Method: the refusal ValidateListKey gives for `ospf redistribute bogus`,
// wrapped as ApplyActivation wraps its errors, is passed to
// activationRefusal for each verb and position; then a real `deactivate`
// under a key no entry holds is typed, in file mode.
//
// VALIDATES: AC-31 -- refused, nothing marked; the fix the `activate` or
// `deactivate` line with the key replaced (as the web terminal frames it).
// PREVENTS: `deactivate failed: ...` naming the key alone, with a `set` fix
// that is not the command the operator typed.
// Break: return err unframed from activationRefusal; the framed cases go red.
func TestActivationFramesAKeyRefusal(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	listPath := []string{"ospf", "redistribute"}
	list, isList := schema.LookupTokenPath(listPath).(*config.ListNode)
	require.True(t, isList, "ospf redistribute is a list")
	keyErr := config.ValidateListKey(list, listPath, "bogus")
	require.Error(t, keyErr)
	wrapped := fmt.Errorf("deactivate failed: %w", keyErr)

	for _, verb := range []string{"activate", "deactivate"} {
		t.Run(verb+" at the root", func(t *testing.T) {
			m, _ := topTestModel(t)
			got := m.activationRefusal(wrapped, verb, []string{"ospf", "redistribute", "bogus", "nssa-propagate"})
			_, ok := errors.AsType[*configyang.ValidationError](got)
			require.True(t, ok, "the shared refusal: %v", got)
			assert.True(t, strings.HasPrefix(got.Error(), `ospf redistribute bogus: "bogus" is not an accepted value`), "text %q", got)
			assert.Equal(t, "fix: "+verb+" ospf redistribute "+ospfSources+" nssa-propagate", lastLine(got.Error()))
			assert.NotContains(t, got.Error(), "failed")
		})
		t.Run(verb+" in a context", func(t *testing.T) {
			m, _ := topTestModel(t)
			m.contextPath = []string{"ospf"}
			got := m.activationRefusal(wrapped, verb, []string{"redistribute", "bogus", "nssa-propagate"})
			assert.Equal(t, "fix: "+verb+" redistribute "+ospfSources+" nssa-propagate", lastLine(got.Error()))
		})
	}

	t.Run("another error as it is", func(t *testing.T) {
		m, _ := topTestModel(t)
		other := errors.New("deactivate failed: path not found")
		assert.Equal(t, other, m.activationRefusal(other, "deactivate", []string{"ospf"}))
	})

	t.Run("nothing marked under a key no entry holds", func(t *testing.T) {
		m, _ := topTestModel(t)
		before := m.editor.WorkingContent()
		out, err := refusedLine(t, m, "deactivate ospf redistribute bogus nssa-propagate")
		require.Error(t, err)
		assert.Equal(t, before, out.editor.WorkingContent(), "nothing marked")
	})
}
