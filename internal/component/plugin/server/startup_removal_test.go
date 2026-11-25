// Design: docs/architecture/api/process-protocol.md -- live removal callback ordering
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Live removal must permit final engine calls before closing the plugin, while
// daemon shutdown must leave teardown policy with the owning subsystem.
func TestPluginRemovalCallbackKeepsEngineConnection(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		name := "removal"
		if shutdown {
			name = "shutdown"
		}
		t.Run(name, func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })
			const pluginName = "lifecycle-removal-callback"
			const commandName = "show lifecycle removal-probe"
			callback := make(chan error, 1)
			runResult := make(chan error, 1)
			probeSends := []rpc.MessageSendDecl{{Message: busCLIPrefix + commandName}}
			require.NoError(t, registry.Register(registry.Registration{
				Name:        pluginName,
				Description: "lifecycle removal callback test plugin",
				Sends:       probeSends,
				RunEngine: func(conn net.Conn) int {
					p := sdk.NewWithConn(pluginName, conn)
					p.OnBye(func(reason string) error {
						if reason != "removed" {
							err := fmt.Errorf("unexpected bye reason %q", reason)
							callback <- err
							return err
						}
						ctx, cancel := context.WithTimeout(context.Background(), time.Second)
						defer cancel()
						_, _, err := p.DispatchCommand(ctx, commandName)
						callback <- err
						return err
					})
					runResult <- p.Run(context.Background(), &sdk.Registration{
						FailurePolicy: rpc.FailureFatal,
						Sends:         probeSends,
					})
					if err := p.Close(); err != nil {
						return 1
					}
					return 0
				},
				CLIHandler: func([]string) int { return 0 },
			}))
			s, spawner := newLifecycleStartupServer(t)
			if err := s.dispatcher.Register(commandName, func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}, "Read removal probe"); err != nil {
				t.Fatal(err)
			}
			unexpectedShutdown := make(chan struct{}, 1)
			s.SetShutdownFunc(func() { unexpectedShutdown <- struct{}{} })
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON}}))
			if shutdown {
				s.cancel()
				spawner.pm.Stop()
			} else {
				require.NoError(t, s.rollbackStartupProcess(spawner.pm.GetProcess(pluginName)))
			}
			s.wg.Wait()
			require.NoError(t, <-runResult)
			if shutdown {
				select {
				case err := <-callback:
					t.Fatalf("daemon shutdown invoked removal callback: %v", err)
				default:
				}
			} else {
				select {
				case err := <-callback:
					require.NoError(t, err, "removal callback must still reach the engine")
				default:
					t.Fatal("live removal skipped its callback")
				}
			}
			select {
			case <-unexpectedShutdown:
				t.Fatal("intentional removal was treated as a fatal plugin crash")
			default:
			}
		})
	}
}

// Removing a config owner also removes its orphaned hard and optional
// dependencies. The owner must still reach the middleware, and the middleware
// must still reach the backend, until each has released its installed state.
func TestConfigRemovalReleasesStateBeforeDependenciesStop(t *testing.T) {
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const (
		producer       = "z-removal-producer"
		middle         = "m-removal-middle"
		backend        = "a-removal-backend"
		root           = "lifecycle-removal"
		middleCommand  = "request lifecycle middle-release"
		backendCommand = "request lifecycle backend-release"
	)
	var stateMu sync.Mutex
	installed := map[string]bool{producer: true, middle: true}
	backendClosed := false
	residue := make(chan map[string]bool, 1)
	callbacks := make(chan error, 3)
	exited := make(chan error, 3)
	run := func(p *sdk.Plugin, registration *sdk.Registration) int {
		err := p.Run(context.Background(), registration)
		if closeErr := p.Close(); err == nil {
			err = closeErr
		}
		exited <- err
		if err != nil {
			return 1
		}
		return 0
	}
	require.NoError(t, registry.Register(registry.Registration{
		Name: backend, Description: "state-owning removal dependency",
		CLIHandler: func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(backend, conn)
			p.OnExecuteCommand(func(_, _ string, args []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
				stateMu.Lock()
				defer stateMu.Unlock()
				if backendClosed || len(args) != 1 || !installed[args[0]] {
					return "", nil, fmt.Errorf("cannot release resource %v", args)
				}
				delete(installed, args[0])
				return rpc.StatusDone, nil, nil
			})
			p.OnBye(func(string) error {
				stateMu.Lock()
				backendClosed = true
				left := maps.Clone(installed)
				stateMu.Unlock()
				residue <- left
				callbacks <- nil
				return nil
			})
			return run(p, &sdk.Registration{Commands: []sdk.CommandDecl{{Name: backendCommand}}})
		},
	}))
	middleSends := []rpc.MessageSendDecl{{Message: busCLIPrefix + backendCommand}}
	require.NoError(t, registry.Register(registry.Registration{
		Name: middle, Description: "optional backend consumer",
		OptionalDependencies: []string{backend},
		Sends:                middleSends,
		CLIHandler:           func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(middle, conn)
			p.OnExecuteCommand(func(_, _ string, _ []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				status, data, err := p.DispatchCommand(ctx, backendCommand+" "+producer)
				return status, data, err
			})
			p.OnBye(func(string) error {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_, _, err := p.DispatchCommand(ctx, backendCommand+" "+middle)
				callbacks <- err
				return err
			})
			return run(p, &sdk.Registration{
				Commands: []sdk.CommandDecl{{Name: middleCommand}},
				Sends:    middleSends,
			})
		},
	}))
	producerSends := []rpc.MessageSendDecl{{Message: busCLIPrefix + middleCommand}}
	require.NoError(t, registry.Register(registry.Registration{
		Name: producer, Description: "configured state producer",
		ConfigRoots: []string{root}, Dependencies: []string{middle},
		Sends:      producerSends,
		CLIHandler: func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(producer, conn)
			p.OnBye(func(string) error {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_, _, err := p.DispatchCommand(ctx, middleCommand)
				callbacks <- err
				return err
			})
			return run(p, &sdk.Registration{Sends: producerSends})
		},
	}))
	s, spawner := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: backend, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: middle, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: producer, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	require.NoError(t, s.stopCollectedProcesses(s.collectProcessesForRemovedConfigPaths([]string{root}, nil)))
	s.wg.Wait()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for range 3 {
		select {
		case err := <-callbacks:
			require.NoError(t, err)
		default:
			t.Fatal("a removed plugin did not complete its cleanup callback")
		}
		select {
		case err := <-exited:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("removed plugin did not exit")
		}
	}
	require.Empty(t, <-residue, "the backend must stay live until both consumers release their resources")
	require.Nil(t, spawner.pm.GetProcess(producer))
	require.Nil(t, spawner.pm.GetProcess(middle))
	require.Nil(t, spawner.pm.GetProcess(backend))
}

