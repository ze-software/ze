// VALIDATES: spec-fleet-10 AC-1 and AC-2 -- registration, evaluation, opening,
// separation from config-rejected and config-diverged, convergence clearing,
// and hiding behind device-offline for the config-resync-failed hub alarm kind.
// PREVENTS: missing alarm kind registration, duplicate alarms with
// config-rejected or config-diverged, truncated doctor codes or fix commands,
// unhidden alarms while offline, or stuck resync alarms after convergence.

package managed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	resyncTestDeviceEdge1 = "edge-01"
	resyncTestDeviceEdge2 = "edge-02"
	resyncTestDiffStepMsg = "diff verification or apply failed on client"
)

func makeResyncTestConfig(host string) []byte {
	var tb textbuf.Buffer
	for i := range 12 {
		tb.Str("# managed resync test padding line ").Int(int64(i)).Str(" for compact diff delivery\n")
	}
	tb.Str("bus {\n    hub {\n        client ").Str(resyncTestDeviceEdge1).Str(" {\n            host ").Str(host).
		Str(";\n            port 1791;\n            secret \"").Str(testHubToken).Str("\";\n        }\n    }\n}\n")
	return []byte(tb.String())
}

func writeResyncClientConfig(t *testing.T, store storage.Storage, data []byte, stamp time.Time) {
	t.Helper()
	key := pluginserver.ClientConfigKey(resyncTestDeviceEdge1)
	guard, err := store.AcquireLock(key)
	require.NoError(t, err)
	require.NoError(t, guard.WriteVersion(key, data, stamp))
	require.NoError(t, guard.WriteFile(key, data, 0o600))
	require.NoError(t, guard.Release())
}

func startResyncAlarmServer(
	t *testing.T,
	store storage.Storage,
	clk clock.Clock,
	clients ...string,
) (*pluginserver.ManagedServer, string) {
	t.Helper()
	root := testRoot(t)
	installCA(t, root)
	secrets := make(map[string]string, len(clients))
	for _, name := range clients {
		secrets[name] = testHubToken
	}
	srv, err := pluginserver.NewManagedServer(pluginserver.ManagedServerConfig{
		Authority: root,
		Clock:     clk,
		Store:     store,
		Listeners: []pluginserver.ManagedListener{{
			Addr:    "127.0.0.1:0",
			Clients: secrets,
		}},
		ReadConfig: func(name string) ([]byte, error) {
			return store.ReadFile(pluginserver.ClientConfigKey(name))
		},
	})
	require.NoError(t, err)
	require.NoError(t, srv.Start(t.Context()))
	t.Cleanup(srv.Stop)
	return srv, srv.Addrs()[0].String()
}

func connectResyncAlarmClient(t *testing.T, addr, name string) *rpc.MuxConn {
	t.Helper()
	mc := connectReportCommandClient(t, addr, name)
	reqCtx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case req, ok := <-mc.Requests():
				if !ok {
					return
				}
				_ = mc.SendOK(reqCtx, req.ID) //nolint:errcheck // best-effort ack of hub config-changed push in test
			case <-mc.Done():
				return
			case <-reqCtx.Done():
				return
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		_ = mc.Close() //nolint:errcheck // test teardown
		<-done
	})
	return mc
}

func callResyncFetch(t *testing.T, mc *rpc.MuxConn, req fleet.ConfigFetchRequest) fleet.ConfigFetchResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	raw, err := mc.CallRPC(ctx, fleet.VerbConfigFetch, req)
	require.NoError(t, err)
	var resp fleet.ConfigFetchResponse
	require.NoError(t, json.Unmarshal(raw, &resp))
	return resp
}

func callResyncAck(t *testing.T, mc *rpc.MuxConn, ack fleet.ConfigAck) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	raw, err := mc.CallRPC(ctx, fleet.VerbConfigAck, ack)
	require.NoError(t, err)
	_, err = fleet.DecodeManagedReply(raw)
	require.NoError(t, err)
}

