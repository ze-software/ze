//go:build ze_ssh

package hub

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zessh "github.com/ze-software/ze/internal/component/ssh"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// lifecycleReactor counts actual infrastructure graceful stops. It supplies
// the live dispatcher and retains the infrastructure post-start binding.
type lifecycleReactor struct {
	dispatcher      *pluginserver.Dispatcher
	postStart       func()
	stops           atomic.Int32
	stopsForRestart atomic.Int32
}

func (r *lifecycleReactor) SetPostStartFunc(fn func())           { r.postStart = fn }
func (r *lifecycleReactor) Dispatcher() *pluginserver.Dispatcher { return r.dispatcher }
func (r *lifecycleReactor) Stop()                                { r.stops.Add(1) }
func (r *lifecycleReactor) StopForRestart()                      { r.stopsForRestart.Add(1) }

// TestSSHLifecycleEndsServerWait drives the authenticated canonical SSH
// executor through the real infrastructure lifecycle bindings and asserts
// that the plugin server's Wait returns. That Wait is what runYANGConfig's
// waitLoop blocks on, so a callback that stops only the reactor leaves the
// daemon running after the client was told "stopping daemon" and exited 0.
//
// VALIDATES: SSH `stop`, `restart` and `reboot` end the daemon, as
// `request shutdown` and `request reboot` do (handleDaemonShutdown,
// handleDaemonReboot); `reboot` also sets rebootRequested, which runYANGConfig
// reads after waitLoop to reboot the host.
// PREVENTS: a stop that reports success and does not stop.
// It also asserts the stop kind and restart cause the hub's drain RPC will
// carry (stopDrainInput, AC-7), and that `stop` no longer stops the reactor
// before the drain (AC-8).
// MUTATION: drop SignalShutdownRequested from shared lifecycle completion and
// Server.Wait times out; drop the restart-cause action in infraSetup and the
// drain-kind assertion fails. No direct SSH lifecycle callback exists.
func TestSSHLifecycleEndsServerWait(t *testing.T) {
	tests := []struct {
		name            string
		command         string
		stops           int32
		stopsForRestart int32
		reboot          bool
		drain           rpc.DrainInput
	}{
		// stop stops no reactor itself: the hub's teardown sends the Cease
		// after the drain (spec-daemon-drain-before-stop AC-8).
		{name: "stop", command: "request shutdown",
			drain: rpc.DrainInput{Kind: rpc.DrainKindStop}},
		{name: "restart", command: "request restart", stopsForRestart: 1,
			drain: rpc.DrainInput{Kind: rpc.DrainKindRestart, RestartCause: rpc.DrainRestartInPlace}},
		{name: "reboot", command: "request reboot", stopsForRestart: 1, reboot: true,
			drain: rpc.DrainInput{Kind: rpc.DrainKindRestart, RestartCause: rpc.DrainRestartReboot}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetAAABundleForTest(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			apiServer, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, &reloadTestReactor{})
			require.NoError(t, err)
			require.NoError(t, apiServer.StartWithContext(context.Background()))
			t.Cleanup(apiServer.Stop)
			// rebootRequested is package state the reboot callback writes;
			// leave it as every other test expects to find it.
			rebootRequested.Store(false)
			t.Cleanup(func() { rebootRequested.Store(false) })
			stopRestartCause.Store(nil)
			t.Cleanup(func() { stopRestartCause.Store(nil) })

			reactor := &lifecycleReactor{dispatcher: apiServer.Dispatcher()}
			apiServer.SetShutdownFunc(func() {})
			params := infra.HookParams{
				Reactor:            reactor,
				APIServer:          func() *pluginserver.Server { return apiServer },
				FormatResponseData: func(any) string { return "accepted" },
			}
			infraSetup(params, nil, &configMutationOwner{}, nil)
			require.NotNil(t, reactor.postStart)
			reactor.postStart()
			sshSrv, err := zessh.NewServer(zessh.Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
			require.NoError(t, err)
			sshWireImpl(sshSrv, &sshWireInputs{
				Reactor: reactor,
				Params:  params,
			})
			policy := authz.NewStore()
			policy.AddProfile(authz.Profile{Name: "lifecycle-admin", Edit: authz.Section{
				Default: authz.Deny,
				Entries: []authz.Entry{
					{Number: 10, Action: authz.Allow, Match: "request shutdown"},
					{Number: 20, Action: authz.Allow, Match: "request restart"},
					{Number: 30, Action: authz.Allow, Match: "request reboot"},
				},
			}})
			policy.AssignProfiles("admin", []string{"lifecycle-admin"})
			executor := sshSrv.ExecutorForUser(ctx, "admin", "192.0.2.45:2200",
				authz.StoreAuthorizer{Store: policy})
			require.NotNil(t, executor)
			response, dispatchErr := executor(tt.command)
			require.NoError(t, dispatchErr)
			require.NotNil(t, response)
			require.Equal(t, plugin.StatusDone, response.Response.Status)

			waitDone := make(chan error, 1)
			go func() { waitDone <- apiServer.Wait(ctx) }()

			response.TransportComplete()
			select {
			case err := <-waitDone:
				require.NoError(t, err, "Server.Wait ended on its deadline, not on the %s", tt.name)
			case <-ctx.Done():
				t.Fatalf("SSH %s stopped the reactor but never ended Server.Wait", tt.name)
			}
			assert.Equal(t, tt.stops, reactor.stops.Load(), "reactor Stop calls")
			assert.Equal(t, tt.stopsForRestart, reactor.stopsForRestart.Load(), "reactor StopForRestart calls")
			assert.Equal(t, tt.reboot, rebootRequested.Load(), "rebootRequested after the %s", tt.name)
			assert.Equal(t, tt.drain, stopDrainInput(), "the drain RPC's kind and cause after the %s", tt.name)
		})
	}
}