type removalRecoveryReactor struct{ mockReloadReactor }

// ReloadRunning answers the running config: the mock keeps no runtime state across a reload.
func (r *removalRecoveryReactor) ReloadRunning(map[string]any) map[string]any {
	return r.GetConfigTree()
}

func (r *removalRecoveryReactor) SetConfigTree(tree map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tree = tree
	r.setTree = tree
}

// Failed cleanup must keep its process and dependencies usable. A callback
// that succeeds after the caller times out must restore the committed service
// automatically, after the outer reload finishes compensation.
func TestReloadRemovalFailureRecoversCommittedService(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for _, scenario := range []struct {
			name                                                       string
			delayed, crash, outerRejected, failedRecovery, participant bool
		}{
			{name: "refused"},
			{name: "delayed", delayed: true},
			{name: "participant-refused", participant: true},
			{name: "participant-delayed", delayed: true, participant: true},
			{name: "disconnected-generation", crash: true, participant: true},
			{name: "outer-rejected", outerRejected: true, participant: true},
			{name: "failed-restoration", crash: true, failedRecovery: true, participant: true},
		} {
			delayed := scenario.delayed
			name := fmt.Sprintf("direct=%t/%s", direct, scenario.name)
			t.Run(name, func(t *testing.T) {
				snapshot := registry.Snapshot()
				registry.Reset()
				t.Cleanup(func() { registry.Restore(snapshot) })
				const (
					producer    = "removal-recovery-producer"
					backend     = "removal-recovery-backend"
					root        = "removal-recovery"
					command     = "request lifecycle release-recovery-resource"
					counter     = "removal-recovery-counter"
					counterRoot = "removal-counter"
				)
				var stateMu sync.Mutex
				var resourceOwner int32
				backendClosed := false
				var starts, backendStarts, attempts atomic.Int32
				var appliedCounter atomic.Int64
				release := make(chan struct{})
				unblock := sync.OnceFunc(func() { close(release) })
				runErrors := make(chan error, 8)
				client := func(name string, conn net.Conn) *sdk.Plugin {
					if !direct {
						// Hide the optional Bridger interface to exercise the
						// subprocess-facing socket callback and its real ACK.
						conn = struct{ net.Conn }{conn}
					}
					return sdk.NewWithConn(name, conn)
				}
				run := func(p *sdk.Plugin, registration *sdk.Registration) int {
					err := p.Run(context.Background(), registration)
					if closeErr := p.Close(); err == nil {
						err = closeErr
					}
					runErrors <- err
					if err != nil {
						return 1
					}
					return 0
				}
				require.NoError(t, registry.Register(registry.Registration{
					Name: backend, Description: "removal recovery resource backend",
					CLIHandler: func([]string) int { return 0 },
					RunEngine: func(conn net.Conn) int {
						backendStarts.Add(1)
						p := client(backend, conn)
						p.OnConfigure(func([]sdk.ConfigSection) error {
							stateMu.Lock()
							backendClosed = false
							stateMu.Unlock()
							return nil
						})
						p.OnExecuteCommand(func(_, _ string, _ []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
							stateMu.Lock()
							defer stateMu.Unlock()
							if backendClosed || resourceOwner == 0 {
								return "", nil, errors.New("resource backend is not available")
							}
							resourceOwner = 0
							return rpc.StatusDone, nil, nil
						})
						p.OnBye(func(string) error {
							stateMu.Lock()
							defer stateMu.Unlock()
							if resourceOwner != 0 {
								return errors.New("dependency stopped before selected resource cleanup")
							}
							backendClosed = true
							return nil
						})
						return run(p, &sdk.Registration{Commands: []sdk.CommandDecl{{Name: command}}})
					},
				}))
				producerSends := []rpc.MessageSendDecl{{Message: busCLIPrefix + command}}
				require.NoError(t, registry.Register(registry.Registration{
					Name: producer, Description: "retryable removal source",
					ConfigRoots: []string{root}, Dependencies: []string{backend},
					Sends:      producerSends,
					CLIHandler: func([]string) int { return 0 },
					RunEngine: func(conn net.Conn) int {
						generation := starts.Add(1)
						p := client(producer, conn)
						p.OnConfigure(func(sections []sdk.ConfigSection) error {
							if scenario.failedRecovery && generation == 2 {
								return errors.New("selected resource restoration refused")
							}
							if scenario.outerRejected {
								enabled := false
								for _, section := range sections {
									var decoded map[string]struct{ Enabled bool }
									if err := json.Unmarshal([]byte(section.Data), &decoded); err != nil {
										return err
									}
									enabled = decoded[root].Enabled
								}
								if !enabled {
									return nil
								}
							}
							stateMu.Lock()
							resourceOwner = generation
							stateMu.Unlock()
							return nil
						})
						p.OnBye(func(string) error {
							if attempts.Add(1) == 1 && !scenario.outerRejected {
								if scenario.crash {
									if direct {
										panic("removal callback panic")
									}
									return conn.Close()
								}
								if !delayed {
									return errors.New("selected resource cleanup refused")
								}
								<-release
							}
							ctx, cancel := context.WithTimeout(context.Background(), time.Second)
							defer cancel()
							_, _, err := p.DispatchCommand(ctx, command)
							return err
						})
						registration := sdk.Registration{FailurePolicy: rpc.FailureFatal, Sends: producerSends}
						if scenario.crash {
							if generation == 1 {
								registration.FailurePolicy = rpc.FailureRestart
							}
						}
						if scenario.outerRejected {
							registration.WantsConfig = []string{root}
							// It reads its root at configure only and applies no reload.
							p.DeclareAppliesNothing("test producer: reads its root at configure only")
						}
						return run(p, &registration)
					},
				}))
				configs := []plugin.PluginConfig{
					{Name: backend, Internal: true, Encoder: plugin.EncodingJSON},
					{Name: producer, Internal: true, Encoder: plugin.EncodingJSON},
				}
				if scenario.participant {
					require.NoError(t, registry.Register(registry.Registration{
						Name: counter, Description: "independent committed config participant",
						CLIHandler: func([]string) int { return 0 },
						RunEngine: func(conn net.Conn) int {
							p := client(counter, conn)
							var staged int64
							stage := func(sections []sdk.ConfigSection) error {
								for _, section := range sections {
									var decoded map[string]struct{ Value int64 }
									if err := json.Unmarshal([]byte(section.Data), &decoded); err != nil {
										return err
									}
									staged = decoded[counterRoot].Value
								}
								return nil
							}
							p.OnConfigure(func(sections []sdk.ConfigSection) error {
								if err := stage(sections); err != nil {
									return err
								}
								appliedCounter.Store(staged)
								return nil
							})
							p.OnConfigVerify(stage)
							p.OnConfigApply(func([]sdk.ConfigDiffSection) error {
								appliedCounter.Store(staged)
								return nil
							})
							return run(p, &sdk.Registration{WantsConfig: []string{counterRoot}})
						},
					}))
					configs = append(configs, plugin.PluginConfig{Name: counter, Internal: true, Encoder: plugin.EncodingJSON})
				}
				s, spawner := newLifecycleStartupServer(t)
				t.Cleanup(unblock)
				committed := map[string]any{root: map[string]any{"enabled": true}}
				candidate := map[string]any{}
				if scenario.participant {
					committed[counterRoot] = map[string]any{"value": int64(42)}
					candidate[counterRoot] = map[string]any{"value": int64(99)}
				}
				reactor := &removalRecoveryReactor{tree: committed}
				s.reactor = reactor
				unexpectedShutdown := make(chan struct{}, 1)
				s.SetShutdownFunc(func() { unexpectedShutdown <- struct{}{} })
				require.NoError(t, s.runPluginPhase(configs))
				if scenario.participant {
					require.Equal(t, int64(42), appliedCounter.Load())
				}
				old := spawner.pm.GetProcess(producer)
				dependency := spawner.pm.GetProcess(backend)
				reloadCtx, finish := s.DeferReloadAcceptance(t.Context())
				defer finish(false) //nolint:errcheck // Cleanup; the completion outcome is asserted below.
				err := s.ReloadConfig(reloadCtx, candidate)
				if scenario.outerRejected {
					require.NoError(t, err)
					require.Equal(t, int64(99), appliedCounter.Load(), "participant must actually commit before publication is refused")
					require.Equal(t, candidate, reactor.GetConfigTree())
				} else {
					require.Error(t, err, "failed or unfinished cleanup must refuse reload")
					if delayed {
						require.ErrorIs(t, err, context.DeadlineExceeded)
					} else if !scenario.crash {
						require.ErrorContains(t, err, "selected resource cleanup refused")
					}
					require.Equal(t, committed, reactor.GetConfigTree())
					require.Equal(t, committed, reactor.applyTree, "reactor changes must also be compensated")
					if scenario.participant {
						require.Equal(t, int64(42), appliedCounter.Load(), "other committed participants must be restored before reload returns")
					}
					require.Same(t, dependency, spawner.pm.GetProcess(backend))
					require.True(t, dependency.Running(), "cleanup must retain its dependency")
				}
				if delayed {
					unblock()
					select {
					case <-old.EngineDone():
					case <-time.After(5 * time.Second):
						t.Fatal("late cleanup could not finish through its live dependency")
					}
				}
				// No further reload is issued to trigger recovery.
				require.NoError(t, finish(false))
				require.Equal(t, committed, reactor.GetConfigTree())
				if scenario.participant {
					require.Equal(t, int64(42), appliedCounter.Load(), "outer rejection must compensate participants without another reload")
				}
				if scenario.failedRecovery {
					require.Eventually(t, func() bool {
						s.removalMu.Lock()
						attempt := s.removals[old]
						failed := attempt != nil && !attempt.recovering && attempt.recoveryErr != nil
						s.removalMu.Unlock()
						// Recovery publishes its error before releasing transaction ownership.
						_, transactionDone := s.txLock.inFlight()
						return failed && transactionDone == nil
					}, 5*time.Second, time.Millisecond, "failed restoration must retain recovery ownership")
					require.Nil(t, spawner.pm.GetProcess(producer), "a failed replacement must not leave a dead process slot")
					require.NoError(t, s.ReloadConfig(t.Context(), committed), "an explicit retry must restore the committed service")
				}
				wantGeneration := int32(1)
				if delayed || scenario.crash || scenario.outerRejected {
					wantGeneration = 2
				}
				if scenario.failedRecovery {
					wantGeneration = 3
				}
				require.Eventually(t, func() bool {
					current := spawner.pm.GetProcess(producer)
					stateMu.Lock()
					restored := resourceOwner == wantGeneration && !backendClosed
					stateMu.Unlock()
					restored = restored && !s.pluginRemovalPending(old) && !s.pluginRemovalPending(dependency)
					_, transactionDone := s.txLock.inFlight()
					return restored && current != nil && current.Running() &&
						transactionDone == nil
				}, 5*time.Second, time.Millisecond, "committed selected resource was not restored automatically")
				if !delayed && !scenario.crash && !scenario.outerRejected {
					require.Same(t, old, spawner.pm.GetProcess(producer), "a refused cleanup must retain its live process for retry")
				}
				require.NoError(t, s.ReloadConfig(t.Context(), candidate))
				if scenario.participant {
					require.Equal(t, int64(99), appliedCounter.Load())
					require.NoError(t, s.rollbackStartupProcess(spawner.pm.GetProcess(counter)))
				}
				s.wg.Wait()
				stateMu.Lock()
				left, closed := resourceOwner, backendClosed
				stateMu.Unlock()
				require.Zero(t, left)
				require.True(t, closed)
				require.Nil(t, spawner.pm.GetProcess(producer))
				require.Nil(t, spawner.pm.GetProcess(backend))
				var failures int
				completed := starts.Load() + backendStarts.Load()
				if scenario.participant {
					completed++
				}
				for range int(completed) {
					if err := <-runErrors; err != nil {
						failures++
					}
				}
				wantFailures := 0
				if scenario.crash {
					wantFailures++
				}
				if scenario.failedRecovery {
					wantFailures++
				}
				require.Equal(t, wantFailures, failures)
				select {
				case <-unexpectedShutdown:
					t.Fatal("deliberate removal was treated as a fatal plugin crash")
				default:
				}
			})
		}
	}
}

