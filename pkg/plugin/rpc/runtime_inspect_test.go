// Design: docs/architecture/api/process-protocol.md -- runtime-inspect wire validation and transport tests.
// Related: runtime_inspect.go, bridge_runtime_inspect.go -- implementation under test.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRuntimeInspectInputValidate covers closed enum checks, deadline bounds,
// CPU duration bounds, log-count bounds, and maximum frame size enforcement.
//
// VALIDATES: RuntimeInspectInput.Validate admits every legal shape and rejects
// every out-of-range or unknown value before hitting a plugin worker.
// PREVENTS: invalid kinds, unbounded CPU durations, negative/oversized log
// counts, or expired/unbounded deadlines from crossing the process boundary.
func TestRuntimeInspectInputValidate(t *testing.T) {
	t.Parallel()

	now := time.Now()
	liveDeadline := now.Add(5 * time.Second).UnixMilli()

	validCases := []struct {
		name  string
		input RuntimeInspectInput
	}{
		{name: "goroutines", input: RuntimeInspectInput{Kind: RuntimeInspectGoroutines, DeadlineMS: liveDeadline}},
		{name: "runtime-stats", input: RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats, DeadlineMS: liveDeadline}},
		{name: "log-recent-default", input: RuntimeInspectInput{Kind: RuntimeInspectLogRecent, DeadlineMS: liveDeadline, LogCount: 0}},
		{name: "log-recent-max", input: RuntimeInspectInput{Kind: RuntimeInspectLogRecent, DeadlineMS: liveDeadline, LogCount: RuntimeInspectMaxLogEntries, LogLevel: "warn", LogComponent: "bgp"}},
		{name: "profile-heap", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline, ProfileType: RuntimeInspectProfileHeap}},
		{name: "profile-goroutine", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline, ProfileType: RuntimeInspectProfileGoroutine}},
		{name: "profile-allocs", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline, ProfileType: RuntimeInspectProfileAllocs}},
		{name: "profile-mutex", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline, ProfileType: RuntimeInspectProfileMutex}},
		{name: "profile-block", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline, ProfileType: RuntimeInspectProfileBlock}},
		{name: "profile-cpu-default", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline, ProfileType: RuntimeInspectProfileCPU, DurationMS: 0}},
		{name: "profile-cpu-min", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline, ProfileType: RuntimeInspectProfileCPU, DurationMS: RuntimeInspectMinCPUDurationMS}},
		{name: "profile-cpu-max", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: now.Add(RuntimeInspectMaxTimeout).UnixMilli(), ProfileType: RuntimeInspectProfileCPU, DurationMS: RuntimeInspectMaxCPUDurationMS}},
	}
	for _, tc := range validCases {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, tc.input.Validate(now))
		})
	}

	invalidCases := []struct {
		name  string
		input RuntimeInspectInput
	}{
		{name: "empty-kind", input: RuntimeInspectInput{DeadlineMS: liveDeadline}},
		{name: "unknown-kind", input: RuntimeInspectInput{Kind: "bogus", DeadlineMS: liveDeadline}},
		{name: "zero-deadline", input: RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats, DeadlineMS: 0}},
		{name: "expired-deadline", input: RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats, DeadlineMS: now.UnixMilli()}},
		{name: "past-deadline", input: RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats, DeadlineMS: now.Add(-time.Second).UnixMilli()}},
		{name: "over-max-deadline", input: RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats, DeadlineMS: now.Add(RuntimeInspectMaxTimeout + time.Second).UnixMilli()}},
		{name: "profile-empty-type", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline}},
		{name: "profile-unknown-type", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline, ProfileType: "trace"}},
		{name: "profile-cpu-999ms", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline, ProfileType: RuntimeInspectProfileCPU, DurationMS: 999}},
		{name: "profile-cpu-60001ms", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline, ProfileType: RuntimeInspectProfileCPU, DurationMS: 60001}},
		{name: "profile-cpu-negative", input: RuntimeInspectInput{Kind: RuntimeInspectProfile, DeadlineMS: liveDeadline, ProfileType: RuntimeInspectProfileCPU, DurationMS: -1}},
		{name: "log-count-negative", input: RuntimeInspectInput{Kind: RuntimeInspectLogRecent, DeadlineMS: liveDeadline, LogCount: -1}},
		{name: "log-count-257", input: RuntimeInspectInput{Kind: RuntimeInspectLogRecent, DeadlineMS: liveDeadline, LogCount: 257}},
		{name: "oversized-request", input: RuntimeInspectInput{Kind: RuntimeInspectLogRecent, DeadlineMS: liveDeadline, LogComponent: strings.Repeat("x", MaxMessageSize)}},
	}
	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, tc.input.Validate(now))
		})
	}
}

