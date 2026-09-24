// Design: docs/architecture/api/process-protocol.md -- live removal callback ordering
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
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
			registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
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
				runResult <- p.Run(context.Background(), sdk.Registration{FailurePolicy: rpc.FailureFatal})
				if err := p.Close(); err != nil {
					return 1
				}
				return 0
			})
			s, spawner := newLifecycleStartupServer(t)
			s.dispatcher.Register(commandName, func(*CommandContext, []string) (*plugin.Response, error) {
				return &plugin.Response{Status: plugin.StatusDone}, nil
			}, "Read removal probe")
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
	run := func(p *sdk.Plugin, registration sdk.Registration) int {
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
			p.OnExecuteCommand(func(_, _ string, args []string, _ string) (string, any, error) {
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
				left := make(map[string]bool, len(installed))
				for name, present := range installed {
					left[name] = present
				}
				stateMu.Unlock()
				residue <- left
				callbacks <- nil
				return nil
			})
			return run(p, sdk.Registration{Commands: []sdk.CommandDecl{{Name: backendCommand}}})
		},
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name: middle, Description: "optional backend consumer",
		OptionalDependencies: []string{backend},
		CLIHandler:           func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(middle, conn)
			p.OnExecuteCommand(func(_, _ string, _ []string, _ string) (string, any, error) {
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
			return run(p, sdk.Registration{Commands: []sdk.CommandDecl{{Name: middleCommand}}})
		},
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name: producer, Description: "configured state producer",
		ConfigRoots: []string{root}, Dependencies: []string{middle},
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
			return run(p, sdk.Registration{})
		},
	}))
	s, spawner := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: backend, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: middle, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: producer, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	require.NoError(t, s.autoStopForRemovedConfigPaths([]string{root}))
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
				run := func(p *sdk.Plugin, registration sdk.Registration) int {
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
						p.OnExecuteCommand(func(_, _ string, _ []string, _ string) (string, any, error) {
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
						return run(p, sdk.Registration{Commands: []sdk.CommandDecl{{Name: command}}})
					},
				}))
				require.NoError(t, registry.Register(registry.Registration{
					Name: producer, Description: "retryable removal source",
					ConfigRoots: []string{root}, Dependencies: []string{backend},
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
						registration := sdk.Registration{FailurePolicy: rpc.FailureFatal}
						if scenario.outerRejected {
							registration.WantsConfig = []string{root}
						}
						return run(p, registration)
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
							return run(p, sdk.Registration{WantsConfig: []string{counterRoot}})
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
				reactor := &removalRecoveryReactor{mockReloadReactor: mockReloadReactor{tree: committed}}
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
				defer finish(false)
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
				finish(false)
				require.Equal(t, committed, reactor.GetConfigTree())
				if scenario.participant {
					require.Equal(t, int64(42), appliedCounter.Load(), "outer rejection must compensate participants without another reload")
				}
				if scenario.failedRecovery {
					require.Eventually(t, func() bool {
						s.removalMu.Lock()
						defer s.removalMu.Unlock()
						attempt := s.removals[old]
						return attempt != nil && !attempt.recovering && attempt.recoveryErr != nil
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
