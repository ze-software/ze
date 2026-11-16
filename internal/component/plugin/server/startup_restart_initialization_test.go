// Design: docs/architecture/api/process-protocol.md -- bounded replacement initialization.
// Related: startup_budget.go -- outgoing declarations retain both accepted phases.

package server

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// MUTATION: restoring the five-stage-only restart cap expires at 140s during
// the second legal 100s callback, before the replacement can become usable.
func TestRestartJoinsSeparatelyBudgetedConfigurationAndInitialization(t *testing.T) {
	setStartupStageTimeout(t, "5s")
	synctest.Test(t, func(t *testing.T) {
		fixtureCtx, cancelFixture := context.WithCancel(t.Context())
		defer cancelFixture()
		const name = "restart-configure-and-initialize"
		var starts atomic.Int32
		s := startRestartTargetWith(t, name, func(p *sdk.Plugin) *sdk.Registration {
			replacement := starts.Add(1) > 1
			p.OnConfigure(func([]sdk.ConfigSection) error {
				if replacement {
					return waitRestartFixtureWork(fixtureCtx, fixtureCtx, 100*time.Second)
				}
				return nil
			})
			p.OnStarted(func(ctx context.Context) error {
				if replacement {
					return waitRestartFixtureWork(ctx, fixtureCtx, 100*time.Second)
				}
				return nil
			})
			return &sdk.Registration{ApplyBudget: 120}
		})
		before := s.procManager.Load().GetProcess(name)
		began := time.Now()
		if err := s.restartPlugin(name); err != nil {
			t.Fatalf("accepted configuration and initialization failed: %v", err)
		}
		if elapsed := time.Since(began); elapsed < 200*time.Second {
			t.Fatalf("restart returned before both callbacks completed: %s", elapsed)
		}
		replacement := s.procManager.Load().GetProcess(name)
		if replacement == nil || replacement == before || replacement.Stage() != plugin.StageRunning {
			t.Fatal("restart did not admit the initialized replacement")
		}
		requireTxLockFree(t, s)
	})
}

// MUTATION: adding a local 5s deadline to an unbarriered wire wait breaks the
// established failure-policy lifetime for a slow but progressing replacement.
func TestLifetimeRestartRetainsSlowStagePolicy(t *testing.T) {
	for _, initializer := range []bool{false, true} {
		name := "configure"
		if initializer {
			name = "initializer"
		}
		t.Run(name, func(t *testing.T) {
			setStartupStageTimeout(t, "5s")
			synctest.Test(t, func(t *testing.T) {
				fixtureCtx, cancelFixture := context.WithCancel(t.Context())
				defer cancelFixture()
				var starts atomic.Int32
				pluginName := "lifetime-restart-" + name
				s := startRestartTargetWith(t, pluginName, func(p *sdk.Plugin) *sdk.Registration {
					replacement := starts.Add(1) > 1
					if initializer {
						p.OnStarted(func(ctx context.Context) error {
							if replacement {
								return waitRestartFixtureWork(ctx, fixtureCtx, 10*time.Second)
							}
							return nil
						})
					} else {
						p.OnConfigure(func([]sdk.ConfigSection) error {
							if replacement {
								return waitRestartFixtureWork(fixtureCtx, fixtureCtx, 10*time.Second)
							}
							return nil
						})
					}
					return &sdk.Registration{}
				})
				before := s.procManager.Load().GetProcess(pluginName)
				began := time.Now()
				if err := s.restartPluginWithin(pluginName, s.lifetimeRestartHandshake); err != nil {
					t.Fatalf("slow replacement failed before its lifetime ended: %v", err)
				}
				if elapsed := time.Since(began); elapsed < 10*time.Second {
					t.Fatalf("restart admitted an unfinished stage after %s", elapsed)
				}
				replacement := s.procManager.Load().GetProcess(pluginName)
				if replacement == nil || replacement == before || replacement.Stage() != plugin.StageRunning {
					t.Fatal("lifetime restart did not admit the initialized replacement")
				}
			})
		})
	}
}

func TestUnbarrieredStartupRetainsParentBound(t *testing.T) {
	for _, stage := range []plugin.PluginStage{plugin.StageConfig, plugin.StageRunning} {
		t.Run(stage.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				defer cancel()
				sink := &engineStartupSink{s: &Server{ctx: ctx}, proc: process.NewProcess(plugin.PluginConfig{Name: "parent-bound", StageTimeout: time.Second})}
				began := time.Now()
				err := sink.runStartupWire(ctx, stage, 120, func(stageCtx context.Context) error { <-stageCtx.Done(); return stageCtx.Err() })
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("parent deadline was not returned: %v", err)
				}
				if elapsed := time.Since(began); elapsed != 8*time.Second {
					t.Fatalf("unbarriered stage replaced its parent's authority: %s", elapsed)
				}
				canceled, stop := context.WithCancel(context.Background())
				stop()
				err = sink.runStartupWire(canceled, stage, 120, func(stageCtx context.Context) error { return stageCtx.Err() })
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("unbarriered stage lost parent cancellation: %v", err)
				}
			})
		})
	}
}

// The shared runner passes a background context to SDK Run. Fixture work also
// follows test-owned cancellation so a failed assertion unwinds before server
// cleanup joins the callback. The timer models the accepted work duration.
func waitRestartFixtureWork(ctx, fixtureCtx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-fixtureCtx.Done():
		return fixtureCtx.Err()
	}
}
