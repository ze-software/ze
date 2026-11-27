// Design: docs/architecture/hub-architecture.md -- teardown joins already-built borrowers in order.
// Design: docs/architecture/core-design.md -- a transport that cannot build stays down while ze keeps running.
package hub

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/engine"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/env"
)

// TestHubCleanupStartupFailureJoinsBuiltAPI refuses the second transport's
// constructor after the first acquired a borrower. Under the always-starts
// ruling, ze reports the failed transport under doctor-service-not-started and
// keeps booting; subsequent shutdown must still join the built API borrower
// before the plugin server and the engine, turning a failed API shutdown into
// exit 1 with its cause and "ze doctor".
func TestHubCleanupStartupFailureJoinsBuiltAPI(t *testing.T) {
	for _, failCleanup := range []bool{false, true} {
		name := "clean API shutdown"
		if failCleanup {
			name = "failed API shutdown"
		}
		t.Run(name, func(t *testing.T) {
			resetAAABundleForTest(t)
			resetServicesNotStarted()
			clearAPIEnv(t)
			withCleanRegistry(t)
			stopRestartCause.Store(nil)
			rebootRequested.Store(false)
			t.Cleanup(func() {
				resetServicesNotStarted()
				stopRestartCause.Store(nil)
				rebootRequested.Store(false)
			})
			oldDir := env.Get("ze.config.dir")
			require.NoError(t, env.Set("ze.config.dir", t.TempDir()))
			t.Cleanup(func() { require.NoError(t, env.Set("ze.config.dir", oldDir)) })
			oldREST, oldGRPC, oldBNG := restBuild, grpcBuild, bngRegister
			t.Cleanup(func() { restBuild, grpcBuild, bngRegister = oldREST, oldGRPC, oldBNG })
			var calls []string
			bngRegister = func(_ *zeconfig.Tree, _ map[string]any, _ string, e *engine.Engine) ([]webPortalService, []serviceNotStarted) {
				require.NoError(t, e.RegisterSubsystem(&cleanupResultSubsystem{calls: &calls}))
				return nil, nil
			}
			cleanupFailure := errors.New("startup API cleanup sentinel")
			var capturedServer *pluginserver.Server
			restBuild = func(in *apiBuildInputs, _ *apiShared) (apiServerHandle, error) {
				capturedServer = in.Server
				_, err := in.Server.OnStop(func() error {
					calls = append(calls, "server")
					return nil
				})
				require.NoError(t, err)
				return apiServerHandle{
					Server: &cleanupResultService{name: "api", calls: &calls},
					Shutdown: func(context.Context) error {
						calls = append(calls, "api")
						if failCleanup {
							return cleanupFailure
						}
						return nil
					},
				}, nil
			}
			constructorFailure := errors.New("second API constructor sentinel")
			grpcBuild = func(*apiBuildInputs, *apiShared) (apiServerHandle, error) {
				return apiServerHandle{}, constructorFailure
			}
			// Register a post-API service factory to prove ze continued booting
			// past the failed gRPC constructor and then drive real teardown.
			reachedBuildServices := false
			registerService("shutdown-trigger", func(*serviceDeps) (Service, error) {
				reachedBuildServices = true
				require.NotNil(t, capturedServer)
				response, err := serverDispatcher(capturedServer, audit.REST)(t.Context(),
					plugin.CallerIdentity{Username: "admin", Authorizer: authz.StoreAuthorizer{
						Store: daemonRestartSurfacePolicy(),
					}}, "request shutdown")
				require.NoError(t, err)
				require.NotNil(t, response)
				require.Equal(t, plugin.StatusDone, response.Status)
				response.TransportComplete()
				return &cleanupResultService{name: "shutdown-trigger", calls: new([]string)}, nil
			}, nil)
			output, err := os.CreateTemp(t.TempDir(), "cleanup-startup-stderr-")
			require.NoError(t, err)
			original := os.Stderr
			os.Stderr = output
			defer func() {
				os.Stderr = original
				require.NoError(t, output.Close())
			}()
			// Enable both always-on API seams via env vars rather than ze_rest/ze_grpc-gated
			// YANG blocks so this startup-cleanup test runs identically under -tags ze_core.
			require.NoError(t, env.Set("ze.api-server.rest.enabled", "1"))
			require.NoError(t, env.Set("ze.api-server.rest.listen", "127.0.0.1:0"))
			require.NoError(t, env.Set("ze.api-server.grpc.enabled", "1"))
			require.NoError(t, env.Set("ze.api-server.grpc.listen", "127.0.0.1:0"))
			config := []byte("environment {}\n")
			exit := runYANGConfig(newTestStore(t), "-", config, nil, 0, -1,
				false, false, "", false, "", "", false, nil)
			require.True(t, reachedBuildServices, "second transport failure must not abort daemon boot")
			down := servicesNotStartedSnapshot()
			require.Len(t, down, 1)
			require.Equal(t, serviceNameGRPC, down[0].service)
			require.Equal(t, codeDoctorServiceNotStarted, down[0].doctorCode())
			require.ErrorIs(t, down[0].problem, constructorFailure)
			diagnostic, err := os.ReadFile(output.Name())
			require.NoError(t, err)
			require.Contains(t, string(diagnostic), constructorFailure.Error())
			require.Equal(t, []string{"api", "server", "engine"}, calls)
			if failCleanup {
				require.Equal(t, 1, exit)
				require.Contains(t, string(diagnostic), cleanupFailure.Error())
				require.Contains(t, string(diagnostic), "ze doctor")
			} else {
				require.Equal(t, 0, exit)
			}
		})
	}
}
