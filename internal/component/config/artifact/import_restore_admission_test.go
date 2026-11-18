// Design: docs/architecture/web-interface.md -- private import and restore admission.
package artifact_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestArtifactImportRetainsRecordedConfirmWindow proves raw imports keep the
// native stopped-window remedy and the complete unchanged artifact.
func TestArtifactImportRetainsRecordedConfirmWindow(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	require.NoError(t, fixture.owner.ChangeStore(t.Context(), "alice", id, func(target storage.Storage) error {
		return confirm.NewStoreRecorder(target, "router.conf").Save(confirm.Pending{
			User: "recorded-owner", Deadline: time.Now().Add(time.Hour), Rollback: []byte("previous"),
		})
	}))
	before, err := fixture.owner.Inspect(t.Context(), "alice", id)
	require.NoError(t, err)
	input := artifactBinarySource(t, fixture, []byte("replacement"))
	response := artifactOperationCall(t, fixture, "request config artifact import artifact "+id+" name router.conf transfer "+input)
	require.Equal(t, plugin.StatusError, response.Status)
	require.Contains(t, response.Error, "recorded-owner")
	require.Contains(t, response.Error, "ze start router.conf")
	after, err := fixture.owner.Inspect(t.Context(), "alice", id)
	require.NoError(t, err)
	require.Equal(t, before.Digest, after.Digest)
}

// TestArtifactImportRefusesHostPaths proves the token adapter accepts only a
// caller's configuration basename, never a path resolved on the daemon host.
func TestArtifactImportRefusesHostPaths(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	input := artifactBinarySource(t, fixture, []byte("replacement"))
	before, err := fixture.owner.Inspect(t.Context(), "alice", id)
	require.NoError(t, err)
	for _, name := range []string{"../router.conf", "/tmp/router.conf", "file/active/router.conf", "-"} {
		response := artifactOperationCall(t, fixture, "request config artifact import artifact "+id+" name "+name+" transfer "+input)
		require.Equal(t, plugin.StatusError, response.Status)
	}
	after, err := fixture.owner.Inspect(t.Context(), "alice", id)
	require.NoError(t, err)
	require.Equal(t, before.Digest, after.Digest)
}

// TestArtifactSelectedRestoreChecksPrivateAdmission proves authorization,
// account ownership, cancellation and corrupt complete sources all refuse
// before the current runtime's reload or active state changes.
func TestArtifactSelectedRestoreChecksPrivateAdmission(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	id := artifactOperationUpload(t, fixture)
	store, target := selectedRestoreTarget(t)
	calls := 0
	target.Reload = func(context.Context) error { calls++; return storage.PromoteCandidate(store, "device.conf") }
	for _, boundary := range []string{"read-only", "other account", "canceled", "corrupt"} {
		t.Run(boundary, func(t *testing.T) {
			caller := plugin.CallerIdentity{Username: "alice"}
			ctx := t.Context()
			selected := id
			switch boundary {
			case "read-only":
				caller.Authorizer = policyGrant{edit: false}
			case "other account":
				caller.Username = "bob"
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "corrupt":
				selected = artifactBinarySource(t, fixture, []byte("not a framed artifact"))
			}
			response, err := fixture.dispatch(ctx, caller, "request config artifact restore config artifact "+selected+" name router.conf")
			if boundary == "read-only" {
				require.ErrorIs(t, err, pluginserver.ErrUnauthorized)
			} else {
				require.NoError(t, err)
			}
			require.NotNil(t, response)
			require.Equal(t, plugin.StatusError, response.Status)
			if boundary == "read-only" {
				require.Empty(t, response.Data)
			}
			require.Zero(t, calls)
			current, err := storage.ReadActiveConfig(store, "device.conf")
			require.NoError(t, err)
			require.Equal(t, "original runtime", string(current))
		})
	}
}
