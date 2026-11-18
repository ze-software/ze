// Design: docs/architecture/api/commands.md -- artifact refusal and cleanup evidence.
package cli

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/artifact"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestArtifactOperationFailureRetainsJoinedCleanup exercises the actual adapter
// over a real domain refusal: its status and pure text stay intact, while an
// independent joined cleanup failure must remain visible beside that refusal.
func TestArtifactOperationFailureRetainsJoinedCleanup(t *testing.T) {
	owner := artifact.New()
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	_, missing := owner.Inspect(t.Context(), "alice", "absent-artifact")
	var domain *artifact.Refusal
	require.ErrorAs(t, missing, &domain)
	cleanup := errors.New("artifact cleanup could not remove its retained source")
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "native refusal", err: missing},
		{name: "single joined refusal", err: errors.Join(missing)},
		{name: "refusal and cleanup", err: errors.Join(missing, cleanup)},
		{name: "nested refusal and cleanup", err: errors.Join(errors.Join(missing), cleanup)},
	} {
		t.Run(test.name, func(t *testing.T) {
			answer := artifactOperationFailure(test.err)
			require.NotNil(t, answer)
			assert.Equal(t, plugin.StatusError, answer.Status)
			if errors.Is(test.err, cleanup) {
				assert.Contains(t, answer.Error, missing.Error())
				assert.Contains(t, answer.Error, cleanup.Error(), "the domain category must not discard a sibling cleanup failure")
				data, ok := answer.Data.(plugin.Map)
				require.True(t, ok)
				assert.Equal(t, http.StatusInternalServerError, data["http-status"], "an independent cleanup failure is a server failure")
				return
			}
			assert.Equal(t, missing.Error(), answer.Error, "a pure native refusal already owns its complete correction")
			data, ok := answer.Data.(plugin.Map)
			require.True(t, ok)
			assert.Equal(t, domain.Status, data["http-status"])
		})
	}
}
