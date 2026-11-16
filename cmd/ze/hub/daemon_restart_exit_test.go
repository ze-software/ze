// Design: docs/architecture/hub-architecture.md -- supervisor restart exit contract.
package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/env"
)

// TestDaemonRestartExitStatus runs the actual reactorless boot and teardown.
// Its API construction seam dispatches an authorized command without opening
// a listener or delivering a host signal. Only intentional restart exits 75;
// normal shutdown remains a clean stop that on-failure must leave stopped.
func TestDaemonRestartExitStatus(t *testing.T) {
	for _, tc := range []struct {
		command string
		exit    int
	}{
		{command: "request shutdown", exit: 0},
		{command: "request restart", exit: 75},
	} {
		t.Run(tc.command, func(t *testing.T) {
			resetAAABundleForTest(t)
			clearAPIEnv(t)
			stopRestartCause.Store(nil)
			rebootRequested.Store(false)
			t.Cleanup(func() {
				stopRestartCause.Store(nil)
				rebootRequested.Store(false)
			})
			originalDir := env.Get("ze.config.dir")
			require.NoError(t, env.Set("ze.config.dir", t.TempDir()))
			t.Cleanup(func() { require.NoError(t, env.Set("ze.config.dir", originalDir)) })
			originalREST, originalGRPC := restBuild, grpcBuild
			t.Cleanup(func() { restBuild, grpcBuild = originalREST, originalGRPC })
			restBuild = nil
			dispatched := false
			grpcBuild = func(in *apiBuildInputs, _ *apiShared) (apiServerHandle, error) {
				response, err := serverDispatcher(in.Server, audit.GRPC)(t.Context(),
					plugin.CallerIdentity{Username: "admin", Authorizer: authz.StoreAuthorizer{
						Store: daemonRestartSurfacePolicy(),
					}}, tc.command)
				require.NoError(t, err)
				require.NotNil(t, response)
				require.Equal(t, plugin.StatusDone, response.Status)
				dispatched = true
				response.TransportComplete()
				return apiServerHandle{}, nil
			}
			config := []byte(`environment {
	api-server {
		grpc {
			enabled true
			server main { ip 127.0.0.1; port 0; }
		}
	}
}`)
			exit := runYANGConfig(newTestStore(t), "-", config, nil, 0, -1,
				false, false, "", false, "", "", false, nil)
			require.True(t, dispatched, "boot must reach the lifecycle command before judging its exit")
			assert.Equal(t, tc.exit, exit)
		})
	}
}