// TestRuntimeInspectOutputValidate verifies output kind echo, positive PID,
// payload bounds, and MaxMessageSize framing enforcement.
//
// VALIDATES: RuntimeInspectOutput.Validate refuses nil replies, mismatched
// kinds/profile types, non-positive PIDs, and oversized frames.
// PREVENTS: malformed or frame-exceeding plugin replies from propagating.
func TestRuntimeInspectOutputValidate(t *testing.T) {
	t.Parallel()

	zeroRate := 0
	tenRate := 10

	require.NoError(t, (&RuntimeInspectOutput{
		Kind:            RuntimeInspectGoroutines,
		PID:             1234,
		GoroutineStacks: "goroutine 1 [running]:\n",
	}).Validate(RuntimeInspectInput{Kind: RuntimeInspectGoroutines}))

	require.NoError(t, (&RuntimeInspectOutput{
		Kind:         RuntimeInspectProfile,
		PID:          1234,
		ProfileType:  RuntimeInspectProfileMutex,
		ProfileCount: 3,
		ProfileData:  "AAAA",
		SamplingRate: &tenRate,
	}).Validate(RuntimeInspectInput{Kind: RuntimeInspectProfile, ProfileType: RuntimeInspectProfileMutex}))

	require.NoError(t, (&RuntimeInspectOutput{
		Kind:         RuntimeInspectProfile,
		PID:          1234,
		ProfileType:  RuntimeInspectProfileBlock,
		SamplingRate: &zeroRate,
	}).Validate(RuntimeInspectInput{Kind: RuntimeInspectProfile, ProfileType: RuntimeInspectProfileBlock}))

	require.NoError(t, (&RuntimeInspectOutput{
		Kind:       RuntimeInspectLogRecent,
		PID:        1234,
		LogEntries: []RuntimeInspectLogEntry{{Timestamp: "2026-11-27T00:00:00Z", Level: "info", Message: "ok"}},
	}).Validate(RuntimeInspectInput{Kind: RuntimeInspectLogRecent}))

	require.NoError(t, (&RuntimeInspectOutput{
		Kind:         RuntimeInspectRuntimeStats,
		PID:          1234,
		RuntimeStats: &RuntimeInspectStats{NumGoroutines: 4, NumCPU: 2, MaxProcs: 2, GoVersion: "go1.26"},
	}).Validate(RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats}))

	var nilOut *RuntimeInspectOutput
	require.Error(t, nilOut.Validate(RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats}))

	require.Error(t, (&RuntimeInspectOutput{Kind: RuntimeInspectGoroutines, PID: 1234}).Validate(RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats}))
	require.Error(t, (&RuntimeInspectOutput{Kind: RuntimeInspectGoroutines, PID: 0}).Validate(RuntimeInspectInput{Kind: RuntimeInspectGoroutines}))
	require.Error(t, (&RuntimeInspectOutput{Kind: RuntimeInspectGoroutines, PID: -5}).Validate(RuntimeInspectInput{Kind: RuntimeInspectGoroutines}))
	require.Error(t, (&RuntimeInspectOutput{Kind: RuntimeInspectProfile, PID: 1234, ProfileType: RuntimeInspectProfileHeap}).Validate(RuntimeInspectInput{Kind: RuntimeInspectProfile, ProfileType: RuntimeInspectProfileMutex}))
	require.Error(t, (&RuntimeInspectOutput{Kind: RuntimeInspectRuntimeStats, PID: 1234}).Validate(RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats}))
	require.Error(t, (&RuntimeInspectOutput{Kind: RuntimeInspectGoroutines, PID: 1234, GoroutineStacks: strings.Repeat("a", RuntimeInspectMaxGoroutineBytes+1)}).Validate(RuntimeInspectInput{Kind: RuntimeInspectGoroutines}))
	require.Error(t, (&RuntimeInspectOutput{Kind: RuntimeInspectLogRecent, PID: 1234, LogEntries: make([]RuntimeInspectLogEntry, RuntimeInspectMaxLogEntries+1)}).Validate(RuntimeInspectInput{Kind: RuntimeInspectLogRecent}))
	require.Error(t, (&RuntimeInspectOutput{Kind: RuntimeInspectProfile, PID: 1234, ProfileType: RuntimeInspectProfileHeap, ProfileData: strings.Repeat("A", MaxMessageSize)}).Validate(RuntimeInspectInput{Kind: RuntimeInspectProfile, ProfileType: RuntimeInspectProfileHeap}))
}