// TestFinalRemovalLeavesConfiguredSet verifies that a reload removal drops the
// plugin's name from ProcessManager.ConfiguredNames only when the reload scope
// is accepted, and keeps it when the scope is rejected.
//
// VALIDATES: spec-fib-dummy AC-17, A-8 -- finishRemovalScope(scope, true)
// unconfigures each finalized removal; finishRemovalScope(scope, false) keeps
// the name configured so recovery restores it.
// PREVENTS: a committed reload leaving a removed plugin in ConfiguredNames, or
// an uncommitted/rejected reload dropping a plugin before recovery runs.
func TestFinalRemovalLeavesConfiguredSet(t *testing.T) {
	for _, accepted := range []bool{true, false} {
		label := "rejected"
		if accepted {
			label = "accepted"
		}
		t.Run(label, func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })

			const (
				keptName    = "final-removal-kept"
				removedName = "final-removal-target"
				removedRoot = "final-removal-root"
			)
			registerSimple := func(name string, roots []string) {
				require.NoError(t, registry.Register(registry.Registration{
					Name:        name,
					Description: "configured set removal test plugin",
					ConfigRoots: roots,
					CLIHandler:  func([]string) int { return 0 },
					RunEngine: func(conn net.Conn) int {
						p := sdk.NewWithConn(name, conn)
						if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
							return 1
						}
						return 0
					},
				}))
			}
			registerSimple(keptName, nil)
			registerSimple(removedName, []string{removedRoot})

			var s *Server
			t.Cleanup(func() {
				if s != nil {
					s.wg.Wait()
				}
			})
			s, spawner := newLifecycleStartupServer(t)
			s.SetShutdownFunc(func() {})
			committed := map[string]any{removedRoot: map[string]any{"enabled": true}}
			s.reactor = &removalRecoveryReactor{tree: committed}

			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
				{Name: keptName, Internal: true, Encoder: plugin.EncodingJSON},
				{Name: removedName, Internal: true, Encoder: plugin.EncodingJSON},
			}))
			require.Equal(t, []string{keptName, removedName}, spawner.pm.ConfiguredNames())

			reloadCtx, finish := s.DeferReloadAcceptance(t.Context())
			defer finish(false) //nolint:errcheck // Cleanup; outcome is asserted below.
			require.NoError(t, s.ReloadConfig(reloadCtx, map[string]any{}))

			require.Nil(t, spawner.pm.GetProcess(removedName))
			require.Contains(t, spawner.pm.ConfiguredNames(), removedName,
				"a pending reload removal must keep the plugin configured until the scope settles")

			require.NoError(t, finish(accepted))
			if accepted {
				require.Equal(t, []string{keptName}, spawner.pm.ConfiguredNames(),
					"an accepted reload removal must drop the removed plugin from ConfiguredNames")
				require.Nil(t, spawner.pm.GetProcess(removedName))
				return
			}

			require.Eventually(t, func() bool {
				proc := spawner.pm.GetProcess(removedName)
				_, txDone := s.txLock.inFlight()
				return proc != nil && proc.Running() && !s.removalPendingFor(removedName) && txDone == nil
			}, 5*time.Second, time.Millisecond, "rejected reload did not restore the removed plugin")
			require.Contains(t, spawner.pm.ConfiguredNames(), removedName,
				"a rejected reload must keep the recovered plugin in ConfiguredNames")
			require.Contains(t, spawner.pm.ConfiguredNames(), keptName)
		})
	}
}

