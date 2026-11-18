// Design: docs/architecture/hub-architecture.md -- startup failures join already-built borrowers.
package hub

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/engine"
	"github.com/ze-software/ze/internal/core/env"
)

// TestHubCleanupStartupFailureJoinsBuiltAPI refuses the second transport's
// constructor after the first acquired a borrower. Actual startup must join
// that borrower before the engine, including its error in the failed result.
func TestHubCleanupStartupFailureJoinsBuiltAPI(t *testing.T) {
	for _, failCleanup := range []bool{false, true} {
		name := "clean API shutdown"
		if failCleanup {
			name = "failed API shutdown"
		}
		t.Run(name, func(t *testing.T) {
			resetAAABundleForTest(t)
			clearAPIEnv(t)
			withCleanRegistry(t)
			oldDir := env.Get("ze.config.dir")
			require.NoError(t, env.Set("ze.config.dir", t.TempDir()))
			t.Cleanup(func() { require.NoError(t, env.Set("ze.config.dir", oldDir)) })
			oldREST, oldGRPC, oldBNG := restBuild, grpcBuild, bngRegister
			t.Cleanup(func() { restBuild, grpcBuild, bngRegister = oldREST, oldGRPC, oldBNG })
			var calls []string
			bngRegister = func(_ *zeconfig.Tree, _ map[string]any, e *engine.Engine) ([]webPortalService, error) {
				return nil, e.RegisterSubsystem(&cleanupResultSubsystem{calls: &calls})
			}
			cleanupFailure := errors.New("startup API cleanup sentinel")
			restBuild = func(in *apiBuildInputs, _ *apiShared) (apiServerHandle, error) {
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
			output, err := os.CreateTemp(t.TempDir(), "cleanup-startup-stderr-")
			require.NoError(t, err)
			original := os.Stderr
			os.Stderr = output
			defer func() {
				os.Stderr = original
				require.NoError(t, output.Close())
			}()
			config := []byte(`environment {
	api-server {
		rest { enabled true; server main { ip 127.0.0.1; port 0; } }
		grpc { enabled true; server main { ip 127.0.0.1; port 0; } }
	}
}`)
			exit := runYANGConfig(newTestStore(t), "-", config, nil, 0, -1,
				false, false, "", false, "", "", false, nil)
			require.Equal(t, 1, exit)
			diagnostic, err := os.ReadFile(output.Name())
			require.NoError(t, err)
			require.Contains(t, string(diagnostic), constructorFailure.Error())
			require.Equal(t, []string{"api", "server", "engine"}, calls)
			if failCleanup {
				require.Contains(t, string(diagnostic), cleanupFailure.Error())
				require.Contains(t, string(diagnostic), "ze doctor")
			}
		})
	}
}