// TestRuntimeInspectCallContext verifies deadline stamping for default
// snapshots, CPU profiles, and caller-supplied tighter deadlines.
//
// VALIDATES: RuntimeInspectCallContext assigns 10s default, duration+5s up to
// 65s for CPU profiles, and honors tighter caller deadlines.
// PREVENTS: premature timeouts on long CPU profiles or unbounded waits.
func TestRuntimeInspectCallContext(t *testing.T) {
	t.Parallel()

	in := RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats}
	work, cancel := RuntimeInspectCallContext(t.Context(), &in)
	defer cancel()
	dl, ok := work.Deadline()
	require.True(t, ok)
	require.Equal(t, dl.UnixMilli(), in.DeadlineMS)
	remaining := time.Until(dl)
	require.Greater(t, remaining, 8*time.Second)
	require.LessOrEqual(t, remaining, RuntimeInspectDefaultTimeout+time.Second)

	cpuIn := RuntimeInspectInput{Kind: RuntimeInspectProfile, ProfileType: RuntimeInspectProfileCPU, DurationMS: 30000}
	cpuCtx, cpuCancel := RuntimeInspectCallContext(t.Context(), &cpuIn)
	defer cpuCancel()
	cpuDL, ok := cpuCtx.Deadline()
	require.True(t, ok)
	cpuRemaining := time.Until(cpuDL)
	require.Greater(t, cpuRemaining, 33*time.Second)
	require.LessOrEqual(t, cpuRemaining, 36*time.Second)

	tightParent, tightParentCancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer tightParentCancel()
	tightIn := RuntimeInspectInput{Kind: RuntimeInspectProfile, ProfileType: RuntimeInspectProfileCPU, DurationMS: 60000}
	tightCtx, tightCancel := RuntimeInspectCallContext(tightParent, &tightIn)
	defer tightCancel()
	tightDL, ok := tightCtx.Deadline()
	require.True(t, ok)
	require.LessOrEqual(t, time.Until(tightDL), 3*time.Second)
}

