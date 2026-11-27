// Design: docs/architecture/diagnostics/production-diagnostics.md -- show system drift, bad-packets, and triage bad-packet-count tests
// Related: drift.go -- show system drift handler
// Related: bad_packets.go -- show system bad-packets handler
// Related: triage.go -- bad-packet-count in show system triage runtime

package show

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/badpacket"
	"github.com/ze-software/ze/internal/core/drift"
)

// TestDriftAndBadPacketsPipeFilterRegistrations verifies show system drift and
// show system bad-packets register their command-owned pipe filters with
// command.ArgText.
//
// VALIDATES: show system drift registers | subsystem (ArgText) and show system
// bad-packets registers | protocol (ArgText).
// PREVENTS: missing pipe declarations breaking CLI/MCP narrowing.
func TestDriftAndBadPacketsPipeFilterRegistrations(t *testing.T) {
	cases := []struct {
		path   string
		filter string
	}{
		{path: "show system drift", filter: pipeOpSubsystem},
		{path: "show system bad-packets", filter: pipeOpProtocol},
	}
	for _, tc := range cases {
		filters := command.PipeFiltersForCommand(tc.path)
		found := false
		for _, f := range filters {
			if f.Name == tc.filter && f.Argument == command.ArgText {
				found = true
				break
			}
		}
		assert.True(t, found, "command.PipeFiltersForCommand(%q) = %+v, want %q (ArgText)", tc.path, filters, tc.filter)
	}
}

// TestShowSystemDrift verifies full-subsystem aggregation (clean, drifted,
// error precedence), | subsystem <name> narrowing, unknown subsystem refusal
// with opererr, and malformed pipe refusals.
//
// VALIDATES: show system drift aggregates status cleanly, filters via
// | subsystem <name>, and refuses unknown subsystems or malformed pipes.
// PREVENTS: silent acceptance of unknown subsystem filters or wrong aggregate status.
func TestShowSystemDrift(t *testing.T) {
	var mu sync.Mutex
	reportA := drift.Report{
		Subsystem: "test-drift-a",
		Status:    drift.StatusClean,
		Checked:   3,
	}
	reportB := drift.Report{
		Subsystem: "test-drift-b",
		Status:    drift.StatusClean,
		Checked:   2,
	}

	unregA := drift.Register("test-drift-a", func(_ context.Context) drift.Report {
		mu.Lock()
		defer mu.Unlock()
		return reportA
	})
	unregB := drift.Register("test-drift-b", func(_ context.Context) drift.Report {
		mu.Lock()
		defer mu.Unlock()
		return reportB
	})
	t.Cleanup(func() {
		unregA()
		unregB()
	})

	// All clean -> aggregate status clean, drifted-count 0.
	resp, err := handleShowSystemDrift(nil, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, string(drift.StatusClean), data[triageKeyStatus])
	assert.Equal(t, 0, data[keyDriftedCount])
	reports, ok := data[keySubsystems].([]drift.Report)
	require.True(t, ok)
	assert.Equal(t, len(reports), data[keyCount])
	assert.GreaterOrEqual(t, len(reports), 2)

	raw, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	_, subsIsSlice := decoded[keySubsystems].([]any)
	assert.True(t, subsIsSlice, "subsystems must serialize as JSON array")
	assertKebabCaseKeys(t, decoded, "drift")

	// One drifted subsystem -> aggregate status drifted, drifted-count 1.
	mu.Lock()
	reportB = drift.Report{
		Subsystem: "test-drift-b",
		Status:    drift.StatusDrifted,
		Checked:   2,
		Drifted:   1,
		Findings: []drift.Finding{
			{Subject: "eth0", Kind: drift.KindMismatch, Expected: "mtu=1500", Actual: "mtu=1400"},
		},
	}
	mu.Unlock()

	driftedResp, err := handleShowSystemDrift(nil, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, driftedResp.Status)
	driftedData, ok := driftedResp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, string(drift.StatusDrifted), driftedData[triageKeyStatus])
	assert.Equal(t, 1, driftedData[keyDriftedCount])

	// One error subsystem beside drifted -> aggregate status error, drifted-count 1.
	unregC := drift.Register("test-drift-c", func(_ context.Context) drift.Report {
		return drift.Report{
			Subsystem: "test-drift-c",
			Status:    drift.StatusError,
			Error:     "netlink query failed",
		}
	})
	t.Cleanup(unregC)

	errAggResp, err := handleShowSystemDrift(nil, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, errAggResp.Status)
	errAggData, ok := errAggResp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, string(drift.StatusError), errAggData[triageKeyStatus])
	assert.Equal(t, 1, errAggData[keyDriftedCount])

	// Narrow to clean subsystem with | subsystem test-drift-a.
	subArgsA := commandtest.Args().WithPipeOperations([]command.PipeOperation{
		{Name: pipeOpSubsystem, Args: []string{"test-drift-a"}},
	})
	subRespA, err := handleShowSystemDrift(nil, subArgsA)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, subRespA.Status)
	subDataA, ok := subRespA.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, string(drift.StatusClean), subDataA[triageKeyStatus])
	assert.Equal(t, 1, subDataA[keyCount])
	assert.Equal(t, 0, subDataA[keyDriftedCount])
	subReportsA, ok := subDataA[keySubsystems].([]drift.Report)
	require.True(t, ok)
	require.Len(t, subReportsA, 1)
	assert.Equal(t, "test-drift-a", subReportsA[0].Subsystem)

	// Narrow to drifted subsystem with | subsystem test-drift-b.
	subArgsB := commandtest.Args().WithPipeOperations([]command.PipeOperation{
		{Name: pipeOpSubsystem, Args: []string{"test-drift-b"}},
	})
	subRespB, err := handleShowSystemDrift(nil, subArgsB)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, subRespB.Status)
	subDataB, ok := subRespB.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, string(drift.StatusDrifted), subDataB[triageKeyStatus])
	assert.Equal(t, 1, subDataB[keyCount])
	assert.Equal(t, 1, subDataB[keyDriftedCount])

	// Unknown subsystem returns structured opererr naming registered subsystems.
	unknownArgs := commandtest.Args().WithPipeOperations([]command.PipeOperation{
		{Name: pipeOpSubsystem, Args: []string{"does-not-exist"}},
	})
	unknownResp, err := handleShowSystemDrift(nil, unknownArgs)
	require.NoError(t, err)
	require.NotNil(t, unknownResp)
	assert.Equal(t, plugin.StatusError, unknownResp.Status)
	assert.Contains(t, unknownResp.Error, "unknown drift subsystem: does-not-exist")
	assert.Contains(t, unknownResp.Error, "test-drift-a")

	// Malformed | subsystem pipe shapes are refused.
	assertSingleTextPipeRefusals(t, pipeOpSubsystem, func(ops []command.PipeOperation) (*plugin.Response, error) {
		return handleShowSystemDrift(nil, commandtest.Args().WithPipeOperations(ops))
	})
}

