// Design: docs/architecture/hub-architecture.md -- cleanup failures forbid a clean restart.
// Related: main.go -- actual engine teardown and deferred service ownership.
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
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/ze"
)

type cleanupResultSubsystem struct {
	failure error
	calls   *[]string
}

func (*cleanupResultSubsystem) Name() string { return "cleanup-result-subsystem" }
func (*cleanupResultSubsystem) Start(context.Context, ze.EventBus, ze.ConfigProvider) error {
	return nil
}
func (s *cleanupResultSubsystem) Stop(context.Context) error {
	*s.calls = append(*s.calls, "engine")
	return s.failure
}
func (*cleanupResultSubsystem) Reload(context.Context, ze.ConfigProvider) error { return nil }

type cleanupResultService struct {
	name    string
	failure error
	calls   *[]string
}

func (s *cleanupResultService) Name() string                              { return s.name }
func (*cleanupResultService) Addresses() []string                         { return nil }
func (*cleanupResultService) Reconfigure(context.Context, []string) error { return nil }
func (s *cleanupResultService) Shutdown(context.Context) error {
	*s.calls = append(*s.calls, s.name)
	return s.failure
}

// TestHubCleanupPreservesCleanExitCodes runs real reactorless boot, authorized
// stop/restart dispatch and teardown; successful borrowers keep exit 0 or 75.
func TestHubCleanupPreservesCleanExitCodes(t *testing.T) {
	for _, test := range []struct {
		command string
		exit    int
	}{
		{command: "request shutdown", exit: 0},
		{command: "request restart", exit: 75},
	} {
		t.Run(test.command, func(t *testing.T) {
			exit, _, calls := runCleanupResult(t, test.command, nil, nil, nil, nil)
			require.Equal(t, test.exit, exit)
			require.Equal(t, []string{"engine", "service-a", "service-b"}, calls)
		})
	}
}

// TestHubCleanupEngineFailureRefusesRestart injects a real engine subsystem's
// stop failure and proves neither a normal stop nor an intentional restart
// reports success; both deferred service owners still finish.
func TestHubCleanupEngineFailureRefusesRestart(t *testing.T) {
	for _, line := range []string{"request shutdown", "request restart"} {
		t.Run(line, func(t *testing.T) {
			failure := errors.New("engine cleanup result sentinel")
			exit, diagnostic, calls := runCleanupResult(t, line, failure, nil, nil, nil)
			require.Equal(t, 1, exit)
			require.Contains(t, diagnostic, failure.Error())
			require.Equal(t, []string{"engine", "service-a", "service-b"}, calls)
		})
	}
}

// TestHubCleanupDeferredServiceFailureRefusesRestart injects each deferred
// service owner in turn after an accepted real restart. Any failed owner must
// replace exit 75 with 1 without skipping the next owner.
func TestHubCleanupDeferredServiceFailureRefusesRestart(t *testing.T) {
	for _, name := range []string{"service-a", "service-b"} {
		t.Run(name, func(t *testing.T) {
			failure := errors.New(name + " cleanup result sentinel")
			var first, second error
			if name == "service-a" {
				first = failure
			} else {
				second = failure
			}
			exit, diagnostic, calls := runCleanupResult(t, "request restart", nil, first, second, nil)
			require.Equal(t, 1, exit)
			require.Contains(t, diagnostic, failure.Error())
			require.Equal(t, []string{"engine", "service-a", "service-b"}, calls)
		})
	}
}

// TestHubCleanupRetainsEveryReportedCause fails the engine and both deferred
// service owners during real teardown. The final diagnostic must retain each
// cause and the operator remedy, while every registered owner is still called.
func TestHubCleanupRetainsEveryReportedCause(t *testing.T) {
	engineFailure := errors.New("engine cleanup result sentinel")
	first := errors.New("first service cleanup result sentinel")
	second := errors.New("second service cleanup result sentinel")
	exit, diagnostic, calls := runCleanupResult(t, "request restart", engineFailure, first, second, nil)
	require.Equal(t, 1, exit)
	for _, failure := range []error{engineFailure, first, second} {
		require.Contains(t, diagnostic, failure.Error())
	}
	require.Contains(t, diagnostic, "ze doctor")
	require.Equal(t, []string{"engine", "service-a", "service-b"}, calls)
}

// runCleanupResult uses the same native construction seams as the existing
// restart-exit integration test. All mutations are test-owned and restored;
// runYANGConfig still owns the actual stop and every deferred callback.
func runCleanupResult(t *testing.T, line string, engineFailure, first, second error, apiShutdown func(context.Context) error) (int, string, []string) {
	t.Helper()
	resetAAABundleForTest(t)
	clearAPIEnv(t)
	withCleanRegistry(t)
	stopRestartCause.Store(nil)
	rebootRequested.Store(false)
	t.Cleanup(func() {
		stopRestartCause.Store(nil)
		rebootRequested.Store(false)
	})
	oldDir := env.Get("ze.config.dir")
	require.NoError(t, env.Set("ze.config.dir", t.TempDir()))
	t.Cleanup(func() { require.NoError(t, env.Set("ze.config.dir", oldDir)) })
	oldREST, oldGRPC, oldBNG := restBuild, grpcBuild, bngRegister
	t.Cleanup(func() { restBuild, grpcBuild, bngRegister = oldREST, oldGRPC, oldBNG })
	var calls []string
	bngRegister = func(_ *zeconfig.Tree, _ map[string]any, e *engine.Engine) ([]webPortalService, error) {
		return nil, e.RegisterSubsystem(&cleanupResultSubsystem{failure: engineFailure, calls: &calls})
	}
	for _, input := range []struct {
		name    string
		failure error
	}{{name: "service-a", failure: first}, {name: "service-b", failure: second}} {
		registerService(input.name, func(*serviceDeps) (Service, error) {
			return &cleanupResultService{name: input.name, failure: input.failure, calls: &calls}, nil
		}, nil)
	}
	restBuild = nil
	dispatched := false
	grpcBuild = func(in *apiBuildInputs, _ *apiShared) (apiServerHandle, error) {
		response, err := serverDispatcher(in.Server, audit.GRPC)(t.Context(),
			plugin.CallerIdentity{Username: "admin", Authorizer: authz.StoreAuthorizer{
				Store: daemonRestartSurfacePolicy(),
			}}, line)
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, plugin.StatusDone, response.Status)
		dispatched = true
		response.TransportComplete()
		if apiShutdown != nil {
			return apiServerHandle{
				Server: &cleanupResultService{name: "api", calls: &calls},
				Shutdown: func(ctx context.Context) error {
					calls = append(calls, "api")
					return apiShutdown(ctx)
				},
			}, nil
		}
		return apiServerHandle{}, nil
	}
	output, err := os.CreateTemp(t.TempDir(), "cleanup-stderr-")
	require.NoError(t, err)
	original := os.Stderr
	os.Stderr = output
	defer func() {
		os.Stderr = original
		require.NoError(t, output.Close())
	}()
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
	require.True(t, dispatched, "boot must reach real command acceptance before its exit is judged")
	diagnostic, err := os.ReadFile(output.Name())
	require.NoError(t, err)
	return exit, string(diagnostic), calls
}
