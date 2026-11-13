// Design: docs/architecture/api/process-protocol.md -- configure and replacement startup allowances.

package server

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config/transaction"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func setStartupStageTimeout(t *testing.T, value string) {
	t.Helper()
	// Setenv restores the process environment before this cleanup clears the
	// cached value, so later tests read their own restored environment.
	t.Cleanup(env.ResetCache)
	t.Setenv("ZE_PLUGIN_STAGE_TIMEOUT", value)
	env.ResetCache()
}

func TestStartupConfigureBudgetCapsBeforeDuration(t *testing.T) {
	for _, test := range []struct {
		name  string
		input *plugin.PluginRegistration
		want  time.Duration
	}{
		{name: "absent"},
		{name: "trivial", input: &plugin.PluginRegistration{}},
		{name: "negative", input: &plugin.PluginRegistration{ApplyBudget: -1}},
		{name: "ordinary", input: &plugin.PluginRegistration{ApplyBudget: 7}, want: 7 * time.Second},
		{name: "ceiling", input: &plugin.PluginRegistration{ApplyBudget: transaction.MaxBudgetSeconds}, want: transaction.MaxBudgetSeconds * time.Second},
		{name: "oversized", input: &plugin.PluginRegistration{ApplyBudget: math.MaxInt}, want: transaction.MaxBudgetSeconds * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := declaredConfigureBudget(test.input); got != test.want {
				t.Fatalf("declared configure budget = %s, want %s", got, test.want)
			}
		})
	}
}

func TestStartupConfigureBudgetUsesAcceptedRegistrations(t *testing.T) {
	setStartupStageTimeout(t, "1h")
	s, _, _ := registeredConfigureBudgetPair(t, 60)
	if got := s.coordinator.ConfigureStall(time.Second); got != time.Minute {
		t.Fatalf("accepted tier declarations produced %s, want 1m", got)
	}
}

func TestStartupConfigureBudgetTimeoutSelection(t *testing.T) {
	setStartupStageTimeout(t, "1h")
	s, fast, _ := registeredConfigureBudgetPair(t, 60)
	setStartupStageTimeout(t, "100ms")
	for _, test := range []struct {
		name string
		from plugin.PluginStage
		to   plugin.PluginStage
		want time.Duration
	}{
		{name: "registration", from: plugin.StageRegistration, to: plugin.StageConfig, want: 100 * time.Millisecond},
		{name: "configure", from: plugin.StageConfig, to: plugin.StageCapability, want: time.Minute},
		{name: "capability", from: plugin.StageCapability, to: plugin.StageRegistry, want: 100 * time.Millisecond},
		{name: "registry", from: plugin.StageRegistry, to: plugin.StageReady, want: 100 * time.Millisecond},
		{name: "ready", from: plugin.StageReady, to: plugin.StageRunning, want: 100 * time.Millisecond},
		{name: "unrelated transition", from: plugin.StageConfig, to: plugin.StageRegistry, want: 100 * time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := startupStageStall(s.coordinator, fast, test.from, test.to); got != test.want {
				t.Fatalf("stage stall = %s, want %s", got, test.want)
			}
		})
	}
	explicit := process.NewProcess(plugin.PluginConfig{Name: "configure-explicit", StageTimeout: 25 * time.Millisecond})
	if got := startupStageStall(s.coordinator, explicit, plugin.StageConfig, plugin.StageCapability); got != 25*time.Millisecond {
		t.Fatalf("declared allowance replaced the explicit operator bound: %s", got)
	}
	for _, disabled := range []string{"0s", "-1s"} {
		setStartupStageTimeout(t, disabled)
		want, err := time.ParseDuration(disabled)
		if err != nil {
			t.Fatal(err)
		}
		if got := startupStageStall(s.coordinator, fast, plugin.StageConfig, plugin.StageCapability); got != want {
			t.Fatalf("disabled environment stall %s became %s", disabled, got)
		}
	}
}