// TestRolledBackStartLeavesConfiguredSet verifies that a plugin started by a
// rejected reload leaves ConfiguredNames whether it stopped below StageRunning
// (no removal scope) or at StageRunning (settled by rejection recovery), while
// a boot startup failure keeps its configured name for doctor and restart.
//
// VALIDATES: spec-fib-dummy AC-17, A-8 -- rollbackStartupProcess drops a
// rolled-back reload start below StageRunning, restoreRemovedPlugin drops one
// stopped at StageRunning that the committed config does not hold, and a failed
// boot start stays configured.
// PREVENTS: a rejected reload leaving an auto-loaded candidate plugin
// restartable through `request plugin restart`, or unconfiguring a committed
// plugin whose boot start failed.
func TestRolledBackStartLeavesConfiguredSet(t *testing.T) {
	t.Run("reload-below-running", func(t *testing.T) {
		snapshot := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snapshot) })

		const (
			baselineName = "rollback-start-baseline"
			addedName    = "rollback-start-below-running"
			addedRoot    = "rollback-start-below-root"
		)
		require.NoError(t, registry.Register(registry.Registration{
			Name:        baselineName,
			Description: "committed baseline plugin",
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(baselineName, conn)
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))
		require.NoError(t, registry.Register(registry.Registration{
			Name:        addedName,
			Description: "candidate plugin failing configure",
			ConfigRoots: []string{addedRoot},
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(addedName, conn)
				p.OnConfigure(func([]sdk.ConfigSection) error {
					return errors.New("candidate configure refused")
				})
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))

		var s *Server
		t.Cleanup(func() {
			if s != nil {
				s.wg.Wait()
			}
		})
		s, spawner := newLifecycleStartupServer(t)
		s.SetShutdownFunc(func() {})
		s.reactor = &removalRecoveryReactor{tree: map[string]any{}}

		require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
			{Name: baselineName, Internal: true, Encoder: plugin.EncodingJSON},
		}))
		t.Cleanup(func() {
			if spawner.pm != nil {
				spawner.pm.ClearStopped(addedName)
			}
		})

		err := s.ReloadConfig(t.Context(), map[string]any{addedRoot: map[string]any{"enabled": true}})
		require.Error(t, err)
		require.Nil(t, spawner.pm.GetProcess(addedName))
		require.NotContains(t, spawner.pm.ConfiguredNames(), addedName,
			"a rolled-back reload start below StageRunning must leave ConfiguredNames")
		require.Contains(t, spawner.pm.ConfiguredNames(), baselineName)
	})

	t.Run("reload-at-running", func(t *testing.T) {
		snapshot := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snapshot) })

		const (
			counterName = "rollback-start-counter"
			counterRoot = "rollback-start-counter-root"
			addedName   = "rollback-start-at-running"
			addedRoot   = "rollback-start-running-root"
		)
		require.NoError(t, registry.Register(registry.Registration{
			Name:        counterName,
			Description: "committed participant refusing reload verify",
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(counterName, conn)
				p.OnConfigVerify(func([]sdk.ConfigSection) error {
					return errors.New("committed participant refuses candidate")
				})
				p.OnConfigApply(func([]sdk.ConfigDiffSection) error { return nil })
				if err := p.Run(context.Background(), &sdk.Registration{WantsConfig: []string{counterRoot}}); err != nil {
					return 1
				}
				return 0
			},
		}))
		require.NoError(t, registry.Register(registry.Registration{
			Name:        addedName,
			Description: "candidate plugin reaching running before reload failure",
			ConfigRoots: []string{addedRoot},
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(addedName, conn)
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))

		var s *Server
		t.Cleanup(func() {
			if s != nil {
				s.wg.Wait()
			}
		})
		s, spawner := newLifecycleStartupServer(t)
		s.SetShutdownFunc(func() {})
		committed := map[string]any{counterRoot: map[string]any{"value": int64(1)}}
		s.reactor = &removalRecoveryReactor{tree: committed}

		require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
			{Name: counterName, Internal: true, Encoder: plugin.EncodingJSON},
		}))

		candidate := map[string]any{
			counterRoot: map[string]any{"value": int64(2)},
			addedRoot:   map[string]any{"enabled": true},
		}
		require.Error(t, s.ReloadConfig(t.Context(), candidate))

		require.Eventually(t, func() bool {
			_, txDone := s.txLock.inFlight()
			return !s.removalPendingFor(addedName) && txDone == nil
		}, 5*time.Second, time.Millisecond, "rolled-back candidate start did not settle")

		require.Nil(t, spawner.pm.GetProcess(addedName))
		require.NotContains(t, spawner.pm.ConfiguredNames(), addedName,
			"a rolled-back reload start stopped at StageRunning must leave ConfiguredNames once the scope settles")
		require.Contains(t, spawner.pm.ConfiguredNames(), counterName)
	})

	t.Run("boot-failed-start-stays-configured", func(t *testing.T) {
		snapshot := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snapshot) })

		const failedBootName = "rollback-start-boot-failed"
		require.NoError(t, registry.Register(registry.Registration{
			Name:        failedBootName,
			Description: "boot plugin failing configure",
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(failedBootName, conn)
				p.OnConfigure(func([]sdk.ConfigSection) error {
					return errors.New("boot configure refused")
				})
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))

		var s *Server
		t.Cleanup(func() {
			if s != nil {
				s.wg.Wait()
			}
		})
		s, spawner := newLifecycleStartupServer(t)
		s.SetShutdownFunc(func() {})
		t.Cleanup(func() {
			if spawner.pm != nil {
				spawner.pm.ClearStopped(failedBootName)
			}
		})

		err := s.runPluginPhase([]plugin.PluginConfig{
			{Name: failedBootName, Internal: true, Encoder: plugin.EncodingJSON},
		})
		require.Error(t, err)
		require.Contains(t, spawner.pm.ConfiguredNames(), failedBootName,
			"a boot-configured plugin whose start failed must keep its configured name for doctor and restart")
	})
}