func callResyncPing(t *testing.T, mc *rpc.MuxConn, chainHash string) fleet.ManagedReply {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	raw, err := mc.CallRPC(ctx, fleet.VerbPing, fleet.PingRequest{ChainHash: chainHash})
	require.NoError(t, err)
	reply, err := fleet.DecodeManagedReply(raw)
	require.NoError(t, err)
	return reply
}

func callResyncHeld(t *testing.T, mc *rpc.MuxConn, cfg []byte, editedAt time.Time, chainHash string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err := mc.CallRPC(ctx, fleet.VerbConfigHeld, fleet.ConfigHeld{
		Version:   fleet.VersionHash(cfg),
		Config:    base64.StdEncoding.EncodeToString(cfg),
		EditedAt:  editedAt.UTC(),
		ChainHash: chainHash,
	})
	require.NoError(t, err)
}

func findAlarmInPage(page *pluginserver.AlarmListPage, kind string) (pluginserver.AlarmOccurrence, bool) {
	for i := range page.Alarms {
		occ := page.Alarms[i]
		if occ.Kind == kind && occ.Device == resyncTestDeviceEdge1 {
			return occ, true
		}
	}
	return pluginserver.AlarmOccurrence{}, false
}

// TestResyncFailedAlarmOpensAndClears proves spec-fleet-10 AC-1: a non-verify
// resync failure (RejectKind commit or resync) on a non-holding declared device
// opens one Warning config-resync-failed alarm in show fleet alarm whose
// sentence names the device, UTC time, both step causes, doctor code
// doctor-managed-chain-drift, and ends on ze show doctor; verify refusals open
// config-rejected only; holding devices clear config-resync-failed and open
// config-diverged only; and chain convergence via ack, ping, or current fetch
// clears the alarm automatically.
//
// VALIDATES: spec-fleet-10 AC-1, R-11.
// PREVENTS: duplicate config-rejected/config-diverged/config-resync-failed
// alarms, missing fix or code in alarm text, or non-clearing resync alarms.
func TestResyncFailedAlarmOpensAndClears(t *testing.T) {
	base := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	clk := newTestStepClock(base)
	store := storage.CreateMemory()

	cfgV1 := makeResyncTestConfig("10.0.1.1")
	cfgV2 := makeResyncTestConfig("10.0.1.2")
	v1 := fleet.VersionHash(cfgV1)
	v2 := fleet.VersionHash(cfgV2)
	c1 := fleet.FoldFull(cfgV1)
	c2Full := fleet.FoldFull(cfgV2)

	writeResyncClientConfig(t, store, cfgV1, base)
	_, addr := startResyncAlarmServer(t, store, clk, resyncTestDeviceEdge1)
	server := registryDispatcher(t)
	ctx := &pluginserver.CommandContext{Server: server}

	mc := connectResyncAlarmClient(t, addr, resyncTestDeviceEdge1)

	// Initial convergence on V1.
	initResp := callResyncFetch(t, mc, fleet.ConfigFetchRequest{})
	require.Equal(t, v1, initResp.Version)
	require.Equal(t, c1, initResp.ChainHash)
	callResyncAck(t, mc, fleet.ConfigAck{Version: v1, OK: true, ChainHash: c1})

	emptyPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath)
	assert.Zero(t, emptyPage.Total)

	// Advance hub config to V2 so diff is served first, then full replay fails at commit.
	clk.Advance(10 * time.Second)
	writeResyncClientConfig(t, store, cfgV2, clk.Now())

	diffResp := callResyncFetch(t, mc, fleet.ConfigFetchRequest{
		Version:   v1,
		ChainHash: c1,
		Resync:    fleet.ResyncModeDiff,
	})
	require.Equal(t, fleet.FetchModeDiff, diffResp.Mode)
	require.Equal(t, v2, diffResp.Version)

	fullResp := callResyncFetch(t, mc, fleet.ConfigFetchRequest{
		Version:   v1,
		ChainHash: c1,
		Resync:    fleet.ResyncModeFull,
	})
	require.Equal(t, fleet.FetchModeFull, fullResp.Mode)
	require.Equal(t, c2Full, fullResp.ChainHash)

	clk.Advance(5 * time.Second)
	failedAt := clk.Now().UTC()
	const commitCause = "confirmed commit window is open"
	callResyncAck(t, mc, fleet.ConfigAck{
		Version:    v2,
		OK:         false,
		Error:      commitCause,
		RejectKind: fleet.RejectKindCommit,
		ChainHash:  c1,
	})

	// 1. config-resync-failed is OPEN (Warning) in show fleet alarm; config-rejected is NOT open.
	openPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath)
	require.Equal(t, 1, openPage.Total)
	assert.Equal(t, 1, openPage.Counts.Warning)
	assert.Equal(t, 1, openPage.Counts.Visible)
	assert.Equal(t, 0, openPage.Counts.Hidden)

	_, hasRejected := findAlarmInPage(&openPage, pluginserver.AlarmKindConfigRejected)
	assert.False(t, hasRejected, "non-verify commit failure must not open config-rejected")

	occ, hasResync := findAlarmInPage(&openPage, AlarmKindConfigResyncFailed)
	require.True(t, hasResync, "config-resync-failed must open after diff+full non-verify failure")
	assert.Equal(t, pluginserver.AlarmSeverityWarning, occ.Severity)
	assert.Equal(t, pluginserver.AlarmScopeDevice, occ.Scope)
	assert.Equal(t, resyncTestDeviceEdge1, occ.Device)
	assert.Empty(t, occ.Subject)
	assert.Equal(t, failedAt, occ.Since)
	assert.Contains(t, occ.Summary, resyncTestDeviceEdge1)
	assert.Contains(t, occ.Summary, failedAt.Format(time.RFC3339))
	assert.Contains(t, occ.Summary, resyncTestDiffStepMsg)
	assert.Contains(t, occ.Summary, commitCause)
	assert.Contains(t, occ.Summary, pluginserver.CodeDoctorManagedChainDrift)
	assert.True(t, strings.HasSuffix(occ.Summary, resyncAlarmFixCommand), "summary must end with ze show doctor: %q", occ.Summary)
	assert.Contains(t, occ.Detail, "diff="+resyncTestDiffStepMsg)
	assert.Contains(t, occ.Detail, "full="+commitCause)
	assert.Contains(t, occ.Detail, "client-chain="+c1)
	assert.Contains(t, occ.Detail, "expected-chain="+c2Full)
	assert.Contains(t, occ.Detail, "code="+pluginserver.CodeDoctorManagedChainDrift)
	assert.Contains(t, occ.Detail, "fix="+resyncAlarmFixCommand)

	// Pipe filters | kind config-resync-failed and | severity warning both select it.
	kindPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath+" | kind "+AlarmKindConfigResyncFailed)
	require.Equal(t, 1, kindPage.Total)
	assert.Equal(t, occ.ID, kindPage.Alarms[0].ID)

	warnPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath+" | severity warning")
	require.Equal(t, 1, warnPage.Total)
	assert.Equal(t, occ.ID, warnPage.Alarms[0].ID)

	// 2. Verify refusal opens config-rejected ONLY and clears config-resync-failed.
	clk.Advance(5 * time.Second)
	callResyncAck(t, mc, fleet.ConfigAck{
		Version:    v2,
		OK:         false,
		Error:      "yang verify failed: invalid asn",
		RejectKind: fleet.RejectKindVerify,
		ChainHash:  c1,
	})
	verifyPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath)
	require.Equal(t, 1, verifyPage.Total)
	_, hasResync = findAlarmInPage(&verifyPage, AlarmKindConfigResyncFailed)
	assert.False(t, hasResync, "verify refusal must clear config-resync-failed")
	_, hasRejected = findAlarmInPage(&verifyPage, pluginserver.AlarmKindConfigRejected)
	assert.True(t, hasRejected, "verify refusal must open config-rejected")

	// 3. RejectKindResync re-opens config-resync-failed and clears config-rejected.
	clk.Advance(5 * time.Second)
	_ = callResyncFetch(t, mc, fleet.ConfigFetchRequest{Version: v1, ChainHash: c1, Resync: fleet.ResyncModeFull})
	callResyncAck(t, mc, fleet.ConfigAck{
		Version:    v2,
		OK:         false,
		Error:      "reanchor chain-hash mismatch",
		RejectKind: fleet.RejectKindResync,
		ChainHash:  c1,
	})
	resyncAgainPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath)
	require.Equal(t, 1, resyncAgainPage.Total)
	_, hasResync = findAlarmInPage(&resyncAgainPage, AlarmKindConfigResyncFailed)
	assert.True(t, hasResync)
	_, hasRejected = findAlarmInPage(&resyncAgainPage, pluginserver.AlarmKindConfigRejected)
	assert.False(t, hasRejected)

	// 4. Holding client opens config-diverged and clears config-resync-failed.
	clk.Advance(5 * time.Second)
	localEdit := makeResyncTestConfig("10.0.9.9")
	_ = callResyncFetch(t, mc, fleet.ConfigFetchRequest{Version: fleet.VersionHash(localEdit), ChainHash: c1})
	callResyncHeld(t, mc, localEdit, clk.Now(), c1)
	callResyncAck(t, mc, fleet.ConfigAck{
		Version:   v2,
		OK:        false,
		Holding:   true,
		Error:     "holding local edit",
		ChainHash: c1,
	})
	heldPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath)
	require.Equal(t, 1, heldPage.Total)
	_, hasDiverged := findAlarmInPage(&heldPage, pluginserver.AlarmKindConfigDiverged)
	assert.True(t, hasDiverged, "holding client must open config-diverged")
	_, hasResync = findAlarmInPage(&heldPage, AlarmKindConfigResyncFailed)
	assert.False(t, hasResync, "holding client must not hold config-resync-failed open")

	// Clear hold by applying V2 via full replay ack, then test all three convergence clear paths.
	clk.Advance(5 * time.Second)
	_ = callResyncFetch(t, mc, fleet.ConfigFetchRequest{Version: v1, ChainHash: c1, Resync: fleet.ResyncModeFull})
	callResyncAck(t, mc, fleet.ConfigAck{Version: v2, OK: true, ChainHash: c2Full})
	require.Zero(t, decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath).Total)

	// Clear path A: config-ack {ok: true, chain-hash: c2Full}.
	clk.Advance(5 * time.Second)
	_ = callResyncFetch(t, mc, fleet.ConfigFetchRequest{Version: v1, ChainHash: c1, Resync: fleet.ResyncModeFull})
	callResyncAck(t, mc, fleet.ConfigAck{Version: v2, OK: false, Error: "commit failed A", RejectKind: fleet.RejectKindCommit, ChainHash: c1})
	require.Equal(t, 1, decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath+" | kind "+AlarmKindConfigResyncFailed).Total)
	callResyncAck(t, mc, fleet.ConfigAck{Version: v2, OK: true, ChainHash: c2Full})
	assert.Zero(t, decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath).Total, "config-ack ok:true must clear config-resync-failed")

	// Clear path B: matching ping reply/request chain-hash.
	clk.Advance(5 * time.Second)
	_ = callResyncFetch(t, mc, fleet.ConfigFetchRequest{Version: v1, ChainHash: c1, Resync: fleet.ResyncModeFull})
	callResyncAck(t, mc, fleet.ConfigAck{Version: v2, OK: false, Error: "resync failed B", RejectKind: fleet.RejectKindResync, ChainHash: c1})
	require.Equal(t, 1, decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath+" | kind "+AlarmKindConfigResyncFailed).Total)
	pingReply := callResyncPing(t, mc, c2Full)
	assert.Equal(t, c2Full, pingReply.ChainHash)
	assert.Zero(t, decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath).Total, "matching ping must clear config-resync-failed")

	// Clear path C: converged config-fetch returning status: current (Reanchor: false).
	clk.Advance(5 * time.Second)
	_ = callResyncFetch(t, mc, fleet.ConfigFetchRequest{Version: v1, ChainHash: c1, Resync: fleet.ResyncModeFull})
	callResyncAck(t, mc, fleet.ConfigAck{Version: v2, OK: false, Error: "commit failed C", RejectKind: fleet.RejectKindCommit, ChainHash: c1})
	require.Equal(t, 1, decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath+" | kind "+AlarmKindConfigResyncFailed).Total)
	curResp := callResyncFetch(t, mc, fleet.ConfigFetchRequest{Version: v2, ChainHash: c2Full})
	assert.Equal(t, fetchStatusCurrent, curResp.Status)
	assert.False(t, curResp.Reanchor)
	assert.Zero(t, decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath).Total, "converged current fetch must clear config-resync-failed")

	// Direct evaluator guarantees: deterministic sort without mutating input,
	// skips undeclared/diverged devices, and bounds oversized multiline reasons
	// while keeping doctor code and trailing fix intact.
	longReason := strings.Repeat("x", resyncAlarmReasonMax+64) + "\nsecret-line"
	evalDevices := []pluginserver.AlarmEvalDevice{
		{
			Name:     resyncTestDeviceEdge2,
			Declared: true,
			ResyncFailure: &pluginserver.ManagedResyncFailure{
				FailedAt:      failedAt,
				DiffReason:    longReason,
				FullReason:    "commit\nrefused",
				ClientChain:   c1,
				ExpectedChain: c2Full,
			},
		},
		{
			Name:     "undeclared-01",
			Declared: false,
			ResyncFailure: &pluginserver.ManagedResyncFailure{
				FailedAt:   failedAt,
				FullReason: "ignored",
			},
		},
		{
			Name:           "holding-01",
			Declared:       true,
			ConfigDiverged: true,
			ResyncFailure: &pluginserver.ManagedResyncFailure{
				FailedAt:   failedAt,
				FullReason: "ignored",
			},
		},
		{
			Name:     resyncTestDeviceEdge1,
			Declared: true,
			ResyncFailure: &pluginserver.ManagedResyncFailure{
				FailedAt:      time.Time{},
				ClientChain:   "",
				ExpectedChain: "",
			},
		},
	}
	obs := evaluateResyncFailedAlarm(pluginserver.AlarmEvalContext{
		Now:     failedAt,
		Devices: evalDevices,
	})
	require.Len(t, obs, 2)
	assert.Equal(t, resyncTestDeviceEdge2, evalDevices[0].Name, "evaluator must not mutate caller Devices slice")
	assert.Equal(t, resyncTestDeviceEdge1, obs[0].Device, "observations must be sorted deterministically by device name")
	assert.Equal(t, resyncTestDeviceEdge2, obs[1].Device)
	assert.Contains(t, obs[0].Summary, resyncAlarmDefaultDiff)
	assert.Contains(t, obs[0].Summary, resyncAlarmDefaultFull)
	assert.Contains(t, obs[0].Detail, "client-chain="+resyncAlarmHashNone)
	assert.NotContains(t, obs[1].Summary, "\n")
	assert.True(t, strings.HasSuffix(obs[1].Summary, resyncAlarmFixCommand))
	assert.Contains(t, obs[1].Detail, "code="+pluginserver.CodeDoctorManagedChainDrift+"; fix="+resyncAlarmFixCommand)
}