func TestStartupConfigureBudgetKeepsFastSibling(t *testing.T) {
	setStartupStageTimeout(t, "1h")
	s, fast, slow := registeredConfigureBudgetPair(t, 60)
	const base = 100 * time.Millisecond
	setStartupStageTimeout(t, base.String())
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	entered := make(chan struct{})
	observed := &startupWaitObservedContext{Context: ctx, entered: entered}
	result := make(chan bool, 1)
	var worker sync.WaitGroup
	t.Cleanup(func() { cancel(); worker.Wait() })
	worker.Go(func() {
		sink := &engineStartupSink{s: s, proc: fast, startup: observed}
		result <- sink.transition(plugin.StageConfig, plugin.StageCapability)
	})
	select {
	case <-entered:
	case succeeded := <-result:
		t.Fatalf("fast sibling did not enter its configure wait: %v", succeeded)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// The deadline itself is the subject: hold the configuring sibling beyond
	// the old stall, while remaining well inside its declared one-minute work.
	timer := time.NewTimer(10 * base)
	defer timer.Stop()
	select {
	case succeeded := <-result:
		t.Fatalf("fast sibling left a still-configuring declared owner: %v", succeeded)
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	slowSink := &engineStartupSink{s: s, proc: slow, startup: ctx}
	if !slowSink.transition(plugin.StageConfig, plugin.StageCapability) {
		t.Fatal("slow declared owner could not complete its configure stage")
	}
	select {
	case succeeded := <-result:
		if !succeeded || fast.Stage() != plugin.StageCapability || slow.Stage() != plugin.StageCapability {
			t.Fatalf("configured siblings did not advance: result=%v fast=%v slow=%v", succeeded, fast.Stage(), slow.Stage())
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestRestartConfigureBudgetAcceptsOnlyPreviousDeclaration(t *testing.T) {
	proc := process.NewProcess(plugin.PluginConfig{Name: "configure-prior"})
	proc.SetRegistration(&plugin.PluginRegistration{ApplyBudget: 60})
	proc.SetStage(plugin.StageRegistration)
	if got := previousConfigureBudget(proc); got != 0 {
		t.Fatalf("unaccepted Stage1 declaration granted %s", got)
	}
	proc.SetStage(plugin.StageConfig)
	if got := previousConfigureBudget(proc); got != time.Minute {
		t.Fatalf("accepted outgoing declaration granted %s, want 1m", got)
	}
	if got := previousConfigureBudget(nil); got != 0 {
		t.Fatalf("missing outgoing owner granted %s", got)
	}
}

func TestRestartConfigureBudgetReplacesOnlyConfigureWindow(t *testing.T) {
	setStartupStageTimeout(t, "100ms")
	proc := process.NewProcess(plugin.PluginConfig{Name: "configure-restart"})
	if got := restartConfigureHandshakeBound(proc, time.Minute); got != 400*time.Millisecond+time.Minute {
		t.Fatalf("restart allowance = %s, want four ordinary windows plus 1m", got)
	}
	if got := restartConfigureHandshakeBound(proc, 0); got != 500*time.Millisecond {
		t.Fatalf("ordinary restart allowance changed to %s", got)
	}
	explicit := process.NewProcess(plugin.PluginConfig{Name: "configure-restart-explicit", StageTimeout: 30 * time.Millisecond})
	if got := restartConfigureHandshakeBound(explicit, time.Minute); got != 150*time.Millisecond {
		t.Fatalf("declared work replaced explicit restart bound: %s", got)
	}
	for _, disabled := range []string{"0s", "-1s"} {
		setStartupStageTimeout(t, disabled)
		if got, want := restartConfigureHandshakeBound(proc, time.Minute), restartHandshakeBound(proc); got != want {
			t.Fatalf("non-positive ordinary restart allowance changed: got %s want %s", got, want)
		}
	}
}

func TestRestartConfigureBudgetHonorsParentContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	s := &Server{ctx: ctx}
	proc := process.NewProcess(plugin.PluginConfig{Name: "configure-parent"})
	if got := restartConfigureHandshakeBound(proc, time.Minute); got < time.Minute {
		t.Fatalf("configure allowance was not selected: %s", got)
	}
	handshake, release := s.boundedRestartHandshake(proc, time.Minute)
	defer release()
	parentDeadline, _ := ctx.Deadline()
	deadline, ok := handshake.Deadline()
	if !ok || deadline.After(parentDeadline) {
		t.Fatalf("configure allowance outlived its parent: %v, want at most %v", deadline, parentDeadline)
	}
	cancel()
	select {
	case <-handshake.Done():
	default:
		t.Fatal("replacement startup ignored parent cancellation")
	}
}

func TestRestartUsesOutgoingConfigureBudget(t *testing.T) {
	for _, mode := range []string{"operator", "rollback"} {
		t.Run(mode, func(t *testing.T) {
			setStartupStageTimeout(t, "1h")
			name := "configure-restart-" + mode
			entered := make(chan struct{})
			hold := make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(hold) }) }
			var starts atomic.Int32
			s := startRestartTargetWith(t, name, func(p *sdk.Plugin) *sdk.Registration {
				generation := starts.Add(1)
				p.OnConfigure(func([]sdk.ConfigSection) error {
					if generation > 1 {
						close(entered)
						<-hold
					}
					return nil
				})
				if generation == 1 {
					return &sdk.Registration{ApplyBudget: 60}
				}
				// A new generation cannot supply its declaration before its
				// handshake starts; this smaller value must not erase the prior one.
				return &sdk.Registration{}
			})
			t.Cleanup(release)
			const base = 100 * time.Millisecond
			setStartupStageTimeout(t, base.String())
			before := s.procManager.Load().GetProcess(name)
			answer := make(chan error, 1)
			var worker sync.WaitGroup
			t.Cleanup(func() { release(); worker.Wait() })
			worker.Go(func() {
				if mode == "rollback" {
					answer <- s.restartPlugin(name)
					return
				}
				response, err := handlePluginRestart(&CommandContext{Server: s}, commandtest.Args("name", name))
				if err == nil && response.Status != plugin.StatusDone {
					err = &rpc.RPCCallError{Message: response.Error}
				}
				answer <- err
			})
			ctx, cancel := context.WithTimeout(t.Context(), restartAnswerBound)
			defer cancel()
			select {
			case <-entered:
			case err := <-answer:
				t.Fatalf("replacement did not reach configure: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			// Hold real Configure beyond the old complete-handshake allowance;
			// the previous accepted declaration is the permitted extra work.
			timer := time.NewTimer(3 * restartHandshakeStages * base)
			defer timer.Stop()
			select {
			case err := <-answer:
				t.Fatalf("replacement answered before configure completed: %v", err)
			case <-timer.C:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			release()
			select {
			case err := <-answer:
				if err != nil {
					t.Fatalf("declared replacement configuration failed: %v", err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			after := s.procManager.Load().GetProcess(name)
			if after == nil || after == before || after.Stage() != plugin.StageRunning {
				t.Fatal("replacement did not commit a new running generation")
			}
			requireTxLockFree(t, s)
		})
	}
}

type startupWaitObservedContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (ctx *startupWaitObservedContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.entered) })
	return ctx.Context.Done()
}

func registeredConfigureBudgetPair(t *testing.T, slowBudget int) (*Server, *process.Process, *process.Process) {
	t.Helper()
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
	s, _ := newLifecycleStartupServer(t)
	coord := plugin.NewStartupCoordinator(2)
	s.coordinator = coord
	procs := []*process.Process{
		process.NewProcess(plugin.PluginConfig{Name: "configure-budget-fast", Internal: true}),
		process.NewProcess(plugin.PluginConfig{Name: "configure-budget-slow", Internal: true}),
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	var workers sync.WaitGroup
	t.Cleanup(func() { cancel(); workers.Wait() })
	results := make(chan bool, 2)
	for id, proc := range procs {
		proc.SetIndex(id)
		proc.SetStage(plugin.StageRegistration)
		declaration := &rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion}
		if id == 1 {
			declaration.ApplyBudget = slowBudget
		}
		sink := &engineStartupSink{s: s, proc: proc, startup: ctx}
		if err := sink.onRegistration(declaration); err != nil {
			t.Fatalf("accepted registration: %v", err)
		}
		workers.Go(func() { results <- sink.transition(plugin.StageRegistration, plugin.StageConfig) })
	}
	for range procs {
		select {
		case succeeded := <-results:
			if !succeeded {
				t.Fatal("registered owner could not enter Configure")
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	return s, procs[0], procs[1]
}