func hasStoppedBusWarning(name string) bool {
	for _, w := range report.Warnings() {
		if w.Code == diagnostic.CodeDoctorPluginStopped && w.Subject == name {
			return true
		}
	}
	return false
}

func takePluginDownForTest(t *testing.T, s *Server, pm *process.ProcessManager, name string, cause process.StopCause) {
	t.Helper()
	proc := pm.GetProcess(name)
	require.NotNil(t, proc)
	pm.RemoveProcess(name)
	proc.Stop()
	require.NoError(t, proc.Wait(s.Context()))
	proc.WaitRuntimeCleanup()
	pm.RecordStopped(name, process.PluginStop{Cause: cause, Detail: "simulated down state"})
	t.Cleanup(func() { pm.ClearStopped(name) })
	require.Nil(t, pm.GetProcess(name))
	_, down := pm.Stopped(name)
	require.True(t, down)
	require.True(t, hasStoppedBusWarning(name))
}

// TestReloadRemovingDownNonFIBPluginUnconfiguresOnAccept verifies that an
// accepted reload removing the config root of an already-down non-FIB plugin
// (and its down dependency) drops them from ConfiguredNames, pm.stopped, and
// the report bus only once the reload scope is accepted.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-1 --
// collectNonFIBRemovedProcesses, collectOrphanedDependencies,
// stopCollectedProcesses, and finishRemovalScope(scope, true) drop down
// non-FIB plugins via UnconfigureProcess.
// PREVENTS: a removed down plugin keeping a stale doctor-plugin-stopped row
// and remaining restartable through `request plugin restart`.
func TestReloadRemovingDownNonFIBPluginUnconfiguresOnAccept(t *testing.T) {
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })

	const (
		keptName = "down-nonfib-kept"
		depName  = "down-nonfib-dep"
		downName = "down-nonfib-owner"
		downRoot = "down-nonfib-root"
	)
	registerIdle := func(name string, roots, deps []string) {
		require.NoError(t, registry.Register(registry.Registration{
			Name:         name,
			Description:  "down non-FIB removal test plugin",
			ConfigRoots:  roots,
			Dependencies: deps,
			CLIHandler:   func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(name, conn)
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))
	}
	registerIdle(keptName, nil, nil)
	registerIdle(depName, nil, nil)
	registerIdle(downName, []string{downRoot}, []string{depName})

	var s *Server
	t.Cleanup(func() {
		if s != nil {
			s.wg.Wait()
		}
	})
	s, spawner := newLifecycleStartupServer(t)
	s.SetShutdownFunc(func() {})
	s.reactor = &removalRecoveryReactor{tree: map[string]any{downRoot: map[string]any{"enabled": true}}}

	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: keptName, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: depName, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: downName, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	s.recordAutoLoadReasons([]AutoLoadedEndpoint{
		{
			Config: plugin.PluginConfig{Name: depName, Internal: true, Encoder: plugin.EncodingJSON},
			Reason: AutoLoadReason{Kind: AutoLoadReasonDependency, Source: downName},
		},
	})

	takePluginDownForTest(t, s, spawner.pm, downName, process.StopHandshakeFailed)
	takePluginDownForTest(t, s, spawner.pm, depName, process.StopHandshakeFailed)

	reloadCtx, finish := s.DeferReloadAcceptance(t.Context())
	defer finish(false) //nolint:errcheck // Cleanup; outcome is asserted below.
	require.NoError(t, s.ReloadConfig(reloadCtx, map[string]any{}))

	// Pending scope keeps the down plugins configured and reported down until accepted.
	require.Contains(t, spawner.pm.ConfiguredNames(), downName)
	require.Contains(t, spawner.pm.ConfiguredNames(), depName)
	_, stillDown := spawner.pm.Stopped(downName)
	require.True(t, stillDown)

	require.NoError(t, finish(true))

	require.Equal(t, []string{keptName}, spawner.pm.ConfiguredNames())
	_, downAfter := spawner.pm.Stopped(downName)
	require.False(t, downAfter)
	_, depDownAfter := spawner.pm.Stopped(depName)
	require.False(t, depDownAfter)
	require.False(t, hasStoppedBusWarning(downName))
	require.False(t, hasStoppedBusWarning(depName))
	require.False(t, s.isPluginLoaded(downName))
	require.False(t, s.isPluginLoaded(depName))

	refusal := pluginRestartRefusal(spawner.pm, downName)
	require.NotNil(t, refusal)
	require.Contains(t, refusal.Error(), "is not a configured plugin")
}

