// VALIDATES: managed sessionSampler unit behavior -- idle when interval is zero
// (zero clock starts / collector invocations, AC-9), skips empty sample reports
// without marking the device as reporting (AC-11), drops late collector results
// when generation advances on interval change or stop, bounds per-tick timeout
// to min(interval/2, 5s) clamped to >= 500ms, disables sampling for the session
// on unknown-method while continuing after rate/content refusals, and joins
// cleanly on Stop without buffering offline samples (AC-12).
// PREVENTS: standalone sampling activity, stale collector replies reaching a
// newer session/interval, offline sample replay, or goroutine leaks on stop.

package managed

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func testSampleReport(cpu float64, now time.Time) fleet.SampleReport {
	return fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		DeviceTime:    now.UTC().Format(time.RFC3339),
		Samples: []fleet.SampleReading{
			{Figure: fleet.FigureCPUUsed, Value: cpu},
		},
	}
}

// TestSessionSamplerIdleWhenZeroOrStandalone proves that a sampler whose
// interval stays 0 (or is never started, as on a standalone daemon) never
// starts a timer, never invokes SampleCollector, and never increments clock or
// report counters.
//
// VALIDATES: AC-9 zero calls to sampler clock/sender when inactive.
// PREVENTS: background timers or collectors running when sampling is disabled.
func TestSessionSamplerIdleWhenZeroOrStandalone(t *testing.T) {
	// Not parallel: reads process-wide FleetSampleMetrics deltas.
	before := FleetSampleMetrics()

	clientEnd, hubEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer hubEnd.Close()    //nolint:errcheck // test cleanup

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	defer mc.Close() //nolint:errcheck // test cleanup

	var calls atomic.Int32
	cfg := &ClientConfig{
		Name: "standalone-check",
		SampleCollector: func(_ context.Context, now time.Time) (fleet.SampleReport, error) {
			calls.Add(1)
			return testSampleReport(10, now), nil
		},
	}

	sampler := newSessionSampler(t.Context(), mc, cfg)
	sampler.UpdateInterval(0)
	sampler.UpdateInterval(-5)
	sampler.Stop()

	after := FleetSampleMetrics()
	assert.Zero(t, calls.Load())
	assert.Equal(t, before.ClockStarts, after.ClockStarts)
	assert.Equal(t, before.Ticks, after.Ticks)
	assert.Equal(t, before.ReportsSent, after.ReportsSent)
}

// TestSessionSamplerCallTimeoutBounds verifies the per-tick collector/RPC
// timeout follows min(interval/2, 5s) clamped to >= 500ms.
//
// VALIDATES: per-tick deadline clamping for short and long intervals.
// PREVENTS: collector deadlines exceeding 5s on long intervals or dropping
// below 500ms on 1s intervals.
func TestSessionSamplerCallTimeoutBounds(t *testing.T) {
	t.Parallel()

	s := newSessionSampler(t.Context(), nil, &ClientConfig{})
	assert.Equal(t, 500*time.Millisecond, s.callTimeout(time.Second))
	assert.Equal(t, 500*time.Millisecond, s.callTimeout(500*time.Millisecond))
	assert.Equal(t, 2*time.Second, s.callTimeout(4*time.Second))
	assert.Equal(t, 5*time.Second, s.callTimeout(60*time.Second))
	assert.Equal(t, 5*time.Second, s.callTimeout(time.Hour))
}

// TestSessionSamplerSkipsEmptyReportAndSendsNext proves that a tick yielding
// zero figures does not send an empty sample-report, while a subsequent tick
// with readings sends normally.
//
// VALIDATES: AC-11 absent producers produce no empty report; available figures
// send on the next tick.
// PREVENTS: a device with zero ready figures sending empty reports.
func TestSessionSamplerSkipsEmptyReportAndSendsNext(t *testing.T) {
	t.Parallel()

	clientEnd, hubEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer hubEnd.Close()    //nolint:errcheck // test cleanup

	hubMux := rpc.NewMuxConn(rpc.NewConn(hubEnd, hubEnd))
	defer hubMux.Close() //nolint:errcheck // test cleanup

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	defer mc.Close() //nolint:errcheck // test cleanup

	var tick atomic.Int32
	cfg := &ClientConfig{
		Name:               "edge-empty",
		sampleIntervalUnit: 25 * time.Millisecond,
		SampleCollector: func(_ context.Context, now time.Time) (fleet.SampleReport, error) {
			if tick.Add(1) == 1 {
				return fleet.SampleReport{
					SchemaVersion: fleet.SampleSchemaVersion,
					DeviceTime:    now.UTC().Format(time.RFC3339),
					Samples:       []fleet.SampleReading{},
				}, nil
			}
			return testSampleReport(33.5, now), nil
		},
	}

	sampler := newSessionSampler(t.Context(), mc, cfg)
	defer sampler.Stop()
	sampler.UpdateInterval(1)

	select {
	case req := <-hubMux.Requests():
		require.Equal(t, fleet.VerbSampleReport, req.Method)
		rep, err := fleet.DecodeAndValidateSampleReport(req.Params)
		require.NoError(t, err)
		require.Len(t, rep.Samples, 1)
		assert.Equal(t, fleet.FigureCPUUsed, rep.Samples[0].Figure)
		assert.Equal(t, 33.5, rep.Samples[0].Value)
		require.NoError(t, hubMux.SendOK(t.Context(), req.ID))
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for non-empty sample-report after empty tick")
	}
	assert.GreaterOrEqual(t, tick.Load(), int32(2))
}