// TestResyncFailedAlarmHiddenBehindOffline proves spec-fleet-10 AC-2 (choice
// C-1): config-resync-failed registers Device offline as its cause, passes
// fleet-ui-8 cause validation, is hidden behind device-offline while the device
// is disconnected (selectable via | hidden true and excluded by | hidden false),
// retains its occurrence ID and Since across offline/reconnect with history
// overlap marked, and clears once the reconnect resync converges.
//
// VALIDATES: spec-fleet-10 AC-2, choice C-1, fleet-ui-8 cause hiding.
// PREVENTS: missing Causes declaration, cause graph validation failure, or
// config-resync-failed remaining visible beside device-offline.
func TestResyncFailedAlarmHiddenBehindOffline(t *testing.T) {
	spec, registered := pluginserver.LookupAlarmKind(AlarmKindConfigResyncFailed)
	require.True(t, registered, "config-resync-failed must be registered by managed init()")
	assert.Equal(t, pluginserver.AlarmSeverityWarning, spec.Severity)
	assert.Equal(t, pluginserver.AlarmScopeDevice, spec.Scope)
	assert.Equal(t, []string{pluginserver.AlarmKindDeviceOffline}, spec.Causes)
	assert.False(t, spec.Settles)
	assert.NotNil(t, spec.Evaluate)

	base := time.Date(2026, 11, 27, 11, 0, 0, 0, time.UTC)
	clk := newTestStepClock(base)
	store := storage.CreateMemory()

	cfgV1 := makeResyncTestConfig("10.0.2.1")
	cfgV2 := makeResyncTestConfig("10.0.2.2")
	v1 := fleet.VersionHash(cfgV1)
	v2 := fleet.VersionHash(cfgV2)
	c1 := fleet.FoldFull(cfgV1)
	c2Full := fleet.FoldFull(cfgV2)

	writeResyncClientConfig(t, store, cfgV1, base)
	srv, addr := startResyncAlarmServer(t, store, clk, resyncTestDeviceEdge1)
	require.NoError(t, srv.Alarms().ValidateRegistry(), "registered cause graph must pass fleet-ui-8 ValidateCauses")

	server := registryDispatcher(t)
	ctx := &pluginserver.CommandContext{Server: server}

	mc := connectResyncAlarmClient(t, addr, resyncTestDeviceEdge1)
	_ = callResyncFetch(t, mc, fleet.ConfigFetchRequest{})
	callResyncAck(t, mc, fleet.ConfigAck{Version: v1, OK: true, ChainHash: c1})

	// Trigger non-verify resync failure while connected.
	clk.Advance(10 * time.Second)
	writeResyncClientConfig(t, store, cfgV2, clk.Now())
	_ = callResyncFetch(t, mc, fleet.ConfigFetchRequest{Version: v1, ChainHash: c1, Resync: fleet.ResyncModeDiff})
	_ = callResyncFetch(t, mc, fleet.ConfigFetchRequest{Version: v1, ChainHash: c1, Resync: fleet.ResyncModeFull})
	callResyncAck(t, mc, fleet.ConfigAck{
		Version:    v2,
		OK:         false,
		Error:      "commit locked during window",
		RejectKind: fleet.RejectKindCommit,
		ChainHash:  c1,
	})

	onlinePage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath)
	require.Equal(t, 1, onlinePage.Total)
	assert.Equal(t, 1, onlinePage.Counts.Visible)
	assert.Equal(t, 0, onlinePage.Counts.Hidden)
	origResync, ok := findAlarmInPage(&onlinePage, AlarmKindConfigResyncFailed)
	require.True(t, ok)
	assert.Empty(t, origResync.HiddenBehind)

	// Disconnect edge-01 and advance past offline-delay so device-offline opens.
	clk.Advance(5 * time.Second)
	require.NoError(t, mc.Close())
	waitDeviceDisconnected(t, resyncTestDeviceEdge1)

	offlineDelay := time.Duration(plugin.DefaultHubAlarmsConfig().OfflineDelaySeconds+1) * time.Second
	clk.Advance(offlineDelay)
	srv.Alarms().Tick(clk.Now())

	offlinePage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath)
	require.Equal(t, 2, offlinePage.Total)
	assert.Equal(t, 1, offlinePage.Counts.Visible)
	assert.Equal(t, 1, offlinePage.Counts.Hidden)

	offlineOcc, ok := findAlarmInPage(&offlinePage, pluginserver.AlarmKindDeviceOffline)
	require.True(t, ok)
	assert.Empty(t, offlineOcc.HiddenBehind)

	hiddenResync, ok := findAlarmInPage(&offlinePage, AlarmKindConfigResyncFailed)
	require.True(t, ok)
	assert.Equal(t, origResync.ID, hiddenResync.ID)
	assert.Equal(t, origResync.Since, hiddenResync.Since)
	assert.Equal(t, offlineOcc.ID, hiddenResync.HiddenBehind)

	// | hidden true selects ONLY config-resync-failed; | hidden false selects ONLY device-offline.
	hiddenOnly := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath+" | hidden true")
	require.Equal(t, 1, hiddenOnly.Total)
	require.Len(t, hiddenOnly.Alarms, 1)
	assert.Equal(t, AlarmKindConfigResyncFailed, hiddenOnly.Alarms[0].Kind)
	assert.Equal(t, origResync.ID, hiddenOnly.Alarms[0].ID)
	assert.Equal(t, offlineOcc.ID, hiddenOnly.Alarms[0].HiddenBehind)

	visibleOnly := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath+" | hidden false")
	require.Equal(t, 1, visibleOnly.Total)
	require.Len(t, visibleOnly.Alarms, 1)
	assert.Equal(t, pluginserver.AlarmKindDeviceOffline, visibleOnly.Alarms[0].Kind)
	assert.Equal(t, offlineOcc.ID, visibleOnly.Alarms[0].ID)
	assert.Empty(t, visibleOnly.Alarms[0].HiddenBehind)

	// Reconnect edge-01: device-offline clears; config-resync-failed becomes
	// visible again with its original ID and Since until reconnect resync succeeds.
	clk.Advance(10 * time.Second)
	mcNext := connectResyncAlarmClient(t, addr, resyncTestDeviceEdge1)

	reconnPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath)
	require.Equal(t, 1, reconnPage.Total)
	assert.Equal(t, 1, reconnPage.Counts.Visible)
	assert.Equal(t, 0, reconnPage.Counts.Hidden)
	shownAgain, ok := findAlarmInPage(&reconnPage, AlarmKindConfigResyncFailed)
	require.True(t, ok)
	assert.Equal(t, origResync.ID, shownAgain.ID)
	assert.Equal(t, origResync.Since, shownAgain.Since)
	assert.Empty(t, shownAgain.HiddenBehind)

	// Reconnect resync succeeds -> config-resync-failed clears and history
	// preserves both rows with overlap marked.
	clk.Advance(5 * time.Second)
	_ = callResyncFetch(t, mcNext, fleet.ConfigFetchRequest{Version: v1, ChainHash: c1, Resync: fleet.ResyncModeFull})
	callResyncAck(t, mcNext, fleet.ConfigAck{Version: v2, OK: true, ChainHash: c2Full})

	clearedPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath)
	assert.Zero(t, clearedPage.Total)

	histPage := decodePipeData[pluginserver.AlarmHistoryPage](t, server, ctx, fleetAlarmHistoryPath+" "+resyncTestDeviceEdge1)
	require.Equal(t, 2, histPage.Total)
	var sawOffline, sawResync bool
	for _, ev := range histPage.Events {
		switch ev.ID {
		case offlineOcc.ID:
			sawOffline = true
			assert.Equal(t, pluginserver.AlarmKindDeviceOffline, ev.Kind)
			assert.True(t, ev.Cleared)
		case origResync.ID:
			sawResync = true
			assert.Equal(t, AlarmKindConfigResyncFailed, ev.Kind)
			assert.True(t, ev.Cleared)
			assert.True(t, ev.Hidden, "history row must record overlap behind device-offline")
			assert.Equal(t, offlineOcc.ID, ev.HiddenBehind)
		}
	}
	assert.True(t, sawOffline, "device-offline history row missing")
	assert.True(t, sawResync, "config-resync-failed history row missing")
}