// TestReloadRemovingDownFIBPluginUnconfiguresOnAccept verifies that an
// accepted reload removing or displacing an already-down ProgramsFIB plugin
// drops it from ConfiguredNames, pm.stopped, and the report bus.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-1 --
// judgeReloadWriters collects down ProgramsFIB configs from ConfiguredConfigs
// and finishRemovalScope(scope, true) calls UnconfigureProcess.
// PREVENTS: a removed down FIB plugin lingering in ConfiguredNames and keeping
// doctor-plugin-stopped raised after its config block leaves.
func TestReloadRemovingDownFIBPluginUnconfiguresOnAccept(t *testing.T) {
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })

	const (
		keptName = "down-fib-kept"
		fibName  = "down-fib-writer"
		fibRoot  = "down-fib-root"
	)
	require.NoError(t, registry.Register(registry.Registration{
		Name:        keptName,
		Description: "kept baseline plugin",
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(keptName, conn)
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:        fibName,
		Description: "down FIB writer test plugin",
		ProgramsFIB: true,
		ConfigRoots: []string{fibRoot},
		CLIHandler:  func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(fibName, conn)
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
	}))

	var s *Server
	t.Cleanup(func() {
		if s != nil {
			s.wg.Wait()
		}
	})
	s, spawner := newLifecycleStartupServer(t)
	s.SetShutdownFunc(func() {})
	s.reactor = &removalRecoveryReactor{tree: map[string]any{fibRoot: map[string]any{"enabled": true}}}

	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: keptName, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: fibName, Internal: true, Encoder: plugin.EncodingJSON},
	}))

	takePluginDownForTest(t, s, spawner.pm, fibName, process.StopRestartFailed)

	reloadCtx, finish := s.DeferReloadAcceptance(t.Context())
	defer finish(false) //nolint:errcheck // Cleanup; outcome is asserted below.
	require.NoError(t, s.ReloadConfig(reloadCtx, map[string]any{}))

	require.Contains(t, spawner.pm.ConfiguredNames(), fibName)
	_, stillDown := spawner.pm.Stopped(fibName)
	require.True(t, stillDown)

	require.NoError(t, finish(true))

	require.Equal(t, []string{keptName}, spawner.pm.ConfiguredNames())
	_, downAfter := spawner.pm.Stopped(fibName)
	require.False(t, downAfter)
	require.False(t, hasStoppedBusWarning(fibName))

	refusal := pluginRestartRefusal(spawner.pm, fibName)
	require.NotNil(t, refusal)
	require.Contains(t, refusal.Error(), "is not a configured plugin")
}

