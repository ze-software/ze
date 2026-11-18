// Design: docs/architecture/api/process-protocol.md -- negotiated bridge activation receipt.
// Related: startup_driver.go -- the single startup owner joins the SDK callback loop.
package server

import (
	"bytes"
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// One startup-connection owner holds the final ACK after net.Pipe delivered its
// bytes, before the SDK reader can parse it and activate the bridge. The test
// MUST release that read before joining its startup goroutine. This wrapper
// preserves the real Bridger, so the fixture does not turn into framed mode.
type startupBridgeActivationConn struct {
	net.Conn
	bridge   *rpc.DirectBridge
	armed    atomic.Bool
	once     sync.Once
	ackRead  chan struct{}
	release  <-chan struct{}
	lifetime context.Context
}

func (c *startupBridgeActivationConn) Bridge() *rpc.DirectBridge { return c.bridge }

func (c *startupBridgeActivationConn) Write(data []byte) (int, error) {
	if bytes.Contains(data, []byte(methodReady)) {
		c.armed.Store(true)
	}
	return c.Conn.Write(data)
}

func (c *startupBridgeActivationConn) Read(data []byte) (int, error) {
	count, err := c.Conn.Read(data)
	if count > 0 {
		if c.armed.Load() {
			c.once.Do(func() {
				close(c.ackRead)
				select {
				case <-c.release:
				case <-c.lifetime.Done():
				}
			})
		}
	}
	return count, err
}

// A negotiated bridge with no OnStarted handler still owes an activation
// receipt. Hold the actual SDK's final ACK, then observe either the queued
// receipt, a recorded restoration failure, or an erroneous phase result.
// Releasing the ACK must permit typed lookup and successful phase completion.
// MUTATION: omitting the bridge activation receipt exposes an unready typed slot.
func TestStartupBridgeWithoutInitializerJoinsSDKActivation(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	s, _ := newLifecycleStartupServer(t)
	work, cancel := context.WithTimeout(s.Context(), 5*time.Second)
	defer cancel()
	const name = "startup-bridge-activation-test"
	ackRead := make(chan struct{})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)
	var lookups atomic.Int32
	require.NoError(t, registry.Register(registry.Registration{Name: name, CLIHandler: func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			bridger, ok := conn.(rpc.Bridger)
			if !ok {
				return 1
			}
			held := &startupBridgeActivationConn{Conn: conn, bridge: bridger.Bridge(), ackRead: ackRead, release: release, lifetime: work}
			p := sdk.NewWithConn(name, held)
			p.OnCurrentSelection(func(ctx context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				lookups.Add(1)
				output := &rpc.CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession, Cut: 1}
				for _, key := range input.Keys {
					output.Results = append(output.Results, rpc.CurrentSelectionResult{Family: key.Family, KeyNLRI: key.KeyNLRI,
						Revision: key.Revision, Presence: rpc.CurrentSelectionAbsent})
				}
				return output, nil
			})
			if err := p.Run(s.Context(), &sdk.Registration{}); err != nil {
				if s.Context().Err() == nil {
					return 1
				}
			}
			return 0
		},
	}))
	engineOperationMu.Lock()
	previous := processGenerationReady
	processGenerationReady = append(append([]ProcessGenerationReady(nil), previous...), func(ctx context.Context, server *Server, proc *process.Process) error {
		if proc.Name() != name {
			return nil
		}
		if server != s {
			return errors.New("restoration used another server")
		}
		_, err := proc.Conn().SendCurrentSelection(ctx, rpc.CurrentSelectionInput{Destination: "192.0.2.80", DestinationSession: 71,
			Keys: []rpc.CurrentSelectionKey{{Family: "ipv4/unicast", KeyNLRI: []byte{24, 198, 18, 7}, Revision: 1}}})
		return err
	})
	engineOperationMu.Unlock()
	t.Cleanup(func() { engineOperationMu.Lock(); processGenerationReady = previous; engineOperationMu.Unlock() })
	phaseDone := make(chan struct{})
	var phaseErr error
	go func() {
		defer close(phaseDone)
		phaseErr = s.runPluginPhaseWithin(work, []plugin.PluginConfig{{Name: name, Run: name, Internal: true}}, s.currentConfigureConfig())
	}()
	t.Cleanup(func() {
		// Cleanup MUST release the held reader before it joins the phase.
		unblock()
		cancel()
		cleanup, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		select {
		case <-phaseDone:
		case <-cleanup.Done():
			t.Error("startup phase did not join its held ACK cleanup")
		}
	})
	select {
	case <-ackRead:
	case <-work.Done():
		t.Fatal("SDK did not consume the actual final ready ACK")
	}
	proc := s.ProcessManager().GetProcess(name)
	require.NotNil(t, proc)
	require.False(t, proc.Registration().StartedHandler, "the regression must not gain an initializer declaration")
	bridge := proc.Bridge()
	require.NotNil(t, bridge)
	require.False(t, bridge.Ready(), "SDK cannot activate until the held ACK is released")
	require.Eventually(t, func() bool {
		if len(bridge.CallbackCh()) > 0 {
			return true
		}
		if proc.StartupError() != nil {
			return true
		}
		select {
		case <-phaseDone:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond, "startup must queue its activation receipt or expose its failure or result")
	require.NoError(t, proc.StartupError(), "typed restoration must remain behind SDK bridge activation")
	select {
	case <-phaseDone:
		t.Fatalf("startup exposed runtime restoration before SDK bridge activation: %v", phaseErr)
	default:
	}
	require.Len(t, bridge.CallbackCh(), 1, "the existing runtime-ready receipt must wait on the SDK loop")
	require.Zero(t, lookups.Load(), "typed restoration must remain behind activation")
	unblock()
	select {
	case <-phaseDone:
	case <-work.Done():
		t.Fatal("startup did not join the actual SDK activation receipt")
	}
	require.NoError(t, phaseErr)
	require.True(t, bridge.Ready())
	require.Equal(t, int32(1), lookups.Load(), "the activated SDK must answer the useful typed snapshot")
	require.Same(t, proc, s.ProcessManager().GetProcess(name))
	require.True(t, proc.Running())
}
