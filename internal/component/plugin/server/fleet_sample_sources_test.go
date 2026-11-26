// VALIDATES: device-side fleet sample source collector -- discovers hidden
// `show fleet source <owner>` commands from the full dispatcher and plugin
// registries (never visible-only discovery), deduplicates builtin proxies with
// plugin commands, invokes both in-process builtins and forked plugin owners
// over PluginConn under bounded per-source deadlines, validates owner/unit/
// domain/bounds, sums additive l2tp+pppoe subscriber-sessions, rejects
// duplicate non-additive ownership, and drops failed/malformed/timed-out
// sources without synthesizing zero.
// PREVENTS: hidden source commands being missed, forked plugin sources failing
// across the process boundary, absent or malformed sources turning into false
// zeros, or conflicting owners corrupting fleet sample reports.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/command"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type fleetSnapshotResponse struct {
	plugin.DataMarker
	fleet.SourceSnapshot
}

func startMockSourcePlugin(
	t *testing.T,
	d *Dispatcher,
	procName string,
	cmdName string,
	answer func(ctx context.Context, input *rpc.ExecuteCommandInput) []byte,
) *atomic.Int32 {
	t.Helper()
	engineEnd, pluginEnd := net.Pipe()
	var calls atomic.Int32
	done := make(chan struct{})

	proc := process.NewProcess(plugin.PluginConfig{Name: procName})
	proc.SetConn(muxPluginConn(t, engineEnd))
	markProcessRunning(t, proc)

	results := d.Registry().Register(proc, []CommandDef{{
		Name:   cmdName,
		Hidden: true,
	}})
	require.Len(t, results, 1)
	require.True(t, results[0].OK, results[0].Error)

	go func() {
		defer close(done)
		conn := rpc.NewConn(pluginEnd, pluginEnd)
		for {
			req, err := conn.ReadRequest(t.Context())
			if err != nil {
				return
			}
			calls.Add(1)
			var input rpc.ExecuteCommandInput
			_ = json.Unmarshal(req.Params, &input)
			payload := answer(t.Context(), &input)
			if payload == nil {
				return
			}
			_ = rpc.WriteDocumentAnswer(conn.AnswerWriter(t.Context()), req.ID, rpc.AnswerTail{}, payload)
		}
	}()

	t.Cleanup(func() {
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		<-done
	})

	return &calls
}