// TestRejectedReloadKeepsDownPluginConfiguredAndStopped verifies the AC-1
// survivor: when a deferred reload removing down plugins (non-FIB and FIB) is
// rejected, both remain in ConfiguredNames with their stop records and bus
// warnings intact.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-1 survivor
// -- finishRemovalScope(scope, false) discards s.unconfigureOnAccept[scope].
// PREVENTS: a rejected reload prematurely dropping configured down plugins.
func TestRejectedReloadKeepsDownPluginConfiguredAndStopped(t *testing.T) {
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })

	const (
		nonFIBName = "rejected-down-nonfib"
		nonFIBRoot = "rejected-down-nonfib-root"
		fibName    = "rejected-down-fib"
		fibRoot    = "rejected-down-fib-root"
	)
	registerDown := func(name, root string, programsFIB bool) {
		require.NoError(t, registry.Register(registry.Registration{
			Name:        name,
			Description: "rejected reload down plugin",
			ProgramsFIB: programsFIB,
			ConfigRoots: []string{root},
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(name, conn)
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))
	}
	registerDown(nonFIBName, nonFIBRoot, false)
	registerDown(fibName, fibRoot, true)

	var s *Server
	t.Cleanup(func() {
		if s != nil {
			s.wg.Wait()
		}
	})
	s, spawner := newLifecycleStartupServer(t)
	s.SetShutdownFunc(func() {})
	committed := map[string]any{
		nonFIBRoot: map[string]any{"enabled": true},
		fibRoot:    map[string]any{"enabled": true},
	}
	s.reactor = &removalRecoveryReactor{tree: committed}

	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: nonFIBName, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: fibName, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	takePluginDownForTest(t, s, spawner.pm, nonFIBName, process.StopHandshakeFailed)
	takePluginDownForTest(t, s, spawner.pm, fibName, process.StopRestartFailed)

	reloadCtx, finish := s.DeferReloadAcceptance(t.Context())
	defer finish(false) //nolint:errcheck // Cleanup; outcome is asserted below.
	require.NoError(t, s.ReloadConfig(reloadCtx, map[string]any{}))

	require.NoError(t, finish(false))

	require.Contains(t, spawner.pm.ConfiguredNames(), nonFIBName)
	require.Contains(t, spawner.pm.ConfiguredNames(), fibName)
	_, nonFIBDown := spawner.pm.Stopped(nonFIBName)
	require.True(t, nonFIBDown)
	_, fibDown := spawner.pm.Stopped(fibName)
	require.True(t, fibDown)
	require.True(t, hasStoppedBusWarning(nonFIBName))
	require.True(t, hasStoppedBusWarning(fibName))
	require.Nil(t, pluginRestartRefusal(spawner.pm, nonFIBName))
	require.Nil(t, pluginRestartRefusal(spawner.pm, fibName))
}

