// Design: docs/architecture/cli/error-surface.md -- framed refusals keep their type and mask secrets.

package web

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestWebTypedKeyRefusalMasksTheFollowingSecret exercises the shared terminal
// and CLI bar set producer before an HTTP writer renders the error to text.
func TestWebTypedKeyRefusalMasksTheFollowingSecret(t *testing.T) {
	const module = `module ze-web-key-secret-conf {
  yang-version 1.1;
  namespace "urn:ze:web-key-secret";
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
	t.Cleanup(configyang.RegisterModuleForTest("ze-web-key-secret-conf.yang", module))
	for _, secret := range []string{"private-value", "private value", `private"value`, `private\value`} {
		t.Run(secret, func(t *testing.T) {
			mgr, _ := newYANGTerminal(t)
			args := []string{"key-secret", "lane", "blu", "secret", secret}
			_, err := applyWebSet(mgr, mgr.schema, "testuser", nil, nil, args)
			require.Error(t, err)
			var refusal *configyang.ValidationError
			assert.True(t, errors.As(err, &refusal), "the value refusal remains typed")
			var operator *opererr.Error
			assert.True(t, errors.As(err, &operator), "the operator error remains typed")
			assert.NotContains(t, err.Error(), secret)
			assert.Equal(t, "fix: set key-secret lane blue secret /* SECRET-DATA */", lastLine(err.Error()))
			if tree := mgr.Tree("testuser"); tree != nil {
				assert.Nil(t, tree.GetContainer("key-secret"))
			}
		})
	}
}
