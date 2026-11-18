// Design: docs/architecture/api/process-protocol.md -- restoration follows peer admission.
// Related: restart.go -- joined immediate admission; filter_insertion_change.go -- deferred admission.
package server

import (
	"context"
	"errors"
	"net"
	"sync"
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

// TestGenerationReadyRestartFollowsPeerAdmission reaches the real replacement
// handshake on both transports, then joins useful output and refuses early
// post-startup while restoration is held. Refusal stops the exact replacement.
func TestGenerationReadyRestartFollowsPeerAdmission(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			for _, refuse := range []bool{false, true} {
				t.Run(map[bool]string{false: "accepted", true: "refused"}[refuse], func(t *testing.T) {
					checkGenerationReadyReplacement(t, typed, false, refuse)
				})
			}
		})
	}
}

// TestGenerationReadyDeferredAdmissionOwnsRestoration reaches an automatic
// restart during a real held transaction. Its release owns the bounded ready
// join, with no post-startup reservation before completion or after refusal.
func TestGenerationReadyDeferredAdmissionOwnsRestoration(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			for _, refuse := range []bool{false, true} {
				t.Run(map[bool]string{false: "accepted", true: "refused"}[refuse], func(t *testing.T) {
					checkGenerationReadyReplacement(t, typed, true, refuse)
				})
			}
		})
	}
}