// TestFailedReloadAutoLoadLeavesNoStoppedRecordOrConfig verifies that a
// candidate reload auto-load failing below StageRunning (including unstarted
// dependents rolled back by rollbackNonRunningStartupProcesses) or during
// runProcessGenerationReady at StageRunning leaves neither pm.configs nor
// pm.stopped nor a doctor-plugin-stopped bus warning, while a boot failure
// still keeps both.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-10 --
// runPluginPhaseWithin skips RecordStopped and calls UnconfigureProcess when
// unconfigureFailedReloadStart applies, and preserves boot failure stop records.
// PREVENTS: a refused reload leaking doctor-plugin-stopped and leaving an
// uncommitted plugin restartable.
func TestFailedReloadAutoLoadLeavesNoStoppedRecordOrConfig(t *testing.T) {
	t.Run("below-running", func(t *testing.T) {
		snapshot := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snapshot) })

		const (
			baselineName = "ac10-below-baseline"
			depName      = "ac10-below-dep"
			ownerName    = "ac10-below-owner"
			ownerRoot    = "ac10-below-root"
		)
		require.NoError(t, registry.Register(registry.Registration{
			Name:        baselineName,
			Description: "committed baseline plugin",
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(baselineName, conn)
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))
		require.NoError(t, registry.Register(registry.Registration{
			Name:        depName,
			Description: "tier-0 dependency failing Stage 2",
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(depName, conn)
				p.OnConfigure(func([]sdk.ConfigSection) error {
					return errors.New("candidate dep configure refused")
				})
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))
		require.NoError(t, registry.Register(registry.Registration{
			Name:         ownerName,
			Description:  "tier-1 owner blocked behind failing dep",
			ConfigRoots:  []string{ownerRoot},
			Dependencies: []string{depName},
			CLIHandler:   func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(ownerName, conn)
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))

		var s *Server
		t.Cleanup(func() {
			if s != nil {
				s.wg.Wait()
			}
		})
		s, spawner := newLifecycleStartupServer(t)
		s.SetShutdownFunc(func() {})
		s.reactor = &removalRecoveryReactor{tree: map[string]any{}}

		require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
			{Name: baselineName, Internal: true, Encoder: plugin.EncodingJSON},
		}))

		err := s.ReloadConfig(t.Context(), map[string]any{ownerRoot: map[string]any{"enabled": true}})
		require.Error(t, err)

		for _, name := range []string{depName, ownerName} {
			require.Nil(t, spawner.pm.GetProcess(name))
			require.NotContains(t, spawner.pm.ConfiguredNames(), name)
			_, down := spawner.pm.Stopped(name)
			require.False(t, down, "failed candidate reload auto-load %s must not leak pm.stopped", name)
			require.False(t, hasStoppedBusWarning(name), "failed candidate reload auto-load %s must not raise doctor-plugin-stopped", name)
		}
		require.Contains(t, spawner.pm.ConfiguredNames(), baselineName)
		require.NotNil(t, spawner.pm.GetProcess(baselineName))
	})

	t.Run("generation-ready", func(t *testing.T) {
		snapshot := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snapshot) })

		const (
			baselineName  = "ac10-ready-baseline"
			readyFailName = "ac10-ready-candidate"
			readyRoot     = "ac10-ready-root"
		)
		require.NoError(t, registry.Register(registry.Registration{
			Name:        baselineName,
			Description: "committed baseline plugin",
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(baselineName, conn)
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))
		require.NoError(t, registry.Register(registry.Registration{
			Name:        readyFailName,
			Description: "candidate failing generation-ready hook",
			ConfigRoots: []string{readyRoot},
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(readyFailName, conn)
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))

		engineOperationMu.Lock()
		savedReadyHooks := slices.Clone(processGenerationReady)
		processGenerationReady = append(processGenerationReady, func(_ context.Context, _ *Server, proc *process.Process) error {
			if proc.Name() == readyFailName {
				return errors.New("generation ready hook refused candidate")
			}
			return nil
		})
		engineOperationMu.Unlock()
		t.Cleanup(func() {
			engineOperationMu.Lock()
			processGenerationReady = savedReadyHooks
			engineOperationMu.Unlock()
		})

		var s *Server
		t.Cleanup(func() {
			if s != nil {
				s.wg.Wait()
			}
		})
		s, spawner := newLifecycleStartupServer(t)
		s.SetShutdownFunc(func() {})
		s.reactor = &removalRecoveryReactor{tree: map[string]any{}}

		require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
			{Name: baselineName, Internal: true, Encoder: plugin.EncodingJSON},
		}))

		err := s.ReloadConfig(t.Context(), map[string]any{readyRoot: map[string]any{"enabled": true}})
		require.Error(t, err)
		require.ErrorContains(t, err, "generation ready hook refused candidate")

		require.Nil(t, spawner.pm.GetProcess(readyFailName))
		require.NotContains(t, spawner.pm.ConfiguredNames(), readyFailName)
		_, down := spawner.pm.Stopped(readyFailName)
		require.False(t, down, "failed generation-ready candidate must not leak pm.stopped")
		require.False(t, hasStoppedBusWarning(readyFailName), "failed generation-ready candidate must not raise doctor-plugin-stopped")
		require.False(t, s.isPluginLoaded(readyFailName))
		require.Contains(t, spawner.pm.ConfiguredNames(), baselineName)
		require.NotNil(t, spawner.pm.GetProcess(baselineName))
	})

	t.Run("boot-failure-survivor", func(t *testing.T) {
		snapshot := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snapshot) })

		const bootFailName = "ac10-boot-failed-survivor"
		require.NoError(t, registry.Register(registry.Registration{
			Name:        bootFailName,
			Description: "boot plugin failing Stage 2",
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(bootFailName, conn)
				p.OnConfigure(func([]sdk.ConfigSection) error {
					return errors.New("boot configure refused")
				})
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))

		var s *Server
		t.Cleanup(func() {
			if s != nil {
				s.wg.Wait()
			}
		})
		s, spawner := newLifecycleStartupServer(t)
		s.SetShutdownFunc(func() {})
		t.Cleanup(func() {
			if spawner.pm != nil {
				spawner.pm.ClearStopped(bootFailName)
			}
		})

		err := s.runPluginPhase([]plugin.PluginConfig{
			{Name: bootFailName, Internal: true, Encoder: plugin.EncodingJSON},
		})
		require.Error(t, err)
		require.Contains(t, spawner.pm.ConfiguredNames(), bootFailName)
		stop, down := spawner.pm.Stopped(bootFailName)
		require.True(t, down, "boot failure must still record pm.stopped")
		require.Equal(t, process.StopHandshakeFailed, stop.Cause)
		require.True(t, hasStoppedBusWarning(bootFailName), "boot failure must still raise doctor-plugin-stopped")
	})
}