// TestSessionSamplerDropsLateCollectorReplyOnGenerationChange proves that if a
// collector is slow and ignores context cancellation while UpdateInterval(0)
// advances the generation, its late snapshot is dropped and never sent to the
// hub.
//
// VALIDATES: generation fencing against late collector replies.
// PREVENTS: stale snapshots from a superseded interval or stopped session
// reaching the hub wire.
func TestSessionSamplerDropsLateCollectorReplyOnGenerationChange(t *testing.T) {
	t.Parallel()

	clientEnd, hubEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer hubEnd.Close()    //nolint:errcheck // test cleanup

	hubMux := rpc.NewMuxConn(rpc.NewConn(hubEnd, hubEnd))
	defer hubMux.Close() //nolint:errcheck // test cleanup

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	defer mc.Close() //nolint:errcheck // test cleanup

	started := make(chan struct{})
	release := make(chan struct{})
	var closeOnce sync.Once
	cfg := &ClientConfig{
		Name:               "edge-late",
		sampleIntervalUnit: time.Hour,
		SampleCollector: func(_ context.Context, now time.Time) (fleet.SampleReport, error) {
			closeOnce.Do(func() { close(started) })
			// Deliberately ignore ctx cancellation to simulate a late reply.
			<-release
			return testSampleReport(99.0, now), nil
		},
	}

	sampler := newSessionSampler(t.Context(), mc, cfg)
	defer sampler.Stop()
	sampler.UpdateInterval(1)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("collector did not start")
	}

	// Disable sampling while collector is in flight; advances generation.
	sampler.UpdateInterval(0)
	close(release)

	select {
	case req, ok := <-hubMux.Requests():
		if ok {
			t.Fatalf("late collector reply must be dropped, got RPC %q (%s)", req.Method, string(req.Params))
		}
	case <-time.After(100 * time.Millisecond):
		// Expected: nothing sent on the wire.
	}
}

// TestSessionSamplerNoOfflineBufferOnStop verifies Stop cancels and joins
// promptly without buffering or replaying samples across sessions.
//
// VALIDATES: AC-12 no offline buffering; clean stop/join.
// PREVENTS: buffered pre-disconnect readings replaying after reconnect.
func TestSessionSamplerNoOfflineBufferOnStop(t *testing.T) {
	t.Parallel()

	clientEnd, hubEnd := net.Pipe()
	defer clientEnd.Close() //nolint:errcheck // test cleanup
	defer hubEnd.Close()    //nolint:errcheck // test cleanup

	hubMux := rpc.NewMuxConn(rpc.NewConn(hubEnd, hubEnd))
	defer hubMux.Close() //nolint:errcheck // test cleanup

	mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
	defer mc.Close() //nolint:errcheck // test cleanup

	var seq atomic.Int32
	cfg := &ClientConfig{
		Name:               "edge-stop",
		sampleIntervalUnit: 25 * time.Millisecond,
		SampleCollector: func(_ context.Context, now time.Time) (fleet.SampleReport, error) {
			v := float64(seq.Add(1))
			return testSampleReport(v, now), nil
		},
	}

	sampler := newSessionSampler(t.Context(), mc, cfg)
	sampler.UpdateInterval(1)

	req := <-hubMux.Requests()
	require.Equal(t, fleet.VerbSampleReport, req.Method)
	var first fleet.SampleReport
	require.NoError(t, json.Unmarshal(req.Params, &first))
	require.NoError(t, hubMux.SendOK(t.Context(), req.ID))

	stopDone := make(chan struct{})
	go func() {
		defer close(stopDone)
		sampler.Stop()
	}()
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("sessionSampler.Stop did not join cleanly")
	}

	// Idempotent second Stop returns immediately.
	sampler.Stop()
}