// TestRuntimeInspectDirectBridgeRoundTrip verifies DirectBridge readiness,
// validation, error passthrough without bridge closure, and success round-trip.
//
// VALIDATES: DirectBridge.RuntimeInspect enforces validation and keeps the
// bridge alive after a handler error.
// PREVENTS: diagnostic handler errors from failing or closing DirectBridge.
func TestRuntimeInspectDirectBridgeRoundTrip(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()
	_, err := bridge.RuntimeInspect(t.Context(), RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats})
	require.ErrorIs(t, err, ErrBridgeClosed)

	bridge.SetReady()
	_, err = bridge.RuntimeInspect(t.Context(), RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats})
	require.Error(t, err)

	failNext := true
	bridge.SetRuntimeInspect(func(_ context.Context, in RuntimeInspectInput) (*RuntimeInspectOutput, error) {
		if failNext {
			failNext = false
			return nil, errors.New("collector busy")
		}
		return &RuntimeInspectOutput{
			Kind:         in.Kind,
			PID:          4242,
			RuntimeStats: &RuntimeInspectStats{NumGoroutines: 7, NumCPU: 4, MaxProcs: 4, GoVersion: "go1.26"},
		}, nil
	})

	_, err = bridge.RuntimeInspect(t.Context(), RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats})
	require.Error(t, err)
	require.True(t, bridge.Ready())

	out, err := bridge.RuntimeInspect(t.Context(), RuntimeInspectInput{Kind: RuntimeInspectRuntimeStats})
	require.NoError(t, err)
	require.Equal(t, 4242, out.PID)
	require.Equal(t, 7, out.RuntimeStats.NumGoroutines)
}

// TestRuntimeInspectMuxConnRoundTrip verifies kebab-case JSON framing and
// independent ingress routing over MuxConn.
//
// VALIDATES: MuxConn.RuntimeInspect sends kebab-case fields over
// MethodRuntimeInspect and validates the decoded reply.
// PREVENTS: JSON tag drift or broken independent MuxConn routing.
func TestRuntimeInspectMuxConnRoundTrip(t *testing.T) {
	t.Parallel()

	a, b := net.Pipe()
	t.Cleanup(func() {
		a.Close() //nolint:errcheck // test cleanup
		b.Close() //nolint:errcheck // test cleanup
	})

	clientMux := NewMuxConn(NewConn(a, a))
	serverMux := NewMuxConn(NewConn(b, b))
	t.Cleanup(func() {
		clientMux.Close() //nolint:errcheck // test cleanup
		serverMux.Close() //nolint:errcheck // test cleanup
	})

	gotParams := make(chan json.RawMessage, 1)
	remove, err := serverMux.RegisterIndependentRequestHandler(MethodRuntimeInspect, func(req *Request) error {
		gotParams <- req.Params
		var in RuntimeInspectInput
		if unmarshalErr := json.Unmarshal(req.Params, &in); unmarshalErr != nil {
			t.Errorf("unmarshal runtime-inspect params: %v", unmarshalErr)
			return nil
		}
		rate := 10
		go func() {
			serverMux.SendResult(t.Context(), req.ID, &RuntimeInspectOutput{ //nolint:errcheck // test reply
				Kind:         in.Kind,
				PID:          9001,
				ProfileType:  in.ProfileType,
				ProfileCount: 2,
				ProfileData:  "dGVzdA==",
				SamplingRate: &rate,
			})
		}()
		return nil
	})
	require.NoError(t, err)
	defer remove()

	out, err := clientMux.RuntimeInspect(t.Context(), RuntimeInspectInput{
		Kind:        RuntimeInspectProfile,
		ProfileType: RuntimeInspectProfileMutex,
	})
	require.NoError(t, err)
	require.Equal(t, RuntimeInspectProfile, out.Kind)
	require.Equal(t, 9001, out.PID)
	require.Equal(t, RuntimeInspectProfileMutex, out.ProfileType)
	require.NotNil(t, out.SamplingRate)
	require.Equal(t, 10, *out.SamplingRate)

	raw := <-gotParams
	var wire map[string]any
	require.NoError(t, json.Unmarshal(raw, &wire))
	require.Equal(t, RuntimeInspectProfile, wire["kind"])
	require.Equal(t, RuntimeInspectProfileMutex, wire["profile-type"])
	require.Contains(t, wire, "deadline-ms")
}