// checkGenerationReadyReplacement owns one actual server/restart lifecycle.
// The cleanup MUST unblock and join its one worker before the server stops.
func checkGenerationReadyReplacement(t *testing.T, typed, deferred, refuse bool) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "generation-ready-replacement-test"
	s, _ := newLifecycleStartupServer(t)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	var runtimeCalls atomic.Int32
	require.NoError(t, s.Dispatcher().Register("ready-restart-probe", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		runtimeCalls.Add(1)
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "restoration runtime probe"))
	posts := make(chan uint64, 4)
	var generations atomic.Uint64
	require.NoError(t, registry.Register(registry.Registration{Name: name, CLIHandler: func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			generation := generations.Add(1)
			if !typed {
				conn = struct{ net.Conn }{conn}
			}
			provider := sdk.NewWithConn(name, conn)
			provider.OnAllPluginsReady(func() error {
				posts <- generation
				return nil
			})
			provider.OnCurrentSelection(func(work context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
				if _, _, err := provider.DispatchCommand(work, "ready-restart-probe"); err != nil {
					return nil, err
				}
				output := &rpc.CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession, Cut: 1}
				for _, key := range input.Keys {
					output.Results = append(output.Results, rpc.CurrentSelectionResult{Family: key.Family, KeyNLRI: key.KeyNLRI,
						Revision: key.Revision, Presence: rpc.CurrentSelectionAbsent})
				}
				return output, nil
			})
			if err := provider.Run(s.Context(), &sdk.Registration{}); err != nil && s.Context().Err() == nil {
				return 1
			}
			return 0
		},
	}))
	entered := make(chan *process.Process, 1)
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	rejected := errors.New("restoration refused after peer admission")
	var original *process.Process
	reconciled := &atomic.Int64{}
	engineOperationMu.Lock()
	previous := processGenerationReady
	processGenerationReady = append(append([]ProcessGenerationReady(nil), previous...), func(work context.Context, server *Server, proc *process.Process) error {
		if proc.Name() != name {
			return nil
		}
		if original == nil {
			return nil
		}
		if proc == original {
			return errors.New("restoration used the old process")
		}
		if server.ProcessManager().GetProcess(name) != proc {
			return errors.New("restoration used a superseded process")
		}
		if _, err := proc.SelectedOwner(); err != nil {
			return err
		}
		if reconciled.Load() != 1 {
			return errors.New("restoration preceded peer declaration reconciliation")
		}
		if _, bounded := work.Deadline(); !bounded {
			return errors.New("restoration has no owning deadline")
		}
		_, err := proc.Conn().SendCurrentSelection(work, rpc.CurrentSelectionInput{Destination: "192.0.2.80", DestinationSession: 71,
			Keys: []rpc.CurrentSelectionKey{{Family: "ipv4/unicast", KeyNLRI: []byte{24, 198, 18, 7}, Revision: 1}}})
		if err != nil {
			return err
		}
		entered <- proc
		select {
		case <-release:
		case <-work.Done():
			return work.Err()
		}
		if refuse {
			return rejected
		}
		return nil
	})
	engineOperationMu.Unlock()
	t.Cleanup(func() {
		engineOperationMu.Lock()
		processGenerationReady = previous
		engineOperationMu.Unlock()
	})
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: name, Run: name, Internal: true}}))
	original = s.ProcessManager().GetProcess(name)
	require.NotNil(t, original)
	s.Dispatcher().Registry().Freeze()
	s.signalStartupComplete()
	select {
	case first := <-posts:
		require.Equal(t, uint64(1), first)
	case <-ctx.Done():
		t.Fatal("initial process did not receive useful post-startup")
	}
	// The established test seam supplies old metadata which the actual
	// replacement does not declare, requiring a real reconciliation callback.
	original.Registration().FilterInsertions = []rpc.FilterInsertion{thirdPartyInsertion}
	registry.SetPeerDeclarationsChanged(func() error { reconciled.Add(1); return nil })
	t.Cleanup(func() { registry.SetPeerDeclarationsChanged(nil) })
	ended := make(chan error, 1)
	started, joined, locked := false, false, false
	t.Cleanup(func() {
		unblock()
		if locked {
			s.txLock.release()
		}
		if started && !joined {
			s.cancel()
			cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			select {
			case <-ended:
			case <-cleanup.Done():
				t.Error("replacement lifecycle worker did not join cleanup")
			}
		}
	})
	if deferred {
		require.True(t, s.txLock.tryAcquire())
		locked = true
		original.Stop()
		require.NoError(t, original.WaitRuntimeCleanupContext(ctx))
		require.NoError(t, s.restartPluginAfterExit(original))
		require.Equal(t, int64(0), reconciled.Load(), "the held transaction owns the later rebuild")
		pending := s.ProcessManager().GetProcess(name)
		require.NotNil(t, pending)
		require.Error(t, pending.RequirePeerDeclarations())
		require.False(t, pending.BeginPostStartup(), "held restoration cannot spend post-startup")
		locked = false
		started = true
		go func() { s.txLock.release(); ended <- nil }()
	} else {
		started = true
		go func() {
			response, err := s.Dispatcher().Dispatch(&CommandContext{Server: s}, "request plugin restart "+name)
			if err != nil {
				ended <- err
				return
			}
			if response == nil {
				ended <- errors.New("restart returned no command response")
				return
			}
			if response.Status != plugin.StatusDone {
				ended <- errors.New(response.Error)
				return
			}
			ended <- nil
		}()
	}
	var replacement *process.Process
	select {
	case replacement = <-entered:
	case err := <-ended:
		joined = true
		t.Fatalf("restart ended before joined useful restoration: %v", err)
	case <-ctx.Done():
		t.Fatal("restoration did not reach the admitted runtime")
	}
	require.NotSame(t, original, replacement)
	require.NoError(t, replacement.RequirePeerDeclarations())
	require.Equal(t, int64(1), reconciled.Load())
	require.Equal(t, int32(1), runtimeCalls.Load(), "runtime readers serve the held restoration on either transport")
	require.False(t, replacement.BeginPostStartup(), "racing fan-out cannot spend the delivery before the ready join")
	s.sendPostStartupTo(replacement)
	unblock()
	var result error
	select {
	case result = <-ended:
		joined = true
	case <-ctx.Done():
		t.Fatal("restoration did not join its owner")
	}
	if refuse {
		if deferred {
			require.NoError(t, result)
		} else {
			require.ErrorContains(t, result, rejected.Error())
		}
		require.Nil(t, s.ProcessManager().GetProcess(name), "failed restoration stops and releases its exact replacement")
		stopped, recorded := s.ProcessManager().Stopped(name)
		require.True(t, recorded, "doctor must retain the exact restoration failure")
		require.Equal(t, process.StopRestartFailed, stopped.Cause)
		require.Contains(t, stopped.Detail, rejected.Error())
		owner, captured := replacement.SelectedOwnerForCleanup()
		require.True(t, captured)
		require.Error(t, owner.Current())
		require.False(t, replacement.BeginPostStartup(), "failure leaves restoration held")
		select {
		case generation := <-posts:
			t.Fatalf("failed replacement received post-startup generation %d", generation)
		default:
		}
		return
	}
	require.NoError(t, result)
	require.Same(t, replacement, s.ProcessManager().GetProcess(name))
	select {
	case generation := <-posts:
		require.Equal(t, uint64(2), generation)
	case <-ctx.Done():
		t.Fatal("restored replacement never received post-startup")
	}
	require.False(t, replacement.BeginPostStartup(), "completed delivery remains single-owner")
	s.sendPostStartupTo(replacement)
}
