// Design: docs/guide/config-editor.md -- discard is scoped to the editing session.
// Related: editor_session_cleanup_test.go -- same-account ownership fixture.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiscardAllPreservesOtherSessionIntent removes the current session's
// assignment while retaining the same account's other assignment and copy,
// byte-for-byte in their shared change file. Both web and SSH callers use it.
func TestDiscardAllPreservesOtherSessionIntent(t *testing.T) {
	for _, origin := range []string{"ssh", "web"} {
		t.Run(origin, func(t *testing.T) {
			fixture := newSessionCleanupFixture(t, origin)
			require.NoError(t, fixture.current.DiscardSessionPath(nil))
			fixture.checkOther(t)
			assert.Empty(t, fixture.current.PendingChanges(fixture.current.SessionID()))
		})
	}
}

// TestDiscardAllPreservesContestedSameAccountLeaf drops one value of a leaf
// with two same-account owners and retains the other owner's exact metadata
// and value, rather than keeping a sparse record of an already-deleted value.
func TestDiscardAllPreservesContestedSameAccountLeaf(t *testing.T) {
	fixture := newSessionCleanupFixture(t, "ssh")
	ssh := fixture.owner(t, "ssh", 1)
	require.NoError(t, ssh.SetValue([]string{"system"}, "host", "ssh-pending"))
	require.NoError(t, fixture.current.SetValue([]string{"system"}, "host", "web-pending"))
	before, _, err := fixture.readChanges()
	require.NoError(t, err)
	sshEntries := before.SessionEntries(fixture.current.schema, fixture.otherSession)
	require.NoError(t, fixture.current.DiscardSessionPath(nil))
	after, _, err := fixture.readChanges()
	require.NoError(t, err)
	assert.ElementsMatch(t, sshEntries, after.SessionEntries(fixture.current.schema, fixture.otherSession))
	assert.Empty(t, after.SessionEntries(fixture.current.schema, fixture.current.SessionID()))
	host, found := fixture.current.Tree().GetContainer("system").Get("host")
	require.True(t, found)
	assert.Equal(t, "ssh-pending", host)
}