// TestDiscoverFleetSampleSourcesFullRegistry proves hidden source commands are
// discovered from both builtin and plugin registries (even though hidden from
// completion/discovery), invalid suffixes are ignored, stopped plugin owners
// are skipped, and builtin+plugin proxy pairs are deduplicated.
//
// VALIDATES: full-registry discovery including Hidden, token parsing, dedup.
// PREVENTS: hidden source commands going unseen or being invoked twice.
func TestDiscoverFleetSampleSourcesFullRegistry(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	noop := func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{}`)), nil
	}

	require.NoError(t, d.RegisterWithOptions("show fleet source host", noop, "Host source", RegisterOptions{
		Hidden:   true,
		ReadOnly: true,
	}))
	require.NoError(t, d.RegisterWithOptions("show fleet source iface", noop, "Iface proxy", RegisterOptions{
		Hidden:      true,
		ReadOnly:    true,
		PluginProxy: true,
	}))
	// Non-matching or multi-token commands MUST be ignored.
	require.NoError(t, d.RegisterWithOptions("show fleet source", noop, "No owner", RegisterOptions{Hidden: true}))
	require.NoError(t, d.RegisterWithOptions("show fleet source host extra", noop, "Multi-token", RegisterOptions{Hidden: true}))
	require.NoError(t, d.RegisterWithOptions("show fleet status", noop, "Status", RegisterOptions{ReadOnly: true}))

	// Running plugin registering the same proxy command (dedup) and a standalone plugin source.
	startMockSourcePlugin(t, d, "iface", "show fleet source iface", func(context.Context, *rpc.ExecuteCommandInput) []byte {
		return []byte(`{"owner":"iface","readings":[]}`)
	})
	startMockSourcePlugin(t, d, "bgp-rib", "show fleet source bgp-rib", func(context.Context, *rpc.ExecuteCommandInput) []byte {
		return []byte(`{"owner":"bgp-rib","readings":[]}`)
	})

	// Stopped plugin process MUST NOT be discovered.
	stoppedProc := process.NewProcess(plugin.PluginConfig{Name: "stopped-source"})
	res := d.Registry().Register(stoppedProc, []CommandDef{{Name: "show fleet source bgp-peer", Hidden: true}})
	require.True(t, res[0].OK, res[0].Error)

	// Verify operator-discovery hides every hidden plugin source while full
	// discovery still finds all live owners in deterministic sorted order.
	assert.Empty(t, d.Registry().VisibleCommandEntries(), "hidden plugin sources must stay out of visible discovery")
	assert.Empty(t, d.Registry().Complete("show fleet source"), "hidden plugin sources must stay out of completion")

	owners := discoverFleetSampleSources(d)
	assert.Equal(t, []string{"bgp-rib", "host", "iface"}, owners)
}

// TestCollectFleetSampleReportMixedBuiltinAndPlugin proves that the collector
// gathers readings from in-process builtin handlers, builtin proxies forwarding
// to a plugin over PluginConn, and direct plugin commands, using internal RPC
// identity and canonical vocabulary ordering.
//
// VALIDATES: AC-1 mixed builtin + forked plugin collection and Server wiring.
// PREVENTS: builtin proxies lacking CommandContext.Server or plugin commands
// failing across the process boundary.
func TestCollectFleetSampleReportMixedBuiltinAndPlugin(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	s := &Server{dispatcher: d}

	var observedUser string
	require.NoError(t, d.RegisterWithOptions("show fleet source host", func(ctx *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		observedUser = ctx.Username
		return plugin.NewResponse(plugin.StatusDone, fleetSnapshotResponse{
			Owner: fleet.SampleOwnerHost,
			Readings: []fleet.SourceReading{
				{Figure: fleet.FigureUptime, Unit: fleet.SampleUnitSeconds, Value: 3600},
				{Figure: fleet.FigureCPUUsed, Unit: fleet.SampleUnitPercent, Value: 42.5},
			},
		}), nil
	}, "Host source", RegisterOptions{Hidden: true, ReadOnly: true}))

	// Builtin proxy forwarding to a running plugin over PluginConn (mirrors iface/cmd and bgp/plugins/cmd/rib).
	require.NoError(t, d.RegisterWithOptions("show fleet source iface", func(ctx *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		disp := ctx.Dispatcher()
		if disp == nil {
			return nil, errors.New("missing dispatcher on CommandContext.Server")
		}
		return disp.ForwardToPlugin(ctx, "show fleet source iface", nil, ctx.PeerSelector())
	}, "Iface proxy", RegisterOptions{Hidden: true, ReadOnly: true, PluginProxy: true}))

	ifaceCalls := startMockSourcePlugin(t, d, "iface", "show fleet source iface", func(context.Context, *rpc.ExecuteCommandInput) []byte {
		return []byte(`{"owner":"iface","readings":[{"figure":"interfaces-up","unit":"count","value":4},{"figure":"traffic-in","unit":"bps","value":125000}]}`)
	})

	// Direct plugin-backed command without a builtin proxy.
	ribCalls := startMockSourcePlugin(t, d, "bgp-rib", "show fleet source bgp-rib", func(context.Context, *rpc.ExecuteCommandInput) []byte {
		return []byte(`{"owner":"bgp-rib","readings":[{"figure":"routes-held","unit":"count","value":9123}]}`)
	})

	now := time.Date(2026, 11, 26, 12, 30, 0, 0, time.UTC)
	report, err := CollectFleetSampleReportFromServer(t.Context(), s, now)
	require.NoError(t, err)

	assert.Equal(t, aaa.ReservedInternalRPCIdentity, observedUser)
	assert.Equal(t, int32(1), ifaceCalls.Load(), "deduplicated proxy+plugin must be called once per tick")
	assert.Equal(t, int32(1), ribCalls.Load())
	assert.Equal(t, fleet.SampleSchemaVersion, report.SchemaVersion)
	assert.Equal(t, "2026-11-26T12:30:00Z", report.DeviceTime)

	// Samples MUST be in canonical vocabulary order regardless of owner/response order.
	assert.Equal(t, []fleet.SampleReading{
		{Figure: fleet.FigureCPUUsed, Value: 42.5},
		{Figure: fleet.FigureUptime, Value: 3600},
		{Figure: fleet.FigureInterfacesUp, Value: 4},
		{Figure: fleet.FigureTrafficIn, Value: 125000},
		{Figure: fleet.FigureRoutesHeld, Value: 9123},
	}, report.Samples)
}

// TestCollectFleetSampleReportAdditiveSubscribers verifies that l2tp and pppoe
// contributions sum into subscriber-sessions when both exist, preserve a lone
// configured owner (including a measured zero), and omit the figure when
// neither owner is registered.
//
// VALIDATES: AC-1, AC-11 additive subscriber-sessions merge and absence rules.
// PREVENTS: one subscriber owner overwriting the other or synthesizing zero
// when neither L2TP nor PPPoE is present.
func TestCollectFleetSampleReportAdditiveSubscribers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 26, 12, 31, 0, 0, time.UTC)

	t.Run("both l2tp and pppoe present are summed", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher()
		require.NoError(t, d.RegisterWithOptions("show fleet source l2tp", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"owner":"l2tp","readings":[{"figure":"subscriber-sessions","unit":"count","value":12}]}`)), nil
		}, "L2TP", RegisterOptions{Hidden: true, ReadOnly: true}))
		require.NoError(t, d.RegisterWithOptions("show fleet source pppoe", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"owner":"pppoe","readings":[{"figure":"subscriber-sessions","unit":"count","value":30}]}`)), nil
		}, "PPPoE", RegisterOptions{Hidden: true, ReadOnly: true}))

		rep, err := CollectFleetSampleReport(t.Context(), d, now)
		require.NoError(t, err)
		assert.Equal(t, []fleet.SampleReading{
			{Figure: fleet.FigureSubscriberSessions, Value: 42},
		}, rep.Samples)
	})

	t.Run("only l2tp present keeps measured zero", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher()
		require.NoError(t, d.RegisterWithOptions("show fleet source l2tp", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"owner":"l2tp","readings":[{"figure":"subscriber-sessions","unit":"count","value":0}]}`)), nil
		}, "L2TP", RegisterOptions{Hidden: true, ReadOnly: true}))

		rep, err := CollectFleetSampleReport(t.Context(), d, now)
		require.NoError(t, err)
		assert.Equal(t, []fleet.SampleReading{
			{Figure: fleet.FigureSubscriberSessions, Value: 0},
		}, rep.Samples)
	})

	t.Run("neither l2tp nor pppoe registered omits subscriber-sessions", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher()
		require.NoError(t, d.RegisterWithOptions("show fleet source host", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"owner":"host","readings":[{"figure":"load-1m","unit":"number","value":0.5}]}`)), nil
		}, "Host", RegisterOptions{Hidden: true, ReadOnly: true}))

		rep, err := CollectFleetSampleReport(t.Context(), d, now)
		require.NoError(t, err)
		assert.Equal(t, []fleet.SampleReading{
			{Figure: fleet.FigureLoad1m, Value: 0.5},
		}, rep.Samples)
	})
}

// TestCollectFleetSampleReportDropsMalformedFailedAndTimedOut proves that
// malformed, failing, unauthorized-owner, wrong-unit, out-of-domain, and
// timed-out sources are cleanly skipped without synthesizing zero values and
// without disrupting healthy sources.
//
// VALIDATES: AC-11, AC-13 per-source isolation, validation, and timeout bounds.
// PREVENTS: a broken or hung producer stalling sampling or injecting bad data.
func TestCollectFleetSampleReportDropsMalformedFailedAndTimedOut(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	// Healthy host source.
	require.NoError(t, d.RegisterWithOptions("show fleet source host", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"owner":"host","readings":[{"figure":"memory-used","unit":"bytes","value":1024}]}`)), nil
	}, "Host", RegisterOptions{Hidden: true, ReadOnly: true}))

	// Malformed: wrong unit + wrong owner figure.
	require.NoError(t, d.RegisterWithOptions("show fleet source iface", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"owner":"iface","readings":[{"figure":"interfaces-up","unit":"bytes","value":3}]}`)), nil
	}, "Iface bad unit", RegisterOptions{Hidden: true, ReadOnly: true}))

	// Malformed: mismatched owner field.
	require.NoError(t, d.RegisterWithOptions("show fleet source bgp-peer", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"owner":"host","readings":[{"figure":"cpu-used","unit":"percent","value":10}]}`)), nil
	}, "Mismatched owner", RegisterOptions{Hidden: true, ReadOnly: true}))

	// Failing source (error response).
	require.NoError(t, d.RegisterWithOptions("show fleet source l2tp", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusError, Error: "l2tp unavailable"}, errors.New("l2tp unavailable")
	}, "Failing", RegisterOptions{Hidden: true, ReadOnly: true}))

	// Out-of-domain value (cpu > 100 or negative).
	require.NoError(t, d.RegisterWithOptions("show fleet source pppoe", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"owner":"pppoe","readings":[{"figure":"subscriber-sessions","unit":"count","value":-1}]}`)), nil
	}, "Negative count", RegisterOptions{Hidden: true, ReadOnly: true}))

	// Timed-out plugin source (hangs until per-source deadline expires).
	startMockSourcePlugin(t, d, "bgp-rib", "show fleet source bgp-rib", func(ctx context.Context, _ *rpc.ExecuteCommandInput) []byte {
		<-ctx.Done()
		return nil
	})

	collector := NewFleetSampleCollectorWithDispatcher(d)
	collector.SetPerSourceTimeout(40 * time.Millisecond)

	now := time.Date(2026, 11, 26, 12, 32, 0, 0, time.UTC)
	report, err := collector.Collect(t.Context(), now)
	require.NoError(t, err)

	// Only the healthy host reading is present; none of the failed/malformed/timed-out sources synthesized zeros.
	assert.Equal(t, []fleet.SampleReading{
		{Figure: fleet.FigureMemoryUsed, Value: 1024},
	}, report.Samples)
}

// TestMergeSourceSnapshotsRejectsDuplicateNonAdditiveOwnership proves that two
// distinct owners cannot claim the same non-additive figure, and a single owner
// cannot emit the same additive figure twice.
//
// VALIDATES: ownership uniqueness guard in mergeSourceSnapshots.
// PREVENTS: non-additive figures being silently overwritten or summed.
func TestMergeSourceSnapshotsRejectsDuplicateNonAdditiveOwnership(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 26, 12, 33, 0, 0, time.UTC)

	t.Run("duplicate non-additive figure across two owners is rejected", func(t *testing.T) {
		t.Parallel()
		rep, err := mergeSourceSnapshots([]*fleet.SourceSnapshot{
			{
				Owner: fleet.SampleOwnerHost,
				Readings: []fleet.SourceReading{
					{Figure: fleet.FigureCPUUsed, Unit: fleet.SampleUnitPercent, Value: 20},
				},
			},
			{
				Owner: fleet.SampleOwnerIface,
				Readings: []fleet.SourceReading{
					{Figure: fleet.FigureCPUUsed, Unit: fleet.SampleUnitPercent, Value: 30},
				},
			},
		}, now)
		require.ErrorIs(t, err, errDuplicateSampleFigureOwner)
		assert.Empty(t, rep.Samples)
	})

	t.Run("duplicate additive figure from same owner is rejected", func(t *testing.T) {
		t.Parallel()
		rep, err := mergeSourceSnapshots([]*fleet.SourceSnapshot{
			{
				Owner: fleet.SampleOwnerL2TP,
				Readings: []fleet.SourceReading{
					{Figure: fleet.FigureSubscriberSessions, Unit: fleet.SampleUnitCount, Value: 5},
				},
			},
			{
				Owner: fleet.SampleOwnerL2TP,
				Readings: []fleet.SourceReading{
					{Figure: fleet.FigureSubscriberSessions, Unit: fleet.SampleUnitCount, Value: 6},
				},
			},
		}, now)
		require.ErrorIs(t, err, errDuplicateSampleFigureOwner)
		assert.Empty(t, rep.Samples)
	})

	t.Run("oversized and unknown-field snapshots rejected before merge", func(t *testing.T) {
		t.Parallel()
		oversized := []byte(`{"owner":"host","readings":[],"pad":"` + strings.Repeat("x", fleet.SampleReportBodyMax) + `"}`)
		_, err := decodeAndValidateSourceSnapshot(fleet.SampleOwnerHost, oversized)
		require.ErrorIs(t, err, errFleetSampleSnapshotTooLarge)

		unknownField := []byte(`{"owner":"host","readings":[],"extra":1}`)
		_, err = decodeAndValidateSourceSnapshot(fleet.SampleOwnerHost, unknownField)
		require.Error(t, err)
	})
}
