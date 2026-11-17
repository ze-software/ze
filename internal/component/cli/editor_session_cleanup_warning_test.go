// Design: docs/guide/config-editor.md -- applied cleanup warnings retain recovery advice.
// Related: editor_session_cleanup_failure_test.go -- real post-publication removal refusal.

package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestSessionCleanupWarningRecoveryAdvice drives real publication followed by
// the existing guarded removal fault. Both routes must retain the cause and
// applied status, and name a shared read-only command that inspects remaining
// session intent without asking the operator to commit or discard it.
func TestSessionCleanupWarningRecoveryAdvice(t *testing.T) {
	for _, route := range []string{"direct", "candidate"} {
		t.Run(route, func(t *testing.T) {
			fixture := newSessionCleanupFixture(t, "ssh")
			store := &sessionCleanupFailureStore{Storage: fixture.store,
				configPath: fixture.path, draftPath: DraftPath(fixture.path)}
			fixture.store = store
			fixture.current.store = store
			var warning string
			if route == "direct" {
				result, err := fixture.current.CommitSession()
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, 1, result.Applied)
				require.Len(t, result.Warnings, 1)
				warning = result.Warnings[0]
			} else {
				result, content, err := fixture.current.CommitSessionCandidate(time.Now())
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, 1, result.Applied)
				require.NoError(t, storage.PromoteCandidate(store, fixture.path))
				err = fixture.current.MarkCommittedContent(content)
				require.ErrorIs(t, err, errSessionCleanupRemove)
				var operatorWarning *opererr.Error
				require.ErrorAs(t, err, &operatorWarning)
				assert.Equal(t, "show | changes all", operatorWarning.Fix())
				warning = err.Error()
			}
			require.True(t, store.published)
			require.Equal(t, 1, store.refused)
			assert.Contains(t, warning, "commit applied")
			assert.Contains(t, warning, errSessionCleanupRemove.Error())
			assert.Contains(t, warning, "do not repeat the commit")
			assert.Contains(t, warning, "Keep the session files")
			assert.Contains(t, warning, "review the retained changes")
			assert.True(t, strings.HasSuffix(warning, "\nfix: show | changes all"))
			fixture.checkOther(t)
		})
	}
}
