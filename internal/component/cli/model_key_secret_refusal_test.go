// Design: docs/architecture/cli/error-surface.md -- correction frames mask secret operands.

package cli

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestModelKeyRefusalMasksFollowingSecret exercises cmdSet's complete correction
// after the editor refuses an ancestor key, including a root command under top.
func TestModelKeyRefusalMasksFollowingSecret(t *testing.T) {
	const module = `module ze-model-key-secret-conf {
  yang-version 1.1;
  namespace "urn:ze:model-key-secret";
  prefix probe;
  import ze-extensions { prefix ze; }
  container model-secret {
    list lane {
      key "kind";
      leaf kind { type enumeration { enum blue; enum red; } }
      leaf secret { type string; ze:sensitive; }
    }
  }
}`
	t.Cleanup(configyang.RegisterModuleForTest("ze-model-key-secret-conf.yang", module))
	for _, user := range []string{"", "alice"} {
		for _, nested := range []bool{false, true} {
			for _, secret := range []string{"private value", `private"value`, `private\value`} {
				t.Run(user+"/"+secret+"/nested="+strconv.FormatBool(nested), func(t *testing.T) {
					m, _, _ := newCommitGrammarModel(t, user)
					prefix := ""
					if nested {
						m.contextPath = []string{"bgp"}
						prefix = "top "
					}
					before := m.editor.WorkingContent()
					updated := enterLine(t, *m, prefix+"set model-secret lane blu secret "+config.QuoteIfNeeded(secret))
					err := updated.err
					require.Error(t, err)
					var refusal *configyang.ValidationError
					assert.True(t, errors.As(err, &refusal))
					var operator *opererr.Error
					assert.True(t, errors.As(err, &operator))
					assert.Equal(t, "fix: "+prefix+"set model-secret lane blue secret /* SECRET-DATA */", lastLine(err.Error()))
					assert.NotContains(t, err.Error(), secret)
					assert.Equal(t, before, m.editor.WorkingContent())
					if nested {
						assert.Equal(t, []string{"bgp"}, updated.contextPath)
					}
				})
			}
		}
	}
}
