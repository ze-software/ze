// Design: docs/architecture/api/process-protocol.md -- joined managed runtime restoration.
package server

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestGenerationReadyJoinsFramedRuntimeBeforeStartupSuccess(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "accepted", true: "refused"}[refuse], func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			const name = "generation-ready-framed-test"
			s, _ := newLifecycleStartupServer(t)
			var dispatched atomic.Int32
			require.NoError(t, s.Dispatcher().Register("ready-probe", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
				dispatched.Add(1)
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}, "runtime restoration probe"))
			probeSends := []rpc.MessageSendDecl{{Message: "cli:ready-probe"}}
			require.NoError(t, registry.Register(registry.Registration{Name: name, Sends: probeSends, CLIHandler: func([]string) int { return 0 }, RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(name, struct{ net.Conn }{conn})
				p.OnCurrentSelection(func(ctx context.Context, in rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
					_, _, err := p.DispatchCommand(ctx, "ready-probe")
					if err != nil {
						return nil, err
					}
					out := &rpc.CurrentSelectionOutput{Destination: in.Destination, DestinationSession: in.DestinationSession, Cut: 1, Results: []rpc.CurrentSelectionResult{}}
					for _, key := range in.Keys {
						out.Results = append(out.Results, rpc.CurrentSelectionResult{Family: key.Family, KeyNLRI: key.KeyNLRI, Revision: key.Revision, Presence: rpc.CurrentSelectionAbsent})
					}
					return out, nil
				})
				if err := p.Run(s.Context(), &sdk.Registration{Sends: probeSends}); err != nil && s.Context().Err() == nil {
					return 1
				}
				return 0
			}}))
			rejected := errors.New("registered runtime restoration refused")
			var captured *process.Process
			engineOperationMu.Lock()
			previous := processGenerationReady
			processGenerationReady = append(append([]ProcessGenerationReady(nil), previous...), func(ctx context.Context, server *Server, proc *process.Process) error {
				if proc.Name() != name {
					return nil
				}
				captured = proc
				if _, err := proc.SelectedOwner(); err != nil {
					return err
				}
				if server.ProcessManager().GetProcess(name) != proc {
					return errors.New("ready selected a different process")
				}
				bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
				defer cancel()
				_, err := proc.Conn().SendCurrentSelection(bounded, rpc.CurrentSelectionInput{Destination: "192.0.2.80", DestinationSession: 71, Keys: []rpc.CurrentSelectionKey{{Family: "ipv4/unicast", KeyNLRI: []byte{24, 198, 18, 7}, Revision: 1}}})
				if err != nil {
					return err
				}
				if refuse {
					return rejected
				}
				return nil
			})
			engineOperationMu.Unlock()
			t.Cleanup(func() { engineOperationMu.Lock(); processGenerationReady = previous; engineOperationMu.Unlock() })
			err := s.runPluginPhase([]plugin.PluginConfig{{Name: name, Run: name, Internal: true}})
			require.Equal(t, int32(1), dispatched.Load(), "framed runtime service must be live while ready joins")
			require.NotNil(t, captured)
			if refuse {
				require.ErrorIs(t, err, rejected)
				require.Nil(t, s.ProcessManager().GetProcess(name), "failed ready must stop and release its exact generation")
				owner, ok := captured.SelectedOwnerForCleanup()
				require.True(t, ok)
				require.Error(t, owner.Current())
				return
			}
			require.NoError(t, err)
			require.Same(t, captured, s.ProcessManager().GetProcess(name))
			require.Equal(t, plugin.StageRunning, captured.Stage())
		})
	}
}