// TestShowSystemBadPackets verifies default count, explicit count bounds
// (1 and 16), boundary refusals (0 and 17), keyed count handling,
// | protocol <name> filtering, empty protocol match, and pipe refusals.
//
// VALIDATES: show system bad-packets honors count 1..16, refuses out-of-range
// values with opererr, and filters via | protocol <name>.
// PREVENTS: out-of-range counts being silently ignored or null JSON arrays.
func TestShowSystemBadPackets(t *testing.T) {
	before := badpacket.Count()
	id1 := badpacket.Record("bgp", "192.0.2.1", "UPDATE", "treat-as-withdraw", "malformed attr", []byte{0x01, 0x02})
	id2 := badpacket.Record("test-ospf", "192.0.2.2", "HELLO", "drop", "bad checksum", []byte{0x03, 0x04})
	id3 := badpacket.Record("bgp", "192.0.2.3", "HEADER", "session-reset", "bad length", []byte{0xff, 0xff})

	// Default count (16) returns newest-first records with metadata.
	resp, err := handleShowSystemBadPackets(nil, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	packets, ok := data["packets"].([]badpacket.PacketRecord)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(packets), 3)
	assert.Equal(t, len(packets), data[keyCount])
	assert.GreaterOrEqual(t, data["total-recorded"], before+3)
	assert.Equal(t, badpacket.MaxRecords, data["capacity"])
	assert.Equal(t, id3, packets[0].ID)
	assert.Equal(t, id2, packets[1].ID)
	assert.Equal(t, id1, packets[2].ID)

	// Explicit count 1 returns only the newest record.
	oneResp, err := handleShowSystemBadPackets(nil, commandtest.Args(argCount, "1"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, oneResp.Status)
	oneData, ok := oneResp.Data.(plugin.Map)
	require.True(t, ok)
	onePackets, ok := oneData["packets"].([]badpacket.PacketRecord)
	require.True(t, ok)
	require.Len(t, onePackets, 1)
	assert.Equal(t, 1, oneData[keyCount])
	assert.Equal(t, id3, onePackets[0].ID)

	// Explicit count 16 succeeds at the upper boundary.
	maxResp, err := handleShowSystemBadPackets(nil, commandtest.Args(argCount, "16"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, maxResp.Status)
	maxData, ok := maxResp.Data.(plugin.Map)
	require.True(t, ok)
	maxPackets, ok := maxData["packets"].([]badpacket.PacketRecord)
	require.True(t, ok)
	assert.LessOrEqual(t, len(maxPackets), badpacket.MaxRecords)

	// Keyed count (as delivered by YANG validation / structured API calls) works.
	keyedArgs, err := command.ValidateArgs(nil, nil, map[string]string{argCount: "2"})
	require.NoError(t, err)
	keyedResp, err := handleShowSystemBadPackets(nil, keyedArgs)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, keyedResp.Status)
	keyedData, ok := keyedResp.Data.(plugin.Map)
	require.True(t, ok)
	keyedPackets, ok := keyedData["packets"].([]badpacket.PacketRecord)
	require.True(t, ok)
	require.Len(t, keyedPackets, 2)
	assert.Equal(t, 2, keyedData[keyCount])

	// Boundary and malformed count refusals.
	for _, badArgs := range [][]string{
		{argCount},
		{argCount, "0"},
		{argCount, "-1"},
		{argCount, "17"},
		{argCount, "abc"},
		{"bogus"},
	} {
		badResp, badErr := handleShowSystemBadPackets(nil, commandtest.Args(badArgs...))
		require.NoError(t, badErr)
		require.NotNil(t, badResp)
		assert.Equal(t, plugin.StatusError, badResp.Status, "args %v should be refused", badArgs)
	}
	for _, badKeyed := range []string{"0", "17"} {
		vArgs, vErr := command.ValidateArgs(nil, nil, map[string]string{argCount: badKeyed})
		require.NoError(t, vErr)
		badResp, badErr := handleShowSystemBadPackets(nil, vArgs)
		require.NoError(t, badErr)
		require.NotNil(t, badResp)
		assert.Equal(t, plugin.StatusError, badResp.Status, "keyed count=%s should be refused", badKeyed)
	}

	// | protocol bgp filters case-insensitively.
	bgpArgs := commandtest.Args().WithPipeOperations([]command.PipeOperation{
		{Name: pipeOpProtocol, Args: []string{"bgp"}},
	})
	bgpResp, err := handleShowSystemBadPackets(nil, bgpArgs)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, bgpResp.Status)
	bgpData, ok := bgpResp.Data.(plugin.Map)
	require.True(t, ok)
	bgpPackets, ok := bgpData["packets"].([]badpacket.PacketRecord)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(bgpPackets), 2)
	for i := range bgpPackets {
		assert.Equal(t, "bgp", bgpPackets[i].Protocol)
	}

	// Empty protocol match returns non-nil empty JSON array.
	emptyArgs := commandtest.Args().WithPipeOperations([]command.PipeOperation{
		{Name: pipeOpProtocol, Args: []string{"no-such-protocol-xyz"}},
	})
	emptyResp, err := handleShowSystemBadPackets(nil, emptyArgs)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, emptyResp.Status)
	emptyData, ok := emptyResp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 0, emptyData[keyCount])
	raw, err := json.Marshal(emptyResp.Data)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	emptySlice, isSlice := decoded["packets"].([]any)
	assert.True(t, isSlice, "packets must serialize as JSON array")
	assert.Empty(t, emptySlice)
	assertKebabCaseKeys(t, decoded, "bad-packets")

	// Malformed | protocol pipe shapes are refused.
	assertSingleTextPipeRefusals(t, pipeOpProtocol, func(ops []command.PipeOperation) (*plugin.Response, error) {
		return handleShowSystemBadPackets(nil, commandtest.Args().WithPipeOperations(ops))
	})
}

