// Design: docs/architecture/doctor-and-health-checks.md — per-generation doctor worker lifecycle
// Related: doctor_worker.go — doctorWorker, startDoctorWorker, RunDoctorChecks
// Related: failure_policy.go — superviseProcess, which owns each worker
// Related: drain_lifecycle_test.go — sibling per-generation lifecycle proof

package server

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type doctorWorkerPluginSpec struct {
	name   string
	checks []rpc.DoctorCheckDecl
	policy rpc.FailurePolicy
	handle sdk.DoctorCheckHandler
}

type doctorWorkerPluginProbe struct {
	starts    atomic.Int64
	respawned atomic.Pointer[func()]
}

func registerDoctorWorkerPlugin(t *testing.T, spec doctorWorkerPluginSpec, crash <-chan struct{}) *doctorWorkerPluginProbe {
	t.Helper()
	probe := &doctorWorkerPluginProbe{}

	require.NoError(t, registry.Register(registry.Registration{
		Name:        spec.name,
		Description: "doctor worker lifecycle test plugin",
		RunEngine: func(conn net.Conn) int {
			generation := probe.starts.Add(1)
			p := sdk.NewWithConn(spec.name, conn)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if generation == 1 && crash != nil {
				go crashOnSignal(crash, cancel)
			}
			if check := probe.respawned.Load(); generation > 1 && check != nil {
				(*check)()
			}
			if spec.handle != nil {
				p.OnDoctorCheck(spec.handle)
			}
			if err := p.Run(ctx, &sdk.Registration{
				FailurePolicy: spec.policy,
				DoctorChecks:  spec.checks,
			}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	return probe
}

func doctorWorkerOf(s *Server, proc *process.Process) *doctorWorker {
	s.doctorWorkersMu.Lock()
	defer s.doctorWorkersMu.Unlock()
	return s.doctorWorkers[proc]
}

func doctorWorkerCount(s *Server) int {
	s.doctorWorkersMu.Lock()
	defer s.doctorWorkersMu.Unlock()
	return len(s.doctorWorkers)
}

// TestDoctorWorkerLifecycle proves AC-1 over real plugin generations started
// through runPluginPhase and supervised by superviseProcess.
//
// VALIDATES: a generation declaring DoctorChecks gets one worker started before
// serving RPCs and stopped/joined before applyFailurePolicy; a plugin declaring
// none gets no worker; a respawn gets a fresh worker with no leaked old worker;
// submission to a stopped worker returns unreachable immediately; full-queue
// waits expire at queueWait and expired queued requests are discarded unrun; a
// late callback answer never blocks worker exit.
// PREVENTS: leaked per-generation doctor workers across restarts, workers
// created for plugins without checks, blocking on dead/saturated plugins, and
// running already-expired queued doctor checks.
func TestDoctorWorkerLifecycle(t *testing.T) {
	t.Run("generation-lifecycle-and-respawn", func(t *testing.T) {
		snap := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snap) })

		crash := make(chan struct{})
		life := doctorWorkerPluginSpec{
			name:   "doctor-life",
			policy: rpc.FailureRestart,
			checks: []rpc.DoctorCheckDecl{{
				Name: "life-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"},
				Codes: []string{"doctor-life-ok"},
			}},
			handle: func(string) ([]rpc.DoctorCheckDiagnostic, error) {
				return []rpc.DoctorCheckDiagnostic{{
					Code: "doctor-life-ok", Severity: "info", Message: "life ok",
				}}, nil
			},
		}
		none := doctorWorkerPluginSpec{
			name:   "doctor-none",
			policy: rpc.FailureIgnore,
		}
		probe := registerDoctorWorkerPlugin(t, life, crash)
		registerDoctorWorkerPlugin(t, none, nil)

		s, _ := newLifecycleStartupServer(t)
		require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
			{Name: life.name, Internal: true, Encoder: plugin.EncodingJSON},
			{Name: none.name, Internal: true, Encoder: plugin.EncodingJSON},
		}))
		pm := s.procManager.Load()
		require.NotNil(t, pm)
		first := pm.GetProcess(life.name)
		require.NotNil(t, first)
		noChecks := pm.GetProcess(none.name)
		require.NotNil(t, noChecks)

		require.Eventually(t, func() bool { return doctorWorkerOf(s, first) != nil }, failureWait, 10*time.Millisecond,
			"a generation declaring DoctorChecks has no doctor worker")
		assert.Equal(t, 1, doctorWorkerCount(s), "plugin without DoctorChecks must not receive a worker")
		assert.Nil(t, doctorWorkerOf(s, noChecks), "a generation declaring no DoctorChecks got a doctor worker")
		old := doctorWorkerOf(s, first)

		var oldReturned, oldRemoved atomic.Bool
		checked := make(chan struct{})
		var checkOnce sync.Once
		check := func() {
			oldReturned.Store(isClosed(old.done))
			oldRemoved.Store(doctorWorkerOf(s, first) == nil)
			checkOnce.Do(func() { close(checked) })
		}
		probe.respawned.Store(&check)
		close(crash)

		require.Eventually(t, func() bool { return pm.GetProcess(life.name) != first }, failureWait, 10*time.Millisecond,
			"plugin exited and failure policy never respawned it")
		select {
		case <-checked:
		case <-time.After(failureWait):
			t.Fatal("respawned generation never ran its lifecycle check")
		}
		assert.True(t, oldReturned.Load(), "old doctor worker had not returned before respawn")
		assert.True(t, oldRemoved.Load(), "old doctor worker was still registered before respawn")

		second := pm.GetProcess(life.name)
		require.Eventually(t, func() bool { return doctorWorkerOf(s, second) != nil }, failureWait, 10*time.Millisecond,
			"respawned generation has no doctor worker")
		assert.NotSame(t, old, doctorWorkerOf(s, second), "respawn reused old doctor worker")
		assert.Equal(t, 1, doctorWorkerCount(s), "respawn must leave exactly one doctor worker")
		assert.Equal(t, int64(2), probe.starts.Load())

		results := s.RunDoctorChecks([]DoctorCheckPluginRequest{
			{Plugin: life.name, Checks: []string{"life-check"}},
		}, 2*time.Second, 2*time.Second)
		require.Len(t, results, 1)
		require.Len(t, results[0].Outcomes, 1)
		require.NoError(t, results[0].Outcomes[0].Err)
		require.NotNil(t, results[0].Outcomes[0].Output)
		require.Len(t, results[0].Outcomes[0].Output.Diagnostics, 1)
		assert.Equal(t, "doctor-life-ok", results[0].Outcomes[0].Output.Diagnostics[0].Code)
	})

	t.Run("stopped-worker-returns-unreachable-immediately", func(t *testing.T) {
		snap := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snap) })

		crash := make(chan struct{})
		spec := doctorWorkerPluginSpec{
			name:   "doctor-stop",
			policy: rpc.FailureIgnore,
			checks: []rpc.DoctorCheckDecl{{
				Name: "stop-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"},
				Codes: []string{"doctor-stop-ok"},
			}},
		}
		registerDoctorWorkerPlugin(t, spec, crash)

		s, _ := newLifecycleStartupServer(t)
		require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
			{Name: spec.name, Internal: true, Encoder: plugin.EncodingJSON},
		}))
		pm := s.procManager.Load()
		require.NotNil(t, pm)
		proc := pm.GetProcess(spec.name)
		require.NotNil(t, proc)

		require.Eventually(t, func() bool { return doctorWorkerOf(s, proc) != nil }, failureWait, 10*time.Millisecond,
			"generation declaring DoctorChecks has no doctor worker")
		worker := doctorWorkerOf(s, proc)

		close(crash)
		require.Eventually(t, func() bool {
			return doctorWorkerOf(s, proc) == nil && isClosed(worker.done)
		}, failureWait, 10*time.Millisecond, "stopped generation did not remove and join its doctor worker")

		start := time.Now()
		results := s.RunDoctorChecks([]DoctorCheckPluginRequest{
			{Plugin: spec.name, Checks: []string{"stop-check"}},
		}, 2*time.Second, 2*time.Second)
		elapsed := time.Since(start)

		assert.Less(t, elapsed, 500*time.Millisecond, "stopped worker request waited %v instead of failing immediately", elapsed)
		require.Len(t, results, 1)
		require.Len(t, results[0].Outcomes, 1)
		require.Error(t, results[0].Outcomes[0].Err)
		assert.Contains(t, results[0].Outcomes[0].Err.Error(), spec.name)
	})

	t.Run("full-queue-wait-expires-and-discards-unrun", func(t *testing.T) {
		snap := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snap) })

		var invocations atomic.Int32
		entered := make(chan struct{}, 1)
		gate := make(chan struct{})
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(gate) }) }
		defer release()

		spec := doctorWorkerPluginSpec{
			name:   "doctor-full",
			policy: rpc.FailureIgnore,
			checks: []rpc.DoctorCheckDecl{{
				Name: "full-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"},
				Codes: []string{"doctor-full-ok"},
			}},
			handle: func(string) ([]rpc.DoctorCheckDiagnostic, error) {
				if invocations.Add(1) == 1 {
					select {
					case entered <- struct{}{}:
					default:
					}
					<-gate
				}
				return []rpc.DoctorCheckDiagnostic{{
					Code: "doctor-full-ok", Severity: "info", Message: "ok",
				}}, nil
			},
		}
		registerDoctorWorkerPlugin(t, spec, nil)

		s, _ := newLifecycleStartupServer(t)
		require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
			{Name: spec.name, Internal: true, Encoder: plugin.EncodingJSON},
		}))
		pm := s.procManager.Load()
		require.NotNil(t, pm)
		proc := pm.GetProcess(spec.name)
		require.NotNil(t, proc)
		require.Eventually(t, func() bool { return doctorWorkerOf(s, proc) != nil }, failureWait, 10*time.Millisecond,
			"generation declaring DoctorChecks has no doctor worker")
		worker := doctorWorkerOf(s, proc)

		// 1st request occupies the worker in-flight.
		firstDone := make(chan []DoctorCheckPluginResult, 1)
		go func() {
			firstDone <- s.RunDoctorChecks([]DoctorCheckPluginRequest{
				{Plugin: spec.name, Checks: []string{"full-check"}},
			}, 2*time.Second, 2*time.Second)
		}()
		select {
		case <-entered:
		case <-time.After(failureWait):
			t.Fatal("first doctor check never reached plugin handler")
		}

		// Fill all doctorWorkerQueueDepth buffer slots with requests that will
		// expire before gate opens, plus one extra overflow request past queue depth.
		var wg sync.WaitGroup
		overflowDone := make(chan []DoctorCheckPluginResult, 1)
		fillerResults := make([][]DoctorCheckPluginResult, doctorWorkerQueueDepth)
		for i := range doctorWorkerQueueDepth {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				fillerResults[idx] = s.RunDoctorChecks([]DoctorCheckPluginRequest{
					{Plugin: spec.name, Checks: []string{"full-check"}},
				}, 2*time.Second, 300*time.Millisecond)
			}(i)
		}
		// Wait until all 16 filler requests sit in the buffered queue before
		// sending the 17th queued request so it is guaranteed to hit Pass 1b.
		require.Eventually(t, func() bool { return len(worker.queue) == doctorWorkerQueueDepth },
			failureWait, 5*time.Millisecond, "16 filler requests did not fill the doctor worker queue")
		overflowStart := time.Now()
		go func() {
			overflowDone <- s.RunDoctorChecks([]DoctorCheckPluginRequest{
				{Plugin: spec.name, Checks: []string{"full-check"}},
			}, 2*time.Second, 80*time.Millisecond)
		}()

		var overflow []DoctorCheckPluginResult
		select {
		case overflow = <-overflowDone:
		case <-time.After(2 * time.Second):
			t.Fatal("overflow request past full queue did not expire at queueWait")
		}
		assert.Less(t, time.Since(overflowStart), 1*time.Second)
		require.Len(t, overflow, 1)
		require.Len(t, overflow[0].Outcomes, 1)
		require.Error(t, overflow[0].Outcomes[0].Err)
		assert.Contains(t, overflow[0].Outcomes[0].Err.Error(), spec.name)

		// Wait for all 16 filler requests to expire too, then release the 1st.
		wg.Wait()
		for i, res := range fillerResults {
			require.Len(t, res, 1, "filler %d missing result", i)
			require.Len(t, res[0].Outcomes, 1, "filler %d missing outcome", i)
			require.Error(t, res[0].Outcomes[0].Err, "filler %d should have expired at queueWait", i)
		}

		release()
		firstRes := <-firstDone
		require.Len(t, firstRes, 1)
		require.Len(t, firstRes[0].Outcomes, 1)
		require.NoError(t, firstRes[0].Outcomes[0].Err)

		// Submit one fresh probe to prove the worker drained all expired
		// entries without running them and still serves new requests.
		probeRes := s.RunDoctorChecks([]DoctorCheckPluginRequest{
			{Plugin: spec.name, Checks: []string{"full-check"}},
		}, 2*time.Second, 2*time.Second)
		require.Len(t, probeRes, 1)
		require.Len(t, probeRes[0].Outcomes, 1)
		require.NoError(t, probeRes[0].Outcomes[0].Err)
		assert.Equal(t, int32(2), invocations.Load(), "expired queued requests must be discarded unrun")
	})

	t.Run("late-callback-does-not-block-worker-exit", func(t *testing.T) {
		snap := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snap) })

		crash := make(chan struct{})
		releaseLate := make(chan struct{})
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(releaseLate) }) }
		defer release()

		spec := doctorWorkerPluginSpec{
			name:   "doctor-late",
			policy: rpc.FailureIgnore,
			checks: []rpc.DoctorCheckDecl{{
				Name: "late-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"},
				Codes: []string{"doctor-late-ok"},
			}},
			handle: func(string) ([]rpc.DoctorCheckDiagnostic, error) {
				<-releaseLate
				return []rpc.DoctorCheckDiagnostic{{
					Code: "doctor-late-ok", Severity: "info", Message: "late answer",
				}}, nil
			},
		}
		registerDoctorWorkerPlugin(t, spec, crash)

		s, _ := newLifecycleStartupServer(t)
		require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
			{Name: spec.name, Internal: true, Encoder: plugin.EncodingJSON},
		}))
		pm := s.procManager.Load()
		require.NotNil(t, pm)
		proc := pm.GetProcess(spec.name)
		require.NotNil(t, proc)
		require.Eventually(t, func() bool { return doctorWorkerOf(s, proc) != nil }, failureWait, 10*time.Millisecond,
			"generation declaring DoctorChecks has no doctor worker")
		worker := doctorWorkerOf(s, proc)

		results := s.RunDoctorChecks([]DoctorCheckPluginRequest{
			{Plugin: spec.name, Checks: []string{"late-check"}},
		}, 80*time.Millisecond, 500*time.Millisecond)
		require.Len(t, results, 1)
		require.Len(t, results[0].Outcomes, 1)
		require.Error(t, results[0].Outcomes[0].Err)

		// Let the late callback finish and end the generation: worker must exit
		// cleanly without blocking on an abandoned reply channel.
		release()
		close(crash)
		require.Eventually(t, func() bool {
			return doctorWorkerOf(s, proc) == nil && isClosed(worker.done)
		}, failureWait, 10*time.Millisecond, "late callback answer blocked worker exit")
	})
}
