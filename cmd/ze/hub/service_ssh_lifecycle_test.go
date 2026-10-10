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

	"github.com/ze-software/ze/internal/component/config/infra"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zessh "github.com/ze-software/ze/internal/component/ssh"
)

// lifecycleReactor counts the two reactor stops an SSH lifecycle command can
// take. It holds no dispatcher: sshWireImpl only captures it in closures.
type lifecycleReactor struct {
	stops           atomic.Int32
	stopsForRestart atomic.Int32
}

func (r *lifecycleReactor) SetPostStartFunc(func())              {}
func (r *lifecycleReactor) Dispatcher() *pluginserver.Dispatcher { return nil }
func (r *lifecycleReactor) Stop()                                { r.stops.Add(1) }
func (r *lifecycleReactor) StopForRestart()                      { r.stopsForRestart.Add(1) }

// TestSSHLifecycleEndsServerWait drives the lifecycle callbacks sshWireImpl
// installs on the SSH server, the ones `ze signal stop`, `ze signal restart`
// and the SSH exec `reboot` reach through the SSH exec middleware, and asserts
// that the plugin server's Wait returns. That Wait is what runYANGConfig's
// waitLoop blocks on, so a callback that stops only the reactor leaves the
// daemon running after the client was told "stopping daemon" and exited 0.
//
// VALIDATES: SSH `stop`, `restart` and `reboot` end the daemon, as
// `request shutdown` and `request reboot` do (handleDaemonShutdown,
// handleDaemonReboot); `reboot` also sets rebootRequested, which runYANGConfig
// reads after waitLoop to reboot the host.
// PREVENTS: a stop that reports success and does not stop.
// MUTATION: drop apiServer.SignalShutdownRequested from any callback in
// sshWireImpl and its subtest times out; drop rebootRequested.Store(true) from
// the reboot callback and the reboot subtest fails its last assertion.
func TestSSHLifecycleEndsServerWait(t *testing.T) {
	tests := []struct {
		name            string
		action          func(*zessh.Server) func()
		stops           int32
		stopsForRestart int32
		reboot          bool
	}{
		{name: "stop", action: func(s *zessh.Server) func() { return s.ShutdownFunc() }, stops: 1},
		{name: "restart", action: func(s *zessh.Server) func() { return s.RestartFunc() }, stopsForRestart: 1},
		{name: "reboot", action: func(s *zessh.Server) func() { return s.RebootFunc() }, stopsForRestart: 1, reboot: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			apiServer, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
			require.NoError(t, err)
			require.NoError(t, apiServer.StartWithContext(context.Background()))
			t.Cleanup(apiServer.Stop)
			// rebootRequested is package state the reboot callback writes;
			// leave it as every other test expects to find it.
			rebootRequested.Store(false)
			t.Cleanup(func() { rebootRequested.Store(false) })

			reactor := &lifecycleReactor{}
			sshSrv, err := zessh.NewServer(zessh.Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
			require.NoError(t, err)
			sshWireImpl(sshSrv, &sshWireInputs{
				Reactor:        reactor,
				Params:         infra.HookParams{APIServer: func() *pluginserver.Server { return apiServer }},
				StopForRestart: reactor.StopForRestart,
			})
			action := tt.action(sshSrv)
			require.NotNil(t, action, "sshWireImpl installed no %s callback", tt.name)

			waitDone := make(chan error, 1)
			go func() { waitDone <- apiServer.Wait(ctx) }()

			action()
			select {
			case err := <-waitDone:
				require.NoError(t, err, "Server.Wait ended on its deadline, not on the %s", tt.name)
			case <-ctx.Done():
				t.Fatalf("SSH %s stopped the reactor but never ended Server.Wait", tt.name)
			}
			assert.Equal(t, tt.stops, reactor.stops.Load(), "reactor Stop calls")
			assert.Equal(t, tt.stopsForRestart, reactor.stopsForRestart.Load(), "reactor StopForRestart calls")
			assert.Equal(t, tt.reboot, rebootRequested.Load(), "rebootRequested after the %s", tt.name)
		})
	}
}