// TestShowSystemTriageBadPacketCount verifies show system triage surfaces the
// atomic bad-packet-count under runtime.
//
// VALIDATES: show system triage runtime section includes uint64
// bad-packet-count matching badpacket.Count().
// PREVENTS: triage runtime dropping bad-packet-count or using a wrong key spelling.
func TestShowSystemTriageBadPacketCount(t *testing.T) {
	before := badpacket.Count()
	badpacket.Record("bgp", "192.0.2.9", "UPDATE", "attribute-discard", "triage count check", []byte{0xde, 0xad})

	resp, err := handleShowSystemTriage(nil, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	runtimeSec := requireTriageMap(t, data, "runtime")
	got, ok := runtimeSec["bad-packet-count"].(uint64)
	require.True(t, ok, "triage runtime missing uint64 bad-packet-count")
	assert.GreaterOrEqual(t, got, before+1)
	assert.Equal(t, badpacket.Count(), got)
}

func assertSingleTextPipeRefusals(t *testing.T, pipeName string, invoke func([]command.PipeOperation) (*plugin.Response, error)) {
	t.Helper()
	cases := []struct {
		name string
		ops  []command.PipeOperation
	}{
		{
			name: "duplicate",
			ops: []command.PipeOperation{
				{Name: pipeName, Args: []string{"one"}},
				{Name: pipeName, Args: []string{"two"}},
			},
		},
		{
			name: "blank",
			ops: []command.PipeOperation{
				{Name: pipeName, Args: []string{"   "}},
			},
		},
		{
			name: "missing",
			ops: []command.PipeOperation{
				{Name: pipeName},
			},
		},
		{
			name: "extra-args",
			ops: []command.PipeOperation{
				{Name: pipeName, Args: []string{"one", "two"}},
			},
		},
	}
	for _, tc := range cases {
		t.Run("refuse-"+pipeName+"-"+tc.name, func(t *testing.T) {
			resp, err := invoke(tc.ops)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, "| "+pipeName)
		})
	}
}
