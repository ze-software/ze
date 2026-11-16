// Design: docs/architecture/web-interface.md -- editor pending-path projection.

package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
)

// pendingPathContractEditor injects only an editor-contract result. Browser
// requests and saved data never construct PendingChange display strings.
type pendingPathContractEditor struct {
	contract.Editor
	pending []contract.PendingChange
}

func (e *pendingPathContractEditor) PendingChanges(string) []contract.PendingChange {
	return e.pending
}

// TestPendingPathProjectionRefusesBrokenEditorContract distinguishes valid
// quoted data from a programming error in an editor's pending-path renderer.
func TestPendingPathProjectionRefusesBrokenEditorContract(t *testing.T) {
	mgr := newTestEditorManager(t)
	us, err := mgr.GetOrCreate("alice")
	require.NoError(t, err)
	editor := &pendingPathContractEditor{Editor: us.editor}
	us.editor = editor
	valid := []string{"bgp", "peer", `"unterminated`}
	editor.pending = []contract.PendingChange{{Kind: contract.PendingChangeSet, Path: config.FormatEditPath(valid)}}
	assert.Equal(t, []string{encodeConfigPath(valid)}, mgr.pendingChangePaths("alice"))

	invalid := `bgp peer "unterminated`
	_, parseErr := config.ParseEditPath(invalid)
	require.Error(t, parseErr)
	for _, change := range []contract.PendingChange{
		{Kind: contract.PendingChangeSet, Path: invalid},
		{Kind: contract.PendingChangeCopy, OldPath: invalid, NewPath: "bgp peer valid"},
		{Kind: contract.PendingChangeRename, OldPath: "bgp peer valid", NewPath: invalid},
	} {
		editor.pending = []contract.PendingChange{change}
		require.PanicsWithValue(t, "BUG: pending change has invalid editor path: "+parseErr.Error(), func() {
			mgr.pendingChangePaths("alice")
		})
	}
}
