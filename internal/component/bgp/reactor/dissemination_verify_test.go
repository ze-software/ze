// Design: docs/architecture/api/process-protocol.md -- runtime verify ownership gate.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestVerifyRejectsIncompleteRuntimeOwner proves a prospective ownership refusal
// happens in verification, preserving the useful established destination.
func TestVerifyRejectsIncompleteRuntimeOwner(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	before := f.peer.selectedOwnerSnapshot()
	candidate := f.peer.settingsSnapshot()
	f.api.r.SetConfigPath("runtime-owner-test")
	f.api.r.SetReloadFunc(func(string) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error) {
		return []*PeerSettings{candidate}, Globals{}, nil, nil
	})
	err := f.api.VerifyConfig(map[string]any{"bgp": map[string]any{}})
	require.Error(t, err, "destination grants alone cannot complete a runtime output owner")
	require.Same(t, before, f.peer.selectedOwnerSnapshot(), "failed candidate must not replace live authority")
	require.Same(t, f.peer, f.api.r.peers[f.peer.settings.PeerKey()])
}
