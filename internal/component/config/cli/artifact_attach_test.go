// Design: docs/architecture/web-interface.md -- one server lifetime owns each private artifact owner.
package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/artifact"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestArtifactOwnerAttachmentPreservesItsLifetime proves an embedding host uses
// the same retained owner, cannot replace it, and transfers cleanup only on success.
func TestArtifactOwnerAttachmentPreservesItsLifetime(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	owner := artifact.New()
	require.NoError(t, AttachArtifactOwner(server, owner))
	borrowed, err := serverArtifacts(server)
	require.NoError(t, err)
	require.Same(t, owner, borrowed)
	other := artifact.New()
	t.Cleanup(func() { require.NoError(t, other.Close()) })
	require.Error(t, AttachArtifactOwner(server, other))
	_, err = other.List(t.Context(), "alice")
	require.NoError(t, err, "refused attachment must retain the caller's live owner")
	server.Stop()
	_, err = owner.List(t.Context(), "alice")
	var refusal *artifact.Refusal
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, 410, refusal.Status)
	require.Error(t, AttachArtifactOwner(server, other))
	require.Error(t, AttachArtifactOwner(nil, other))
	require.Error(t, AttachArtifactOwner(server, nil))
}
